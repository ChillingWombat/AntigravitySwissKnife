package process

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareIsolatedHomeSharesCredentialsAndKeepsStatePrivate(t *testing.T) {
	realHome := t.TempDir()
	t.Setenv("HOME", realHome)
	gemini := filepath.Join(realHome, ".gemini")
	if err := os.MkdirAll(filepath.Join(gemini, "antigravity"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gemini, "oauth_creds.json"), []byte(`{"access_token":"tok"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gemini, "antigravity", "conversation_summaries.db"), []byte("shared"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(realHome, ".config", "antigravity"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realHome, ".config", "antigravity", "plugin-secrets.json"), []byte(`{"k":"v"}`), 0600); err != nil {
		t.Fatal(err)
	}

	isolated := filepath.Join(t.TempDir(), "home")
	if err := PrepareIsolatedHome(isolated); err != nil {
		t.Fatalf("PrepareIsolatedHome: %v", err)
	}

	// Credentials are visible so the window stays signed in.
	creds := filepath.Join(isolated, ".gemini", "oauth_creds.json")
	if data, err := os.ReadFile(creds); err != nil || string(data) != `{"access_token":"tok"}` {
		t.Fatalf("credentials not mirrored: %q err=%v", data, err)
	}
	secrets := filepath.Join(isolated, ".config", "antigravity", "plugin-secrets.json")
	if data, err := os.ReadFile(secrets); err != nil || string(data) != `{"k":"v"}` {
		t.Fatalf("extension secrets not mirrored: %q err=%v", data, err)
	}

	// State stays private: the shared summaries db must not appear.
	stateDir := filepath.Join(isolated, ".gemini", "antigravity")
	if info, err := os.Lstat(stateDir); err != nil {
		t.Fatalf("state dir missing: %v", err)
	} else if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("state dir is a symlink: an isolated window would write the shared tree")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "conversation_summaries.db")); err == nil {
		t.Fatal("shared conversation summaries leaked into the isolated profile")
	}

	// Refresh is idempotent.
	if err := PrepareIsolatedHome(isolated); err != nil {
		t.Fatalf("second PrepareIsolatedHome: %v", err)
	}
}

func TestHomeEnvOverrideReplacesInheritedHome(t *testing.T) {
	env := homeEnvOverride([]string{"HOME=/real", "PATH=/bin", "USERPROFILE=C:\\real"}, "/isolated")
	count := 0
	for _, entry := range env {
		if entry == "HOME=/isolated" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one HOME override, got %d in %v", count, env)
	}
	for _, entry := range env {
		if entry == "HOME=/real" || entry == "USERPROFILE=C:\\real" {
			t.Fatalf("inherited home survived: %s", entry)
		}
	}
}
