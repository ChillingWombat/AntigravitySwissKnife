package custommodels

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ServiceAccountKey represents the parsed Google Cloud service account JSON key file.
type ServiceAccountKey struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri,omitempty"`
	TokenURI                string `json:"token_uri,omitempty"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url,omitempty"`
	ClientX509CertURL       string `json:"client_x509_cert_url,omitempty"`
}

// ParseServiceAccountKey parses and validates a GCP service account JSON key string or bytes.
func ParseServiceAccountKey(data []byte) (*ServiceAccountKey, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("service account json key is empty")
	}

	var key ServiceAccountKey
	if err := json.Unmarshal(data, &key); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	if key.Type != "service_account" {
		return nil, errors.New("not a valid service_account key (type field must be 'service_account')")
	}
	if strings.TrimSpace(key.PrivateKey) == "" {
		return nil, errors.New("missing private_key in service account json")
	}
	if strings.TrimSpace(key.ClientEmail) == "" {
		return nil, errors.New("missing client_email in service account json")
	}
	if strings.TrimSpace(key.ProjectID) == "" {
		return nil, errors.New("missing project_id in service account json")
	}

	return &key, nil
}

// parseRSAPrivateKey extracts an RSA private key from PEM bytes (PKCS#8 or PKCS#1).
func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode pem block from private key")
	}

	// Try PKCS#8 format (standard for modern Google service account keys)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("pkcs8 key is not an rsa private key")
		}
		return rsaKey, nil
	}

	// Try PKCS#1 format fallback
	rsaKey, err1 := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err1 == nil {
		return rsaKey, nil
	}

	return nil, fmt.Errorf("failed to parse rsa private key: pkcs8 error: %v, pkcs1 error: %v", err, err1)
}

// ValidateTokenURI verifies that the token URI uses HTTPS and points to trusted Google identity servers.
func ValidateTokenURI(rawURI string) error {
	trimmed := strings.TrimSpace(rawURI)
	if trimmed == "" {
		return nil
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid token_uri: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("token_uri must use https scheme: %s", rawURI)
	}
	host := strings.ToLower(u.Hostname())
	if host != "oauth2.googleapis.com" &&
		host != "accounts.google.com" &&
		!strings.HasSuffix(host, ".googleapis.com") {
		return fmt.Errorf("untrusted token_uri host %q; must be a googleapis.com domain", host)
	}
	return nil
}

// GenerateGCPAccessToken mints a fresh Google Cloud OAuth2 access token using the service account key.
func GenerateGCPAccessToken(ctx context.Context, key *ServiceAccountKey) (string, error) {
	rsaKey, err := parseRSAPrivateKey(key.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	tokenURI := strings.TrimSpace(key.TokenURI)
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	} else if err := ValidateTokenURI(tokenURI); err != nil {
		return "", fmt.Errorf("rejected unsafe token_uri: %w", err)
	}

	now := time.Now().Unix()
	headerJSON := `{"alg":"RS256","typ":"JWT"}`
	headerEnc := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))

	claimsMap := map[string]interface{}{
		"iss":   key.ClientEmail,
		"scope": "https://www.googleapis.com/auth/cloud-platform",
		"aud":   tokenURI,
		"exp":   now + 3600,
		"iat":   now,
	}
	claimsBytes, err := json.Marshal(claimsMap)
	if err != nil {
		return "", err
	}
	claimsEnc := base64.RawURLEncoding.EncodeToString(claimsBytes)

	toSign := headerEnc + "." + claimsEnc
	hashed := sha256.Sum256([]byte(toSign))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign jwt: %w", err)
	}
	sigEnc := base64.RawURLEncoding.EncodeToString(sig)
	signedJWT := toSign + "." + sigEnc

	// Exchange signed JWT with Google token endpoint
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", signedJWT)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token endpoint request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google token endpoint returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", errors.New("token response missing access_token")
	}

	return tokenResp.AccessToken, nil
}

// NormalizeVertexAIModel parses a model name or resource path, extracting the publisher, clean model ID, and endpoint action.
func NormalizeVertexAIModel(rawModel string) (publisher, cleanModel, action string) {
	s := strings.TrimSpace(rawModel)

	// Check for explicit action suffix (e.g. :generateContent, :rawPredict, :streamRawPredict, :predict)
	if idx := strings.LastIndex(s, ":"); idx != -1 {
		action = s[idx+1:]
		s = s[:idx]
	}

	// If full URL or resource path contains /publishers/, strip everything up to publishers/
	if idx := strings.Index(s, "/publishers/"); idx != -1 {
		s = s[idx+1:]
	}

	// Strip leading "publishers/" if present
	if strings.HasPrefix(s, "publishers/") {
		s = strings.TrimPrefix(s, "publishers/")
		if idx := strings.Index(s, "/models/"); idx != -1 {
			publisher = s[:idx]
			cleanModel = s[idx+len("/models/"):]
		}
	}

	if publisher == "" {
		if strings.Contains(s, "/") {
			parts := strings.SplitN(s, "/", 2)
			publisher = parts[0]
			cleanModel = parts[1]
		} else {
			cleanModel = s
			lower := strings.ToLower(s)
			if strings.Contains(lower, "claude") {
				publisher = "anthropic"
			} else if strings.HasPrefix(lower, "llama") || strings.HasPrefix(lower, "meta-") {
				publisher = "meta"
			} else if strings.HasPrefix(lower, "mistral") || strings.HasPrefix(lower, "mixtral") {
				publisher = "mistralai"
			} else {
				publisher = "google"
			}
		}
	}

	publisher = strings.TrimSpace(publisher)
	cleanModel = strings.TrimSpace(cleanModel)

	// Strip duplicate publisher or models/ prefixes
	if strings.HasPrefix(cleanModel, publisher+"/") {
		cleanModel = strings.TrimPrefix(cleanModel, publisher+"/")
	}
	cleanModel = strings.TrimPrefix(cleanModel, "models/")

	// Normalize Anthropic Claude model names for Vertex AI
	if strings.EqualFold(publisher, "anthropic") || strings.Contains(strings.ToLower(cleanModel), "claude") {
		publisher = "anthropic"
		// Normalize period separators in version numbers (e.g. claude-3.7 -> claude-3-7)
		if strings.HasPrefix(cleanModel, "claude-3.7-") {
			cleanModel = "claude-3-7-" + strings.TrimPrefix(cleanModel, "claude-3.7-")
		} else if strings.HasPrefix(cleanModel, "claude-3.5-") {
			cleanModel = "claude-3-5-" + strings.TrimPrefix(cleanModel, "claude-3.5-")
		}

		// Convert hyphenated date suffix to @ separator if present (e.g. claude-3-7-sonnet-20250219 -> claude-3-7-sonnet@20250219)
		if !strings.Contains(cleanModel, "@") && len(cleanModel) >= 9 && cleanModel[len(cleanModel)-9] == '-' {
			suffix := cleanModel[len(cleanModel)-8:]
			isDate := true
			for _, ch := range suffix {
				if ch < '0' || ch > '9' {
					isDate = false
					break
				}
			}
			if isDate && strings.HasPrefix(suffix, "202") {
				base := cleanModel[:len(cleanModel)-9]
				cleanModel = base + "@" + suffix
			}
		}

		// Expand unversioned aliases to official Vertex AI model identifiers
		if !strings.Contains(cleanModel, "@") {
			lowerClean := strings.ToLower(cleanModel)
			switch {
			case lowerClean == "claude-3-7-sonnet" || lowerClean == "claude-3.7-sonnet":
				cleanModel = "claude-3-7-sonnet@20250219"
			case lowerClean == "claude-3-5-sonnet" || lowerClean == "claude-3.5-sonnet" || lowerClean == "claude-3-5-sonnet-v2" || lowerClean == "claude-3.5-sonnet-v2":
				cleanModel = "claude-3-5-sonnet-v2@20241022"
			case lowerClean == "claude-3-5-haiku" || lowerClean == "claude-3.5-haiku":
				cleanModel = "claude-3-5-haiku@20241022"
			case lowerClean == "claude-3-opus" || lowerClean == "claude-3.0-opus":
				cleanModel = "claude-3-opus@20240229"
			case lowerClean == "claude-3-haiku" || lowerClean == "claude-3.0-haiku":
				cleanModel = "claude-3-haiku@20240307"
			case lowerClean == "claude-3-sonnet" || lowerClean == "claude-3.0-sonnet":
				cleanModel = "claude-3-sonnet@20240229"
			}
		}

		// Vertex AI specific: Claude 3.5 Sonnet 20241022 is versioned as claude-3-5-sonnet-v2
		if cleanModel == "claude-3-5-sonnet@20241022" || cleanModel == "claude-3.5-sonnet@20241022" {
			cleanModel = "claude-3-5-sonnet-v2@20241022"
		}
	}

	// Assign default action if not explicitly given
	if action == "" {
		switch strings.ToLower(publisher) {
		case "google":
			action = "generateContent"
		case "anthropic":
			action = "rawPredict"
		default:
			action = "rawPredict"
		}
	}

	return publisher, cleanModel, action
}

// BuildVertexAIEndpoint constructs the REST endpoint URL for a Vertex AI model.
func BuildVertexAIEndpoint(region, projectID, modelName string) string {
	region = strings.TrimSpace(region)
	if region == "" {
		region = "us-central1"
	}
	projectID = strings.TrimSpace(projectID)
	publisher, cleanModel, action := NormalizeVertexAIModel(modelName)
	if cleanModel == "" {
		cleanModel = "gemini-2.5-flash"
	}
	if publisher == "" {
		publisher = "google"
	}

	// Claude on Vertex AI is only available in us-east5 and europe-west1; remap us-central1 to us-east5
	if strings.EqualFold(publisher, "anthropic") && (region == "us-central1" || region == "") {
		region = "us-east5"
	}

	return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/%s/models/%s:%s",
		url.PathEscape(region), url.PathEscape(projectID), url.PathEscape(region),
		url.PathEscape(publisher), url.PathEscape(cleanModel), action)
}

// TestGCPModel tests connectivity to the GCP Vertex AI model using the Service Account credentials.
func TestGCPModel(ctx context.Context, saKeyJSON string, projectID, region, modelName string) (*TestResult, error) {
	return TestGCPModelWithEndpoint(ctx, saKeyJSON, "", projectID, region, modelName, false)
}

// TestGCPModelWithEndpoint tests connectivity with an optional endpoint override and loopback allowance for testing.
func TestGCPModelWithEndpoint(ctx context.Context, saKeyJSON string, endpointOverride string, projectID, region, modelName string, allowLoopback bool) (*TestResult, error) {
	start := time.Now()
	saKey, err := ParseServiceAccountKey([]byte(saKeyJSON))
	if err != nil {
		return &TestResult{
			Success:   false,
			LatencyMs: time.Since(start).Milliseconds(),
			Message:   fmt.Sprintf("Invalid Service Account Key: %v", err),
		}, nil
	}

	if projectID == "" {
		projectID = saKey.ProjectID
	}
	if region == "" {
		region = "us-central1"
	}
	if modelName == "" {
		modelName = "gemini-2.5-flash"
	}

	endpoint := strings.TrimSpace(endpointOverride)
	if projectID == "" && strings.Contains(endpoint, "/projects/") {
		rem := endpoint[strings.Index(endpoint, "/projects/")+len("/projects/"):]
		if slash := strings.Index(rem, "/"); slash != -1 {
			projectID = rem[:slash]
		}
	}
	if region == "" && strings.Contains(endpoint, "-aiplatform.googleapis.com") {
		if u, err := url.Parse(endpoint); err == nil {
			host := u.Hostname()
			if idx := strings.Index(host, "-aiplatform.googleapis.com"); idx != -1 {
				region = host[:idx]
			}
		}
	}

	publisher, _, _ := NormalizeVertexAIModel(modelName)
	if endpoint == "" || strings.Contains(endpoint, "aiplatform.googleapis.com") || !strings.Contains(endpoint, "/models/") {
		endpoint = BuildVertexAIEndpoint(region, projectID, modelName)
	}

	if _, _, ssrfErr := ValidateEndpointSSRF(endpoint, ProviderGCP, allowLoopback); ssrfErr != nil {
		return &TestResult{
			Success:   false,
			LatencyMs: time.Since(start).Milliseconds(),
			Endpoint:  endpoint,
			Message:   fmt.Sprintf("SSRF validation blocked: %v", ssrfErr),
		}, nil
	}

	accessToken, err := GenerateGCPAccessToken(ctx, saKey)
	if err != nil {
		return &TestResult{
			Success:   false,
			LatencyMs: time.Since(start).Milliseconds(),
			Message:   fmt.Sprintf("Authentication failed: %v", err),
		}, nil
	}

	// Send a lightweight test ping request to Vertex AI tailored to provider
	var reqBody []byte
	switch strings.ToLower(publisher) {
	case "anthropic":
		reqBody, _ = json.Marshal(map[string]interface{}{
			"anthropic_version": "vertex-2023-10-16",
			"max_tokens":        1,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		})
	case "google":
		reqBody, _ = json.Marshal(map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"role": "user",
					"parts": []map[string]string{
						{"text": "ping"},
					},
				},
			},
			"generationConfig": map[string]interface{}{
				"maxOutputTokens": 1,
			},
		})
	default:
		reqBody, _ = json.Marshal(map[string]interface{}{
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
			"max_tokens": 1,
		})
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return &TestResult{
			Success:   false,
			LatencyMs: time.Since(start).Milliseconds(),
			Endpoint:  endpoint,
			Message:   fmt.Sprintf("Failed to construct request: %v", err),
		}, nil
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &TestResult{
			Success:   false,
			LatencyMs: latency,
			Endpoint:  endpoint,
			Message:   fmt.Sprintf("Vertex AI connection failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		billingDesc := "Google Cloud Platform Vertex AI Managed"
		balanceVal := "Active (GCP Billing)"
		if bInfo, bErr := FetchGCPBillingInfoWithToken(ctx, accessToken, projectID); bErr == nil && bInfo != nil {
			if bInfo.BillingEnabled {
				billingDesc = fmt.Sprintf("Billing Account: %s", bInfo.BillingAccountName)
				balanceVal = bInfo.StatusMessage
			} else {
				billingDesc = bInfo.StatusMessage
				balanceVal = "Disabled / No Account"
			}
		}

		return &TestResult{
			Success:    true,
			StatusCode: resp.StatusCode,
			LatencyMs:  latency,
			Endpoint:   endpoint,
			Message:    "OK",
			QuotaResult: &QuotaResult{
				QuotaType:    QuotaTypeBalance,
				QuotaValue:   balanceVal,
				BalanceValue: balanceVal,
				Message:      billingDesc,
			},
		}, nil
	}

	errMsg := parseGCPErrorMessage(resp.StatusCode, bodyBytes)

	return &TestResult{
		Success:    false,
		StatusCode: resp.StatusCode,
		LatencyMs:  latency,
		Endpoint:   endpoint,
		Message:    errMsg,
	}, nil
}

// parseGCPErrorMessage extracts human-readable diagnostic error text from Vertex AI or Anthropic error bodies.
func parseGCPErrorMessage(statusCode int, bodyBytes []byte) string {
	var gcpErr struct {
		Error struct {
			Code    interface{} `json:"code"`
			Message string      `json:"message"`
			Status  string      `json:"status"`
		} `json:"error"`
	}
	if json.Unmarshal(bodyBytes, &gcpErr) == nil && gcpErr.Error.Message != "" {
		return fmt.Sprintf("HTTP %d: %s", statusCode, gcpErr.Error.Message)
	}

	var arrErr []struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(bodyBytes, &arrErr) == nil && len(arrErr) > 0 {
		if arrErr[0].Error.Message != "" {
			return fmt.Sprintf("HTTP %d: %s", statusCode, arrErr[0].Error.Message)
		} else if arrErr[0].Message != "" {
			return fmt.Sprintf("HTTP %d: %s", statusCode, arrErr[0].Message)
		}
	}

	var altErr struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(bodyBytes, &altErr) == nil {
		if altErr.Message != "" {
			return fmt.Sprintf("HTTP %d: %s", statusCode, altErr.Message)
		} else if altErr.Error != "" {
			return fmt.Sprintf("HTTP %d: %s", statusCode, altErr.Error)
		} else if altErr.Detail != "" {
			return fmt.Sprintf("HTTP %d: %s", statusCode, altErr.Detail)
		}
	}

	if len(bodyBytes) > 0 && len(bodyBytes) < 1500 {
		raw := strings.TrimSpace(string(bodyBytes))
		if !strings.HasPrefix(raw, "<") && raw != "" {
			return fmt.Sprintf("HTTP %d: %s", statusCode, raw)
		}
	}

	statusText := http.StatusText(statusCode)
	if statusText != "" {
		return fmt.Sprintf("HTTP %d %s", statusCode, statusText)
	}
	return fmt.Sprintf("HTTP %d", statusCode)
}

// GCPBillingInfo reports the Cloud Billing status and associated billing account.
type GCPBillingInfo struct {
	Name               string `json:"name"`
	ProjectID          string `json:"projectId"`
	BillingAccountName string `json:"billingAccountName"`
	BillingEnabled     bool   `json:"billingEnabled"`
	StatusMessage      string `json:"statusMessage"`
}

// FetchGCPBillingInfoWithEndpoint queries a specific Cloud Billing endpoint using an existing OAuth2 access token.
func FetchGCPBillingInfoWithEndpoint(ctx context.Context, billingURL, accessToken, projectID string) (*GCPBillingInfo, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.New("project ID is required")
	}
	if accessToken == "" {
		return nil, errors.New("access token is required")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", billingURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("billing api request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	// If HTTP 403 Forbidden: the Service Account lacks billing.resourceAssociations.list permission.
	// Since model inference succeeded with HTTP 200, billing is actively enabled on the GCP project.
	if resp.StatusCode == http.StatusForbidden {
		return &GCPBillingInfo{
			ProjectID:          projectID,
			BillingEnabled:     true,
			BillingAccountName: "Project Billing",
			StatusMessage:      "Active (GCP Billing)",
		}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return &GCPBillingInfo{
			ProjectID:          projectID,
			BillingEnabled:     true,
			BillingAccountName: "Vertex AI Managed",
			StatusMessage:      "Active (Vertex AI Managed)",
		}, nil
	}

	var info GCPBillingInfo
	if err := json.Unmarshal(bodyBytes, &info); err != nil {
		return nil, fmt.Errorf("failed to parse billing info response: %w", err)
	}
	if info.BillingEnabled {
		acctName := info.BillingAccountName
		if parts := strings.Split(acctName, "/"); len(parts) > 1 {
			acctName = parts[len(parts)-1]
		}
		info.StatusMessage = fmt.Sprintf("Active (Account: %s)", acctName)
	} else {
		info.StatusMessage = "No billing account linked"
	}
	return &info, nil
}

// FetchGCPBillingInfoWithToken queries the Google Cloud Billing API using an existing OAuth2 access token.
func FetchGCPBillingInfoWithToken(ctx context.Context, accessToken, projectID string) (*GCPBillingInfo, error) {
	billingURL := fmt.Sprintf("https://cloudbilling.googleapis.com/v1/projects/%s/billingInfo", url.PathEscape(strings.TrimSpace(projectID)))
	return FetchGCPBillingInfoWithEndpoint(ctx, billingURL, accessToken, projectID)
}

// FetchGCPBillingInfo queries the Google Cloud Billing API for the project's billing status.
func FetchGCPBillingInfo(ctx context.Context, saKey *ServiceAccountKey, projectID string) (*GCPBillingInfo, error) {
	if saKey == nil {
		return nil, errors.New("service account key is required")
	}
	if projectID == "" {
		projectID = saKey.ProjectID
	}
	if projectID == "" {
		return nil, errors.New("project ID is required")
	}

	accessToken, err := GenerateGCPAccessToken(ctx, saKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token for billing check: %w", err)
	}
	return FetchGCPBillingInfoWithToken(ctx, accessToken, projectID)
}
