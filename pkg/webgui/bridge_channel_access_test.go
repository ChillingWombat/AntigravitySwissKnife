package webgui

import "testing"

// An empty whitelist means "no restriction"; a populated one must be enforced, or a
// stranger can spend the user's quota.
func TestWhatsAppNumberAllowed(t *testing.T) {
	if !whatsAppNumberAllowed(nil, "61400000000") {
		t.Error("empty whitelist must allow everyone")
	}
	allowed := []string{"+61400000000", "61411111111"}
	if !whatsAppNumberAllowed(allowed, "61400000000") {
		t.Error("listed number was rejected")
	}
	if !whatsAppNumberAllowed(allowed, "61411111111") {
		t.Error("listed number without a plus was rejected")
	}
	if whatsAppNumberAllowed(allowed, "61499999999") {
		t.Error("unlisted number was allowed")
	}
}

func TestTelegramChatAllowed(t *testing.T) {
	if !telegramChatAllowed(nil, "-100123") {
		t.Error("empty whitelist must allow every chat")
	}
	allowed := []string{"-100123"}
	if !telegramChatAllowed(allowed, "-100123") {
		t.Error("listed chat was rejected")
	}
	if telegramChatAllowed(allowed, "-100999") {
		t.Error("unlisted chat was allowed")
	}
}
