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
)

const (
	DefaultAGMMasterKeyHex = "6f26654ecace3a34ecf9761867eb6edc09450ee4480d61e69a9c581782548c8d"
	AGMPrefix              = "agm_enc_v1:"
)

// DiscoveredCloudAccount represents a decrypted account from cloud_accounts.db.
type DiscoveredCloudAccount struct {
	Email                string  `json:"email"`
	Name                 string  `json:"name"`
	IsActive             bool    `json:"is_active"`
	AccessToken          string  `json:"access_token"`
	RefreshToken         string  `json:"refresh_token"`
	IDToken              string  `json:"id_token"`
	PlanTier             string  `json:"plan_tier"`
	Credits              float64 `json:"credits"`
	Quota5h              float64 `json:"quota_5h"`
	QuotaWeekly          float64 `json:"quota_weekly"`
	Quota5hClaudeGPT     float64 `json:"quota_5h_claude_gpt"`
	QuotaWeeklyClaudeGPT float64 `json:"quota_weekly_claude_gpt"`
	ResetTime5h          string  `json:"reset_time_5h"`
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
    rows = cur.execute("SELECT email, name, is_active, token_json, quota_json FROM accounts").fetchall()
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

		if email == "" {
			continue
		}

		acc := DiscoveredCloudAccount{
			Email:    email,
			Name:     name,
			IsActive: isActive,
			PlanTier: "Google AI Pro", // default for subscribed accounts
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
					SubscriptionTier string `json:"subscription_tier"`
					QuotaGroups      []struct {
						DisplayName string `json:"display_name"`
						Buckets     []struct {
							BucketID          string  `json:"bucket_id"`
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
					if qObj.SubscriptionTier != "" {
						acc.PlanTier = qObj.SubscriptionTier
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
								}
							} else if strings.Contains(gName, "claude") || strings.Contains(gName, "gpt") || strings.Contains(bid, "3p") {
								if w == "5h" || strings.Contains(bid, "5h") {
									acc.Quota5hClaudeGPT = b.RemainingFraction
								} else if w == "weekly" || strings.Contains(bid, "weekly") {
									acc.QuotaWeeklyClaudeGPT = b.RemainingFraction
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
				if ca.PlanTier != "" && (acc.PlanTier == "" || acc.PlanTier == "Free") {
					acc.PlanTier = ca.PlanTier
					modified = true
				}
				if ca.Credits > 0 && acc.Credits == 0 {
					acc.Credits = ca.Credits
					modified = true
				}
				break
			}
		}
	}

	if modified {
		// unlock temporarily to save
		s.mu.Unlock()
		saveErr := s.save()
		s.mu.Lock()
		return saveErr
	}

	return nil
}
