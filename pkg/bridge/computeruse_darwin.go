//go:build darwin

package bridge

import (
	"fmt"
	"os/exec"
	"strings"
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
	// Newlines cannot be typed as keystrokes; press Return between lines instead.
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			script := fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escapeAppleScript(line))
			if err := exec.Command("osascript", "-e", script).Run(); err != nil {
				return fmt.Errorf("type failed: %w", err)
			}
		}
	}
	return nil
}

// escapeAppleScript keeps text inside one string literal.
func escapeAppleScript(text string) string {
	escaped := strings.ReplaceAll(text, "\\", "\\\\")
	return strings.ReplaceAll(escaped, `"`, `\\"`)
}
