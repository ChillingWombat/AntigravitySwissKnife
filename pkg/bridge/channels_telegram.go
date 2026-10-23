package bridge

import (
	"encoding/json"
	"fmt"
	"strings"
)

// telegramUpdate matches the subset of an update we act on.
type telegramUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64  `json:"message_id"`
		Text      string `json:"text"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From *struct {
			ID int64 `json:"id"`
		} `json:"from"`
	} `json:"message"`
}

// ParseTelegramInbound extracts text messages from a webhook body. Non-message
// updates (edits, joins, callback queries) yield nothing, which is not an error.
func ParseTelegramInbound(body []byte) ([]InboundMessage, error) {
	var update telegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		return nil, fmt.Errorf("invalid Telegram payload: %w", err)
	}
	if update.Message == nil {
		return nil, nil
	}
	text := strings.TrimSpace(update.Message.Text)
	if text == "" {
		return nil, nil
	}
	return []InboundMessage{{
		ID:   fmt.Sprintf("tg-%d-%d", update.Message.Chat.ID, update.Message.MessageID),
		From: fmt.Sprintf("%d", update.Message.Chat.ID),
		Text: text,
	}}, nil
}

// SendTelegramReply posts a message back to the chat a prompt came from.
func SendTelegramReply(token, chatID, text string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("telegram bot token is not configured")
	}
	body, err := json.Marshal(map[string]interface{}{"chat_id": chatID, "text": text})
	if err != nil {
		return err
	}
	return postJSON("https://api.telegram.org/bot"+token+"/sendMessage", "", body)
}
