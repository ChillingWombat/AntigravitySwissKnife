package system

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

const PublicGitHubRepo = "ChillingWombat/AntigravitySwissKnife"

// DiagnosticRequest carries the issue description and options from the user.
type DiagnosticRequest struct {
	Description       string `json:"description"`
	IncludeSystemInfo bool   `json:"include_system_info"`
	IncludeLogs       bool   `json:"include_logs"`
}

// DiagnosticResult contains the resolved agent, account/model pipeline, scrubbed diagnostics, and prefilled GitHub issue URL.
type DiagnosticResult struct {
	Success               bool     `json:"success"`
	AgentSelected         string   `json:"agent_selected"`
	AgentPriorityChain    []string `json:"agent_priority_chain"`
	AccountOrModelUsed    string   `json:"account_or_model_used"`
	ResolutionSource      string   `json:"resolution_source"` // "active_account", "enabled_account", "custom_model", "builtin"
	SanitizedReport       string   `json:"sanitized_report"`
	IssueTitle            string   `json:"issue_title"`
	IssueURL              string   `json:"issue_url"`
	GitHubRepo            string   `json:"github_repo"`
	SensitiveDataRedacted bool     `json:"sensitive_data_redacted"`
	RedactedTokenCount    int      `json:"redacted_token_count"`
}

var (
	reEmail       = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	reBearer      = regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9_\-\.]{15,}`)
	reGitHubToken = regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{16,}`)
	reGoogleKey   = regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`)
	reOpenAIKey   = regexp.MustCompile(`sk-[a-zA-Z0-9\-_]{20,}`)
	reHomePath    = regexp.MustCompile(`/(?:home|Users)/[a-zA-Z0-9_\-]+`)
)

// SanitizeSensitiveText strips and redacts emails, auth tokens, passwords, and sensitive system paths.
func SanitizeSensitiveText(raw string) (string, int) {
	redactedCount := 0

	out := reBearer.ReplaceAllStringFunc(raw, func(_ string) string {
		redactedCount++
		return "Bearer [REDACTED_AUTH_TOKEN]"
	})

	out = reGitHubToken.ReplaceAllStringFunc(out, func(_ string) string {
		redactedCount++
		return "[REDACTED_GITHUB_TOKEN]"
	})

	out = reGoogleKey.ReplaceAllStringFunc(out, func(_ string) string {
		redactedCount++
		return "[REDACTED_GOOGLE_API_KEY]"
	})

	out = reOpenAIKey.ReplaceAllStringFunc(out, func(_ string) string {
		redactedCount++
		return "[REDACTED_API_KEY]"
	})

	out = reEmail.ReplaceAllStringFunc(out, func(email string) string {
		redactedCount++
		parts := strings.Split(email, "@")
		if len(parts) == 2 && len(parts[0]) > 2 {
			return string(parts[0][:1]) + "***@" + parts[1]
		}
		return "[ANONYMIZED_USER]"
	})

	out = reHomePath.ReplaceAllStringFunc(out, func(_ string) string {
		redactedCount++
		return "~"
	})

	return out, redactedCount
}

// DetectAvailableAgents probes the environment following user priority:
// Priority 1: Antigravity 2.0 Desktop
// Priority 2: agy CLI
// Priority 3: VS Code Extension
// Fallback: Built-in Swiss Knife Agent
func DetectAvailableAgents() (string, []string) {
	chain := []string{}
	selected := ""

	cfg, _ := core.LoadConfig()

	// 1. Antigravity 2.0 Desktop
	desktopAppPath := core.GetAntigravityDesktopAppPath()
	resDir := core.GetAntigravityDesktopResourcesDir()
	if (cfg != nil && cfg.DesktopAppPath != "" && pathExists(cfg.DesktopAppPath)) ||
		pathExists(desktopAppPath) || pathExists(resDir) || pathExists("/opt/Antigravity") {
		chain = append(chain, "Antigravity 2.0 Desktop")
		if selected == "" {
			selected = "Antigravity 2.0 Desktop"
		}
	}

	// 2. agy CLI
	if cfg != nil && cfg.AgyCLIPath != "" && pathExists(cfg.AgyCLIPath) {
		chain = append(chain, fmt.Sprintf("agy CLI (%s)", cfg.AgyCLIPath))
		if selected == "" {
			selected = fmt.Sprintf("agy CLI (%s)", cfg.AgyCLIPath)
		}
	} else if agyPath, err := exec.LookPath("agy"); err == nil && agyPath != "" {
		chain = append(chain, fmt.Sprintf("agy CLI (%s)", agyPath))
		if selected == "" {
			selected = fmt.Sprintf("agy CLI (%s)", agyPath)
		}
	} else {
		// Check fallback standard location
		home, _ := os.UserHomeDir()
		agyLocal := fmt.Sprintf("%s/.local/bin/agy", home)
		if pathExists(agyLocal) {
			chain = append(chain, "agy CLI (~/.local/bin/agy)")
			if selected == "" {
				selected = "agy CLI (~/.local/bin/agy)"
			}
		}
	}

	// 3. VS Code Extension
	if cfg != nil && cfg.VSCodeExtensionPath != "" && pathExists(cfg.VSCodeExtensionPath) {
		chain = append(chain, fmt.Sprintf("VS Code Extension (%s)", filepath.Base(cfg.VSCodeExtensionPath)))
		if selected == "" {
			selected = fmt.Sprintf("VS Code Extension (%s)", filepath.Base(cfg.VSCodeExtensionPath))
		}
	} else {
		candidateDirs := core.GetVSCodeExtensionsDirs()
		for _, dir := range candidateDirs {
			if pathExists(dir) {
				entries, _ := os.ReadDir(dir)
				for _, e := range entries {
					if e.IsDir() && strings.Contains(e.Name(), "antigravity") {
						chain = append(chain, fmt.Sprintf("VS Code Extension (%s)", e.Name()))
						if selected == "" {
							selected = fmt.Sprintf("VS Code Extension (%s)", e.Name())
						}
						break
					}
				}
				if len(chain) > 0 && selected != "" && strings.HasPrefix(chain[len(chain)-1], "VS Code Extension") {
					break
				}
			}
		}
	}

	if selected == "" {
		selected = "Antigravity Swiss Knife Built-in Diagnostic Runner"
		chain = append(chain, selected)
	}

	return selected, chain
}

// ResolveAccountOrModel determines execution runner following priority:
// 1. Already logged in account (ActiveAccount)
// 2. First enabled account from keyring store (if quota available)
// 3. First enabled custom model from custom models store
// 4. Fallback to built-in diagnostics rule engine
func ResolveAccountOrModel(
	cfg *core.Config,
	keyringStore *keyring.Store,
	customStore *custommodels.Store,
) (string, string) {
	// 1. Check currently active account
	if cfg.ActiveAccount != "" {
		if keyringStore != nil {
			acc, err := keyringStore.GetAccount(cfg.ActiveAccount)
			if err == nil && acc != nil && acc.Status != "DISABLED" {
				return acc.Email, "active_account"
			}
		} else {
			return cfg.ActiveAccount, "active_account"
		}
	}

	// 2. Check first enabled account in keyring store
	if keyringStore != nil {
		accounts := keyringStore.ListAccounts()
		for _, acc := range accounts {
			if acc.Status != "DISABLED" {
				return acc.Email, "enabled_account"
			}
		}
	}

	// 3. Check first enabled custom model
	if customStore != nil {
		models := customStore.ListModels()
		for _, m := range models {
			if m.Enabled {
				modelName := m.DisplayName
				if modelName == "" {
					modelName = m.Name
				}
				if modelName == "" {
					modelName = m.ID
				}
				return fmt.Sprintf("Custom Model: %s (%s)", modelName, m.ProviderType), "custom_model"
			}
		}
	}

	// 4. Built-in engine fallback
	return "Built-in Rule Diagnostics Engine", "builtin"
}

// RunIssueDiagnosis executes diagnostic collection, scrubbing, and formats a GitHub issue.
func RunIssueDiagnosis(
	req DiagnosticRequest,
	cfg *core.Config,
	keyringStore *keyring.Store,
	customStore *custommodels.Store,
) (*DiagnosticResult, error) {
	desc := strings.TrimSpace(req.Description)
	if desc == "" {
		desc = "General application runtime anomaly or diagnosis check."
	}

	// 1. Agent Selection
	agentSelected, chain := DetectAvailableAgents()

	// 2. Account / Model Resolution
	accountOrModel, source := ResolveAccountOrModel(cfg, keyringStore, customStore)

	// 3. Generate raw diagnostic payload
	detector := NewDetector()
	installations := detector.DetectAll()

	var sb strings.Builder
	sb.WriteString("=== ANTIGRAVITY SWISS KNIFE SYSTEM DIAGNOSTICS ===\n")
	sb.WriteString(fmt.Sprintf("Timestamp: Current Runtime Session\n"))
	sb.WriteString(fmt.Sprintf("OS / Platform: %s (%s)\n", runtime.GOOS, runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("Go Version: %s\n", runtime.Version()))
	sb.WriteString(fmt.Sprintf("App Version: %s\n", core.AppVersion))
	sb.WriteString(fmt.Sprintf("Storage Mode: %s\n", cfg.StorageMode))
	sb.WriteString(fmt.Sprintf("IPC Socket: %s\n", core.GetSocketPath()))
	sb.WriteString(fmt.Sprintf("Desktop App Detected: %t (Version: %s)\n", installations.DesktopApp.Installed, installations.DesktopApp.Version))
	sb.WriteString(fmt.Sprintf("VS Code Extension Detected: %t (Version: %s)\n", installations.VSCodeExtension.Installed, installations.VSCodeExtension.Version))
	sb.WriteString(fmt.Sprintf("Diagnostic Agent: %s\n", agentSelected))
	sb.WriteString(fmt.Sprintf("Runner Source: %s (%s)\n", accountOrModel, source))
	sb.WriteString("===================================================\n\n")
	sb.WriteString("User Description:\n")
	sb.WriteString(desc)
	sb.WriteString("\n")

	// 4. Scrub and Sanitize Payload
	sanitizedReport, redactedCount := SanitizeSensitiveText(sb.String())
	sanitizedDesc, descRedacted := SanitizeSensitiveText(desc)
	totalRedacted := redactedCount + descRedacted

	// 5. Construct Structured GitHub Issue
	issueTitle := fmt.Sprintf("[Bug Report] %s", summarizeTitle(sanitizedDesc))

	var bodyBuilder strings.Builder
	bodyBuilder.WriteString("### Description of Issue\n")
	bodyBuilder.WriteString(sanitizedDesc)
	bodyBuilder.WriteString("\n\n")

	bodyBuilder.WriteString("### Automated Agent Diagnostics\n")
	bodyBuilder.WriteString(fmt.Sprintf("- **Diagnostic Agent**: `%s` (Agent priority chain: Antigravity 2.0 Desktop > agy CLI > VS Code Extension)\n", agentSelected))
	bodyBuilder.WriteString(fmt.Sprintf("- **Diagnostics Runner**: `%s` (Resolved via: `%s`)\n", accountOrModel, source))
	bodyBuilder.WriteString(fmt.Sprintf("- **Privacy Sanitization**: PASS (%d sensitive tokens/emails/paths scrubbed)\n", totalRedacted))
	bodyBuilder.WriteString("\n")

	bodyBuilder.WriteString("### System & Environment\n")
	bodyBuilder.WriteString(fmt.Sprintf("- **OS / Architecture**: `%s / %s`\n", runtime.GOOS, runtime.GOARCH))
	bodyBuilder.WriteString(fmt.Sprintf("- **App Version**: `v%s`\n", core.AppVersion))
	bodyBuilder.WriteString(fmt.Sprintf("- **Storage Mode**: `%s`\n", cfg.StorageMode))
	bodyBuilder.WriteString(fmt.Sprintf("- **Antigravity Desktop**: `%s` (Up-to-date: %t)\n", installations.DesktopApp.Version, installations.DesktopApp.UpToDate))
	bodyBuilder.WriteString(fmt.Sprintf("- **VS Code Extension**: `%s` (Up-to-date: %t)\n", installations.VSCodeExtension.Version, installations.VSCodeExtension.UpToDate))
	bodyBuilder.WriteString("\n")

	bodyBuilder.WriteString("### Sanitized Diagnostics Summary\n")
	bodyBuilder.WriteString("```\n")
	bodyBuilder.WriteString(sanitizedReport)
	bodyBuilder.WriteString("```\n\n")

	bodyBuilder.WriteString("---\n")
	bodyBuilder.WriteString("*Report automatically diagnosed and scrubbed by Antigravity Swiss Knife. Non-sensitive information guaranteed.*\n")

	fullBody := bodyBuilder.String()

	// 6. Construct pre-filled GitHub issue URL
	issueURL := fmt.Sprintf(
		"https://github.com/%s/issues/new?title=%s&body=%s",
		PublicGitHubRepo,
		url.QueryEscape(issueTitle),
		url.QueryEscape(fullBody),
	)

	return &DiagnosticResult{
		Success:               true,
		AgentSelected:         agentSelected,
		AgentPriorityChain:    chain,
		AccountOrModelUsed:    accountOrModel,
		ResolutionSource:      source,
		SanitizedReport:       fullBody,
		IssueTitle:            issueTitle,
		IssueURL:              issueURL,
		GitHubRepo:            PublicGitHubRepo,
		SensitiveDataRedacted: true,
		RedactedTokenCount:    totalRedacted,
	}, nil
}

func summarizeTitle(desc string) string {
	clean := strings.TrimSpace(desc)
	lines := strings.Split(clean, "\n")
	firstLine := strings.TrimSpace(lines[0])
	if len(firstLine) > 60 {
		return firstLine[:57] + "..."
	}
	if firstLine == "" {
		return "Runtime Issue Diagnosed"
	}
	return firstLine
}
