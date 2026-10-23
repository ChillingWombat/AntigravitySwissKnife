//go:build linux

package ls

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// csrfFromProc reads the token from the language server's argv via procfs.
func csrfFromProc() (string, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name()[0] < '0' || entry.Name()[0] > '9' {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if readErr != nil || !bytes.Contains(raw, []byte("language_server")) {
			continue
		}
		if token := tokenFromArgs(strings.Split(string(raw), "\x00")); token != "" {
			return token, nil
		}
	}
	return "", fmt.Errorf("no running language server carries a csrf token")
}
