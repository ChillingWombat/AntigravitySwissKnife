//go:build darwin

package bridge

import (
	"fmt"
	"os/exec"
)

// inputDriver reports the macOS automation path, which needs no extra binary.
func inputDriver() string { return "osascript" }

func performClick(x, y int) error {
	script := fmt.Sprintf(`tell application "System Events" to click at {x:%d, y:%d}`, x, y)
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return fmt.Errorf("click failed: %w", err)
	}
	return nil
}

func performType(text string) error {
	// Escaping quotes keeps the text inside a single AppleScript string literal.
	escaped := ""
	for _, r := range text {
		if r == '"' || r == '\\' {
			escaped += "\\"
		}
		escaped += string(r)
	}
	script := fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escaped)
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return fmt.Errorf("type failed: %w", err)
	}
	return nil
}
