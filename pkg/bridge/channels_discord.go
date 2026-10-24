package bridge

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// discordGatewayPayload represents an incoming or outgoing Discord Gateway frame.
type discordGatewayPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  *int64          `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

type discordHelloData struct {
	HeartbeatInterval int64 `json:"heartbeat_interval"`
}

type discordIdentifyData struct {
	Token      string                    `json:"token"`
	Intents    int64                     `json:"intents"`
	Properties discordIdentifyProperties `json:"properties"`
}

type discordIdentifyProperties struct {
	OS      string `json:"os"`
	Browser string `json:"browser"`
	Device  string `json:"device"`
}

type DiscordMessageAuthor struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Bot           bool   `json:"bot"`
}

// DiscordMessage captures incoming Discord chat message event fields.
type DiscordMessage struct {
	ID        string               `json:"id"`
	ChannelID string               `json:"channel_id"`
	GuildID   string               `json:"guild_id,omitempty"`
	Author    DiscordMessageAuthor `json:"author"`
	Content   string               `json:"content"`
}

// ParseDiscordInbound parses a Discord gateway MESSAGE_CREATE event payload into InboundMessage.
// Non-MESSAGE_CREATE events, bot messages, or empty text messages yield nothing (nil, nil).
func ParseDiscordInbound(payload []byte) ([]InboundMessage, error) {
	var gp discordGatewayPayload
	if err := json.Unmarshal(payload, &gp); err != nil {
		return nil, fmt.Errorf("invalid Discord gateway payload: %w", err)
	}
	if gp.Op != 0 || gp.T != "MESSAGE_CREATE" {
		return nil, nil
	}
	var msg DiscordMessage
	if err := json.Unmarshal(gp.D, &msg); err != nil {
		return nil, fmt.Errorf("invalid Discord message payload: %w", err)
	}
	if msg.Author.Bot {
		return nil, nil
	}
	text := strings.TrimSpace(msg.Content)
	if text == "" {
		return nil, nil
	}
	return []InboundMessage{{
		ID:        fmt.Sprintf("dc-%s", msg.ID),
		From:      msg.Author.ID,
		ChannelID: msg.ChannelID,
		Text:      text,
	}}, nil
}

// discordChannelAllowed checks whether channelID matches configured allowed channels (empty = allow all).
func discordChannelAllowed(allowed []string, channelID string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, id := range allowed {
		if strings.TrimSpace(id) == strings.TrimSpace(channelID) {
			return true
		}
	}
	return false
}

// discordUserAllowed checks whether userID matches configured allowed users (empty = allow all).
func discordUserAllowed(allowed []string, userID string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, id := range allowed {
		if strings.TrimSpace(id) == strings.TrimSpace(userID) {
			return true
		}
	}
	return false
}

var discordHTTPClient = &http.Client{Timeout: 15 * time.Second}

// SendDiscordReply posts a text message back to the specified Discord channel.
func SendDiscordReply(token, channelID, text string) error {
	return SendDiscordReplyWithClient(discordHTTPClient, token, channelID, text)
}

// SendDiscordReplyWithClient posts a text message using the specified HTTP client.
func SendDiscordReplyWithClient(client *http.Client, token, channelID, text string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("discord bot token is not configured")
	}
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return fmt.Errorf("discord channel id is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	baseURL := os.Getenv("DISCORD_API_BASE_URL")
	if baseURL == "" {
		baseURL = "https://discord.com/api/v10"
	}
	endpoint := fmt.Sprintf("%s/channels/%s/messages", strings.TrimRight(baseURL, "/"), channelID)

	authHeader := strings.TrimSpace(token)
	if !strings.HasPrefix(authHeader, "Bot ") {
		authHeader = "Bot " + authHeader
	}

	// Discord enforces a maximum limit of 2000 runes per message content.
	runes := []rune(text)
	if len(runes) == 0 {
		runes = []rune("(empty response)")
	}

	for len(runes) > 0 {
		chunkSize := 2000
		if len(runes) < chunkSize {
			chunkSize = len(runes)
		}
		chunk := string(runes[:chunkSize])
		runes = runes[chunkSize:]

		body, err := json.Marshal(map[string]interface{}{
			"content": chunk,
		})
		if err != nil {
			return fmt.Errorf("failed to marshal Discord message: %w", err)
		}

		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create Discord request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", authHeader)

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("discord send request failed: %w", err)
		}
		if resp.StatusCode >= 300 {
			snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		}
		resp.Body.Close()
	}
	return nil
}

// --------------------------------------------------------------------------
// Pure Go Lightweight RFC 6455 WebSocket Client (Zero External Dependencies)
// --------------------------------------------------------------------------

type wsConn struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
}

func dialWS(targetURL string, timeout time.Duration) (*wsConn, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid websocket URL %q: %w", targetURL, err)
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" || u.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	var rawConn net.Conn
	if u.Scheme == "wss" || u.Scheme == "https" {
		hostOnly := u.Hostname()
		tlsConfig := &tls.Config{
			ServerName: hostOnly,
		}
		if hostOnly == "127.0.0.1" || hostOnly == "localhost" {
			tlsConfig.InsecureSkipVerify = true
		}
		rawConn, err = tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", host, tlsConfig)
	} else {
		rawConn, err = net.DialTimeout("tcp", host, timeout)
	}
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}

	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	secKey := base64.StdEncoding.EncodeToString(nonce)

	reqPath := u.Path
	if reqPath == "" {
		reqPath = "/"
	}
	if u.RawQuery != "" {
		reqPath += "?" + u.RawQuery
	}

	upgradeReq := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", reqPath, u.Host, secKey)
	if _, err := rawConn.Write([]byte(upgradeReq)); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("failed to send websocket upgrade: %w", err)
	}

	reader := bufio.NewReader(rawConn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: "GET"})
	if err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("failed to read websocket upgrade response: %w", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = rawConn.Close()
		return nil, fmt.Errorf("websocket handshake rejected with status: %d", resp.StatusCode)
	}

	return &wsConn{
		conn:   rawConn,
		reader: reader,
	}, nil
}

func (ws *wsConn) writeFrame(opcode byte, payload []byte) error {
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}

	var header []byte
	header = append(header, 0x80|opcode)
	length := len(payload)
	if length < 126 {
		header = append(header, 0x80|byte(length))
	} else if length <= 65535 {
		header = append(header, 0x80|126, byte(length>>8), byte(length))
	} else {
		header = append(header, 0x80|127)
		lenBuf := make([]byte, 8)
		binary.BigEndian.PutUint64(lenBuf, uint64(length))
		header = append(header, lenBuf...)
	}
	header = append(header, mask...)

	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = payload[i] ^ mask[i%4]
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()
	_, err := ws.conn.Write(append(header, masked...))
	return err
}

func (ws *wsConn) readFrame() (byte, []byte, error) {
	for {
		b0, err := ws.reader.ReadByte()
		if err != nil {
			return 0, nil, err
		}
		opcode := b0 & 0x0F

		b1, err := ws.reader.ReadByte()
		if err != nil {
			return 0, nil, err
		}
		isMasked := (b1 & 0x80) != 0
		payloadLen := int(b1 & 0x7F)

		if payloadLen == 126 {
			var extLen uint16
			if err := binary.Read(ws.reader, binary.BigEndian, &extLen); err != nil {
				return 0, nil, err
			}
			payloadLen = int(extLen)
		} else if payloadLen == 127 {
			var extLen uint64
			if err := binary.Read(ws.reader, binary.BigEndian, &extLen); err != nil {
				return 0, nil, err
			}
			if extLen > 20*1024*1024 {
				return 0, nil, fmt.Errorf("websocket frame length %d exceeds 20MB limit", extLen)
			}
			payloadLen = int(extLen)
		}

		var mask [4]byte
		if isMasked {
			if _, err := io.ReadFull(ws.reader, mask[:]); err != nil {
				return 0, nil, err
			}
		}

		payload := make([]byte, payloadLen)
		if _, err := io.ReadFull(ws.reader, payload); err != nil {
			return 0, nil, err
		}
		if isMasked {
			for i := 0; i < payloadLen; i++ {
				payload[i] ^= mask[i%4]
			}
		}

		if opcode == 0x09 { // Ping -> reply with Pong
			_ = ws.writeFrame(0x0A, payload)
			continue
		}
		if opcode == 0x08 { // Close
			return opcode, nil, io.EOF
		}

		return opcode, payload, nil
	}
}

func (ws *wsConn) Close() error {
	return ws.conn.Close()
}

// --------------------------------------------------------------------------
// Discord Gateway Client Listener
// --------------------------------------------------------------------------

// DiscordGatewayClient maintains a resilient WebSocket connection to Discord's Gateway.
type DiscordGatewayClient struct {
	gatewayURL string
	token      string
	intents    int64
	onMessage  func(msg DiscordMessage)
	onReady    func(username, id string)

	lastSequence atomic.Int64
	activeConn   *wsConn
	connMu       sync.Mutex
	stopCh       chan struct{}
	stopped      atomic.Bool
}

// NewDiscordGatewayClient creates a new Discord Gateway WebSocket listener.
func NewDiscordGatewayClient(gatewayURL, token string, onMessage func(msg DiscordMessage), onReady func(username, id string)) *DiscordGatewayClient {
	cleanToken := strings.TrimSpace(token)
	cleanToken = strings.TrimPrefix(cleanToken, "Bot ")

	if gatewayURL == "" {
		gatewayURL = "wss://gateway.discord.gg/?v=10&encoding=json"
	}

	return &DiscordGatewayClient{
		gatewayURL: gatewayURL,
		token:      cleanToken,
		// 37376 = (1<<9 GUILD_MESSAGES) | (1<<12 DIRECT_MESSAGES) | (1<<15 MESSAGE_CONTENT)
		intents:   37376,
		onMessage: onMessage,
		onReady:   onReady,
		stopCh:    make(chan struct{}),
	}
}

// Start runs the gateway client connection loop until ctx is cancelled or Stop is called.
func (c *DiscordGatewayClient) Start(ctx context.Context) {
	backoff := 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		err := c.runSession(ctx)
		if c.stopped.Load() {
			return
		}

		if err != nil {
			log.Printf("[discord-gateway] session ended: %v, reconnecting in %v...", err, backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-time.After(backoff):
		}

		if backoff < 10*time.Second {
			backoff *= 2
		}
	}
}

// Stop closes the active gateway connection and terminates the run loop.
func (c *DiscordGatewayClient) Stop() {
	if c.stopped.CompareAndSwap(false, true) {
		close(c.stopCh)
		c.connMu.Lock()
		if c.activeConn != nil {
			_ = c.activeConn.Close()
		}
		c.connMu.Unlock()
	}
}

func (c *DiscordGatewayClient) runSession(ctx context.Context) error {
	ws, err := dialWS(c.gatewayURL, 15*time.Second)
	if err != nil {
		return fmt.Errorf("gateway dial failed: %w", err)
	}
	defer ws.Close()

	c.connMu.Lock()
	c.activeConn = ws
	c.connMu.Unlock()

	defer func() {
		c.connMu.Lock()
		if c.activeConn == ws {
			c.activeConn = nil
		}
		c.connMu.Unlock()
	}()

	// 1. Await Opcode 10 (Hello) to obtain heartbeat interval
	_, payload, err := ws.readFrame()
	if err != nil {
		return fmt.Errorf("failed reading hello frame: %w", err)
	}

	var helloFrame discordGatewayPayload
	if err := json.Unmarshal(payload, &helloFrame); err != nil || helloFrame.Op != 10 {
		return fmt.Errorf("expected Opcode 10 Hello frame, got: %s", string(payload))
	}

	var helloData discordHelloData
	if err := json.Unmarshal(helloFrame.D, &helloData); err != nil || helloData.HeartbeatInterval <= 0 {
		return fmt.Errorf("invalid hello data: %s", string(helloFrame.D))
	}

	// 2. Start heartbeat goroutine
	heartbeatInterval := time.Duration(helloData.HeartbeatInterval) * time.Millisecond
	heartbeatStop := make(chan struct{})
	defer close(heartbeatStop)

	var lastHeartbeatSent atomic.Int64
	var lastHeartbeatAck atomic.Int64

	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatStop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if sent := lastHeartbeatSent.Load(); sent > 0 && sent > lastHeartbeatAck.Load() {
					log.Printf("[discord-gateway] heartbeat ACK timed out, closing connection to trigger reconnect")
					_ = ws.Close()
					return
				}
				var seqVal interface{}
				seq := c.lastSequence.Load()
				if seq > 0 {
					seqVal = seq
				}
				hb, _ := json.Marshal(map[string]interface{}{
					"op": 1,
					"d":  seqVal,
				})
				lastHeartbeatSent.Store(time.Now().UnixNano())
				_ = ws.writeFrame(1, hb)
			}
		}
	}()

	// 3. Send Opcode 2 (Identify)
	identifyPayload, err := json.Marshal(map[string]interface{}{
		"op": 2,
		"d": discordIdentifyData{
			Token:   c.token,
			Intents: c.intents,
			Properties: discordIdentifyProperties{
				OS:      runtime.GOOS,
				Browser: "antigravity-swiss-knife",
				Device:  "antigravity-swiss-knife",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal identify: %w", err)
	}

	if err := ws.writeFrame(1, identifyPayload); err != nil {
		return fmt.Errorf("failed to send identify: %w", err)
	}

	// 4. Inbound Event Dispatch Loop
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.stopCh:
			return nil
		default:
		}

		_, frameData, err := ws.readFrame()
		if err != nil {
			return err
		}

		var event discordGatewayPayload
		if err := json.Unmarshal(frameData, &event); err != nil {
			continue
		}

		if event.S != nil {
			c.lastSequence.Store(*event.S)
		}

		switch event.Op {
		case 0: // Dispatch
			if event.T == "READY" {
				var readyData struct {
					User struct {
						ID       string `json:"id"`
						Username string `json:"username"`
					} `json:"user"`
				}
				if json.Unmarshal(event.D, &readyData) == nil {
					log.Printf("[discord-gateway] connected and authenticated as @%s (%s)", readyData.User.Username, readyData.User.ID)
					if c.onReady != nil {
						c.onReady(readyData.User.Username, readyData.User.ID)
					}
				}
			} else if event.T == "MESSAGE_CREATE" {
				var msg DiscordMessage
				if json.Unmarshal(event.D, &msg) == nil && c.onMessage != nil {
					c.onMessage(msg)
				}
			}
		case 1: // Heartbeat requested immediately by server
			var seqVal interface{}
			seq := c.lastSequence.Load()
			if seq > 0 {
				seqVal = seq
			}
			hb, _ := json.Marshal(map[string]interface{}{"op": 1, "d": seqVal})
			_ = ws.writeFrame(1, hb)
		case 7: // Reconnect requested by Discord
			return fmt.Errorf("discord gateway requested reconnect (Opcode 7)")
		case 9: // Invalid session
			c.lastSequence.Store(0)
			return fmt.Errorf("discord gateway invalid session (Opcode 9)")
		case 11: // Heartbeat ACK
			lastHeartbeatAck.Store(time.Now().UnixNano())
		}
	}
}
