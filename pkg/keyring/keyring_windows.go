//go:build windows

package keyring

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// The Antigravity desktop app (through its language server's authstore)
// persists the signed-in session in the OS keyring under a single logical
// identity: service "gemini", account "antigravity". On Windows that is the
// Credential Manager entry "gemini:antigravity". The app restores this session
// on every launch and republishes it to the on-disk token files
// (jetski-standalone-oauth-token, app_storage.json, ...), which therefore act
// as outputs rather than inputs. Writing the keyring entry is what makes
// desktop account switching take effect on Windows.
const (
	winKeyringTarget   = "gemini:antigravity"
	winKeyringUsername = "antigravity"

	credTypeGeneric         = 1
	credPersistLocalMachine = 2
)

var (
	advapi32Keyring = syscall.NewLazyDLL("advapi32.dll")
	procCredReadW   = advapi32Keyring.NewProc("CredReadW")
	procCredWriteW  = advapi32Keyring.NewProc("CredWriteW")
	procCredDeleteW = advapi32Keyring.NewProc("CredDeleteW")
	procCredFree    = advapi32Keyring.NewProc("CredFree")
)

// credentialW mirrors the Win32 CREDENTIALW structure.
type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// readWindowsCredential reads a generic credential blob from the Windows
// Credential Manager.
func readWindowsCredential(target string) ([]byte, error) {
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return nil, err
	}
	var pcred *credentialW
	r, _, credErr := procCredReadW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&pcred)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CredRead %q failed: %w", target, credErr)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pcred)))
	if pcred == nil || pcred.CredentialBlob == nil || pcred.CredentialBlobSize == 0 {
		return nil, fmt.Errorf("CredRead %q returned an empty blob", target)
	}
	return append([]byte(nil), unsafe.Slice(pcred.CredentialBlob, pcred.CredentialBlobSize)...), nil
}

// writeWindowsCredential writes a generic credential blob owned by the current
// user (persist = local machine), creating or overwriting the entry.
func writeWindowsCredential(target, username string, blob []byte) error {
	if len(blob) == 0 {
		return fmt.Errorf("refusing to write an empty credential blob")
	}
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	userPtr, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	cred := credentialW{
		Type:               credTypeGeneric,
		TargetName:         targetPtr,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     &blob[0],
		Persist:            credPersistLocalMachine,
		UserName:           userPtr,
	}
	r, _, credErr := procCredWriteW.Call(uintptr(unsafe.Pointer(&cred)), 0)
	runtime.KeepAlive(blob)
	if r == 0 {
		return fmt.Errorf("CredWrite %q failed: %w", target, credErr)
	}
	return nil
}

// deleteWindowsCredential removes a generic credential entry.
func deleteWindowsCredential(target string) error {
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	r, _, credErr := procCredDeleteW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
	)
	if r == 0 {
		return fmt.Errorf("CredDelete %q failed: %w", target, credErr)
	}
	return nil
}

// windowsKeyringPayload mirrors the exact JSON schema the Antigravity desktop
// authstore writes on Windows (verified against a live entry):
//
//	{"token":{"expiry":"<RFC3339Nano local>","access_token":"...","token_type":"Bearer","refresh_token":"..."},
//	 "auth_method":"consumer","id_token":"<top level>"}
type windowsKeyringPayload struct {
	Token struct {
		Expiry       string `json:"expiry"`
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
	} `json:"token"`
	AuthMethod string `json:"auth_method"`
	IDToken    string `json:"id_token"`
}

func buildWindowsKeyringPayload(acc *Account) ([]byte, error) {
	if acc == nil {
		return nil, fmt.Errorf("nil account")
	}
	var payload windowsKeyringPayload
	payload.AuthMethod = "consumer"
	payload.Token.AccessToken = acc.AccessToken
	payload.Token.TokenType = "Bearer"
	payload.Token.RefreshToken = acc.RefreshToken

	switch {
	case !acc.TokenExpiry.IsZero():
		payload.Token.Expiry = acc.TokenExpiry.In(time.Local).Format(time.RFC3339Nano)
	case acc.AccessToken != "":
		payload.Token.Expiry = time.Now().Add(time.Hour).In(time.Local).Format(time.RFC3339Nano)
	default:
		payload.Token.Expiry = time.Now().Add(-time.Minute).In(time.Local).Format(time.RFC3339Nano)
	}

	payload.IDToken = acc.IDToken
	if payload.IDToken == "" && acc.Email != "" {
		payload.IDToken = mintMinimalIDToken(acc.Email)
	}
	return json.Marshal(payload)
}

// readNativeKeyringToken reads the Antigravity session from the Windows
// Credential Manager entry the desktop authstore restores from.
func readNativeKeyringToken() (*secretTokenData, error) {
	if core.IsRunningTests() || os.Getenv("ANTIGRAVITY_TEST_MODE") == "1" {
		return nil, nil
	}
	blob, err := readWindowsCredential(winKeyringTarget)
	if err != nil {
		return nil, err
	}
	var payload windowsKeyringPayload
	if err := json.Unmarshal(blob, &payload); err != nil {
		return nil, err
	}
	idToken := payload.IDToken
	if idToken == "" {
		// Accept the Linux secret-service shape (id_token nested in token) too.
		var nested secretServicePayload
		if json.Unmarshal(blob, &nested) == nil {
			idToken = nested.Token.IDToken
		}
	}
	return &secretTokenData{
		AccessToken:  payload.Token.AccessToken,
		RefreshToken: payload.Token.RefreshToken,
		IDToken:      idToken,
	}, nil
}

// WriteWindowsKeyringToken stores account credentials into the Windows
// Credential Manager entry used by the Antigravity desktop authstore, so the
// next app launch restores the switched account.
func WriteWindowsKeyringToken(acc *Account) error {
	if os.Getenv("ANTIGRAVITY_TEST_MODE") == "1" || core.IsRunningTests() || acc == nil || isTestMockEmail(acc.Email) || accountHasRedactedCredentials(acc) || (acc.AccessToken == "" && acc.RefreshToken == "") {
		return nil
	}
	blob, err := buildWindowsKeyringPayload(acc)
	if err != nil {
		return fmt.Errorf("failed to marshal Windows keyring payload: %w", err)
	}
	return writeWindowsCredential(winKeyringTarget, winKeyringUsername, blob)
}
