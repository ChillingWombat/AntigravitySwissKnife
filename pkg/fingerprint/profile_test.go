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

func TestStoreSanitizeDeduplicateAndCollisionGuard(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "profiles.json")

	// Seed a file where two accounts share the same non-hex machine_id and duplicate updater_id
	rawJSON := `{
  "alpha@gmail.com": {
    "machine_id": "b1025406-c0de-41d8-99ff-c661143b172f",
    "updater_id": "21431880-0453-4828-ad35-3d22f2f19b41",
    "installation_id": "b656b750-0aed-49f2-842e-05de620f74bd",
    "installation_uuid": "0fa27fd5-8d15-4aa8-83cd-25f574bd39d8"
  },
  "beta@gmail.com": {
    "machine_id": "b1025406-c0de-41d8-99ff-c661143b172f",
    "updater_id": "21431880-0453-4828-ad35-3d22f2f19b41",
    "installation_id": "25ec34b8-7f85-4547-ac05-33b700a69530",
    "installation_uuid": "bbf9f571-b211-45e7-a5eb-fe05ddb3b8f3"
  }
}`
	if err := os.WriteFile(storePath, []byte(rawJSON), 0600); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(storePath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	alpha := store.GetProfile("alpha@gmail.com")
	beta := store.GetProfile("beta@gmail.com")
	if alpha == nil || beta == nil {
		t.Fatalf("expected both profiles to exist")
	}

	if err := alpha.Validate(); err != nil {
		t.Errorf("expected alpha profile to be healed to valid format, got: %v", err)
	}
	if err := beta.Validate(); err != nil {
		t.Errorf("expected beta profile to be healed to valid format, got: %v", err)
	}
	if alpha.MachineID == beta.MachineID {
		t.Errorf("expected deduplicated machine_id, both had %s", alpha.MachineID)
	}
	if alpha.UpdaterID == beta.UpdaterID {
		t.Errorf("expected deduplicated updater_id, both had %s", alpha.UpdaterID)
	}

	// Attempting to save a colliding profile for beta must fail
	colliding := *alpha
	if err := store.SetProfile("beta@gmail.com", &colliding); err == nil {
		t.Errorf("expected SetProfile to reject cross-account collision")
	}

	slice := store.ListProfilesSlice()
	if len(slice) != 2 || slice[0].AccountEmail != "alpha@gmail.com" || slice[1].AccountEmail != "beta@gmail.com" {
		t.Errorf("unexpected ListProfilesSlice: %+v", slice)
	}
}

