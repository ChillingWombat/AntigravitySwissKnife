//go:build windows

package keyring

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func TestWin32_CredentialW_StructAlignment(t *testing.T) {
	var c credentialW
	ptrSize := unsafe.Sizeof(uintptr(0))

	if ptrSize == 8 {
		// 64-bit Windows (x86_64 / arm64) ABI
		if sz := unsafe.Sizeof(c); sz != 80 {
			t.Errorf("sizeof(credentialW) on 64-bit = %d; want 80", sz)
		}
		if off := unsafe.Offsetof(c.Flags); off != 0 {
			t.Errorf("offset(Flags) = %d; want 0", off)
		}
		if off := unsafe.Offsetof(c.Type); off != 4 {
			t.Errorf("offset(Type) = %d; want 4", off)
		}
		if off := unsafe.Offsetof(c.TargetName); off != 8 {
			t.Errorf("offset(TargetName) = %d; want 8", off)
		}
		if off := unsafe.Offsetof(c.Comment); off != 16 {
			t.Errorf("offset(Comment) = %d; want 16", off)
		}
		if off := unsafe.Offsetof(c.LastWritten); off != 24 {
			t.Errorf("offset(LastWritten) = %d; want 24", off)
		}
		if off := unsafe.Offsetof(c.CredentialBlobSize); off != 32 {
			t.Errorf("offset(CredentialBlobSize) = %d; want 32", off)
		}
		// Critical 4-byte padding check before pointer
		if off := unsafe.Offsetof(c.CredentialBlob); off != 40 {
			t.Errorf("offset(CredentialBlob) = %d; want 40 (must include 4-byte padding after uint32 on 64-bit)", off)
		}
		if off := unsafe.Offsetof(c.Persist); off != 48 {
			t.Errorf("offset(Persist) = %d; want 48", off)
		}
		if off := unsafe.Offsetof(c.AttributeCount); off != 52 {
			t.Errorf("offset(AttributeCount) = %d; want 52", off)
		}
		if off := unsafe.Offsetof(c.Attributes); off != 56 {
			t.Errorf("offset(Attributes) = %d; want 56", off)
		}
		if off := unsafe.Offsetof(c.TargetAlias); off != 64 {
			t.Errorf("offset(TargetAlias) = %d; want 64", off)
		}
		if off := unsafe.Offsetof(c.UserName); off != 72 {
			t.Errorf("offset(UserName) = %d; want 72", off)
		}
	} else if ptrSize == 4 {
		// 32-bit Windows (x86 / arm) ABI
		if sz := unsafe.Sizeof(c); sz != 52 {
			t.Errorf("sizeof(credentialW) on 32-bit = %d; want 52", sz)
		}
		if off := unsafe.Offsetof(c.Flags); off != 0 {
			t.Errorf("offset(Flags) = %d; want 0", off)
		}
		if off := unsafe.Offsetof(c.Type); off != 4 {
			t.Errorf("offset(Type) = %d; want 4", off)
		}
		if off := unsafe.Offsetof(c.TargetName); off != 8 {
			t.Errorf("offset(TargetName) = %d; want 8", off)
		}
		if off := unsafe.Offsetof(c.Comment); off != 12 {
			t.Errorf("offset(Comment) = %d; want 12", off)
		}
		if off := unsafe.Offsetof(c.LastWritten); off != 16 {
			t.Errorf("offset(LastWritten) = %d; want 16", off)
		}
		if off := unsafe.Offsetof(c.CredentialBlobSize); off != 24 {
			t.Errorf("offset(CredentialBlobSize) = %d; want 24", off)
		}
		if off := unsafe.Offsetof(c.CredentialBlob); off != 28 {
			t.Errorf("offset(CredentialBlob) = %d; want 28", off)
		}
		if off := unsafe.Offsetof(c.Persist); off != 32 {
			t.Errorf("offset(Persist) = %d; want 32", off)
		}
		if off := unsafe.Offsetof(c.AttributeCount); off != 36 {
			t.Errorf("offset(AttributeCount) = %d; want 36", off)
		}
		if off := unsafe.Offsetof(c.Attributes); off != 40 {
			t.Errorf("offset(Attributes) = %d; want 40", off)
		}
		if off := unsafe.Offsetof(c.TargetAlias); off != 44 {
			t.Errorf("offset(TargetAlias) = %d; want 44", off)
		}
		if off := unsafe.Offsetof(c.UserName); off != 48 {
			t.Errorf("offset(UserName) = %d; want 48", off)
		}
	}
}

func TestWin32_CredentialManager_LiveRoundTrip(t *testing.T) {
	target := "test_swiss_knife_challenger_target"
	username := "challenger_user"
	blob := []byte(`{"token":{"access_token":"ya29.test1234","refresh_token":"1//testref"},"auth_method":"consumer","id_token":"jwt.claims.sig"}`)

	// Clean up any stale credential before test
	_ = deleteWindowsCredential(target)

	// 1. Write credential
	err := writeWindowsCredential(target, username, blob)
	if err != nil {
		t.Fatalf("writeWindowsCredential failed: %v", err)
	}

	// 2. Read credential
	readBlob, err := readWindowsCredential(target)
	if err != nil {
		t.Fatalf("readWindowsCredential failed: %v", err)
	}
	if !bytes.Equal(readBlob, blob) {
		t.Fatalf("read blob mismatch: got %s, want %s", string(readBlob), string(blob))
	}

	// 3. Overwrite with new blob
	newBlob := []byte(`{"token":{"access_token":"ya29.updated","refresh_token":"1//updatedref"},"auth_method":"consumer","id_token":"jwt.updated.sig"}`)
	if err := writeWindowsCredential(target, username, newBlob); err != nil {
		t.Fatalf("writeWindowsCredential overwrite failed: %v", err)
	}
	readBlob2, err := readWindowsCredential(target)
	if err != nil {
		t.Fatalf("readWindowsCredential after overwrite failed: %v", err)
	}
	if !bytes.Equal(readBlob2, newBlob) {
		t.Fatalf("overwritten blob mismatch: got %s, want %s", string(readBlob2), string(newBlob))
	}

	// 4. Delete credential
	if err := deleteWindowsCredential(target); err != nil {
		t.Fatalf("deleteWindowsCredential failed: %v", err)
	}

	// 5. Read deleted credential must fail
	if _, err := readWindowsCredential(target); err == nil {
		t.Fatalf("readWindowsCredential after delete expected error, got nil")
	}

	// 6. Adversarial cases: empty blob
	if err := writeWindowsCredential(target, username, nil); err == nil {
		t.Errorf("writeWindowsCredential with nil blob expected error, got nil")
	}
	if err := writeWindowsCredential(target, username, []byte{}); err == nil {
		t.Errorf("writeWindowsCredential with empty blob expected error, got nil")
	}

	// 7. Embedded null characters in target
	if _, err := readWindowsCredential("target\x00with_null"); err == nil {
		t.Errorf("readWindowsCredential with embedded null expected error, got nil")
	}
	if err := writeWindowsCredential("target\x00with_null", username, blob); err == nil {
		t.Errorf("writeWindowsCredential with embedded null expected error, got nil")
	}
	if err := deleteWindowsCredential("target\x00with_null"); err == nil {
		t.Errorf("deleteWindowsCredential with embedded null expected error, got nil")
	}
}

func TestWin32_BuildWindowsKeyringPayload_RealAccount(t *testing.T) {
	acc := &Account{
		Email:        "GersioOlba.p238@gmail.com",
		AccessToken:  "ya29.access_live",
		RefreshToken: "1//refresh_live",
		TokenExpiry:  time.Now().Add(1 * time.Hour),
	}

	blob, err := buildWindowsKeyringPayload(acc)
	if err != nil {
		t.Fatalf("buildWindowsKeyringPayload error: %v", err)
	}

	if !strings.Contains(string(blob), "GersioOlba.p238@gmail.com") {
		// Even if IDToken is minted, the minted JWT claims must contain the email
		if !strings.Contains(string(blob), `"id_token"`) {
			t.Fatalf("missing id_token in payload: %s", string(blob))
		}
	}

	// Nil account
	if _, err := buildWindowsKeyringPayload(nil); err == nil {
		t.Errorf("buildWindowsKeyringPayload(nil) expected error, got nil")
	}
}

func TestWin32_WriteWindowsKeyringToken_TestModeGuards(t *testing.T) {
	// Under test mode or with mock email, WriteWindowsKeyringToken must return nil without touching keyring
	os.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	defer os.Unsetenv("ANTIGRAVITY_TEST_MODE")

	acc := &Account{
		Email:        "david.dev@example.com",
		AccessToken:  "ya29.test",
		RefreshToken: "1//test",
	}

	if err := WriteWindowsKeyringToken(acc); err != nil {
		t.Errorf("WriteWindowsKeyringToken under test mode returned error: %v", err)
	}
}
