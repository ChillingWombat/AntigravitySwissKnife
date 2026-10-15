package webgui

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEmpirical_RepoWipeViaParentTraversal verifies whether validateDeletablePath
// permits deletion of parent directories (such as the repository root or parent workspace)
// when the process runs inside a directory nested under an allowed root (like user home).
func TestEmpirical_RepoWipeViaParentTraversal(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("HOME", tempHome)

	// Simulate a repository nested 2 levels inside the home directory
	repoDir := filepath.Join(tempHome, "work", "MyRepo")
	subPkgDir := filepath.Join(repoDir, "pkg", "webgui")
	if err := os.MkdirAll(subPkgDir, 0755); err != nil {
		t.Fatalf("Failed to create subPkgDir: %v", err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current wd: %v", err)
	}
	if err := os.Chdir(subPkgDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	// Traversal candidate "../.." targeting the repo root from pkg/webgui
	target := "../.."
	resolved, err := validateDeletablePath(target)
	if err == nil {
		t.Errorf("[CRITICAL VULNERABILITY] validateDeletablePath(%q) succeeded and returned %q! It allowed deletion of the parent repository!", target, resolved)
	} else {
		t.Logf("[SAFE] validateDeletablePath(%q) blocked: %v", target, err)
	}
}

// TestEmpirical_DanglingSymlinkBypass verifies whether validateJailedPath
// allows dangling symlinks targeting non-existent system paths on Windows or Unix.
func TestEmpirical_DanglingSymlinkBypass(t *testing.T) {
	tempDir := t.TempDir()

	symlinkPath := filepath.Join(tempDir, "evil_symlink_to_hosts")
	// Target /etc/hosts (which does not exist on Windows)
	target := "/etc/hosts"
	if err := os.Symlink(target, symlinkPath); err != nil {
		t.Skipf("Cannot create symlinks in this environment: %v", err)
	}

	resolved, err := validateJailedPath(symlinkPath)
	if err == nil {
		t.Errorf("[VULNERABILITY] validateJailedPath accepted dangling symlink to %s! Resolved to %q", target, resolved)
	} else {
		t.Logf("[SAFE] validateJailedPath rejected dangling symlink: %v", err)
	}
}
