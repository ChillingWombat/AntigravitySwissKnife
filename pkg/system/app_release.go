package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

const (
	DefaultGitHubRepoOwner   = "ChillingWombat"
	DefaultGitHubRepoName    = "AntigravitySwissKnife"
	DefaultGitHubReleasesURL = "https://api.github.com/repos/ChillingWombat/AntigravitySwissKnife/releases"
)

// GitHubReleaseAsset represents a binary or archive attached to a GitHub release.
type GitHubReleaseAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}

// GitHubRelease represents a release object returned by GitHub REST API.
type GitHubRelease struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	Body        string               `json:"body"`
	Draft       bool                 `json:"draft"`
	Prerelease  bool                 `json:"prerelease"`
	PublishedAt time.Time            `json:"published_at"`
	HTMLURL     string               `json:"html_url"`
	Assets      []GitHubReleaseAsset `json:"assets"`
}

// AppReleaseInfo represents the live release status and diagnostics for Antigravity Swiss Knife.
type AppReleaseInfo struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	ReleaseName    string `json:"release_name"`
	ReleaseNotes   string `json:"release_notes"`
	PublishedAt    string `json:"published_at"`
	HTMLURL        string `json:"html_url"`
	DownloadURL    string `json:"download_url"`
	AssetName      string `json:"asset_name,omitempty"`
	AssetSize      int64  `json:"asset_size,omitempty"`
	Platform       string `json:"platform"`
	Arch           string `json:"arch"`
	AutoCheck      bool   `json:"auto_check"`
	AutoUpgrade    bool   `json:"auto_upgrade"`
	LastChecked    string `json:"last_checked"`
	StatusMessage  string `json:"status_message"`
}

// UpgradeResult represents the outcome of an automated or manual upgrade request.
type UpgradeResult struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	DownloadURL   string `json:"download_url,omitempty"`
	LocalPath     string `json:"local_path,omitempty"`
	TargetVersion string `json:"target_version,omitempty"`
}

// AppReleaseManager manages version inspection, GitHub release checks, and upgrades.
type AppReleaseManager struct {
	mu         sync.RWMutex
	httpClient *http.Client
	apiURL     string
	lastInfo   *AppReleaseInfo
}

// NewAppReleaseManager creates a new manager for app releases.
func NewAppReleaseManager(customURL ...string) *AppReleaseManager {
	url := DefaultGitHubReleasesURL
	if len(customURL) > 0 && customURL[0] != "" {
		url = customURL[0]
	}
	return &AppReleaseManager{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		apiURL: url,
	}
}

// GetCachedReleaseInfo returns the cached or baseline release diagnostics.
func (m *AppReleaseManager) GetCachedReleaseInfo(cfg *core.Config) *AppReleaseInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	autoCheck := true
	autoUpgrade := false
	lastChecked := ""
	if cfg != nil {
		autoCheck = cfg.AutoCheckUpdates
		autoUpgrade = cfg.AutoUpgrade
		lastChecked = cfg.LastUpdateCheckTime
	}

	if m.lastInfo != nil {
		copyInfo := *m.lastInfo
		copyInfo.AutoCheck = autoCheck
		copyInfo.AutoUpgrade = autoUpgrade
		if lastChecked != "" {
			copyInfo.LastChecked = lastChecked
		}
		return &copyInfo
	}

	return &AppReleaseInfo{
		CurrentVersion: "N/A",
		LatestVersion:  "N/A",
		HasUpdate:      false,
		ReleaseName:    "",
		ReleaseNotes:   "Pre-release build. No public releases published yet.",
		PublishedAt:    "",
		HTMLURL:        fmt.Sprintf("https://github.com/%s/%s/releases", DefaultGitHubRepoOwner, DefaultGitHubRepoName),
		DownloadURL:    "",
		Platform:       runtime.GOOS,
		Arch:           runtime.GOARCH,
		AutoCheck:      autoCheck,
		AutoUpgrade:    autoUpgrade,
		LastChecked:    lastChecked,
		StatusMessage:  "No published releases yet.",
	}
}

// CheckForUpdates queries GitHub releases and determines if a newer release exists.
func (m *AppReleaseManager) CheckForUpdates(ctx context.Context, cfg *core.Config) (*AppReleaseInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentVer := core.AppVersion
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	autoCheck := true
	autoUpgrade := false
	if cfg != nil {
		autoCheck = cfg.AutoCheckUpdates
		autoUpgrade = cfg.AutoUpgrade
	}

	nowStr := time.Now().Format("2006-01-02 15:04:05")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build update check request: %w", err)
	}
	req.Header.Set("User-Agent", fmt.Sprintf("AntigravitySwissKnife/%s (%s; %s)", currentVer, goos, goarch))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		// Return graceful offline/fallback state without crashing
		info := &AppReleaseInfo{
			CurrentVersion: currentVer,
			LatestVersion:  currentVer,
			HasUpdate:      false,
			ReleaseName:    "v" + currentVer,
			ReleaseNotes:   "Could not connect to release server. Offline or network unavailable.",
			HTMLURL:        fmt.Sprintf("https://github.com/%s/%s/releases", DefaultGitHubRepoOwner, DefaultGitHubRepoName),
			Platform:       goos,
			Arch:           goarch,
			AutoCheck:      autoCheck,
			AutoUpgrade:    autoUpgrade,
			LastChecked:    nowStr,
			StatusMessage:  "Unable to reach update server. Running version " + currentVer,
		}
		m.lastInfo = info
		if cfg != nil {
			cfg.LastUpdateCheckTime = nowStr
			_ = cfg.Save()
		}
		return info, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Repository has no releases published yet
		info := &AppReleaseInfo{
			CurrentVersion: "N/A",
			LatestVersion:  "N/A",
			HasUpdate:      false,
			ReleaseName:    "",
			ReleaseNotes:   "No remote releases found on repository yet.",
			HTMLURL:        fmt.Sprintf("https://github.com/%s/%s/releases", DefaultGitHubRepoOwner, DefaultGitHubRepoName),
			Platform:       goos,
			Arch:           goarch,
			AutoCheck:      autoCheck,
			AutoUpgrade:    autoUpgrade,
			LastChecked:    nowStr,
			StatusMessage:  "No published releases yet.",
		}
		m.lastInfo = info
		if cfg != nil {
			cfg.LastUpdateCheckTime = nowStr
			_ = cfg.Save()
		}
		return info, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release check HTTP status %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var releases []GitHubRelease
	// Attempt array decoding (GET /releases), fallback to single object (GET /releases/latest)
	if err := json.Unmarshal(body, &releases); err != nil {
		var singleRelease GitHubRelease
		if err2 := json.Unmarshal(body, &singleRelease); err2 == nil && singleRelease.TagName != "" {
			releases = []GitHubRelease{singleRelease}
		} else {
			return nil, fmt.Errorf("failed to decode release payload: %w", err)
		}
	}

	if len(releases) == 0 {
		info := &AppReleaseInfo{
			CurrentVersion: "N/A",
			LatestVersion:  "N/A",
			HasUpdate:      false,
			ReleaseName:    "",
			ReleaseNotes:   "No remote releases found on repository yet.",
			HTMLURL:        fmt.Sprintf("https://github.com/%s/%s/releases", DefaultGitHubRepoOwner, DefaultGitHubRepoName),
			Platform:       goos,
			Arch:           goarch,
			AutoCheck:      autoCheck,
			AutoUpgrade:    autoUpgrade,
			LastChecked:    nowStr,
			StatusMessage:  "No published releases yet.",
		}
		m.lastInfo = info
		if cfg != nil {
			cfg.LastUpdateCheckTime = nowStr
			_ = cfg.Save()
		}
		return info, nil
	}

	// Find the latest valid release
	var targetRelease *GitHubRelease
	for i := range releases {
		r := &releases[i]
		if !r.Draft {
			targetRelease = r
			break
		}
	}
	if targetRelease == nil {
		targetRelease = &releases[0]
	}

	cleanLatestTag := strings.TrimPrefix(targetRelease.TagName, "v")
	hasUpdate := compareVersions(cleanLatestTag, currentVer) > 0
	effectiveLatestVer := cleanLatestTag
	if compareVersions(currentVer, effectiveLatestVer) > 0 {
		effectiveLatestVer = currentVer
	}

	bestAsset := m.findBestAsset(targetRelease.Assets, goos, goarch)
	downloadURL := targetRelease.HTMLURL
	var assetName string
	var assetSize int64

	if bestAsset != nil {
		downloadURL = bestAsset.BrowserDownloadURL
		assetName = bestAsset.Name
		assetSize = bestAsset.Size
	}

	statusMsg := "Application is up to date."
	if hasUpdate {
		statusMsg = fmt.Sprintf("A newer release (%s) is available!", targetRelease.TagName)
	}

	pubDate := ""
	if !targetRelease.PublishedAt.IsZero() {
		pubDate = targetRelease.PublishedAt.Format("2006-01-02 15:04:05")
	}

	info := &AppReleaseInfo{
		CurrentVersion: currentVer,
		LatestVersion:  effectiveLatestVer,
		HasUpdate:      hasUpdate,
		ReleaseName:    targetRelease.Name,
		ReleaseNotes:   targetRelease.Body,
		PublishedAt:    pubDate,
		HTMLURL:        targetRelease.HTMLURL,
		DownloadURL:    downloadURL,
		AssetName:      assetName,
		AssetSize:      assetSize,
		Platform:       goos,
		Arch:           goarch,
		AutoCheck:      autoCheck,
		AutoUpgrade:    autoUpgrade,
		LastChecked:    nowStr,
		StatusMessage:  statusMsg,
	}

	m.lastInfo = info

	if cfg != nil {
		cfg.LastUpdateCheckTime = nowStr
		_ = cfg.Save()
	}

	return info, nil
}

// findBestAsset picks the most appropriate asset based on OS and architecture.
func (m *AppReleaseManager) findBestAsset(assets []GitHubReleaseAsset, goos, goarch string) *GitHubReleaseAsset {
	if len(assets) == 0 {
		return nil
	}

	goosLower := strings.ToLower(goos)
	goarchLower := strings.ToLower(goarch)

	// Preference scoring for platform matches
	type scoredAsset struct {
		asset *GitHubReleaseAsset
		score int
	}

	var candidates []scoredAsset

	for i := range assets {
		a := &assets[i]
		name := strings.ToLower(a.Name)
		score := 0

		switch goosLower {
		case "linux":
			if strings.HasSuffix(name, ".appimage") {
				score += 50
			} else if strings.HasSuffix(name, ".deb") {
				score += 30
			} else if strings.HasSuffix(name, ".tar.gz") || strings.Contains(name, "linux") {
				score += 20
			}
			if (goarchLower == "amd64" && (strings.Contains(name, "x86_64") || strings.Contains(name, "amd64"))) ||
				(goarchLower == "arm64" && strings.Contains(name, "arm64")) {
				score += 20
			}
		case "windows":
			if strings.HasSuffix(name, ".exe") {
				score += 50
			} else if strings.HasSuffix(name, ".zip") && strings.Contains(name, "win") {
				score += 30
			}
		case "darwin":
			if strings.HasSuffix(name, ".dmg") {
				score += 50
			} else if strings.HasSuffix(name, ".zip") && strings.Contains(name, "mac") {
				score += 30
			}
		}

		if score > 0 {
			candidates = append(candidates, scoredAsset{asset: a, score: score})
		}
	}

	if len(candidates) > 0 {
		best := candidates[0]
		for _, c := range candidates[1:] {
			if c.score > best.score {
				best = c
			}
		}
		return best.asset
	}

	// Fallback to first asset if no scoring match
	return &assets[0]
}

// ExecuteUpgrade handles downloading the newer release asset into a safe location.
func (m *AppReleaseManager) ExecuteUpgrade(ctx context.Context, releaseInfo *AppReleaseInfo) (*UpgradeResult, error) {
	if releaseInfo == nil {
		return &UpgradeResult{
			Success: false,
			Message: "No release diagnostic provided for upgrade.",
		}, nil
	}

	if !releaseInfo.HasUpdate {
		return &UpgradeResult{
			Success:       true,
			Message:       fmt.Sprintf("Already running the latest version (%s).", releaseInfo.CurrentVersion),
			TargetVersion: releaseInfo.CurrentVersion,
		}, nil
	}

	if releaseInfo.DownloadURL == "" {
		return &UpgradeResult{
			Success:       false,
			Message:       "No release download asset URL available. Please visit the GitHub release page.",
			DownloadURL:   releaseInfo.HTMLURL,
			TargetVersion: releaseInfo.LatestVersion,
		}, nil
	}

	// Prepare target downloads directory
	homeDir, _ := os.UserHomeDir()
	targetDir := filepath.Join(homeDir, "Downloads")
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		targetDir = filepath.Join(os.TempDir(), "antigravity-swiss-updates")
		_ = os.MkdirAll(targetDir, 0755)
	}

	assetName := releaseInfo.AssetName
	if assetName == "" {
		assetName = fmt.Sprintf("antigravity-swiss-%s-%s-%s", releaseInfo.LatestVersion, releaseInfo.Platform, releaseInfo.Arch)
		if releaseInfo.Platform == "linux" {
			assetName += ".AppImage"
		} else if releaseInfo.Platform == "windows" {
			assetName += ".exe"
		}
	}

	destPath := filepath.Join(targetDir, assetName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseInfo.DownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build download request: %w", err)
	}
	req.Header.Set("User-Agent", "AntigravitySwissKnife-Updater/"+core.AppVersion)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download release asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &UpgradeResult{
			Success:       false,
			Message:       fmt.Sprintf("Download server returned HTTP %d. Please download manually.", resp.StatusCode),
			DownloadURL:   releaseInfo.DownloadURL,
			TargetVersion: releaseInfo.LatestVersion,
		}, nil
	}

	out, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to write asset contents: %w", err)
	}

	// Make executable if on Unix
	if runtime.GOOS != "windows" {
		_ = os.Chmod(destPath, 0755)
	}

	return &UpgradeResult{
		Success:       true,
		Message:       fmt.Sprintf("Successfully downloaded Antigravity Swiss Knife v%s to %s", releaseInfo.LatestVersion, destPath),
		DownloadURL:   releaseInfo.DownloadURL,
		LocalPath:     destPath,
		TargetVersion: releaseInfo.LatestVersion,
	}, nil
}
