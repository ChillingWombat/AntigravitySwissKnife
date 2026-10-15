//go:build windows

package keyring

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestBuildWindowsKeyringPayload_Schema(t *testing.T) {
	acc := &Account{
		Email:        "user@example.com",
		AccessToken:  "ya29.access",
		RefreshToken: "1//refresh",
		IDToken:      "header.payload.signature",
		TokenExpiry:  time.Date(2026, 10, 15, 5, 24, 18, 154744400, time.Local),
	}
	blob, err := buildWindowsKeyringPayload(acc)
	if err != nil {
		t.Fatalf("buildWindowsKeyringPayload error: %v", err)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(blob, &root); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if len(root) != 3 {
		t.Errorf("expected 3 root keys, got %v", keysOf(root))
	}
	if root["id_token"] != "header.payload.signature" {
		t.Errorf("id_token must be at top level, got %v", root["id_token"])
	}
	if root["auth_method"] != "consumer" {
		t.Errorf("auth_method = %v, want consumer", root["auth_method"])
	}
	tok, ok := root["token"].(map[string]interface{})
	if !ok {
		t.Fatalf("token object missing")
	}
	if len(tok) != 4 {
		t.Errorf("expected 4 token keys (expiry, access_token, token_type, refresh_token), got %v", keysOf(tok))
	}
	if tok["token_type"] != "Bearer" {
		t.Errorf("token_type = %v, want Bearer", tok["token_type"])
	}
	if tok["access_token"] != "ya29.access" || tok["refresh_token"] != "1//refresh" {
		t.Errorf("token values not carried over")
	}
	expiry, _ := tok["expiry"].(string)
	if _, perr := time.Parse(time.RFC3339Nano, expiry); perr != nil {
		t.Errorf("expiry %q is not RFC3339Nano parseable: %v", expiry, perr)
	}
	if _, idtInToken := tok["id_token"]; idtInToken {
		t.Errorf("id_token must NOT be nested inside token for the Windows schema")
	}
}

func TestWindowsCredentialRoundTrip(t *testing.T) {
	target := fmt.Sprintf("gemini:antigravity-unit-test-%d", time.Now().UnixNano())
	blob := []byte(`{"token":{"expiry":"2026-10-15T05:24:18.1547444+11:00","access_token":"a","token_type":"Bearer","refresh_token":"r"},"auth_method":"consumer","id_token":"x.y.z"}`)

	if err := writeWindowsCredential(target, "antigravity", blob); err != nil {
		t.Fatalf("writeWindowsCredential error: %v", err)
	}
	t.Cleanup(func() { _ = deleteWindowsCredential(target) })

	got, err := readWindowsCredential(target)
	if err != nil {
		t.Fatalf("readWindowsCredential error: %v", err)
	}
	if string(got) != string(blob) {
		t.Errorf("round-trip mismatch:\n got: %s\nwant: %s", got, blob)
	}

	if err := deleteWindowsCredential(target); err != nil {
		t.Errorf("deleteWindowsCredential error: %v", err)
	}
	if _, err := readWindowsCredential(target); err == nil {
		t.Errorf("credential still readable after delete")
	}
}

func TestBuildWindowsKeyringPayload_MintsIDTokenWhenMissing(t *testing.T) {
	acc := &Account{Email: "someone@gmail.com", RefreshToken: "1//r"}
	blob, err := buildWindowsKeyringPayload(acc)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var root map[string]interface{}
	_ = json.Unmarshal(blob, &root)
	idt, _ := root["id_token"].(string)
	if idt == "" {
		t.Errorf("expected a minted minimal id_token")
	}
}

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
