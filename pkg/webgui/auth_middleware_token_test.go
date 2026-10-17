package webgui

import (
	"os"
	"path/filepath"
	"testing"
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
