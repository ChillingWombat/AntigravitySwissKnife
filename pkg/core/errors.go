package core

import "errors"

var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrInvalidSecret        = errors.New("invalid base32 TOTP secret")
	ErrDaemonUnavailable    = errors.New("daemon socket unavailable")
	ErrHostProtected        = errors.New("host IDE process is protected by safety shield")
	ErrActiveCascadeShield = errors.New("active cascade conversation cannot be pruned")
	ErrQuotaExhausted       = errors.New("model quota exhausted across all accounts")
	ErrProfileCorrupted     = errors.New("device fingerprint profile corrupted")
)
