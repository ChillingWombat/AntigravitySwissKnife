package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

const (
	GoogleCloudCodeRetrieveQuotaURL = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"
	GoogleCloudCodeFetchModelsURL   = "https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels"
	GoogleTokenRefreshURL           = "https://oauth2.googleapis.com/token"
	DefaultGoogleClientID           = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	DefaultGoogleClientSecret       = "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"
)

// RawCloudCodeQuotaResponse represents the JSON returned by retrieveUserQuotaSummary.
type RawCloudCodeQuotaResponse struct {
	Buckets []struct {
		ModelID           string  `json:"modelId"`
		RemainingFraction float64 `json:"remainingFraction"`
		ResetTime         string  `json:"resetTime"`
	} `json:"buckets"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// RefreshGoogleToken exchanges a refresh token for a valid access token.
func RefreshGoogleToken(refreshToken, clientID, clientSecret string) (string, error) {
	if refreshToken == "" {
		return "", fmt.Errorf("empty refresh token")
	}
	if clientID == "" {
		clientID = DefaultGoogleClientID
	}
	if clientSecret == "" {
		clientSecret = DefaultGoogleClientSecret
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("refresh_token", refreshToken)
	form.Set("grant_type", "refresh_token")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(GoogleTokenRefreshURL, form)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read refresh response: %w", err)
	}

	var res struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("malformed refresh response: %w", err)
	}

	if res.Error != "" {
		return "", fmt.Errorf("token refresh error %s: %s", res.Error, res.ErrorDesc)
	}
	if res.AccessToken == "" {
		return "", fmt.Errorf("no access token in refresh response")
	}

	return res.AccessToken, nil
}

// FetchLiveQuota queries Google CloudCode API for real account quota metrics.
func FetchLiveQuota(accessToken string, project string) ([]ModelQuota, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("empty access token")
	}

	payload := map[string]string{"project": project}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, GoogleCloudCodeRetrieveQuotaURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "antigravity-swiss-knife/2.0 linux/amd64")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error querying quota: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read quota body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp RawCloudCodeQuotaResponse
		_ = json.Unmarshal(body, &errResp)
		msg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if errResp.Error != nil && errResp.Error.Message != "" {
			msg = fmt.Sprintf("HTTP %d %s: %s", resp.StatusCode, errResp.Error.Status, errResp.Error.Message)
		}
		return nil, fmt.Errorf("upstream error: %s", msg)
	}

	var quotaResp RawCloudCodeQuotaResponse
	if err := json.Unmarshal(body, &quotaResp); err != nil {
		return nil, fmt.Errorf("failed to parse quota JSON: %w", err)
	}

	now := time.Now()
	var models []ModelQuota
	for _, b := range quotaResp.Buckets {
		var resetTime time.Time
		if b.ResetTime != "" {
			resetTime, _ = time.Parse(time.RFC3339, b.ResetTime)
		}
		frac := b.RemainingFraction
		models = append(models, ModelQuota{
			ModelName:    b.ModelID,
			Fraction:     frac,
			ResetTime:    resetTime,
			ResetText:    FormatResetHorizon(resetTime, now),
			HealthStatus: ComputeHealth(frac),
		})
	}

	return models, nil
}

// PollAccountLiveQuota attempts to query Google CloudCode for real quota, refreshing tokens if needed.
func PollAccountLiveQuota(acc *keyring.Account) (*QuotaSummary, error) {
	if acc == nil {
		return nil, fmt.Errorf("nil account")
	}

	accessToken := acc.AccessToken
	models, err := FetchLiveQuota(accessToken, "")
	if err != nil && (strings.Contains(err.Error(), "HTTP 401") || accessToken == "") && acc.RefreshToken != "" {
		// Attempt token refresh
		newTok, refErr := RefreshGoogleToken(acc.RefreshToken, "", "")
		if refErr == nil && newTok != "" {
			acc.AccessToken = newTok
			accessToken = newTok
			models, err = FetchLiveQuota(accessToken, "")
		}
	}

	now := time.Now()
	summary := &QuotaSummary{
		AccountEmail: acc.Email,
		Models:       models,
		LastPolled:   now,
	}

	if len(models) == 0 {
		summary.MinFraction = 0.0
		summary.OverallHealth = "STANDBY"
		if err != nil {
			summary.OverallHealth = core.StatusExhausted
		}
		return summary, err
	}

	minFrac := 1.0
	for _, m := range models {
		if m.Fraction < minFrac {
			minFrac = m.Fraction
		}
	}
	summary.MinFraction = minFrac
	summary.OverallHealth = ComputeHealth(minFrac)
	return summary, nil
}
