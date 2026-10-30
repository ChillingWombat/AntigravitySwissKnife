package daemon

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/backend"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

const activeStateCacheTTL = 15 * time.Second

type activeStateCache struct {
	mu         sync.Mutex
	data       *backend.ActiveStateSnapshot
	builtAt    time.Time
	generation uint64
	hits       uint64
	misses     uint64
	loading    chan struct{}
}

type daemonActiveState struct {
	daemon *Daemon
}

func (s daemonActiveState) Snapshot(force bool) backend.ActiveStateSnapshot {
	return s.daemon.activeState.snapshot(force, s.daemon.buildActiveStateSnapshot)
}

func (s daemonActiveState) Invalidate() {
	s.daemon.invalidateActiveState()
}

func (d *Daemon) invalidateActiveState() {
	d.activeState.invalidate()
}

func (c *activeStateCache) snapshot(force bool, build func() backend.ActiveStateSnapshot) backend.ActiveStateSnapshot {
	missCounted := false
	for {
		c.mu.Lock()
		if !force && c.data != nil && time.Since(c.builtAt) < activeStateCacheTTL {
			c.hits++
			out := cloneSnapshot(*c.data)
			out.Cache = c.statsLocked()
			c.mu.Unlock()
			return out
		}
		if !missCounted {
			c.misses++
			missCounted = true
		}
		if c.loading != nil {
			wait := c.loading
			c.mu.Unlock()
			<-wait
			force = false
			continue
		}
		loading := make(chan struct{})
		c.loading = loading
		generation := c.generation
		c.mu.Unlock()

		snapshot := build()

		c.mu.Lock()
		valid := generation == c.generation
		if valid {
			c.data = &snapshot
			c.builtAt = time.Now()
		}
		close(loading)
		c.loading = nil
		if !valid {
			c.mu.Unlock()
			force = false
			continue
		}
		out := cloneSnapshot(snapshot)
		out.Cache = c.statsLocked()
		c.mu.Unlock()
		return out
	}
}

func (c *activeStateCache) invalidate() {
	c.mu.Lock()
	c.generation++
	c.data = nil
	c.builtAt = time.Time{}
	c.mu.Unlock()
}

func (c *activeStateCache) statsLocked() backend.ActiveStateCacheStats {
	age := int64(0)
	if !c.builtAt.IsZero() {
		age = time.Since(c.builtAt).Milliseconds()
		if age < 0 {
			age = 0
		}
	}
	return backend.ActiveStateCacheStats{
		Generation: c.generation,
		AgeMS:      age,
		Hits:       c.hits,
		Misses:     c.misses,
	}
}

func cloneSnapshot(snapshot backend.ActiveStateSnapshot) backend.ActiveStateSnapshot {
	snapshot.ActiveEmails = append([]string(nil), snapshot.ActiveEmails...)
	snapshot.RunningEmails = append([]string(nil), snapshot.RunningEmails...)
	return snapshot
}

func (d *Daemon) buildActiveStateSnapshot() backend.ActiveStateSnapshot {
	procs := []processInfo{}
	if d.Shield != nil {
		found, _ := d.Shield.FindAntigravityUIProcesses()
		procs = make([]processInfo, len(found))
		for i, proc := range found {
			procs[i] = processInfo{PID: proc.PID}
		}
	}
	hostPID := 0
	if len(procs) > 0 {
		hostPID = procs[0].PID
	}
	if d.Shield != nil && d.Shield.ProtectedPID() > 0 {
		hostPID = d.Shield.ProtectedPID()
	}

	allEmails := make([]string, 0)
	for _, account := range d.Keyring.ListAccounts() {
		allEmails = append(allEmails, account.Email)
	}
	_, _ = d.Keyring.ReconcileActiveAccount(d.Config.AutoImportActiveAccount, allEmails, d.Profiles)
	active := d.Keyring.ActiveAccount()

	activeEmails := appendUnique(nil, active)
	for _, email := range d.Config.GetActiveAppAccounts() {
		activeEmails = appendUnique(activeEmails, email)
	}
	sort.Strings(activeEmails)

	runningEmails := make([]string, 0, 1)
	if running := runningEmail(keyring.ResolveRunningAntigravityAccountRedacted("", "")); running != "" {
		runningEmails = append(runningEmails, running)
	}

	return backend.ActiveStateSnapshot{
		AntigravityRunning: len(procs) > 0,
		AntigravityPID:     hostPID,
		ActiveAccount:      active,
		ActiveEmails:       activeEmails,
		RunningEmails:      runningEmails,
		TotalAccounts:      len(d.Keyring.ListAccounts()),
	}
}

type processInfo struct {
	PID int
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}

func runningEmail(value interface{}) string {
	raw, err := json.Marshal(value)
	if err != nil || string(raw) == "null" {
		return ""
	}
	var fields struct {
		Email          string `json:"email"`
		Account        string `json:"account"`
		ActiveAccount  string `json:"active_account"`
		RunningAccount string `json:"running_account"`
	}
	if json.Unmarshal(raw, &fields) != nil {
		return ""
	}
	for _, candidate := range []string{fields.Email, fields.Account, fields.ActiveAccount, fields.RunningAccount} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return ""
}

func (d *Daemon) relaunchHostIDE() error {
	err := d.Shield.RelaunchHostIDE()
	d.invalidateActiveState()
	return err
}

func (d *Daemon) setActiveAccount(email string) error {
	err := d.Keyring.SetActiveAccount(email)
	if err == nil {
		d.invalidateActiveState()
	}
	return err
}

func statusPayloadFromActiveState(snapshot backend.ActiveStateSnapshot) map[string]interface{} {
	var runningAccount interface{}
	if len(snapshot.RunningEmails) > 0 {
		runningAccount = map[string]string{"email": snapshot.RunningEmails[0]}
	}
	return map[string]interface{}{
		"daemon_running":              true,
		"daemon_pid":                  os.Getpid(),
		"pid":                         os.Getpid(),
		"version":                     core.AppVersion,
		"active_account":              snapshot.ActiveAccount,
		"active_emails":               snapshot.ActiveEmails,
		"running_emails":              snapshot.RunningEmails,
		"total_accounts":              snapshot.TotalAccounts,
		"antigravity_running":         snapshot.AntigravityRunning,
		"antigravity_pid":             snapshot.AntigravityPID,
		"running_antigravity_account": runningAccount,
		"status_cache":                snapshot.Cache,
	}
}
