package webgui

import (
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/backend"
)

func TestActiveStateStatusPayloadSnapshotCache(t *testing.T) {
	payload := activeStateStatusPayload(backend.ActiveStateSnapshot{
		AntigravityRunning: true,
		AntigravityPID:     77,
		ActiveAccount:      "active@example.com",
		ActiveEmails:       []string{"active@example.com"},
		RunningEmails:      []string{"running@example.com"},
		TotalAccounts:      2,
		Cache:              backend.ActiveStateCacheStats{Generation: 3, AgeMS: 12, Hits: 4, Misses: 1},
	})
	if payload["active_account"] != "active@example.com" || payload["antigravity_pid"] != 77 {
		t.Fatalf("unexpected status payload: %#v", payload)
	}
	cache, ok := payload["status_cache"].(backend.ActiveStateCacheStats)
	if !ok || cache.Generation != 3 || cache.Hits != 4 || cache.Misses != 1 {
		t.Fatalf("missing cache diagnostics: %#v", payload["status_cache"])
	}
}

func TestLoadStatusUsesActiveStateSnapshot(t *testing.T) {
	server := &Server{backend: testBackend{activeState: testActiveStateService{snapshot: backend.ActiveStateSnapshot{
		AntigravityRunning: true,
		AntigravityPID:     91,
		ActiveAccount:      "snapshot@example.com",
		TotalAccounts:      4,
		Cache:              backend.ActiveStateCacheStats{Generation: 2, AgeMS: 9, Hits: 3, Misses: 1},
	}}}}
	payload, err := server.loadStatus()
	if err != nil {
		t.Fatalf("loadStatus: %v", err)
	}
	if payload["active_account"] != "snapshot@example.com" || payload["antigravity_pid"] != 91 || payload["total_accounts"] != 4 {
		t.Fatalf("status did not consume snapshot: %#v", payload)
	}
}
