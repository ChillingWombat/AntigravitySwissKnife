//go:build windows

package bridge

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// inputDriver reports the PowerShell user32 bridge used for input.
func inputDriver() string { return "powershell-user32" }

const user32Click = `Add-Type -TypeDefinition 'using System;using System.Runtime.InteropServices;public class U{[DllImport("user32.dll")]public static extern bool SetCursorPos(int X,int Y);[DllImport("user32.dll")]public static extern void mouse_event(uint f,uint x,uint y,uint d,int e);}'; [U]::SetCursorPos(%d,%d); [U]::mouse_event(0x02,0,0,0,0); [U]::mouse_event(0x04,0,0,0,0)`

func performClick(x, y int) error {
	script := fmt.Sprintf(user32Click, x, y)
	if err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script).Run(); err != nil {
		return fmt.Errorf("click failed: %w", err)
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
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("type failed: %w", err)
	}
	return nil
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
