package core

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// EnsureVSCodeWindowCloseGuard ensures that VS Code User settings.json has
// "window.closeWhenEmpty": false so that closing the last open editor tab
// never closes the entire application window.
func EnsureVSCodeWindowCloseGuard() error {
	return EnsureVSCodeWindowCloseGuardAtPath(GetVSCodeHostConfigDir())
}

// EnsureVSCodeWindowCloseGuardAtPath enforces "window.closeWhenEmpty": false in the specified config dir.
func EnsureVSCodeWindowCloseGuardAtPath(configDir string) error {
	if configDir == "" {
		return nil
	}
	userDir := filepath.Join(configDir, "User")
	settingsPath := filepath.Join(userDir, "settings.json")

	// If configDir does not exist and is not within a temporary test directory,
	// do nothing (VS Code is not installed or configured).
	if _, err := os.Stat(configDir); os.IsNotExist(err) && !IsRunningTests() {
		return nil
	}

	if err := os.MkdirAll(userDir, 0755); err != nil {
		return err
	}

	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if jsonErr := json.Unmarshal(data, &settings); jsonErr != nil {
			// Malformed or comments in JSON: avoid destructive overwrite
			return nil
		}
	} else if os.IsNotExist(err) {
		settings = make(map[string]interface{})
	} else {
		return err
	}

	if val, ok := settings["window.closeWhenEmpty"]; ok && val == false {
		return nil // already properly configured
	}

	settings["window.closeWhenEmpty"] = false
	updatedData, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return err
	}

	tmpFile := settingsPath + ".tmp"
	if err := os.WriteFile(tmpFile, updatedData, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, settingsPath)
}
