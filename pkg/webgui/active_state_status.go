package webgui

import (
	"os"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/backend"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func activeStateStatusPayload(snapshot backend.ActiveStateSnapshot) map[string]interface{} {
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
