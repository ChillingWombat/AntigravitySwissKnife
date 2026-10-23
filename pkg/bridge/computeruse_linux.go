//go:build linux

package bridge

import (
	"fmt"
	"os/exec"
	"strconv"
)

// inputDriver reports which X11 automation tool is available.
func inputDriver() string {
	if _, err := exec.LookPath("xdotool"); err == nil {
		return "xdotool"
	}
	return ""
}

func performClick(x, y int) error {
	bin, err := exec.LookPath("xdotool")
	if err != nil {
		return fmt.Errorf("click not configured: xdotool is required on Linux")
	}
	return exec.Command(bin, "mousemove", strconv.Itoa(x), strconv.Itoa(y), "click", "1").Run()
}

func performType(text string) error {
	bin, err := exec.LookPath("xdotool")
	if err != nil {
		return fmt.Errorf("type not configured: xdotool is required on Linux")
	}
	return exec.Command(bin, "type", "--clearmodifiers", text).Run()
}
