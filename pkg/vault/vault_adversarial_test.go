package vault

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAdversarial_VaultPathTraversalSuite stress-tests validateConversationID,
// RestoreConversation, and VaultConversation against a comprehensive battery
// of path traversal and injection attack vectors.
func TestAdversarial_VaultPathTraversalSuite(t *testing.T) {
	tmpDir := t.TempDir()
	convDir := filepath.Join(tmpDir, "conversations")
	vaultDir := filepath.Join(tmpDir, "vault")
	_ = os.MkdirAll(convDir, 0700)
	_ = os.MkdirAll(vaultDir, 0700)

	mgr := NewManager(convDir, vaultDir)

	maliciousVectors := []struct {
		name     string
		input    string
		scenario string
	}{
		{"Relative parent traversal", "../../../etc/shadow", "unix shadow file traversal"},
		{"Parent traversal with dot-dot", "../../../etc/passwd", "unix passwd traversal"},
		{"Windows style traversal", "..\\..\\..\\windows\\system32", "windows backslash traversal"},
		{"Direct unix root path", "/etc/shadow", "absolute unix path"},
		{"Direct unix root passwd", "/etc/passwd", "absolute unix path"},
		{"Windows drive path", "C:\\Windows\\System32\\cmd.exe", "windows drive traversal"},
		{"UNC network path", "\\\\server\\share\\evil", "UNC share path"},
		{"Single dot", ".", "current directory dot"},
		{"Double dot", "..", "parent directory dot dot"},
		{"Slash dot dot", "/..", "root parent traversal"},
		{"Dot dot slash", "../", "parent slash traversal"},
		{"Nested path slash", "nested/directory/conv", "directory delimiter traversal"},
		{"Nested path backslash", "nested\\directory\\conv", "windows directory delimiter traversal"},
		{"Url encoded traversal dot dot slash", "%2e%2e%2fetc%2fshadow", "URL-encoded traversal"},
		{"Double url encoded traversal", "%252e%252e%252fetc", "double URL-encoded traversal"},
		{"Null byte injection", "conv\x00evil", "null byte poison string"},
		{"Newline injection", "conv\nevil", "newline injection"},
		{"Carriage return injection", "conv\revil", "carriage return injection"},
		{"Tab injection", "conv\tevil", "tab injection"},
		{"Shell semicolon injection", "conv; rm -rf /", "shell semicolon chaining"},
		{"Shell pipe injection", "conv | cat /etc/shadow", "shell pipe injection"},
		{"Shell background injection", "conv & calc.exe", "shell background operator"},
		{"Shell backtick injection", "conv`id`", "shell command substitution backtick"},
		{"Shell subshell injection", "conv$(id)", "shell command substitution subshell"},
		{"Shell redirection injection", "conv > /tmp/pwn", "shell output redirection"},
		{"Only extension", ".db", "extension without conversation ID"},
		{"Whitespace with extension", " .db ", "whitespace padded extension"},
		{"Dot dot db extension", "..db", "dot dot db string"},
		{"Dot dot slash with db", "../conv.db", "parent traversal with db suffix"},
		{"Slash with db", "/conv.db", "root path with db suffix"},
		{"Empty string", "", "empty string"},
		{"Single space", " ", "whitespace single space"},
		{"Multiple spaces", "     ", "whitespace multiple spaces"},
		{"NTFS alternate data stream", "conv:stream:$DATA", "NTFS alternate stream"},
		{"Colon in ID", "conv:123", "colon character"},
		{"Question mark in ID", "conv?id", "wildcard question mark"},
		{"Asterisk in ID", "conv*id", "wildcard asterisk"},
		{"HTML tag injection", "<script>alert(1)</script>", "XSS HTML tags"},
	}

	for _, tc := range maliciousVectors {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Direct validation check
			cid, err := validateConversationID(tc.input)
			if err == nil {
				t.Fatalf("SECURITY FLAW: validateConversationID(%q) succeeded with cid=%q, expected error", tc.input, cid)
			}

			// 2. RestoreConversation must fail
			rescued, err := mgr.RestoreConversation(tc.input)
			if err == nil || rescued {
				t.Fatalf("SECURITY FLAW: RestoreConversation(%q) succeeded (rescued=%v, err=%v)", tc.input, rescued, err)
			}

			// 3. VaultConversation must fail
			vaulted, err := mgr.VaultConversation(tc.input)
			if err == nil || vaulted {
				t.Fatalf("SECURITY FLAW: VaultConversation(%q) succeeded (vaulted=%v, err=%v)", tc.input, vaulted, err)
			}
		})
	}

	// Verify valid conversation IDs succeed in validateConversationID
	validIDs := []struct {
		input    string
		expected string
	}{
		{"99887766-5544-3322-1100-aabbccddeeff", "99887766-5544-3322-1100-aabbccddeeff"},
		{"99887766-5544-3322-1100-aabbccddeeff.db", "99887766-5544-3322-1100-aabbccddeeff"},
		{"conv_123_test-ABC", "conv_123_test-ABC"},
		{"conv_123_test-ABC.db", "conv_123_test-ABC"},
		{" 12345678-abcd-ef01-2345-6789abcdef01 ", "12345678-abcd-ef01-2345-6789abcdef01"},
	}

	for _, tc := range validIDs {
		cid, err := validateConversationID(tc.input)
		if err != nil {
			t.Errorf("validateConversationID(%q) unexpectedly failed: %v", tc.input, err)
		}
		if cid != tc.expected {
			t.Errorf("validateConversationID(%q): expected cid %q, got %q", tc.input, tc.expected, cid)
		}
	}
}

// TestAdversarial_VaultFilePermissions_DeepAudit verifies 0600 file modes
// on both hardlink and byte-copy pathways, and 0700 modes on directories.
func TestAdversarial_VaultFilePermissions_DeepAudit(t *testing.T) {
	tmpDir := t.TempDir()
	convDir := filepath.Join(tmpDir, "conversations")
	vaultDir := filepath.Join(tmpDir, "vault", "conversations")
	annoDir := filepath.Join(tmpDir, "annotations")
	vaultAnnoDir := filepath.Join(tmpDir, "vault", "annotations")

	_ = os.MkdirAll(convDir, 0700)
	_ = os.MkdirAll(annoDir, 0700)

	// Create test file with loose permissions (0666) to verify hardening to 0600
	convID := "perm-test-conv-001"
	dbFile := filepath.Join(convDir, convID+".db")
	_ = os.WriteFile(dbFile, []byte("test-data-for-permissions"), 0666)

	mgr := &Manager{
		conversationsDir: convDir,
		vaultDir:         vaultDir,
		annotationsDir:   annoDir,
		vaultAnnoDir:     vaultAnnoDir,
	}

	// 1. Initial Sync
	res, err := mgr.Sync()
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if res.NewVaulted != 1 {
		t.Fatalf("expected 1 vaulted, got %d", res.NewVaulted)
	}

	// 2. Check vault directory permissions
	vDirInfo, err := os.Stat(vaultDir)
	if err != nil {
		t.Fatalf("stat vaultDir failed: %v", err)
	}
	if vDirInfo.Mode().Perm() != 0700 {
		t.Errorf("expected vaultDir permissions 0700, got %o", vDirInfo.Mode().Perm())
	}

	// 3. Check vaulted file permissions
	vaultFile := filepath.Join(vaultDir, convID+".db")
	vFileInfo, err := os.Stat(vaultFile)
	if err != nil {
		t.Fatalf("stat vaulted file failed: %v", err)
	}
	if vFileInfo.Mode().Perm() != 0600 {
		t.Errorf("expected vaulted file permissions 0600, got %o", vFileInfo.Mode().Perm())
	}

	// 4. Check live file permissions (hardlink syncs inode permissions)
	liveFileInfo, err := os.Stat(dbFile)
	if err != nil {
		t.Fatalf("stat live file failed: %v", err)
	}
	if liveFileInfo.Mode().Perm() != 0600 {
		t.Errorf("expected live file permissions hardened to 0600, got %o", liveFileInfo.Mode().Perm())
	}

	// 5. Explicitly test linkOrCopy fallback path
	// Simulate copy to an independent destination
	copyDst := filepath.Join(tmpDir, "copy-target", "copied-conv.db")
	err = linkOrCopy(dbFile, copyDst)
	if err != nil {
		t.Fatalf("linkOrCopy failed: %v", err)
	}
	copyInfo, err := os.Stat(copyDst)
	if err != nil {
		t.Fatalf("stat copyDst failed: %v", err)
	}
	if copyInfo.Mode().Perm() != 0600 {
		t.Errorf("expected copyDst permissions 0600, got %o", copyInfo.Mode().Perm())
	}
}
