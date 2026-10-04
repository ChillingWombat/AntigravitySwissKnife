package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDefaultTemplates(t *testing.T) {
	tmpls := GetDefaultTemplates()
	if len(tmpls) == 0 {
		t.Fatalf("expected templates to not be empty")
	}

	foundDailyNews := false
	foundPortfolio := false
	foundCI := false

	for _, tmpl := range tmpls {
		if tmpl.ID == "daily-news-feed-digest" {
			foundDailyNews = true
		}
		if tmpl.ID == "portfolio-market-sentinel" {
			foundPortfolio = true
		}
		if tmpl.Category == "CI/CD & Development" {
			foundCI = true
		}
	}

	if !foundDailyNews {
		t.Errorf("expected daily-news-feed-digest template to be present")
	}
	if !foundPortfolio {
		t.Errorf("expected portfolio-market-sentinel template to be present")
	}
	if !foundCI {
		t.Errorf("expected CI/CD templates to be present")
	}
}

func TestStore_DeployAndListSidecar(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sidecars-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	req := DeployTaskRequest{
		TemplateID:     "daily-news-feed-digest",
		DisplayName:    "My Morning Feed Brief",
		CronExpression: "0 8 * * *",
		TargetProject:  "Antigravity Swiss Knife",
		Parameters: map[string]string{
			"sources": "https://news.ycombinator.com, Reuters Tech",
		},
	}

	info, err := store.DeploySidecar(req)
	if err != nil {
		t.Fatalf("failed to deploy sidecar: %v", err)
	}

	if info.DisplayName != "My Morning Feed Brief" {
		t.Errorf("expected display name %s, got %s", req.DisplayName, info.DisplayName)
	}
	if info.CronExpression != "0 8 * * *" {
		t.Errorf("expected cron 0 8 * * *, got %s", info.CronExpression)
	}

	// Verify file was written
	if _, err := os.Stat(filepath.Join(tmpDir, info.ID, "sidecar.json")); err != nil {
		t.Errorf("sidecar.json does not exist: %v", err)
	}

	// List sidecars
	list, err := store.ListSidecars()
	if err != nil {
		t.Fatalf("failed to list sidecars: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 sidecar, got %d", len(list))
	}
	if list[0].ID != info.ID {
		t.Errorf("expected sidecar ID %s, got %s", info.ID, list[0].ID)
	}

	// Delete sidecar
	if err := store.DeleteSidecar(info.ID); err != nil {
		t.Fatalf("failed to delete sidecar: %v", err)
	}

	listAfter, err := store.ListSidecars()
	if err != nil {
		t.Fatalf("failed to list sidecars after delete: %v", err)
	}
	if len(listAfter) != 0 {
		t.Errorf("expected 0 sidecars after delete, got %d", len(listAfter))
	}
}
