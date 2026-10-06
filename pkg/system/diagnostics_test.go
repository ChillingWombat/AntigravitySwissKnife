package system

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestSanitizeSensitiveText(t *testing.T) {
	raw := "User email is test.user@gmail.com with token Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xyz and api key sk-1234567890abcdef1234567890 and path /home/developer/secret.txt"
	sanitized, count := SanitizeSensitiveText(raw)

	if count < 4 {
		t.Errorf("expected at least 4 redacted items, got %d", count)
	}

	if strings.Contains(sanitized, "test.user@gmail.com") {
		t.Errorf("email was not sanitized: %s", sanitized)
	}
	if strings.Contains(sanitized, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xyz") {
		t.Errorf("bearer token was not sanitized: %s", sanitized)
	}
	if strings.Contains(sanitized, "sk-1234567890abcdef1234567890") {
		t.Errorf("openai key was not sanitized: %s", sanitized)
	}
	if strings.Contains(sanitized, "/home/developer") {
		t.Errorf("home directory path was not sanitized: %s", sanitized)
	}
}

func TestResolveAccountOrModel_FallbackHierarchy(t *testing.T) {
	tmpDir := t.TempDir()
	keyringStore, err := keyring.NewStore(filepath.Join(tmpDir, "accounts.json"))
	if err != nil {
		t.Fatalf("failed to create keyring store: %v", err)
	}
	customStore, err := custommodels.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create custom store: %v", err)
	}
	cfg := core.DefaultConfig()

	// 1. Initially no accounts and no models -> Built-in
	cfg.ActiveAccount = ""
	runner, src := ResolveAccountOrModel(cfg, keyringStore, customStore)
	if src != "builtin" {
		t.Errorf("expected builtin fallback, got %q (runner: %s)", src, runner)
	}

	// 2. Add an enabled custom model -> custom_model selected
	err = customStore.SaveModel(custommodels.CustomModel{
		ID:           "deepseek-chat",
		Name:         "deepseek-chat",
		DisplayName:  "DeepSeek V3",
		BaseURL:      "https://api.deepseek.com/v1",
		ProviderType: custommodels.ProviderCustom,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("failed to save custom model: %v", err)
	}
	runner, src = ResolveAccountOrModel(cfg, keyringStore, customStore)
	if src != "custom_model" {
		t.Errorf("expected custom_model fallback, got %q (runner: %s)", src, runner)
	}
	if !strings.Contains(runner, "DeepSeek V3") {
		t.Errorf("expected runner to mention DeepSeek V3, got %q", runner)
	}

	// 3. Add an enabled account to keyring -> enabled_account selected over custom model
	_, _ = keyringStore.ImportAccount("pilot@company.org", "token123", "", "Work Account", "")
	runner, src = ResolveAccountOrModel(cfg, keyringStore, customStore)
	if src != "enabled_account" {
		t.Errorf("expected enabled_account, got %q (runner: %s)", src, runner)
	}
	if runner != "pilot@company.org" {
		t.Errorf("expected runner to be pilot@company.org, got %q", runner)
	}

	// 4. Set ActiveAccount in config -> active_account selected
	cfg.ActiveAccount = "pilot@company.org"
	runner, src = ResolveAccountOrModel(cfg, keyringStore, customStore)
	if src != "active_account" {
		t.Errorf("expected active_account, got %q (runner: %s)", src, runner)
	}
}

func TestRunIssueDiagnosis_CompleteFlow(t *testing.T) {
	cfg := core.DefaultConfig()
	req := DiagnosticRequest{
		Description:       "Daemon socket disconnected when switching to custom models with api key sk-secret1234567890abcdef and user john@domain.com",
		IncludeSystemInfo: true,
		IncludeLogs:       true,
	}

	res, err := RunIssueDiagnosis(req, cfg, nil, nil)
	if err != nil {
		t.Fatalf("RunIssueDiagnosis failed: %v", err)
	}

	if !res.Success {
		t.Errorf("expected res.Success to be true")
	}
	if res.AgentSelected == "" {
		t.Errorf("expected non-empty agent selected")
	}
	if !res.SensitiveDataRedacted {
		t.Errorf("expected SensitiveDataRedacted to be true")
	}
	if res.RedactedTokenCount <= 0 {
		t.Errorf("expected RedactedTokenCount > 0, got %d", res.RedactedTokenCount)
	}
	if strings.Contains(res.SanitizedReport, "sk-secret1234567890abcdef") {
		t.Errorf("sanitized report contains unredacted API key")
	}
	if strings.Contains(res.SanitizedReport, "john@domain.com") {
		t.Errorf("sanitized report contains unredacted email")
	}
	if !strings.HasPrefix(res.IssueURL, "https://github.com/ChillingWombat/AntigravitySwissKnife/issues/new") {
		t.Errorf("unexpected issue URL: %s", res.IssueURL)
	}
}
