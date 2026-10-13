package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfigDir_TestIsolation(t *testing.T) {
	// Without ANTIGRAVITY_SWISS_CONFIG_DIR, running in test runner must isolate to temp dir
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", "")
	dir := GetConfigDir()
	expectedPrefix := filepath.Clean(os.TempDir())
	if !filepath.IsAbs(dir) {
		t.Fatalf("expected absolute path, got %q", dir)
	}
	expected := filepath.Join(expectedPrefix, "antigravity-swiss-test-config")
	if dir != expected {
		t.Fatalf("expected test config dir %q, got %q", expected, dir)
	}

	// Verify the directory exists
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected %q to exist and be a directory, err: %v", dir, err)
	}
}

func TestGetConfigDir_ExplicitEnvOverride(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", customDir)
	dir := GetConfigDir()
	if dir != customDir {
		t.Fatalf("expected %q, got %q", customDir, dir)
	}
}
