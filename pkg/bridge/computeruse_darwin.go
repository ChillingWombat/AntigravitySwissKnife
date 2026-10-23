//go:build darwin

package bridge

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// inputDriver reports the macOS automation path.
func inputDriver() string {
	if _, err := exec.LookPath("cliclick"); err == nil {
		return "cliclick"
	}
	return "osascript"
}

func performClick(x, y int) error {
	if bin, err := exec.LookPath("cliclick"); err == nil {
		cmd := exec.Command(bin, fmt.Sprintf("c:%d,%d", x, y))
		core.SetHiddenConsole(cmd)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	script := fmt.Sprintf(`tell application "System Events" to click at {x:%d, y:%d}`, x, y)
	cmd := exec.Command("osascript", "-e", script)
	core.SetHiddenConsole(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("click failed: %w", err)
	}
	return nil
}

func performMove(x, y int) error {
	if bin, err := exec.LookPath("cliclick"); err == nil {
		cmd := exec.Command(bin, fmt.Sprintf("m:%d,%d", x, y))
		core.SetHiddenConsole(cmd)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	jxa := fmt.Sprintf(`ObjC.import('CoreGraphics'); $.CGEventPost($.kCGHIDEventTap, $.CGEventCreateMouseEvent(null, $.kCGEventMouseMoved, {x:%d, y:%d}, 0));`, x, y)
	cmd := exec.Command("osascript", "-l", "JavaScript", "-e", jxa)
	core.SetHiddenConsole(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("move failed: %w", err)
	}
	return nil
}

func performScroll(amount int) error {
	jxa := fmt.Sprintf(`ObjC.import('CoreGraphics'); var e = $.CGEventCreateScrollWheelEvent(null, 0, 1, %d); $.CGEventPost($.kCGHIDEventTap, e);`, -amount)
	cmd := exec.Command("osascript", "-l", "JavaScript", "-e", jxa)
	core.SetHiddenConsole(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("scroll failed: %w", err)
	}
	return nil
}

func performType(text string) error {
	if bin, err := exec.LookPath("cliclick"); err == nil {
		cmd := exec.Command(bin, fmt.Sprintf("t:%s", text))
		core.SetHiddenConsole(cmd)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			script := fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escapeAppleScript(line))
			cmd := exec.Command("osascript", "-e", script)
			core.SetHiddenConsole(cmd)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("type failed: %w", err)
			}
		}
	}
	return nil
}

func performKey(keys string) error {
	script := convertAppleScriptKey(keys)
	cmd := exec.Command("osascript", "-e", script)
	core.SetHiddenConsole(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("key failed: %w", err)
	}
	return nil
}

// convertAppleScriptKey turns key descriptions like "Return", "ctrl+c", "command+v" into AppleScript.
func convertAppleScriptKey(raw string) string {
	parts := strings.Split(raw, "+")
	var modifiers []string
	var key string
	for _, p := range parts {
		normalized := strings.TrimSpace(strings.ToLower(p))
		switch normalized {
		case "ctrl", "control":
			modifiers = append(modifiers, "control down")
		case "cmd", "command":
			modifiers = append(modifiers, "command down")
		case "alt", "opt", "option":
			modifiers = append(modifiers, "option down")
		case "shift":
			modifiers = append(modifiers, "shift down")
		default:
			key = strings.TrimSpace(p)
		}
	}

	keyLower := strings.ToLower(key)
	switch keyLower {
	case "return", "enter":
		return buildAppleScriptKeyCode(36, modifiers)
	case "tab":
		return buildAppleScriptKeyCode(48, modifiers)
	case "escape", "esc":
		return buildAppleScriptKeyCode(53, modifiers)
	case "space":
		return buildAppleScriptKeyCode(49, modifiers)
	case "delete", "backspace":
		return buildAppleScriptKeyCode(51, modifiers)
	case "up":
		return buildAppleScriptKeyCode(126, modifiers)
	case "down":
		return buildAppleScriptKeyCode(125, modifiers)
	case "left":
		return buildAppleScriptKeyCode(123, modifiers)
	case "right":
		return buildAppleScriptKeyCode(124, modifiers)
	default:
		if len(modifiers) > 0 {
			return fmt.Sprintf(`tell application "System Events" to keystroke "%s" using {%s}`, escapeAppleScript(key), strings.Join(modifiers, ", "))
		}
		return fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escapeAppleScript(key))
	}
}

func buildAppleScriptKeyCode(code int, modifiers []string) string {
	if len(modifiers) > 0 {
		return fmt.Sprintf(`tell application "System Events" to key code %d using {%s}`, code, strings.Join(modifiers, ", "))
	}
	return fmt.Sprintf(`tell application "System Events" to key code %d`, code)
}

// escapeAppleScript keeps text inside one string literal.
func escapeAppleScript(text string) string {
	escaped := strings.ReplaceAll(text, "\\", "\\\\")
	return strings.ReplaceAll(escaped, `"`, `\\"`)
}
