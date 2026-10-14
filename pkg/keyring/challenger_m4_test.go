package keyring

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint"
)

// TestChallenger_SyncHardwareProfileToDirs_PermissionsAndRegexStress tests that:
// 1. All synchronized hardware profile files are written with strict 0600 permissions.
// 2. The installation_uuid regex correctly matches and replaces varied whitespace and indentation
//    without destroying adjacent protobuf text lines, comments, or blank lines.
func TestChallenger_SyncHardwareProfileToDirs_PermissionsAndRegexStress(t *testing.T) {
	tmpDir := t.TempDir()
	agConfigDir := filepath.Join(tmpDir, ".config", "Antigravity")
	geminiAgDir := filepath.Join(tmpDir, ".gemini", "antigravity")

	prof, err := fingerprint.GenerateRandom()
	if err != nil {
		t.Fatalf("GenerateRandom failed: %v", err)
	}

	// Hostile pbtxt with comments, tabs, varied whitespace, and multiple fields
	hostilePbtxt := `# Header comment
agent_onboarding_completed: true

  	installation_uuid:   "legacy-indented-uuid-9999"  	
# Middle comment
last_selected_model: "gemini-3.8-flash"

trailing_flag: false
`
	if err := os.MkdirAll(geminiAgDir, 0755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(geminiAgDir, "antigravity_state.pbtxt")
	if err := os.WriteFile(statePath, []byte(hostilePbtxt), 0644); err != nil {
		t.Fatal(err)
	}

	if err := SyncHardwareProfileToDirs(prof, agConfigDir, geminiAgDir); err != nil {
		t.Fatalf("SyncHardwareProfileToDirs failed: %v", err)
	}

	// 1. Verify strict 0600 permissions across all files
	filesToCheck := []string{
		filepath.Join(agConfigDir, "machineid"),
		filepath.Join(agConfigDir, ".updaterId"),
		filepath.Join(geminiAgDir, "installation_id"),
		statePath,
	}

	for _, f := range filesToCheck {
		info, err := os.Stat(f)
		if err != nil {
			t.Fatalf("failed to stat file %s: %v", filepath.Base(f), err)
		}
		perm := info.Mode().Perm()
		if runtime.GOOS != "windows" && perm != 0600 {
			t.Errorf("file %s has permissions %o; want strict 0600", filepath.Base(f), perm)
		}
	}

	// 2. Verify pbtxt replacement accuracy
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read statePath: %v", err)
	}
	content := string(data)

	// Old indented uuid should be gone
	if strings.Contains(content, "legacy-indented-uuid-9999") {
		t.Errorf("state file still contains old UUID")
	}

	// New uuid must be present
	expectedUUIDLine := `installation_uuid: "` + prof.InstallationUUID + `"`
	if !strings.Contains(content, expectedUUIDLine) {
		t.Errorf("state file does not contain new UUID line %q; content:\n%s", expectedUUIDLine, content)
	}

	// Surrounding comments and fields must remain intact
	for _, expectedSnippet := range []string{
		"# Header comment",
		"agent_onboarding_completed: true",
		"# Middle comment",
		`last_selected_model: "gemini-3.8-flash"`,
		"trailing_flag: false",
	} {
		if !strings.Contains(content, expectedSnippet) {
			t.Errorf("state file missing preserved snippet %q after sync", expectedSnippet)
		}
	}
}

// TestChallenger_SyncHardwareProfileToDirs_MissingPbtxtCreation verifies that when antigravity_state.pbtxt
// is initially absent, it is created cleanly with 0600 permissions.
func TestChallenger_SyncHardwareProfileToDirs_MissingPbtxtCreation(t *testing.T) {
	tmpDir := t.TempDir()
	agConfigDir := filepath.Join(tmpDir, ".config", "Antigravity")
	geminiAgDir := filepath.Join(tmpDir, ".gemini", "antigravity")

	prof, err := fingerprint.GenerateRandom()
	if err != nil {
		t.Fatal(err)
	}

	statePath := filepath.Join(geminiAgDir, "antigravity_state.pbtxt")

	if err := SyncHardwareProfileToDirs(prof, agConfigDir, geminiAgDir); err != nil {
		t.Fatalf("SyncHardwareProfileToDirs failed: %v", err)
	}

	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("statePath was not created: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Errorf("created statePath has perm %o; want 0600", info.Mode().Perm())
	}

	data, _ := os.ReadFile(statePath)
	expected := `installation_uuid: "` + prof.InstallationUUID + `"` + "\n"
	if string(data) != expected {
		t.Errorf("content of created statePath = %q; want %q", string(data), expected)
	}
}
