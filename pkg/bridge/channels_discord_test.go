package bridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const discordTextPayload = `{
  "op": 0,
  "t": "MESSAGE_CREATE",
  "s": 10,
  "d": {
    "id": "msg-987654321",
    "channel_id": "chan-123456",
    "guild_id": "guild-555555",
    "author": {
      "id": "usr-8888",
      "username": "david_tester",
      "discriminator": "0",
      "bot": false
    },
    "content": "   hello agent from discord   "
  }
}`

func TestParseDiscordInboundExtractsText(t *testing.T) {
	messages, err := ParseDiscordInbound([]byte(discordTextPayload))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	msg := messages[0]
	if msg.ID != "dc-msg-987654321" {
		t.Errorf("expected ID 'dc-msg-987654321', got %q", msg.ID)
	}
	if msg.From != "usr-8888" {
		t.Errorf("expected From 'usr-8888', got %q", msg.From)
	}
	if msg.ChannelID != "chan-123456" {
		t.Errorf("expected ChannelID 'chan-123456', got %q", msg.ChannelID)
	}
	if msg.Text != "hello agent from discord" {
		t.Errorf("expected trimmed text, got %q", msg.Text)
	}
}

func TestParseDiscordInboundIgnoresBotMessages(t *testing.T) {
	botPayload := `{
  "op": 0,
  "t": "MESSAGE_CREATE",
  "s": 11,
  "d": {
    "id": "msg-bot-1",
    "channel_id": "chan-123456",
    "author": {
      "id": "bot-9999",
      "username": "another_bot",
      "bot": true
    },
    "content": "bot message"
  }
}`
	messages, err := ParseDiscordInbound([]byte(botPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected bot message to be ignored, got %d messages", len(messages))
	}
}

func TestParseDiscordInboundIgnoresNonMessageEvents(t *testing.T) {
	typingPayload := `{
  "op": 0,
  "t": "TYPING_START",
  "s": 12,
  "d": {
    "channel_id": "chan-123456",
    "user_id": "usr-8888"
  }
}`
	messages, err := ParseDiscordInbound([]byte(typingPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected non-MESSAGE_CREATE event to yield 0 messages, got %d", len(messages))
	}
}

func TestParseDiscordInboundRejectsGarbage(t *testing.T) {
	if _, err := ParseDiscordInbound([]byte("invalid json")); err == nil {
		t.Error("expected error for malformed payload")
	}
}

func TestDiscordWhitelisting(t *testing.T) {
	// Empty allowlist allows all
	if !discordChannelAllowed(nil, "chan-1") || !discordChannelAllowed([]string{}, "chan-1") {
		t.Error("empty channel list must allow all channels")
	}
	if !discordUserAllowed(nil, "usr-1") || !discordUserAllowed([]string{}, "usr-1") {
		t.Error("empty user list must allow all users")
	}

	// Populated allowlist enforces exact match
	allowedChans := []string{"chan-100", "chan-200"}
	if !discordChannelAllowed(allowedChans, "chan-100") {
		t.Error("expected chan-100 to be allowed")
	}
	if discordChannelAllowed(allowedChans, "chan-999") {
		t.Error("expected chan-999 to be rejected")
	}

	allowedUsers := []string{"usr-alice", "usr-bob"}
	if !discordUserAllowed(allowedUsers, "usr-alice") {
		t.Error("expected usr-alice to be allowed")
	}
	if discordUserAllowed(allowedUsers, "usr-eve") {
		t.Error("expected usr-eve to be rejected")
	}
}

func TestSendDiscordReply(t *testing.T) {
	var receivedAuth string
	var receivedPath string
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"reply-123"}`))
	}))
	defer server.Close()

	t.Setenv("DISCORD_API_BASE_URL", server.URL)

	err := SendDiscordReply("my_bot_token", "chan-test-99", "hello from swiss")
	if err != nil {
		t.Fatalf("SendDiscordReply failed: %v", err)
	}

	if receivedAuth != "Bot my_bot_token" {
		t.Errorf("expected Authorization 'Bot my_bot_token', got %q", receivedAuth)
	}
	if receivedPath != "/channels/chan-test-99/messages" {
		t.Errorf("expected path '/channels/chan-test-99/messages', got %q", receivedPath)
	}
	if receivedBody["content"] != "hello from swiss" {
		t.Errorf("expected body content 'hello from swiss', got %v", receivedBody["content"])
	}

	// Empty token fails
	if err := SendDiscordReply("", "chan-1", "test"); err == nil {
		t.Error("expected error for empty bot token")
	}

	// Empty channel fails
	if err := SendDiscordReply("token", "", "test"); err == nil {
		t.Error("expected error for empty channel ID")
	}
}

// TestDiscordGateway_MockWebSocket exercises the complete WebSocket handshake,
// Opcode 10 Hello processing, Opcode 2 Identify dispatch, and Opcode 0 MESSAGE_CREATE handling.
func TestDiscordGateway_MockWebSocket(t *testing.T) {
	messageReceived := make(chan DiscordMessage, 1)
	readyReceived := make(chan string, 1)

	// Local mock WebSocket server using http.Hijacker
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
			http.Error(w, "expected websocket upgrade", http.StatusBadRequest)
			return
		}

		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack not supported", http.StatusInternalServerError)
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()

		// Complete RFC 6455 Handshake
		_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=\r\n\r\n")
		_ = buf.Flush()

		// 1. Send Opcode 10 Hello Frame (unmasked server frame)
		helloPayload := []byte(`{"op":10,"d":{"heartbeat_interval":500}}`)
		frame := []byte{0x81, byte(len(helloPayload))}
		frame = append(frame, helloPayload...)
		_, _ = buf.Write(frame)
		_ = buf.Flush()

		// 2. Read Opcode 2 Identify from Client
		b0, err := buf.ReadByte()
		if err != nil {
			return
		}
		if b0&0x0F != 1 { // Text frame
			return
		}
		b1, _ := buf.ReadByte()
		pLen := int(b1 & 0x7F)
		if pLen == 126 {
			bLen := make([]byte, 2)
			_, _ = io.ReadFull(buf, bLen)
			pLen = int(bLen[0])<<8 | int(bLen[1])
		}
		var mask [4]byte
		_, _ = io.ReadFull(buf, mask[:])
		pData := make([]byte, pLen)
		_, _ = io.ReadFull(buf, pData)
		for i := 0; i < pLen; i++ {
			pData[i] ^= mask[i%4]
		}

		var idData discordGatewayPayload
		_ = json.Unmarshal(pData, &idData)
		if idData.Op != 2 {
			return
		}

		// 3. Send Opcode 0 Dispatch (READY)
		readyPayload := []byte(`{"op":0,"t":"READY","d":{"user":{"id":"bot-777","username":"TestBot"}}}`)
		rf := []byte{0x81, byte(len(readyPayload))}
		rf = append(rf, readyPayload...)
		_, _ = buf.Write(rf)
		_ = buf.Flush()

		// 4. Send Opcode 0 Dispatch (MESSAGE_CREATE)
		msgPayload := []byte(`{"op":0,"t":"MESSAGE_CREATE","s":1,"d":{"id":"gw-101","channel_id":"c-99","author":{"id":"u-55","username":"tester","bot":false},"content":"gateway message"}}`)
		var mf []byte
		if len(msgPayload) < 126 {
			mf = []byte{0x81, byte(len(msgPayload))}
		} else {
			mf = []byte{0x81, 126, byte(len(msgPayload) >> 8), byte(len(msgPayload))}
		}
		mf = append(mf, msgPayload...)
		_, _ = buf.Write(mf)
		_ = buf.Flush()

		// Keep connection open for heartbeat
		time.Sleep(300 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws://" + server.Listener.Addr().String() + "/?v=10&encoding=json"

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	gw := NewDiscordGatewayClient(wsURL, "mock-bot-token", func(msg DiscordMessage) {
		messageReceived <- msg
	}, func(username, id string) {
		readyReceived <- username
	})

	go gw.Start(ctx)
	defer gw.Stop()

	select {
	case username := <-readyReceived:
		if username != "TestBot" {
			t.Errorf("expected ready username 'TestBot', got %q", username)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for READY event from gateway")
	}

	select {
	case msg := <-messageReceived:
		if msg.ID != "gw-101" || msg.Content != "gateway message" || msg.ChannelID != "c-99" {
			t.Errorf("unexpected gateway message: %+v", msg)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for MESSAGE_CREATE event from gateway")
	}
}
