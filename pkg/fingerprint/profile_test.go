package fingerprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndValidateRandomProfile(t *testing.T) {
	prof, err := GenerateRandom()
	if err != nil {
		t.Fatalf("unexpected error generating profile: %v", err)
	}

	if err := prof.Validate(); err != nil {
		t.Fatalf("expected generated profile to be valid, got: %v", err)
	}

	if len(prof.MachineID) != 64 {
		t.Errorf("expected 64-char machine_id, got %d", len(prof.MachineID))
	}
	if len(prof.UpdaterID) != 36 {
		t.Errorf("expected 36-char updater_id, got %d", len(prof.UpdaterID))
	}
}

func TestStoreCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_fp_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storePath := filepath.Join(tmpDir, "profiles.json")
	store, err := NewStore(storePath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	email := "test@example.com"
	prof, err := store.GetOrCreateProfile(email)
	if err != nil {
		t.Fatalf("GetOrCreateProfile error: %v", err)
	}

	// Retrieve again
	got := store.GetProfile(email)
	if got == nil || got.MachineID != prof.MachineID {
		t.Errorf("expected to retrieve stored profile, got %+v", got)
	}

	// Reload from new instance to test persistence
	store2, err := NewStore(storePath)
	if err != nil {
		t.Fatalf("NewStore(storePath) error: %v", err)
	}
	got2 := store2.GetProfile(email)
	if got2 == nil || got2.MachineID != prof.MachineID {
		t.Errorf("expected persisted profile, got %+v", got2)
	}
}
