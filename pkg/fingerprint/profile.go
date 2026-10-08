package fingerprint

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
)

var (
	uuidRegex    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	machineRegex = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// DeviceProfile holds virtualized hardware and installation telemetry UUIDs.
type DeviceProfile struct {
	AccountEmail     string `json:"account_email,omitempty"`
	MachineID        string `json:"machine_id"`
	UpdaterID        string `json:"updater_id"`
	InstallationID   string `json:"installation_id"`
	InstallationUUID string `json:"installation_uuid"`
}

// GenerateUUIDv4 generates a compliant RFC 4122 version 4 UUID.
func GenerateUUIDv4() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// set version 4
	b[6] = (b[6] & 0x0f) | 0x40
	// set variant (RFC 4122)
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// GenerateMachineID generates a 64-character lowercase hex string matching machineid.
func GenerateMachineID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateRandom creates a new randomized profile with valid UUIDv4 and machine ID.
func GenerateRandom() (*DeviceProfile, error) {
	mID, err := GenerateMachineID()
	if err != nil {
		return nil, err
	}
	uID, err := GenerateUUIDv4()
	if err != nil {
		return nil, err
	}
	iID, err := GenerateUUIDv4()
	if err != nil {
		return nil, err
	}
	sUUID, err := GenerateUUIDv4()
	if err != nil {
		return nil, err
	}

	return &DeviceProfile{
		MachineID:        mID,
		UpdaterID:        uID,
		InstallationID:   iID,
		InstallationUUID: sUUID,
	}, nil
}

// Validate checks whether all identifiers in the profile adhere to the expected format.
func (p *DeviceProfile) Validate() error {
	if !machineRegex.MatchString(p.MachineID) {
		return fmt.Errorf("invalid machine_id: must be 64-char hex, got %q", p.MachineID)
	}
	if !uuidRegex.MatchString(p.UpdaterID) {
		return fmt.Errorf("invalid updater_id: must be valid UUIDv4, got %q", p.UpdaterID)
	}
	if !uuidRegex.MatchString(p.InstallationID) {
		return fmt.Errorf("invalid installation_id: must be valid UUIDv4, got %q", p.InstallationID)
	}
	if !uuidRegex.MatchString(p.InstallationUUID) {
		return fmt.Errorf("invalid installation_uuid: must be valid UUIDv4, got %q", p.InstallationUUID)
	}
	return nil
}
