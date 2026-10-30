package custommodels

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanSingleShellRC(t *testing.T) {
	dir := t.TempDir()
	rcFile := filepath.Join(dir, ".zshrc")
	initialContent := "# Existing user zshrc configuration\nalias ls='ls --color=auto'\n# Antigravity Swiss Knife - CLI Integration Environment\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\nexport AGY_LLM_GATEWAY_URL=\"http://127.0.0.1:8765\"\n"
	if err := os.WriteFile(rcFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("write initial zshrc: %v", err)
	}

	cleanSingleShellRC(rcFile)
	d1, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("read zshrc after clean: %v", err)
	}
	if strings.Contains(string(d1), "CLOUD_CODE_URL") || strings.Contains(string(d1), "AGY_LLM_GATEWAY_URL") {
		t.Errorf("expected zshrc to not contain proxy variables, got:\n%s", string(d1))
	}
	if !strings.Contains(string(d1), "alias ls=") {
		t.Errorf("expected non-proxy user settings to be preserved")
	}
}

func TestCheckCLIGatewayStatus_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	status := CheckCLIGatewayStatus(store)
	if status.GatewayURL != "http://127.0.0.1:8765" {
		t.Errorf("expected gateway URL http://127.0.0.1:8765, got %q", status.GatewayURL)
	}
}
