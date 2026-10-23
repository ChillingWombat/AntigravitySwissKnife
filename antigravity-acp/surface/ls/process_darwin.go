//go:build darwin

package ls

import (
	"fmt"
	"os/exec"
	"strings"
)

// csrfFromProc reads the token from `ps` output, since macOS has no procfs.
func csrfFromProc() (string, error) {
	out, err := exec.Command("ps", "-axo", "command=").Output()
	if err != nil {
		return "", fmt.Errorf("ps: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "language_server") || !strings.Contains(line, "--csrf_token") {
			continue
		}
		if token := tokenFromArgs(strings.Fields(line)); token != "" {
			return token, nil
		}
	}
	return "", fmt.Errorf("no running language server carries a csrf token")
}
