package webgui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The session token must survive daemon restarts: injected IDE pages, the
// desktop renderer and the tray all hold a copy, and rotating per boot 401'd
// every one of them.
func TestGenerateLocalSessionToken_PersistsAcrossRestarts(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())

	tok1, err := GenerateLocalSessionToken()
	if err != nil {
		t.Fatalf("first GenerateLocalSessionToken: %v", err)
	}
	if len(tok1) != 64 {
		t.Fatalf("expected 64-char hex token, got %q", tok1)
	}

	tok2, err := GenerateLocalSessionToken()
	if err != nil {
		t.Fatalf("second GenerateLocalSessionToken: %v", err)
	}
	if tok2 != tok1 {
		t.Fatalf("token rotated across restarts: %q -> %q", tok1, tok2)
	}
}

func TestGenerateLocalSessionToken_RegeneratesCorruptFile(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", cfgDir)
	tokenPath := filepath.Join(cfgDir, "session.token")
	if err := os.WriteFile(tokenPath, []byte("not-a-valid-token"), 0600); err != nil {
		t.Fatalf("seed corrupt token: %v", err)
	}

	tok, err := GenerateLocalSessionToken()
	if err != nil {
		t.Fatalf("GenerateLocalSessionToken: %v", err)
	}
	if len(tok) != 64 {
		t.Fatalf("expected regenerated 64-char hex token, got %q", tok)
	}
}

func TestGetLocalSessionToken_UsesCacheForUnchangedFile(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", cfgDir)
	tokenPath := filepath.Join(cfgDir, "session.token")
	cachedToken := strings.Repeat("a", 64)
	replacementToken := strings.Repeat("b", 64)
	if err := os.WriteFile(tokenPath, []byte(cachedToken), 0600); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	fixedTime := time.Now().Add(-time.Minute).Truncate(time.Second)
	if err := os.Chtimes(tokenPath, fixedTime, fixedTime); err != nil {
		t.Fatalf("set initial mtime: %v", err)
	}

	first, err := GetLocalSessionToken()
	if err != nil {
		t.Fatalf("first GetLocalSessionToken: %v", err)
	}
	if first != cachedToken {
		t.Fatalf("first token = %q, want %q", first, cachedToken)
	}

	if err := os.WriteFile(tokenPath, []byte(replacementToken), 0600); err != nil {
		t.Fatalf("replace token: %v", err)
	}
	if err := os.Chtimes(tokenPath, fixedTime, fixedTime); err != nil {
		t.Fatalf("restore mtime: %v", err)
	}

	second, err := GetLocalSessionToken()
	if err != nil {
		t.Fatalf("second GetLocalSessionToken: %v", err)
	}
	if second != cachedToken {
		t.Fatalf("unchanged file should use cached token %q, got %q", cachedToken, second)
	}

	bumpedTime := fixedTime.Add(time.Second)
	if err := os.Chtimes(tokenPath, bumpedTime, bumpedTime); err != nil {
		t.Fatalf("bump mtime: %v", err)
	}

	third, err := GetLocalSessionToken()
	if err != nil {
		t.Fatalf("third GetLocalSessionToken: %v", err)
	}
	if third != replacementToken {
		t.Fatalf("rewritten token should refresh cache to %q, got %q", replacementToken, third)
	}
}
