package templates

import (
	"encoding/json"
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

func TestStore_UpdateSidecar(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sidecars-update-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Deploy initial task
	req := DeployTaskRequest{
		TemplateID:     "daily-news-feed-digest",
		DisplayName:    "Original Feed Digest",
		CronExpression: "0 8 * * *",
		TargetProject:  "Antigravity Swiss Knife",
	}

	info, err := store.DeploySidecar(req)
	if err != nil {
		t.Fatalf("failed to deploy sidecar: %v", err)
	}

	// Update sidecar
	updateReq := UpdateSidecarRequest{
		ID:             info.ID,
		DisplayName:    "Updated Evening Digest",
		CronExpression: "0 18 * * 1-5",
		Prompt:         "Execute refined evening briefing with multiline\ninstructions and \"quotes\".",
	}

	updated, err := store.UpdateSidecar(updateReq)
	if err != nil {
		t.Fatalf("failed to update sidecar: %v", err)
	}

	if updated.DisplayName != "Updated Evening Digest" {
		t.Errorf("expected updated display name 'Updated Evening Digest', got %q", updated.DisplayName)
	}
	if updated.CronExpression != "0 18 * * 1-5" {
		t.Errorf("expected updated cron '0 18 * * 1-5', got %q", updated.CronExpression)
	}
	if updated.Prompt != "Execute refined evening briefing with multiline\ninstructions and \"quotes\"." {
		t.Errorf("expected updated prompt, got %q", updated.Prompt)
	}

	// Verify persistence via ListSidecars
	list, err := store.ListSidecars()
	if err != nil {
		t.Fatalf("failed to list sidecars: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 sidecar, got %d", len(list))
	}
	if list[0].DisplayName != "Updated Evening Digest" {
		t.Errorf("expected persisted display name 'Updated Evening Digest', got %q", list[0].DisplayName)
	}
	if list[0].Prompt != "Execute refined evening briefing with multiline\ninstructions and \"quotes\"." {
		t.Errorf("expected persisted prompt, got %q", list[0].Prompt)
	}
}

func TestSplitCronAndTimezone(t *testing.T) {
	tests := []struct {
		raw      string
		wantTZ   string
		wantCron string
	}{
		{"CRON_TZ=Australia/Melbourne 0 9 * * *", "Australia/Melbourne", "0 9 * * *"},
		{"  CRON_TZ=Europe/London 30 7 * * 1-5  ", "Europe/London", "30 7 * * 1-5"},
		{"0 9 * * *", "", "0 9 * * *"},
		{"CRON_TZ=UTC", "UTC", ""},
	}

	for _, tt := range tests {
		tz, cronExpr := splitCronAndTimezone(tt.raw)
		if tz != tt.wantTZ || cronExpr != tt.wantCron {
			t.Errorf("splitCronAndTimezone(%q) = (%q, %q), want (%q, %q)", tt.raw, tz, cronExpr, tt.wantTZ, tt.wantCron)
		}
	}
}

func TestFormatCronWithTimezone(t *testing.T) {
	tests := []struct {
		tz       string
		cronExpr string
		want     string
	}{
		{"Australia/Melbourne", "0 9 * * *", "CRON_TZ=Australia/Melbourne 0 9 * * *"},
		{"", "CRON_TZ=Europe/London 30 7 * * 1-5", "30 7 * * 1-5"},
		{"America/New_York", "CRON_TZ=Europe/London 30 7 * * 1-5", "CRON_TZ=America/New_York 30 7 * * 1-5"},
	}

	for _, tt := range tests {
		if got := formatCronWithTimezone(tt.tz, tt.cronExpr); got != tt.want {
			t.Errorf("formatCronWithTimezone(%q, %q) = %q, want %q", tt.tz, tt.cronExpr, got, tt.want)
		}
	}
}

func TestStore_TimeZoneDeployListAndUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	info, err := store.DeploySidecar(DeployTaskRequest{
		TemplateID:     "daily-news-feed-digest",
		DisplayName:    "Timezone Digest",
		CronExpression: "CRON_TZ=Europe/London 0 9 * * *",
		TimeZone:       "Australia/Melbourne",
	})
	if err != nil {
		t.Fatalf("failed to deploy sidecar: %v", err)
	}

	assertTimezoneInfo(t, info, "Australia/Melbourne", "0 9 * * *")
	assertStoredCron(t, filepath.Join(tmpDir, info.ID, "sidecar.json"), "CRON_TZ=Australia/Melbourne 0 9 * * *")

	list, err := store.ListSidecars()
	if err != nil {
		t.Fatalf("failed to list sidecars: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 sidecar, got %d", len(list))
	}
	assertTimezoneInfo(t, &list[0], "Australia/Melbourne", "0 9 * * *")

	updated, err := store.UpdateSidecar(UpdateSidecarRequest{
		ID:             info.ID,
		CronExpression: "0 18 * * 1-5",
	})
	if err != nil {
		t.Fatalf("failed to update sidecar: %v", err)
	}
	assertTimezoneInfo(t, updated, "Australia/Melbourne", "0 18 * * 1-5")
	assertStoredCron(t, filepath.Join(tmpDir, info.ID, "sidecar.json"), "CRON_TZ=Australia/Melbourne 0 18 * * 1-5")

	updated, err = store.UpdateSidecar(UpdateSidecarRequest{
		ID:             info.ID,
		CronExpression: "CRON_TZ=Europe/London 30 7 * * 1-5",
		TimeZone:       "America/New_York",
	})
	if err != nil {
		t.Fatalf("failed to update sidecar timezone: %v", err)
	}
	assertTimezoneInfo(t, updated, "America/New_York", "30 7 * * 1-5")
	assertStoredCron(t, filepath.Join(tmpDir, info.ID, "sidecar.json"), "CRON_TZ=America/New_York 30 7 * * 1-5")
}

func assertTimezoneInfo(t *testing.T, info *SidecarTaskInfo, wantTZ, wantCron string) {
	t.Helper()
	if info.TimeZone != wantTZ {
		t.Errorf("expected timezone %q, got %q", wantTZ, info.TimeZone)
	}
	if info.CronExpression != wantCron {
		t.Errorf("expected clean cron %q, got %q", wantCron, info.CronExpression)
	}
	wantText := CronToHuman(wantCron)
	if info.ScheduleText != wantText {
		t.Errorf("expected schedule text %q, got %q", wantText, info.ScheduleText)
	}
}

func assertStoredCron(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read sidecar payload: %v", err)
	}
	var payload SidecarPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to decode sidecar payload: %v", err)
	}
	if len(payload.Args) == 0 {
		t.Fatal("expected sidecar args to contain the cron expression")
	}
	if payload.Args[0] != want {
		t.Errorf("expected stored cron %q, got %q", want, payload.Args[0])
	}
}
