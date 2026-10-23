package bridge

import "testing"

const whatsappTextPayload = `{
  "object":"whatsapp_business_account",
  "entry":[{"changes":[{"value":{
    "messaging_product":"whatsapp",
    "messages":[
      {"id":"wamid.1","from":"61400000000","type":"text","text":{"body":"  hello agent  "}},
      {"id":"wamid.2","from":"61400000000","type":"image","image":{"id":"img1"}}
    ],
    "statuses":[{"id":"wamid.0","status":"delivered"}]
  }}]}]
}`

func TestParseWhatsAppInboundKeepsOnlyText(t *testing.T) {
	messages, err := ParseWhatsAppInbound([]byte(whatsappTextPayload))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected one text message, got %d", len(messages))
	}
	if messages[0].ID != "wamid.1" || messages[0].From != "61400000000" {
		t.Errorf("unexpected identity: %+v", messages[0])
	}
	if messages[0].Text != "hello agent" {
		t.Errorf("text was not trimmed: %q", messages[0].Text)
	}
}

func TestParseWhatsAppInboundRejectsGarbage(t *testing.T) {
	if _, err := ParseWhatsAppInbound([]byte("not json")); err == nil {
		t.Error("expected an error for a malformed payload")
	}
}

// WhatsApp retries webhooks for days; the same message must be relayed once.
func TestMarkMessageSeenIsIdempotent(t *testing.T) {
	if !MarkMessageSeen("wamid.dedupe-1") {
		t.Fatal("first delivery must be accepted")
	}
	if MarkMessageSeen("wamid.dedupe-1") {
		t.Error("retry must be rejected")
	}
	if !MarkMessageSeen("wamid.dedupe-2") {
		t.Error("a different message must be accepted")
	}
	if !MarkMessageSeen("") {
		t.Error("a message without an id cannot be deduplicated and should pass")
	}
}

const telegramTextPayload = `{"update_id":42,"message":{"message_id":7,"text":"  ping  ","chat":{"id":-100123},"from":{"id":9}}}`

func TestParseTelegramInboundExtractsText(t *testing.T) {
	messages, err := ParseTelegramInbound([]byte(telegramTextPayload))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected one message, got %d", len(messages))
	}
	if messages[0].Text != "ping" || messages[0].From != "-100123" {
		t.Errorf("unexpected message: %+v", messages[0])
	}
	if messages[0].ID != "tg--100123-7" {
		t.Errorf("unexpected idempotency key: %q", messages[0].ID)
	}
}

func TestParseTelegramInboundIgnoresNonMessages(t *testing.T) {
	messages, err := ParseTelegramInbound([]byte(`{"update_id":43,"edited_message":{"message_id":7,"text":"x","chat":{"id":1}}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected no messages, got %d", len(messages))
	}
}

// Only the surfaces that share the user's context are accepted as backends. The
// standalone ACP agent keeps its own store, so it is not offered.
func TestNormalizeAgentBackendRejectsACP(t *testing.T) {
	for _, raw := range []string{"acp", "agy_acp", "antigravity-acp"} {
		if got := normalizeAgentBackend(raw); got != "" {
			t.Errorf("normalizeAgentBackend(%q) = %q, want rejected", raw, got)
		}
	}
	for _, raw := range []string{"cli", "agy", "app", "gui"} {
		if got := normalizeAgentBackend(raw); got == "" {
			t.Errorf("normalizeAgentBackend(%q) was rejected", raw)
		}
	}
}
