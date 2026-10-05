package core

import (
	"testing"
)

func TestCryptoRoundtrip(t *testing.T) {
	orig := "1//0eTestingOAuthRefreshTokenSecret"
	enc := EncryptCredential(orig)
	if enc == orig {
		t.Fatalf("expected encrypted text to differ from original")
	}
	if !stringsHasPrefix(enc, EncryptedPrefix) {
		t.Fatalf("expected encrypted text to have %s prefix, got: %s", EncryptedPrefix, enc)
	}

	dec := DecryptCredential(enc)
	if dec != orig {
		t.Fatalf("expected decrypted %s, got: %s", orig, dec)
	}

	// Idempotent test
	encAgain := EncryptCredential(enc)
	if encAgain != enc {
		t.Fatalf("expected idempotent encryption")
	}
}

func TestAppPasswordHashing(t *testing.T) {
	pass := "Secure123!"
	hashed := HashAppPassword(pass)

	if !VerifyAppPassword(pass, hashed) {
		t.Fatalf("password verification failed")
	}
	if VerifyAppPassword("WrongPassword", hashed) {
		t.Fatalf("wrong password should not verify")
	}

	ok, _ := ValidateAppPassword("123456")
	if !ok {
		t.Fatalf("expected 6 char password to be valid")
	}
	okShort, _ := ValidateAppPassword("12345")
	if okShort {
		t.Fatalf("expected 5 char password to fail validation")
	}
}

func stringsHasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}
