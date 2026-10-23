package bridge

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// InboundMessage is one chat message from an external channel, reduced to what the
// relay needs: who sent it, what it says, and an id for idempotency.
type InboundMessage struct {
	ID   string
	From string
	Text string
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
	seenMu  sync.Mutex
	seenIDs = map[string]time.Time{}
	seenTTL = 48 * time.Hour
)

// MarkMessageSeen reports whether this message is new, recording it when it is.
func MarkMessageSeen(id string) bool {
	if strings.TrimSpace(id) == "" {
		return true // nothing to deduplicate on
	}
	seenMu.Lock()
	defer seenMu.Unlock()
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
	return true
}

// AnswerChannelMessage runs one turn on the configured backend and returns the
// agent's reply text. The project acts as the working directory so every channel
// lands in the same workspace.
func (s *Service) AnswerChannelMessage(text, project string) (string, error) {
	args := map[string]interface{}{"prompt": text}
	if strings.TrimSpace(project) != "" {
		args["cwd"] = project
	}
	raw, err := s.agent().CreateConversation(args)
	if err != nil {
		return "", err
	}
	var parsed map[string]interface{}
	if json.Unmarshal([]byte(raw), &parsed) == nil {
		for _, key := range []string{"response", "text", "message"} {
			if value, ok := parsed[key].(string); ok && strings.TrimSpace(value) != "" {
				return value, nil
			}
		}
	}
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("agent returned no reply")
	}
	return raw, nil
}
