package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// InboundMessage is one chat message from an external channel, reduced to what the
// relay needs: who sent it, what it says, and an id for idempotency.
type InboundMessage struct {
	ID        string
	From      string
	ChannelID string
	Text      string
}

// whatsAppInbound matches the subset of the Cloud API payload we act on.
type whatsAppInbound struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Messages []struct {
					ID   string `json:"id"`
					From string `json:"from"`
					Type string `json:"type"`
					Text struct {
						Body string `json:"body"`
					} `json:"text"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// ParseWhatsAppInbound extracts text messages. Status callbacks and non-text
// messages yield nothing, which is not an error: WhatsApp sends those constantly.
func ParseWhatsAppInbound(body []byte) ([]InboundMessage, error) {
	var payload whatsAppInbound
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid WhatsApp payload: %w", err)
	}
	var out []InboundMessage
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, message := range change.Value.Messages {
				text := strings.TrimSpace(message.Text.Body)
				if message.Type != "text" || text == "" {
					continue
				}
				out = append(out, InboundMessage{ID: message.ID, From: message.From, Text: text})
			}
		}
	}
	return out, nil
}

// seenMessageIDs remembers which messages were already relayed. WhatsApp retries a
// webhook for days, and answering the same question repeatedly is worse than
// dropping it.
var (
	seenMu     sync.Mutex
	seenIDs    = map[string]time.Time{}
	seenTTL    = 48 * time.Hour
	seenLoaded bool
)

// seenStorePath is where the dedupe window survives a restart. A daemon restart must
// not re-answer messages WhatsApp is still retrying.
func seenStorePath() string {
	return filepath.Join(core.GetConfigDir(), "channel_seen.json")
}

func loadSeenStore() {
	data, err := os.ReadFile(seenStorePath())
	if err != nil {
		return
	}
	var stored map[string]time.Time
	if json.Unmarshal(data, &stored) == nil {
		for id, at := range stored {
			seenIDs[id] = at
		}
	}
}

func saveSeenStore() {
	data, err := json.Marshal(seenIDs)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(seenStorePath()), 0700)
	_ = os.WriteFile(seenStorePath(), data, 0600)
}

// MarkMessageSeen reports whether this message is new, recording it when it is.
func MarkMessageSeen(id string) bool {
	if strings.TrimSpace(id) == "" {
		return true // nothing to deduplicate on
	}
	seenMu.Lock()
	defer seenMu.Unlock()
	if !seenLoaded {
		loadSeenStore()
		seenLoaded = true
	}
	now := time.Now()
	for key, at := range seenIDs {
		if now.Sub(at) > seenTTL {
			delete(seenIDs, key)
		}
	}
	if _, exists := seenIDs[id]; exists {
		return false
	}
	seenIDs[id] = now
	saveSeenStore()
	return true
}

// channelTurnSerial keeps inbound turns from colliding on the backend's pending-run
// lock. Chats are low volume; queueing costs nothing and losing a message costs trust.
var channelTurnSerial sync.Mutex

// AnswerChannelMessage runs one turn on the backend chosen for channels and returns
// the reply text. The project becomes the working directory so every channel lands in
// the same workspace.
func (s *Service) AnswerChannelMessage(text, project string) (string, error) {
	channelTurnSerial.Lock()
	defer channelTurnSerial.Unlock()

	backend := s.agentForChannel()
	args := map[string]interface{}{"prompt": text}
	if strings.TrimSpace(project) != "" {
		args["cwd"] = project
	}
	raw, err := backend.CreateConversation(args)
	if err != nil {
		return "", err
	}

	var parsed map[string]interface{}
	jsonOK := json.Unmarshal([]byte(raw), &parsed) == nil

	// The desktop app dispatches and returns instructions, not an answer; the reply
	// only exists after the turn is awaited.
	if jsonOK {
		if status, _ := parsed["status"].(string); status == "dispatched" {
			conversationID, _ := parsed["conversation_id"].(string)
			awaited, awaitErr := backend.AwaitResponse(map[string]interface{}{
				"conversation_id": conversationID,
				"timeout_seconds": 180,
			})
			if awaitErr != nil {
				return "", awaitErr
			}
			raw = awaited
			parsed = nil
			jsonOK = json.Unmarshal([]byte(raw), &parsed) == nil
		}
	}

	if jsonOK {
		for _, key := range []string{"response", "text", "content"} {
			if value, ok := parsed[key].(string); ok && strings.TrimSpace(value) != "" {
				return value, nil
			}
		}
		// Never hand internal metadata to a chat participant.
		return "", fmt.Errorf("the agent finished without a text reply")
	}

	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("the agent returned no reply")
	}
	return strings.TrimSpace(raw), nil
}
