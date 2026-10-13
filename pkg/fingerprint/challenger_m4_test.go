package fingerprint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestChallenger_FingerprintStore_EmailNormalization tests that Store methods normalize
// emails across arbitrary casings and whitespace variations to the exact same profile key.
func TestChallenger_FingerprintStore_EmailNormalization(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_fp_store_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storePath := filepath.Join(tmpDir, "device_profiles.json")
	store, err := NewStore(storePath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	mixedEmail := "Developer.Ops@ACME-CORP.com"
	upperEmail := "DEVELOPER.OPS@ACME-CORP.COM"
	lowerEmail := "developer.ops@acme-corp.com"
	paddedEmail := "   Developer.Ops@ACME-CORP.com   "

	// 1. Create with mixed-case
	prof1, err := store.GetOrCreateProfile(mixedEmail)
	if err != nil {
		t.Fatalf("GetOrCreateProfile(%q) failed: %v", mixedEmail, err)
	}
	if prof1.AccountEmail != lowerEmail {
		t.Errorf("expected AccountEmail normalized to %q, got %q", lowerEmail, prof1.AccountEmail)
	}

	// 2. Retrieve with all casing variants
	for _, query := range []string{upperEmail, lowerEmail, paddedEmail, mixedEmail} {
		got := store.GetProfile(query)
		if got == nil {
			t.Fatalf("GetProfile(%q) returned nil, expected profile for %q", query, lowerEmail)
		}
		if got.MachineID != prof1.MachineID || got.InstallationID != prof1.InstallationID {
			t.Errorf("GetProfile(%q) returned mismatched profile identifiers", query)
		}
	}

	// 3. GetOrCreate with uppercase returns the identical existing profile, NOT a new one
	prof2, err := store.GetOrCreateProfile(upperEmail)
	if err != nil {
		t.Fatalf("GetOrCreateProfile(%q) failed: %v", upperEmail, err)
	}
	if prof2.MachineID != prof1.MachineID || prof2.InstallationUUID != prof1.InstallationUUID {
		t.Errorf("GetOrCreateProfile(%q) generated a second profile instead of reusing existing", upperEmail)
	}

	// 4. Persistence round-trip: reload store and verify only 1 normalized entry exists
	store2, err := NewStore(storePath)
	if err != nil {
		t.Fatalf("NewStore reload failed: %v", err)
	}
	if len(store2.profiles) != 1 {
		t.Errorf("expected exactly 1 profile in reloaded store, found %d", len(store2.profiles))
	}
	reloaded := store2.GetProfile(lowerEmail)
	if reloaded == nil || reloaded.MachineID != prof1.MachineID {
		t.Errorf("reloaded profile did not match original")
	}

	// 5. Update with uppercase email via SetProfile
	updatedProf, _ := GenerateRandom()
	if err := store2.SetProfile(upperEmail, updatedProf); err != nil {
		t.Fatalf("SetProfile(%q) failed: %v", upperEmail, err)
	}
	if len(store2.profiles) != 1 {
		t.Errorf("expected still exactly 1 profile after SetProfile with uppercase, found %d", len(store2.profiles))
	}
	gotUpdated := store2.GetProfile(lowerEmail)
	if gotUpdated == nil || gotUpdated.MachineID != strings.ToLower(updatedProf.MachineID) {
		t.Errorf("SetProfile with uppercase email did not update normalized entry")
	}
}

// TestChallenger_DeviceProfile_RFC4122_CaseInsensitiveValidation tests that Validate() accepts
// uppercase RFC 4122 UUIDs and uppercase machine IDs, normalizes them to lowercase, and strictly
// rejects invalid formats.
func TestChallenger_DeviceProfile_RFC4122_CaseInsensitiveValidation(t *testing.T) {
	// Valid uppercase UUIDv4 (version 4: 4XXX, variant: 9XXX or 8XXX or AXXX or BXXX)
	validUpperUUID := "D83E92B1-4F42-4A82-9B7E-7F89A0BC1234"
	validUpperMachine := strings.Repeat("A1B2C3D4", 8) // 64 chars hex

	prof := &DeviceProfile{
		MachineID:        validUpperMachine,
		UpdaterID:        validUpperUUID,
		InstallationID:   validUpperUUID,
		InstallationUUID: validUpperUUID,
	}

	if err := prof.Validate(); err != nil {
		t.Fatalf("Validate() failed on valid uppercase UUID: %v", err)
	}

	// Verify all identifiers normalized to lowercase
	if prof.MachineID != strings.ToLower(validUpperMachine) {
		t.Errorf("expected MachineID normalized to lowercase, got %q", prof.MachineID)
	}
	if prof.UpdaterID != strings.ToLower(validUpperUUID) {
		t.Errorf("expected UpdaterID normalized to lowercase, got %q", prof.UpdaterID)
	}
	if prof.InstallationID != strings.ToLower(validUpperUUID) {
		t.Errorf("expected InstallationID normalized to lowercase, got %q", prof.InstallationID)
	}
	if prof.InstallationUUID != strings.ToLower(validUpperUUID) {
		t.Errorf("expected InstallationUUID normalized to lowercase, got %q", prof.InstallationUUID)
	}

	// Adversarial test cases: invalid UUIDs and MachineIDs
	invalidCases := []struct {
		name string
		prof DeviceProfile
	}{
		{
			"Version5InsteadOf4",
			DeviceProfile{
				MachineID:        strings.Repeat("a", 64),
				UpdaterID:        "d83e92b1-4f42-5a82-9b7e-7f89a0bc1234", // version 5
				InstallationID:   validUpperUUID,
				InstallationUUID: validUpperUUID,
			},
		},
		{
			"InvalidVariant0",
			DeviceProfile{
				MachineID:        strings.Repeat("a", 64),
				UpdaterID:        "d83e92b1-4f42-4a82-0b7e-7f89a0bc1234", // variant 0
				InstallationID:   validUpperUUID,
				InstallationUUID: validUpperUUID,
			},
		},
		{
			"NonHexCharacters",
			DeviceProfile{
				MachineID:        strings.Repeat("a", 64),
				UpdaterID:        "d83e92b1-4f42-4a82-zb7e-7f89a0bc1234", // 'z' is not hex
				InstallationID:   validUpperUUID,
				InstallationUUID: validUpperUUID,
			},
		},
		{
			"TooShortUUID",
			DeviceProfile{
				MachineID:        strings.Repeat("a", 64),
				UpdaterID:        "d83e92b1-4f42-4a82-9b7e-7f89a0bc123", // missing last char
				InstallationID:   validUpperUUID,
				InstallationUUID: validUpperUUID,
			},
		},
		{
			"MachineID_63Chars",
			DeviceProfile{
				MachineID:        strings.Repeat("a", 63),
				UpdaterID:        validUpperUUID,
				InstallationID:   validUpperUUID,
				InstallationUUID: validUpperUUID,
			},
		},
		{
			"MachineID_NonHex",
			DeviceProfile{
				MachineID:        strings.Repeat("a", 63) + "z",
				UpdaterID:        validUpperUUID,
				InstallationID:   validUpperUUID,
				InstallationUUID: validUpperUUID,
			},
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			cp := tc.prof
			if err := cp.Validate(); err == nil {
				t.Errorf("Validate() succeeded on invalid input %s, expected error", tc.name)
			}
		})
	}
}
