package keyring

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

const (
	DefaultAGMMasterKeyHex = "6f26654ecace3a34ecf9761867eb6edc09450ee4480d61e69a9c581782548c8d"
	AGMPrefix              = "agm_enc_v1:"
)

// DiscoveredCloudAccount represents a decrypted account from cloud_accounts.db.
type DiscoveredCloudAccount struct {
	Email                    string  `json:"email"`
	Name                     string  `json:"name"`
	IsActive                 bool    `json:"is_active"`
	Status                   string  `json:"status"`
	AccessToken              string  `json:"access_token"`
	RefreshToken             string  `json:"refresh_token"`
	IDToken                  string  `json:"id_token"`
	PlanTier                 string  `json:"plan_tier"`
	Credits                  float64 `json:"credits"`
	Quota5h                  float64 `json:"quota_5h"`
	QuotaWeekly              float64 `json:"quota_weekly"`
	Quota5hClaudeGPT         float64 `json:"quota_5h_claude_gpt"`
	QuotaWeeklyClaudeGPT     float64 `json:"quota_weekly_claude_gpt"`
	ResetTime5h              string  `json:"reset_time_5h"`
	ResetTimeWeekly          string  `json:"reset_time_weekly"`
	ResetTime5hClaudeGPT     string  `json:"reset_time_5h_claude_gpt"`
	ResetTimeWeeklyClaudeGPT string  `json:"reset_time_weekly_claude_gpt"`
}

// DecryptAGM decrypts an agm_enc_v1:<iv>:<tag>:<ct> AES-256-GCM string.
func DecryptAGM(keyHex string, encStr string) (string, error) {
	if encStr == "" {
		return "", nil
	}
	if !strings.HasPrefix(encStr, AGMPrefix) {
		if strings.HasPrefix(encStr, "{") || strings.HasPrefix(encStr, "[") {
			return encStr, nil
		}
		return "", fmt.Errorf("unknown encryption prefix: %s", encStr)
	}

	trimmed := strings.TrimPrefix(encStr, AGMPrefix)
	parts := strings.Split(trimmed, ":")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid AGM format, expected 3 parts, got %d", len(parts))
	}

	iv, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid iv hex: %w", err)
	}
	tag, err := hex.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid tag hex: %w", err)
	}
	ct, err := hex.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext hex: %w", err)
	}

	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", fmt.Errorf("invalid key hex: %w", err)
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Go cipher.GCM expects ciphertext concatenated with the authentication tag
	ciphertextWithTag := append(ct, tag...)
	plaintext, err := gcm.Open(nil, iv, ciphertextWithTag, nil)
	if err != nil {
		return "", fmt.Errorf("GCM decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// ReadCloudAccountsDB reads and decrypts all accounts from ~/.antigravity-agent/cloud_accounts.db.
func ReadCloudAccountsDB(homeDir string) ([]DiscoveredCloudAccount, error) {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	dbPath := filepath.Join(homeDir, ".antigravity-agent", "cloud_accounts.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}

	// Query sqlite via python3
	pyScript := fmt.Sprintf(`import sqlite3, json
try:
    con = sqlite3.connect(%q)
    cur = con.cursor()
    rows = cur.execute("SELECT email, name, is_active, token_json, quota_json, status FROM accounts").fetchall()
    print(json.dumps([list(r) for r in rows]))
except Exception as e:
    print(json.dumps({"error": str(e)}))
`, dbPath)

	cmd := exec.Command("python3", "-c", pyScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read cloud_accounts.db: %w", err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" || strings.HasPrefix(trimmed, "{\"error\":") {
		return nil, fmt.Errorf("error querying cloud_accounts.db: %s", trimmed)
	}

	var rawRows [][]interface{}
	if err := json.Unmarshal([]byte(trimmed), &rawRows); err != nil {
		return nil, fmt.Errorf("failed to parse rows: %w", err)
	}

	masterKey := os.Getenv("ANTIGRAVITY_MANAGER_MASTER_KEY")
	if masterKey == "" {
		masterKey = DefaultAGMMasterKeyHex
	}

	results := make([]DiscoveredCloudAccount, 0, len(rawRows))
	for _, row := range rawRows {
		if len(row) < 5 {
			continue
		}
		email, _ := row[0].(string)
		name, _ := row[1].(string)
		isActiveNum, _ := row[2].(float64)
		isActive := isActiveNum == 1
		tokenEnc, _ := row[3].(string)
		quotaEnc, _ := row[4].(string)
		rawStatus := ""
		if len(row) >= 6 {
			rawStatus, _ = row[5].(string)
		}
		upperStatus := strings.ToUpper(strings.TrimSpace(rawStatus))
		status := "STANDBY"
		if upperStatus == "BANNED" || upperStatus == "SUSPENDED" || upperStatus == "DISABLED" {
			status = "BANNED"
		} else if upperStatus == "ERROR" || upperStatus == "INVALID" || upperStatus == "EXPIRED" {
			status = "ERROR"
		} else if upperStatus == "COOLDOWN" || upperStatus == "COOLING" {
			status = core.AccountStatusCooling
		} else if isActive {
			status = "ACTIVE"
		}

		if email == "" {
			continue
		}

		acc := DiscoveredCloudAccount{
			Email:    email,
			Name:     name,
			IsActive: isActive,
			Status:   status,
			PlanTier: "Pro", // default for subscribed accounts
		}

		if tokenEnc != "" {
			decToken, decErr := DecryptAGM(masterKey, tokenEnc)
			if decErr == nil && decToken != "" {
				var tObj struct {
					AccessToken  string `json:"access_token"`
					RefreshToken string `json:"refresh_token"`
					IDToken      string `json:"id_token"`
				}
				if json.Unmarshal([]byte(decToken), &tObj) == nil {
					acc.AccessToken = tObj.AccessToken
					acc.RefreshToken = tObj.RefreshToken
					acc.IDToken = tObj.IDToken
				}
			}
		}

		if quotaEnc != "" {
			decQuota, decErr := DecryptAGM(masterKey, quotaEnc)
			if decErr == nil && decQuota != "" {
				var qObj struct {
					SubscriptionTier      string `json:"subscription_tier"`
					SubscriptionTierCamel string `json:"subscriptionTier"`
					UserTier              string `json:"user_tier"`
					UserTierCamel         string `json:"userTier"`
					PlanTier              string `json:"plan_tier"`
					PlanTierCamel         string `json:"planTier"`
					IsTrial               bool   `json:"is_trial"`
					IsTrialCamel          bool   `json:"isTrial"`
					TrialStatus           string `json:"trial_status"`
					TrialStatusCamel      string `json:"trialStatus"`
					WarningMessage        string `json:"warning_message"`
					WarningMessageCamel   string `json:"warningMessage"`
					Notice                string `json:"notice"`
					Models                map[string]interface{} `json:"models"`
					QuotaGroups           []struct {
						DisplayName string `json:"display_name"`
						Description string `json:"description"`
						Buckets     []struct {
							BucketID          string  `json:"bucket_id"`
							DisplayName       string  `json:"display_name"`
							Window            string  `json:"window"`
							RemainingFraction float64 `json:"remaining_fraction"`
							ResetTime         string  `json:"reset_time"`
						} `json:"buckets"`
					} `json:"quota_groups"`
					AICredits *struct {
						Credits interface{} `json:"credits"`
					} `json:"ai_credits"`
				}
				if json.Unmarshal([]byte(decQuota), &qObj) == nil {
					subTier := qObj.SubscriptionTier
					if subTier == "" {
						subTier = qObj.SubscriptionTierCamel
					}
					if subTier == "" {
						subTier = qObj.UserTier
					}
					if subTier == "" {
						subTier = qObj.UserTierCamel
					}
					if subTier == "" {
						subTier = qObj.PlanTier
					}
					if subTier == "" {
						subTier = qObj.PlanTierCamel
					}

					has3PModels := false
					hasSonnet55 := false
					if qObj.Models != nil {
						for mName := range qObj.Models {
							mLow := strings.ToLower(mName)
							if strings.Contains(mLow, "sonnet-5-5") || strings.Contains(mLow, "sonnet-5.5") || strings.Contains(mLow, "opus-5-5") || strings.Contains(mLow, "opus-5.5") {
								hasSonnet55 = true
							}
							if strings.Contains(mLow, "claude") || strings.Contains(mLow, "gpt-oss") {
								has3PModels = true
							}
						}
					}
					isMissingSonnet55Trial := has3PModels && !hasSonnet55

					isTrial := qObj.IsTrial || qObj.IsTrialCamel ||
						strings.Contains(strings.ToLower(qObj.TrialStatus+" "+qObj.TrialStatusCamel), "trial") ||
						strings.Contains(strings.ToLower(qObj.TrialStatus+" "+qObj.TrialStatusCamel), "promo") ||
						strings.Contains(strings.ToLower(subTier), "google ai pro") ||
						isTrialWarningText(qObj.WarningMessage) || isTrialWarningText(qObj.WarningMessageCamel) ||
						isTrialWarningText(qObj.Notice) || isTrialWarningText(subTier) ||
						isMissingSonnet55Trial

					for _, g := range qObj.QuotaGroups {
						if isTrialWarningText(g.DisplayName) || isTrialWarningText(g.Description) {
							isTrial = true
						}
						for _, b := range g.Buckets {
							if isTrialWarningText(b.DisplayName) {
								isTrial = true
							}
						}
					}

					if isTrial {
						acc.PlanTier = "Pro - Trial"
					} else if subTier != "" {
						acc.PlanTier = normalizeCloudTier(subTier)
					}
					if qObj.AICredits != nil && qObj.AICredits.Credits != nil {
						if cNum, ok := qObj.AICredits.Credits.(float64); ok {
							acc.Credits = cNum
						}
					}
					for _, g := range qObj.QuotaGroups {
						gName := strings.ToLower(g.DisplayName)
						for _, b := range g.Buckets {
							w := strings.ToLower(b.Window)
							bid := strings.ToLower(b.BucketID)
							if strings.Contains(gName, "gemini") {
								if w == "5h" || strings.Contains(bid, "5h") {
									acc.Quota5h = b.RemainingFraction
									acc.ResetTime5h = b.ResetTime
								} else if w == "weekly" || strings.Contains(bid, "weekly") {
									acc.QuotaWeekly = b.RemainingFraction
									acc.ResetTimeWeekly = b.ResetTime
								}
							} else if strings.Contains(gName, "claude") || strings.Contains(gName, "gpt") || strings.Contains(bid, "3p") {
								if w == "5h" || strings.Contains(bid, "5h") {
									acc.Quota5hClaudeGPT = b.RemainingFraction
									acc.ResetTime5hClaudeGPT = b.ResetTime
								} else if w == "weekly" || strings.Contains(bid, "weekly") {
									acc.QuotaWeeklyClaudeGPT = b.RemainingFraction
									acc.ResetTimeWeeklyClaudeGPT = b.ResetTime
								}
							}
						}
					}
				}
			}
		}

		results = append(results, acc)
	}

	return results, nil
}

// SyncStoreFromCloudAccountsDB updates vault accounts in store with valid tokens and tiers from cloud_accounts.db.
func SyncStoreFromCloudAccountsDB(s *Store, homeDir string) error {
	if s == nil {
		return nil
	}
	cloudAccounts, err := ReadCloudAccountsDB(homeDir)
	if err != nil || len(cloudAccounts) == 0 {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	modified := false
	for _, ca := range cloudAccounts {
		normEmail := strings.TrimSpace(strings.ToLower(ca.Email))
		for k, acc := range s.accounts {
			if strings.TrimSpace(strings.ToLower(k)) == normEmail {
				// Update refresh token if vault has empty or invalid one, or if cloud_accounts has a fresher one
				if ca.RefreshToken != "" && (acc.RefreshToken == "" || strings.HasPrefix(acc.RefreshToken, "1//0e3Vl") || strings.HasPrefix(acc.RefreshToken, "1//0gSBc") || strings.HasPrefix(acc.RefreshToken, "1//06OLV")) {
					acc.RefreshToken = ca.RefreshToken
					modified = true
				}
				if ca.AccessToken != "" && acc.AccessToken == "" {
					acc.AccessToken = ca.AccessToken
					modified = true
				}
				if ca.IDToken != "" && acc.IDToken == "" {
					acc.IDToken = ca.IDToken
					modified = true
				}
				if ca.PlanTier != "" {
					pTier := normalizeCloudTier(ca.PlanTier)
					if acc.PlanTier == "" || acc.PlanTier == "Free" || (pTier == "Pro - Trial" && (acc.PlanTier == "Pro" || acc.PlanTier == "")) {
						acc.PlanTier = pTier
						modified = true
					}
				}
				if ca.Credits > 0 && acc.Credits == 0 {
					acc.Credits = ca.Credits
					modified = true
				}
				caSt := strings.ToUpper(strings.TrimSpace(ca.Status))
				if caSt == "BANNED" || caSt == "ERROR" || caSt == "COOLDOWN" || caSt == "COOLING" {
					if caSt == "COOLDOWN" {
						caSt = core.AccountStatusCooling
					}
					if acc.Status != caSt {
						acc.Status = caSt
						modified = true
					}
				} else if !strings.EqualFold(acc.Email, s.activeEmail) && acc.Status == "ACTIVE" {
					acc.Status = "STANDBY"
					modified = true
				} else if acc.Status == "" {
					if strings.EqualFold(acc.Email, s.activeEmail) {
						acc.Status = "ACTIVE"
					} else {
						acc.Status = "STANDBY"
					}
					modified = true
				}
				break
			}
		}
	}

	if modified {
		return s.save()
	}

	return nil
}

// SyncCloudAccountsAutoSwitch synchronizes the auto_switch_enabled setting in ~/.antigravity-agent/cloud_accounts.db
// so background agent services do not auto-rotate accounts when Auto-Switch is disabled in Swiss Knife.
func SyncCloudAccountsAutoSwitch(homeDir string, enabled bool) error {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	dbPath := filepath.Join(homeDir, ".antigravity-agent", "cloud_accounts.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil
	}
	valStr := "false"
	if enabled {
		valStr = "true"
	}
	pyScript := fmt.Sprintf(`import sqlite3
try:
    con = sqlite3.connect(%q, timeout=5.0)
    con.execute("INSERT OR REPLACE INTO settings (key, value) VALUES ('auto_switch_enabled', ?)", (%q,))
    con.commit()
    con.close()
except Exception:
    pass
`, dbPath, valStr)
	cmd := exec.Command("python3", "-c", pyScript)
	_ = cmd.Run()
	return nil
}

// SyncCloudAccountsActiveAccount updates is_active in ~/.antigravity-agent/cloud_accounts.db
// so that third-party tools and queries see the same active account.
func SyncCloudAccountsActiveAccount(homeDir, activeEmail string) error {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	dbPath := filepath.Join(homeDir, ".antigravity-agent", "cloud_accounts.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil
	}
	pyScript := fmt.Sprintf(`import sqlite3
try:
    con = sqlite3.connect(%q, timeout=5.0)
    con.execute("UPDATE accounts SET is_active = CASE WHEN LOWER(email) = LOWER(?) THEN 1 ELSE 0 END", (%q,))
    con.commit()
    con.close()
except Exception:
    pass
`, dbPath, activeEmail)
	cmd := exec.Command("python3", "-c", pyScript)
	_ = cmd.Run()
	return nil
}

func isTrialWarningText(text string) bool {
	if text == "" {
		return false
	}
	low := strings.ToLower(text)
	if strings.Contains(low, "third-party model access will no longer be available on your current plan") ||
		strings.Contains(low, "sonnet 5.5 is now available on paid pro and ultra plans") ||
		strings.Contains(low, "paid pro and ultra plans") ||
		strings.Contains(low, "will no longer be available on your current plan") ||
		(strings.Contains(low, "third-party model access") && (strings.Contains(low, "current plan") || strings.Contains(low, "november 2"))) ||
		strings.Contains(low, "current plan starting on november 2, 2026") ||
		strings.Contains(low, "starter quota") ||
		strings.Contains(low, "trial") ||
		strings.Contains(low, "promo") ||
		strings.Contains(low, "partner offer") ||
		strings.Contains(low, "jio") {
		return true
	}
	return false
}

func normalizeCloudTier(raw string) string {
	t := strings.TrimSpace(raw)
	if t == "" {
		return "Pro"
	}
	low := strings.ToLower(t)
	if low == "free" || low == "free-tier" || low == "tier_free" || low == "starter" || low == "starter-tier" || low == "starter quota" {
		return "Free"
	}
	if strings.Contains(low, "trial") ||
		strings.Contains(low, "promo") ||
		strings.Contains(low, "google ai pro") ||
		strings.Contains(low, "starter pro") ||
		strings.Contains(low, "jio") ||
		strings.Contains(low, "partner") ||
		strings.Contains(low, "bundle") ||
		isTrialWarningText(low) {
		return "Pro - Trial"
	}
	if strings.Contains(low, "20x") || strings.Contains(low, "ultra_20x") || strings.Contains(low, "ultra 20x") {
		return "Ultra 20X"
	}
	if strings.Contains(low, "10x") || strings.Contains(low, "ultra_10x") || strings.Contains(low, "ultra 10x") {
		return "Ultra 10X"
	}
	if strings.Contains(low, "5x") || strings.Contains(low, "ultra_5x") || strings.Contains(low, "ultra 5x") {
		return "Ultra 5X"
	}
	if strings.Contains(low, "ultra") {
		return "Ultra 20X"
	}
	if strings.Contains(low, "edu") || strings.Contains(low, "education") || strings.Contains(low, "student") || strings.Contains(low, "academic") {
		return "Edu"
	}
	if strings.Contains(low, "enterprise") || strings.Contains(low, "teams_tier_enterprise") {
		return "Enterprise"
	}
	if strings.Contains(low, "plus") {
		return "Plus"
	}
	if strings.Contains(low, "pro") || strings.Contains(low, "standard") || strings.Contains(low, "code assist") || strings.Contains(low, "ai premium") || strings.Contains(low, "g1_ai") || strings.Contains(low, "team") {
		return "Pro"
	}
	return t
}
