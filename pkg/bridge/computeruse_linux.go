//go:build linux

package bridge

import (
	"fmt"
	"os/exec"
	"strconv"
)

// inputDriver reports which automation tool is available on Linux.
func inputDriver() string {
	if _, err := exec.LookPath("xdotool"); err == nil {
		return "xdotool"
	}
	if _, err := exec.LookPath("ydotool"); err == nil {
		return "ydotool"
	}
	return ""
}

func performClick(x, y int) error {
	if bin, err := exec.LookPath("xdotool"); err == nil {
		return exec.Command(bin, "mousemove", strconv.Itoa(x), strconv.Itoa(y), "click", "1").Run()
	}
	if bin, err := exec.LookPath("ydotool"); err == nil {
		_ = exec.Command(bin, "mousemove", strconv.Itoa(x), strconv.Itoa(y)).Run()
		return exec.Command(bin, "click", "0xC0").Run()
	}
	return fmt.Errorf("click not configured: xdotool or ydotool is required on Linux")
}

func performMove(x, y int) error {
	if bin, err := exec.LookPath("xdotool"); err == nil {
		return exec.Command(bin, "mousemove", strconv.Itoa(x), strconv.Itoa(y)).Run()
	}
	if bin, err := exec.LookPath("ydotool"); err == nil {
		return exec.Command(bin, "mousemove", strconv.Itoa(x), strconv.Itoa(y)).Run()
	}
	return fmt.Errorf("move not configured: xdotool or ydotool is required on Linux")
}

func performType(text string) error {
	if bin, err := exec.LookPath("xdotool"); err == nil {
		return exec.Command(bin, "type", "--clearmodifiers", text).Run()
	}
	if bin, err := exec.LookPath("ydotool"); err == nil {
		return exec.Command(bin, "type", text).Run()
	}
	return fmt.Errorf("type not configured: xdotool or ydotool is required on Linux")
}

func performKey(keys string) error {
	if bin, err := exec.LookPath("xdotool"); err == nil {
		return exec.Command(bin, "key", keys).Run()
	}
	if bin, err := exec.LookPath("ydotool"); err == nil {
		return exec.Command(bin, "key", keys).Run()
	}
	return fmt.Errorf("key not configured: xdotool or ydotool is required on Linux")
}
