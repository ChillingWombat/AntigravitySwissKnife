package process

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// The desktop app resolves its Gemini state as os.homedir()/.gemini/<app name>
// (dist/paths.js: getAppDataDir -> path.join(os.homedir(), '.gemini',
// getAppDataDirName())). Nothing consults app.commandLine or an env flag for that
// path, so a home-scoped launch is the only non-destructive way to isolate it:
// os.homedir() honours HOME on POSIX and USERPROFILE on Windows, and Node, Go and
// Chromium all agree on those two variables.
//
// An isolated home links everything the app shares globally (credentials, config)
// and keeps the actual state directory private, so a second window writes only to
// its own tree.

// sharedHomeEntries are the ~/.gemini children a second profile may read but must
// not share the state of.
var sharedHomeEntries = []string{"antigravity", "antigravity-ide", "antigravity-backup"}

// IsolatedHomeDir returns the throwaway HOME for an isolated profile.
func IsolatedHomeDir(app string) string {
	return filepath.Join(core.GetConfigDir(), "profiles", app+customProfileSuffix+"-home")
}

// PrepareIsolatedHome builds (or refreshes) an isolated HOME: a private .gemini
// state directory plus links to the real credentials and config so the window
// stays signed in. Links are refreshed on every launch, so tokens rotated by the
// native window are immediately visible.
func PrepareIsolatedHome(home string) error {
	realHome, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0700); err != nil {
		return fmt.Errorf("create isolated .gemini: %w", err)
	}
	entries, err := os.ReadDir(filepath.Join(realHome, ".gemini"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", filepath.Join(realHome, ".gemini"), err)
	}
	for _, entry := range entries {
		if isSharedStateEntry(entry.Name()) {
			continue
		}
		if err := linkOrCopy(filepath.Join(realHome, ".gemini", entry.Name()), filepath.Join(geminiDir, entry.Name())); err != nil {
			if isCredentialEntry(entry.Name()) {
				return fmt.Errorf("mirror credential %s into the isolated home: %w", entry.Name(), err)
			}
			log.Printf("[model-source] could not mirror ~/.gemini/%s into the isolated home: %v", entry.Name(), err)
		}
	}
	for _, name := range sharedHomeEntries {
		if err := os.MkdirAll(filepath.Join(geminiDir, name), 0700); err != nil {
			return err
		}
	}
	// Extension credentials live outside ~/.gemini on every platform.
	if err := linkOrCopy(filepath.Join(realHome, ".config", "antigravity"), filepath.Join(home, ".config", "antigravity")); err != nil {
		return fmt.Errorf("mirror extension credentials: %w", err)
	}
	return nil
}

// isCredentialEntry reports whether a ~/.gemini entry carries authentication, in
// which case failing to mirror it must abort the launch rather than start a
// signed-out window.
func isCredentialEntry(name string) bool {
	switch name {
	case "oauth_creds.json", "google_accounts.json", "config":
		return true
	}
	return false
}

func isSharedStateEntry(name string) bool {
	for _, shared := range sharedHomeEntries {
		if name == shared {
			return true
		}
	}
	return false
}

// linkOrCopy mirrors src at dst, preferring a symlink. Windows needs privileges for
// symlinks, so fall back to a copy; missing sources are not an error (the app may
// not have created them yet).
func linkOrCopy(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	// Refresh: drop whatever a previous launch left, then prefer a link. Windows
	// without Developer Mode has directories here, and Remove cannot delete those.
	_ = os.RemoveAll(dst)
	if linkErr := os.Symlink(src, dst); linkErr == nil {
		return nil
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
		return err
	}
	return os.WriteFile(to, data, 0644)
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0700); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		from := filepath.Join(src, entry.Name())
		to := filepath.Join(dst, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			if target, linkErr := os.Readlink(from); linkErr == nil {
				if os.Symlink(target, to) == nil {
					continue
				}
			}
		}
		if entry.IsDir() {
			if err := copyDir(from, to); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(from, to); err != nil {
			return err
		}
	}
	return nil
}

// isolatedHomeEnv returns the environment overrides that scope a child process to
// the given home directory.
func isolatedHomeEnv(home string) []string {
	env := []string{"HOME=" + home}
	if runtime.GOOS == "windows" {
		env = append(env, "USERPROFILE="+home)
	}
	return env
}

// homeEnvOverride strips any inherited HOME/USERPROFILE so the isolated value wins.
func homeEnvOverride(base []string, home string) []string {
	kept := make([]string, 0, len(base)+2)
	for _, entry := range base {
		upper := strings.ToUpper(entry)
		if strings.HasPrefix(upper, "HOME=") || strings.HasPrefix(upper, "USERPROFILE=") {
			continue
		}
		kept = append(kept, entry)
	}
	return append(kept, isolatedHomeEnv(home)...)
}
