//go:build windows

package bridge

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// inputDriver reports the PowerShell user32 bridge used for input.
func inputDriver() string { return "powershell-user32" }

const user32Click = `Add-Type -TypeDefinition 'using System;using System.Runtime.InteropServices;public class U{[DllImport("user32.dll")]public static extern bool SetCursorPos(int X,int Y);[DllImport("user32.dll")]public static extern void mouse_event(uint f,uint x,uint y,uint d,int e);}'; [U]::SetCursorPos(%d,%d); [U]::mouse_event(0x02,0,0,0,0); [U]::mouse_event(0x04,0,0,0,0)`

const user32Move = `Add-Type -TypeDefinition 'using System;using System.Runtime.InteropServices;public class U{[DllImport("user32.dll")]public static extern bool SetCursorPos(int X,int Y);}; [U]::SetCursorPos(%d,%d)`

const user32Scroll = `Add-Type -TypeDefinition 'using System;using System.Runtime.InteropServices;public class U{[DllImport("user32.dll")]public static extern void mouse_event(uint f,uint x,uint y,uint d,int e);}'; [U]::mouse_event(0x0800,0,0,%d,0)`

func performClick(x, y int) error {
	script := fmt.Sprintf(user32Click, x, y)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	core.SetHiddenConsole(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("click failed: %w", err)
	}
	return nil
}

func performMove(x, y int) error {
	script := fmt.Sprintf(user32Move, x, y)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	core.SetHiddenConsole(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("move failed: %w", err)
	}
	return nil
}

func performScroll(amount int) error {
	delta := -amount * 120
	script := fmt.Sprintf(user32Scroll, delta)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	core.SetHiddenConsole(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("scroll failed: %w", err)
	}
	return nil
}

// performType passes the text through the environment, never through the command
// string: a quote in the text would otherwise end the PowerShell literal and run
// whatever follows it.
func performType(text string) error {
	script := `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait($env:SWISS_TYPE_TEXT)`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Env = append(os.Environ(), "SWISS_TYPE_TEXT="+escapeSendKeys(text))
	core.SetHiddenConsole(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("type failed: %w", err)
	}
	return nil
}

func performKey(keys string) error {
	script := `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait($env:SWISS_KEY_TEXT)`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Env = append(os.Environ(), "SWISS_KEY_TEXT="+convertToSendKeys(keys))
	core.SetHiddenConsole(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("key failed: %w", err)
	}
	return nil
}

// convertToSendKeys converts common key descriptions into Windows Forms SendKeys format.
func convertToSendKeys(keys string) string {
	parts := strings.Split(keys, "+")
	var prefix string
	var key string
	for _, p := range parts {
		normalized := strings.TrimSpace(strings.ToLower(p))
		switch normalized {
		case "ctrl", "control":
			prefix += "^"
		case "alt":
			prefix += "%"
		case "shift":
			prefix += "+"
		default:
			key = strings.TrimSpace(p)
		}
	}

	keyLower := strings.ToLower(key)
	var mapped string
	switch keyLower {
	case "enter", "return":
		mapped = "{ENTER}"
	case "tab":
		mapped = "{TAB}"
	case "esc", "escape":
		mapped = "{ESC}"
	case "backspace":
		mapped = "{BACKSPACE}"
	case "delete", "del":
		mapped = "{DELETE}"
	case "up":
		mapped = "{UP}"
	case "down":
		mapped = "{DOWN}"
	case "left":
		mapped = "{LEFT}"
	case "right":
		mapped = "{RIGHT}"
	case "home":
		mapped = "{HOME}"
	case "end":
		mapped = "{END}"
	default:
		mapped = escapeSendKeys(key)
	}

	return prefix + mapped
}

// escapeSendKeys wraps the characters SendKeys reads as modifiers, so text like
// "50%" or "f(x)" arrives literally instead of pressing Alt or opening a group.
func escapeSendKeys(text string) string {
	var out strings.Builder
	for _, r := range text {
		switch r {
		case '+', '^', '%', '~', '(', ')', '{', '}', '[', ']':
			out.WriteString("{")
			out.WriteRune(r)
			out.WriteString("}")
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
