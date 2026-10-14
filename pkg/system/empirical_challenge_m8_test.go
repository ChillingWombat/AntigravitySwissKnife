package system

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChallenge_URLParseWithUserInfo(t *testing.T) {
	u, err := url.Parse("https://github.com:443@attacker.com/asset.exe")
	if err != nil {
		t.Fatalf("url.Parse error: %v", err)
	}
	t.Logf("URL parsed: Scheme=%s, Host=%s, Hostname=%s, User=%v", u.Scheme, u.Host, u.Hostname(), u.User)

	// Now check what IsTrustedReleaseHost receives in ExecuteUpgrade:
	// In ExecuteUpgrade (line 67):
	// parsedURL, _ := url.Parse(releaseInfo.DownloadURL)
	// IsTrustedReleaseHost(parsedURL.Host)
	t.Logf("parsedURL.Host is: %q", u.Host)
	t.Logf("IsTrustedReleaseHost(parsedURL.Host) = %v", IsTrustedReleaseHost(u.Host))
}

func TestChallenge_ReleaseDownload_PartialNetworkDropCleanup(t *testing.T) {
	// Create mock server that abruptly drops connection midway
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		// Flusher
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		// Write initial 1024 bytes
		_, _ = w.Write([]byte(string(make([]byte, 1024))))
		flusher.Flush()

		// Hijack connection and abruptly close it to simulate network cut
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
		}
	}))
	defer mockServer.Close()

	mgr := NewAppReleaseManager()
	testDir := t.TempDir()

	assetName := "test-partial-download.exe"
	expectedDest := filepath.Join(testDir, assetName)

	// Custom home dir for test
	t.Setenv("USERPROFILE", testDir)
	t.Setenv("HOME", testDir)
	downloadsDir := filepath.Join(testDir, "Downloads")
	_ = os.MkdirAll(downloadsDir, 0755)
	destPath := filepath.Join(downloadsDir, assetName)

	releaseInfo := &AppReleaseInfo{
		HasUpdate:     true,
		LatestVersion: "9.9.9",
		DownloadURL:   mockServer.URL,
		AssetName:     assetName,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := mgr.ExecuteUpgrade(ctx, releaseInfo)
	t.Logf("ExecuteUpgrade result: res=%v, err=%v", res, err)

	if err == nil {
		t.Fatalf("expected error on severed network download, but got nil")
	}

	// Verify that partial file was cleaned up and does NOT remain on disk
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Errorf("SECURITY/INTEGRITY BUG: Partial corrupted download file was not deleted: %s still exists!", destPath)
	} else {
		t.Logf("Verified: Partial file %s was cleanly removed after network drop", destPath)
	}
	_ = expectedDest
}

func TestChallenge_ReleaseDownload_ContextCancelledCleanup(t *testing.T) {
	// Mock server that hangs
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(2 * time.Second)
	}))
	defer mockServer.Close()

	mgr := NewAppReleaseManager()
	testDir := t.TempDir()
	t.Setenv("USERPROFILE", testDir)
	t.Setenv("HOME", testDir)
	downloadsDir := filepath.Join(testDir, "Downloads")
	_ = os.MkdirAll(downloadsDir, 0755)

	assetName := "test-timeout-download.exe"
	destPath := filepath.Join(downloadsDir, assetName)

	releaseInfo := &AppReleaseInfo{
		HasUpdate:     true,
		LatestVersion: "9.9.9",
		DownloadURL:   mockServer.URL,
		AssetName:     assetName,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := mgr.ExecuteUpgrade(ctx, releaseInfo)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}

	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Errorf("SECURITY BUG: Partial file was not deleted on context timeout: %s", destPath)
	} else {
		t.Logf("Verified: Partial file %s was removed on context timeout", destPath)
	}
}

func TestChallenge_ReleaseDownload_CorruptChecksumCleanup(t *testing.T) {
	payload := []byte("legitimate looking binary payload")
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer mockServer.Close()

	mgr := NewAppReleaseManager()
	testDir := t.TempDir()
	t.Setenv("USERPROFILE", testDir)
	t.Setenv("HOME", testDir)
	downloadsDir := filepath.Join(testDir, "Downloads")
	_ = os.MkdirAll(downloadsDir, 0755)

	assetName := "test-tampered-download.exe"
	destPath := filepath.Join(downloadsDir, assetName)

	// Provide an incorrect ExpectedSHA256
	badHash := "0000000000000000000000000000000000000000000000000000000000000000"

	releaseInfo := &AppReleaseInfo{
		HasUpdate:      true,
		LatestVersion:  "9.9.9",
		DownloadURL:    mockServer.URL,
		AssetName:      assetName,
		ExpectedSHA256: badHash,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := mgr.ExecuteUpgrade(ctx, releaseInfo)
	if err == nil || (res != nil && res.Success) {
		t.Fatalf("expected integrity violation error on bad hash, got res=%v, err=%v", res, err)
	}

	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Errorf("CRITICAL INTEGRITY VIOLATION: Tampered file was NOT removed: %s", destPath)
	} else {
		t.Logf("Verified: Tampered file with bad SHA-256 was cleanly deleted: %s", destPath)
	}
}

func TestChallenge_IsTrustedReleaseHost_AdversarialDomains(t *testing.T) {
	blockedDomains := []string{
		"evilgithub.com",
		"not-github.com",
		"attacker-github.com",
		"github.com.attacker.com",
		"objects.githubusercontent.com.evil.com",
		"raw.githubusercontent.com.attacker.com",
		"evil-github-releases.githubusercontent.com.attacker.org",
		"http://github.com",
		"",
		" ",
		"localhost",
		"127.0.0.1",
		"169.254.169.254",
	}

	for _, d := range blockedDomains {
		if IsTrustedReleaseHost(d) {
			t.Errorf("CRITICAL SECURITY FLAW: Untrusted domain %q was accepted as trusted release host!", d)
		}
	}

	trustedDomains := []string{
		"github.com",
		"api.github.com",
		"objects.githubusercontent.com",
		"raw.githubusercontent.com",
		"github-releases.githubusercontent.com",
		"release-assets.github.com",
		"sub.github.com",
		"sub.objects.githubusercontent.com",
		"github.com:443",
		"api.github.com:8443",
	}

	for _, d := range trustedDomains {
		if !IsTrustedReleaseHost(d) {
			t.Errorf("Legitimate domain %q was falsely rejected by IsTrustedReleaseHost", d)
		}
	}
}
