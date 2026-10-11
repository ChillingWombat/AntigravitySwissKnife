package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureVSCodeWindowCloseGuard_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	err := EnsureVSCodeWindowCloseGuardAtPath(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settingsPath := filepath.Join(tmpDir, "User", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if val, ok := m["window.closeWhenEmpty"]; !ok || val != false {
		t.Fatalf("expected window.closeWhenEmpty to be false, got %v", val)
	}
}

func TestEnsureVSCodeWindowCloseGuard_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	userDir := filepath.Join(tmpDir, "User")
	_ = os.MkdirAll(userDir, 0755)
	settingsPath := filepath.Join(userDir, "settings.json")

	initial := map[string]interface{}{
		"editor.fontSize":       14,
		"window.closeWhenEmpty": true,
	}
	data, _ := json.Marshal(initial)
	_ = os.WriteFile(settingsPath, data, 0644)

	err := EnsureVSCodeWindowCloseGuardAtPath(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	readData, _ := os.ReadFile(settingsPath)
	var updated map[string]interface{}
	_ = json.Unmarshal(readData, &updated)

	if updated["window.closeWhenEmpty"] != false {
		t.Fatalf("expected window.closeWhenEmpty to be false, got %v", updated["window.closeWhenEmpty"])
	}
	if updated["editor.fontSize"] != float64(14) {
		t.Fatalf("expected editor.fontSize to be preserved, got %v", updated["editor.fontSize"])
	}
}

func TestEnsureVSCodeWindowCloseGuard_AlreadyFalse(t *testing.T) {
	tmpDir := t.TempDir()
	userDir := filepath.Join(tmpDir, "User")
	_ = os.MkdirAll(userDir, 0755)
	settingsPath := filepath.Join(userDir, "settings.json")

	initial := map[string]interface{}{
		"window.closeWhenEmpty": false,
	}
	data, _ := json.Marshal(initial)
	_ = os.WriteFile(settingsPath, data, 0644)

	statBefore, _ := os.Stat(settingsPath)

	err := EnsureVSCodeWindowCloseGuardAtPath(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	statAfter, _ := os.Stat(settingsPath)
	if !statBefore.ModTime().Equal(statAfter.ModTime()) {
		t.Fatalf("expected file to not be touched when already false")
	}
}
