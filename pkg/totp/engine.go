package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Engine implements RFC 6238 Time-Based One-Time Password algorithm.
type Engine struct {
	StepSeconds int
}

// NewEngine returns a new TOTP engine with standard 30s step.
func NewEngine() *Engine {
	return &Engine{StepSeconds: 30}
}

// CleanSecret parses and normalizes the secret key from:
// 1. Raw Base32 string (e.g. "JBSWY3DPEHPK3PXP", case-insensitive, with spaces/dashes)
// 2. otpauth:// URI scheme (e.g. "otpauth://totp/Issuer:user?secret=JBSWY3DPEHPK3PXP")
// 3. Raw Hexadecimal seed (e.g. 32-char or 40-char hex string)
func CleanSecret(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Format 1: otpauth:// URI scheme
	if strings.HasPrefix(strings.ToLower(raw), "otpauth://") {
		if u, err := url.Parse(raw); err == nil {
			if sec := u.Query().Get("secret"); sec != "" {
				raw = sec
			}
		} else {
			if idx := strings.Index(strings.ToLower(raw), "secret="); idx != -1 {
				sub := raw[idx+7:]
				if amp := strings.Index(sub, "&"); amp != -1 {
					sub = sub[:amp]
				}
				raw = sub
			}
		}
	}

	cleaned := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(raw, " ", ""), "-", ""))

	// Format 2: Hexadecimal seed string
	if isHexString(cleaned) && len(cleaned) >= 16 && len(cleaned)%2 == 0 {
		if strings.ContainsAny(cleaned, "8901") || len(cleaned) == 40 || len(cleaned) == 32 {
			if b, err := hex.DecodeString(cleaned); err == nil && len(b) > 0 {
				cleaned = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
			}
		}
	}

	// RFC 4648 Base32 alphabet: A-Z, 2-7
	if pad := len(cleaned) % 8; pad != 0 {
		cleaned += strings.Repeat("=", 8-pad)
	}
	return cleaned
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// ValidateSecret checks if the given secret contains only valid RFC 4648 Base32 characters.
func ValidateSecret(secret string) bool {
	cleaned := CleanSecret(secret)
	if len(cleaned) == 0 {
		return false
	}
	_, err := base32.StdEncoding.DecodeString(cleaned)
	return err == nil
}

// GenerateCode computes the 6-digit TOTP code for the given secret at timestamp t.
func (e *Engine) GenerateCode(secret string, t time.Time) (string, error) {
	cleaned := CleanSecret(secret)
	key, err := base32.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return "", fmt.Errorf("invalid base32 secret: %w", err)
	}

	step := int64(e.StepSeconds)
	if step <= 0 {
		step = 30
	}
	counter := t.Unix() / step

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	hash := mac.Sum(nil)

	offset := hash[len(hash)-1] & 0x0f
	binaryCode := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff

	code := binaryCode % 1000000
	return fmt.Sprintf("%06d", code), nil
}

// VerifyCode verifies a code against the secret within a drift window (e.g. ±1 step).
func (e *Engine) VerifyCode(secret, code string, t time.Time, window int) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}

	step := time.Duration(e.StepSeconds) * time.Second
	for i := -window; i <= window; i++ {
		sampleTime := t.Add(time.Duration(i) * step)
		expected, err := e.GenerateCode(secret, sampleTime)
		if err == nil && expected == code {
			return true
		}
	}
	return false
}

// CountdownInfo contains countdown progress details for a given timestamp.
type CountdownInfo struct {
	RemainingSeconds int     `json:"remaining_seconds"`
	ProgressFraction float64 `json:"progress_fraction"`
}

// GetCountdown computes seconds remaining in the current step and the fractional progress.
func (e *Engine) GetCountdown(t time.Time) CountdownInfo {
	step := int64(e.StepSeconds)
	if step <= 0 {
		step = 30
	}
	elapsed := t.Unix() % step
	remaining := int(step - elapsed)
	if remaining == int(step) && elapsed == 0 {
		// at exact boundary
		remaining = int(step)
	}
	fraction := float64(remaining) / float64(step)
	return CountdownInfo{
		RemainingSeconds: remaining,
		ProgressFraction: fraction,
	}
}
