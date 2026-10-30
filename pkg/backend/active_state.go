package backend

// ActiveStateService owns the cached active/running account projection.
type ActiveStateService interface {
	Snapshot(force bool) ActiveStateSnapshot
	Invalidate()
}

type ActiveStateCacheStats struct {
	Generation uint64 `json:"generation"`
	AgeMS      int64  `json:"age_ms"`
	Hits       uint64 `json:"hits"`
	Misses     uint64 `json:"misses"`
}

// ActiveStateSnapshot deliberately contains only account identities and counts.
type ActiveStateSnapshot struct {
	AntigravityRunning bool                  `json:"antigravity_running"`
	AntigravityPID     int                   `json:"antigravity_pid"`
	ActiveAccount      string                `json:"active_account"`
	ActiveEmails       []string              `json:"active_emails"`
	RunningEmails      []string              `json:"running_emails"`
	TotalAccounts      int                   `json:"total_accounts"`
	Cache              ActiveStateCacheStats `json:"cache"`
}
