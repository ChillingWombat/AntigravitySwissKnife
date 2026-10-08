package system

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func TestAppReleaseManager_DefaultInfo(t *testing.T) {
	mgr := NewAppReleaseManager()
	cfg := core.DefaultConfig()

	info := mgr.GetCachedReleaseInfo(cfg)
	if info.CurrentVersion != core.AppVersion {
		t.Errorf("expected current version %s, got %s", core.AppVersion, info.CurrentVersion)
	}
	if info.HasUpdate {
		t.Errorf("default cached info should not claim has_update is true")
	}
	if !info.AutoCheck {
		t.Errorf("default auto check should be true")
	}
	if info.AutoUpgrade {
		t.Errorf("default auto upgrade should be false")
	}
}

func TestAppReleaseManager_404OrEmptyReleases(t *testing.T) {
	// Test 404 handler (no releases published)
	server404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server404.Close()

	mgr := NewAppReleaseManager(server404.URL)
	cfg := core.DefaultConfig()

	info, err := mgr.CheckForUpdates(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error on 404: %v", err)
	}
	if info.HasUpdate {
		t.Errorf("expected has_update false on 404")
	}
	if info.CurrentVersion != core.AppVersion {
		t.Errorf("expected current version %s, got %s", core.AppVersion, info.CurrentVersion)
	}

	// Test empty list `[]`
	serverEmpty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer serverEmpty.Close()

	mgrEmpty := NewAppReleaseManager(serverEmpty.URL)
	infoEmpty, err := mgrEmpty.CheckForUpdates(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error on empty array: %v", err)
	}
	if infoEmpty.HasUpdate {
		t.Errorf("expected has_update false on empty array")
	}
}

func TestAppReleaseManager_OlderRelease(t *testing.T) {
	// Remote release is older than current version (e.g. 1.9.0 vs 2.0.0)
	olderRelease := []GitHubRelease{
		{
			TagName:     "v1.9.0",
			Name:        "v1.9.0 Maintenance",
			Body:        "Older release notes",
			Draft:       false,
			PublishedAt: time.Now().Add(-24 * time.Hour),
			HTMLURL:     "https://github.com/ChillingWombat/AntigravitySwissKnife/releases/tag/v1.9.0",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(olderRelease)
	}))
	defer server.Close()

	mgr := NewAppReleaseManager(server.URL)
	cfg := core.DefaultConfig()

	info, err := mgr.CheckForUpdates(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.HasUpdate {
		t.Errorf("expected has_update false when remote is 1.9.0 and local is 2.0.0")
	}
	if info.LatestVersion != core.AppVersion {
		t.Errorf("expected latest version to match current version %s, got %s", core.AppVersion, info.LatestVersion)
	}
}

func TestAppReleaseManager_NewerReleaseWithAssets(t *testing.T) {
	newerReleases := []GitHubRelease{
		{
			TagName:     "v2.1.0",
			Name:        "Antigravity Swiss Knife v2.1.0",
			Body:        "## What's Changed\n* Added real-time release updater\n* Performance fixes",
			Draft:       false,
			PublishedAt: time.Now(),
			HTMLURL:     "https://github.com/ChillingWombat/AntigravitySwissKnife/releases/tag/v2.1.0",
			Assets: []GitHubReleaseAsset{
				{
					ID:                 101,
					Name:               "Antigravity-Swiss-Knife-2.1.0-x86_64.AppImage",
					Size:               124500000,
					BrowserDownloadURL: "https://github.com/releases/download/v2.1.0/Antigravity-Swiss-Knife-2.1.0-x86_64.AppImage",
				},
				{
					ID:                 102,
					Name:               "Antigravity-Swiss-Knife-2.1.0-Setup.exe",
					Size:               98500000,
					BrowserDownloadURL: "https://github.com/releases/download/v2.1.0/Antigravity-Swiss-Knife-2.1.0-Setup.exe",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(newerReleases)
	}))
	defer server.Close()

	mgr := NewAppReleaseManager(server.URL)
	cfg := core.DefaultConfig()

	info, err := mgr.CheckForUpdates(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.HasUpdate {
		t.Errorf("expected has_update true when remote is v2.1.0 and local is 2.0.0")
	}
	if info.LatestVersion != "2.1.0" {
		t.Errorf("expected latest version 2.1.0, got %s", info.LatestVersion)
	}
	if info.AssetName == "" {
		t.Errorf("expected asset name to be populated")
	}
	if info.DownloadURL == "" {
		t.Errorf("expected download URL to be populated")
	}
}

func TestAppReleaseManager_FindBestAsset(t *testing.T) {
	mgr := NewAppReleaseManager()
	assets := []GitHubReleaseAsset{
		{Name: "Antigravity-Swiss-Knife-2.1.0-Setup.exe", BrowserDownloadURL: "http://example.com/app.exe"},
		{Name: "Antigravity-Swiss-Knife-2.1.0-x86_64.AppImage", BrowserDownloadURL: "http://example.com/app.AppImage"},
		{Name: "Antigravity-Swiss-Knife-2.1.0-arm64.dmg", BrowserDownloadURL: "http://example.com/app.dmg"},
	}

	// Linux amd64
	bestLinux := mgr.findBestAsset(assets, "linux", "amd64")
	if bestLinux == nil || bestLinux.Name != "Antigravity-Swiss-Knife-2.1.0-x86_64.AppImage" {
		t.Errorf("expected Linux AppImage, got %v", bestLinux)
	}

	// Windows
	bestWin := mgr.findBestAsset(assets, "windows", "amd64")
	if bestWin == nil || bestWin.Name != "Antigravity-Swiss-Knife-2.1.0-Setup.exe" {
		t.Errorf("expected Windows exe, got %v", bestWin)
	}

	// Darwin
	bestMac := mgr.findBestAsset(assets, "darwin", "arm64")
	if bestMac == nil || bestMac.Name != "Antigravity-Swiss-Knife-2.1.0-arm64.dmg" {
		t.Errorf("expected Mac dmg, got %v", bestMac)
	}
}

func TestAppReleaseManager_ExecuteUpgrade(t *testing.T) {
	mockPayload := "dummy-binary-payload-data"
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte(mockPayload))
	}))
	defer downloadServer.Close()

	mgr := NewAppReleaseManager()

	// 1. Upgrade when already up-to-date
	upToDateInfo := &AppReleaseInfo{
		CurrentVersion: "2.0.0",
		LatestVersion:  "2.0.0",
		HasUpdate:      false,
	}
	res, err := mgr.ExecuteUpgrade(context.Background(), upToDateInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success true when already up-to-date")
	}

	// 2. Upgrade when update available
	tempFile := filepath.Join(t.TempDir(), "test-upgrade-asset.bin")
	updateInfo := &AppReleaseInfo{
		CurrentVersion: "2.0.0",
		LatestVersion:  "2.1.0",
		HasUpdate:      true,
		DownloadURL:    downloadServer.URL,
		AssetName:      filepath.Base(tempFile),
		Platform:       "linux",
		Arch:           "amd64",
	}

	resUpgrade, err := mgr.ExecuteUpgrade(context.Background(), updateInfo)
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}
	if !resUpgrade.Success {
		t.Errorf("expected success true on valid download")
	}
	if resUpgrade.LocalPath != "" {
		defer os.Remove(resUpgrade.LocalPath)
		data, err := os.ReadFile(resUpgrade.LocalPath)
		if err != nil {
			t.Fatalf("failed to read downloaded file: %v", err)
		}
		if string(data) != mockPayload {
			t.Errorf("downloaded content mismatch, got %q", string(data))
		}
	}
}
