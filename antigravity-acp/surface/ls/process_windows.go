//go:build windows

package ls

import (
	"fmt"
	"os/exec"
	"strings"
)

// csrfFromProc queries the process command line through CIM, since Windows has no
// procfs and wmic is deprecated on current builds.
func csrfFromProc() (string, error) {
	script := `Get-CimInstance Win32_Process -Filter "Name='language_server.exe'" | Select-Object -ExpandProperty CommandLine`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return "", fmt.Errorf("powershell: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if token := tokenFromArgs(strings.Fields(line)); token != "" {
			return token, nil
		}
	}
	return "", fmt.Errorf("no running language server carries a csrf token")
}
