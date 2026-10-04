package totp

import (
	"testing"
	"time"
)

func TestRFC6238TestVectors(t *testing.T) {
	// Secret: ASCII "12345678901234567890" -> Base32 "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	engine := NewEngine()

	tests := []struct {
		unixTime int64
		expected string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}

	for _, tt := range tests {
		tm := time.Unix(tt.unixTime, 0)
		code, err := engine.GenerateCode(secret, tm)
		if err != nil {
			t.Fatalf("at time %d: unexpected error: %v", tt.unixTime, err)
		}
		if code != tt.expected {
			t.Errorf("at time %d: got %s, expected %s", tt.unixTime, code, tt.expected)
		}
	}
}

func TestValidateSecret(t *testing.T) {
	valid := []string{
		"JBSWY3DPEHPK3PXP",
		"jbswy3dpehpk3pxp",
		"JBSW Y3DP EHPK 3PXP",
		"GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ",
	}
	for _, s := range valid {
		if !ValidateSecret(s) {
			t.Errorf("expected secret %q to be valid", s)
		}
	}

	invalid := []string{
		"18901890", // 8 and 9 are invalid in RFC 4648 base32
		"JBSWY3DPEHPK3PX8",
		"???!!!",
		"",
	}
	for _, s := range invalid {
		if ValidateSecret(s) {
			t.Errorf("expected secret %q to be invalid", s)
		}
	}
}

func TestVerifyCodeDrift(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	engine := NewEngine()
	now := time.Now()

	code, err := engine.GenerateCode(secret, now)
	if err != nil {
		t.Fatalf("GenerateCode error: %v", err)
	}

	// Immediate verify
	if !engine.VerifyCode(secret, code, now, 0) {
		t.Errorf("failed to verify code with 0 drift")
	}

	// 1-step drift ahead
	future := now.Add(30 * time.Second)
	if !engine.VerifyCode(secret, code, future, 1) {
		t.Errorf("failed to verify code with 1-step drift")
	}

	// 2-step drift should fail if window is 1
	farFuture := now.Add(65 * time.Second)
	if engine.VerifyCode(secret, code, farFuture, 1) {
		t.Errorf("code should have failed verification with window=1")
	}
}

func TestBothSeedFormats(t *testing.T) {
	engine := NewEngine()
	now := time.Unix(1234567890, 0)
	rawBase32 := "JBSWY3DPEHPK3PXP"
	codeBase32, err := engine.GenerateCode(rawBase32, now)
	if err != nil {
		t.Fatalf("unexpected error for raw Base32: %v", err)
	}

	// Format 2: otpauth:// URI
	otpauthURI := "otpauth://totp/Google:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Google"
	codeURI, err := engine.GenerateCode(otpauthURI, now)
	if err != nil {
		t.Fatalf("unexpected error for otpauth URI: %v", err)
	}
	if codeURI != codeBase32 {
		t.Errorf("otpauth URI produced %s, expected %s matching base32", codeURI, codeBase32)
	}

	// Format 1 with spaces & lowercase
	spaced := "jbsw y3dp ehpk 3pxp"
	codeSpaced, err := engine.GenerateCode(spaced, now)
	if err != nil {
		t.Fatalf("unexpected error for spaced lowercase: %v", err)
	}
	if codeSpaced != codeBase32 {
		t.Errorf("spaced secret produced %s, expected %s", codeSpaced, codeBase32)
	}

	// Format 3: Hex seed format (Hello! in hex = 48656c6c6f21 + padding)
	hexSeed := "48656c6c6f2131323334353637383930"
	if !ValidateSecret(hexSeed) {
		t.Errorf("expected hex seed to be valid")
	}
}

func TestCountdown(t *testing.T) {
	engine := NewEngine()
	tm := time.Unix(100, 0) // 100 % 30 = 10 elapsed -> 20 remaining
	info := engine.GetCountdown(tm)
	if info.RemainingSeconds != 20 {
		t.Errorf("expected 20s remaining, got %d", info.RemainingSeconds)
	}
	if info.ProgressFraction < 0.65 || info.ProgressFraction > 0.67 {
		t.Errorf("expected ~0.66 fraction, got %f", info.ProgressFraction)
	}
}

