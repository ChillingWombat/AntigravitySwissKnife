package keyring

import "time"

// Account represents an authenticated user profile in the keyring.
type Account struct {
	Email        string    `json:"email"`
	Label        string    `json:"label,omitempty"`
	PlanTier     string    `json:"plan_tier,omitempty"`
	Status       string    `json:"status,omitempty"`
	TOTPSecret   string    `json:"totp_secret,omitempty"`
	HasTOTP      bool      `json:"has_totp"`
	IsActive     bool      `json:"is_active"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenExpiry  time.Time `json:"token_expiry,omitempty"`
}

// SessionState captures snapshot information to restore across rotations.
type SessionState struct {
	ActiveAccount string                 `json:"active_account"`
	LastSwitched  time.Time              `json:"last_switched"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
