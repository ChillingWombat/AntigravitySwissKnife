//go:build windows

package bridge

import (
	"fmt"
	"os/exec"
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

func performType(text string) error {
	script := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait('%s')`, text)
	if err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script).Run(); err != nil {
		return fmt.Errorf("type failed: %w", err)
	}
	return nil
}
