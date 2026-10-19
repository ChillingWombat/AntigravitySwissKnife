package webgui

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// createDirOutsideHome returns a temporary directory that is not below the user
// home directory. Browsing used to be restricted to home/config/cwd/temp; this
// helper proves paths outside those roots are now reachable.
func createDirOutsideHome(t *testing.T) string {
	t.Helper()

	home, _ := os.UserHomeDir()
	var bases []string
	if wd, err := os.Getwd(); err == nil && wd != "" {
		bases = append(bases, filepath.VolumeName(wd)+string(os.PathSeparator))
	}
	if runtime.GOOS != "windows" {
		bases = append(bases, "/var/tmp")
	}

	for _, base := range bases {
		if base == "" {
			continue
		}
		dir, err := os.MkdirTemp(base, "swiss-unrestricted-*")
		if err != nil {
			continue
		}
		if home != "" && pathMatchesOrIsSubpath(dir, home) {
			_ = os.RemoveAll(dir)
			continue
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		return dir
	}
	t.Skip("no writable directory outside the user home is available")
	return ""
}

// TestFilesUnrestricted_BrowseOutsideHome verifies issue #157: the file API no
// longer refuses paths outside the home/config/cwd/temp whitelist.
func TestFilesUnrestricted_BrowseOutsideHome(t *testing.T) {
	outsideDir := createDirOutsideHome(t)
	marker := filepath.Join(outsideDir, "hello.txt")
	if err := os.WriteFile(marker, []byte("unrestricted"), 0600); err != nil {
		t.Fatalf("failed to create marker file: %v", err)
	}

	if _, err := validateJailedPath(outsideDir); err != nil {
		t.Errorf("[REGRESSION] validateJailedPath rejected outside-home dir %q: %v", outsideDir, err)
	}

	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()
	baseURL := "http://" + srv.Addr()

	resp, err := http.Get(baseURL + "/api/files/list?path=" + url.QueryEscape(outsideDir))
	if err != nil {
		t.Fatalf("GET /api/files/list failed: %v", err)
	}
	defer resp.Body.Close()
	var listRes struct {
		Success bool `json:"success"`
		Files   []struct {
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listRes); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if !listRes.Success {
		t.Fatalf("listing outside-home directory %q was refused", outsideDir)
	}
	found := false
	for _, f := range listRes.Files {
		if strings.EqualFold(f.Name, "hello.txt") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected hello.txt in listing of %q", outsideDir)
	}

	respRead, err := http.Get(baseURL + "/api/files/read?path=" + url.QueryEscape(marker))
	if err != nil {
		t.Fatalf("GET /api/files/read failed: %v", err)
	}
	defer respRead.Body.Close()
	var readRes struct {
		Success bool   `json:"success"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(respRead.Body).Decode(&readRes); err != nil {
		t.Fatalf("decode read response: %v", err)
	}
	if !readRes.Success || readRes.Content != "unrestricted" {
		t.Errorf("expected successful read of outside-home file, got success=%v content=%q", readRes.Success, readRes.Content)
	}
}

// TestFilesUnrestricted_DestructiveFloors verifies the remaining limits: system
// and credential locations are readable but can never be written, renamed,
// created in, moved into, or deleted.
func TestFilesUnrestricted_DestructiveFloors(t *testing.T) {
	var systemReadPath, systemWritePath, systemDeletePath string
	if runtime.GOOS == "windows" {
		systemReadPath = `C:\Windows\win.ini`
		systemWritePath = `C:\Windows\System32\swiss-unrestricted-test.txt`
		systemDeletePath = `C:\Windows\System32\swiss-unrestricted-nonexistent`
	} else {
		systemReadPath = "/etc/hosts"
		systemWritePath = "/etc/cron.d/evil_cron"
		systemDeletePath = "/etc/swiss-unrestricted-nonexistent"
	}

	if res, err := validateJailedPath(systemReadPath); err != nil {
		t.Errorf("[REGRESSION] reading previously-forbidden path %q was refused: %v", systemReadPath, err)
	} else if res == "" {
		t.Errorf("validateJailedPath(%q) returned empty path", systemReadPath)
	}
	if res, err := validateDestructivePath(systemWritePath); err == nil {
		t.Errorf("[VULNERABILITY] write to system path %q was allowed -> %q", systemWritePath, res)
	}
	if res, err := validateDeletablePath(systemDeletePath); err == nil {
		t.Errorf("[VULNERABILITY] delete under system path %q was allowed -> %q", systemDeletePath, res)
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory available")
	}
	credentialPath := filepath.Join(home, ".ssh", "config")
	if res, err := validateJailedPath(credentialPath); err != nil {
		t.Errorf("[REGRESSION] reading credential path %q was refused: %v", credentialPath, err)
	} else if res == "" {
		t.Errorf("validateJailedPath(%q) returned empty path", credentialPath)
	}
	if res, err := validateDestructivePath(credentialPath); err == nil {
		t.Errorf("[VULNERABILITY] destructive operation on credential path %q was allowed -> %q", credentialPath, res)
	}

	// Relative paths remain invalid: only normalised absolute paths are accepted.
	if res, err := validateJailedPath("relative/file.txt"); err == nil {
		t.Errorf("[VULNERABILITY] relative path accepted -> %q", res)
	}
}
