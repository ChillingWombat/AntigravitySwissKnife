package custommodels

import (
	"strings"
	"testing"
)

// TestChallenger_CustomModels_SecretRedactionAndPreservation tests that:
// 1. GetConfigRedacted and ListModelsRedacted never leak plaintext API keys or GCP keys.
// 2. Updating a model with [REDACTED] or empty APIKey preserves the existing stored secret.
func TestChallenger_CustomModels_SecretRedactionAndPreservation(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	rawKey := "sk-super-secret-production-key-998877"
	rawGCPKey := `{"type": "service_account", "private_key": "MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC"}`

	model := CustomModel{
		ID:                   "prod-model-1",
		Name:                 "gpt-4o",
		DisplayName:          "GPT-4o Production",
		ProviderType:         ProviderOpenAI,
		BaseURL:              "https://api.openai.com/v1",
		APIKey:               rawKey,
		GCPServiceAccountKey: rawGCPKey,
		Enabled:              true,
	}

	if err := store.SaveModel(model); err != nil {
		t.Fatalf("SaveModel failed: %v", err)
	}

	// 1. Verify plaintext is stored internally
	saved, err := store.GetModel("prod-model-1")
	if err != nil || saved == nil {
		t.Fatalf("GetModel failed: %v", err)
	}
	if saved.APIKey != rawKey {
		t.Errorf("expected stored APIKey %q, got %q", rawKey, saved.APIKey)
	}
	if saved.GCPServiceAccountKey != rawGCPKey {
		t.Errorf("expected stored GCPServiceAccountKey to match raw, got %q", saved.GCPServiceAccountKey)
	}

	// 2. Verify ListModelsRedacted redacts all secrets
	redactedList := store.ListModelsRedacted()
	if len(redactedList) != 1 {
		t.Fatalf("expected 1 model in redacted list, got %d", len(redactedList))
	}
	if redactedList[0].APIKey != "[REDACTED]" {
		t.Errorf("expected APIKey to be [REDACTED], got %q", redactedList[0].APIKey)
	}
	if redactedList[0].GCPServiceAccountKey != "[REDACTED]" {
		t.Errorf("expected GCPServiceAccountKey to be [REDACTED], got %q", redactedList[0].GCPServiceAccountKey)
	}

	// 3. Verify GetConfigRedacted redacts all secrets
	redactedCfg := store.GetConfigRedacted()
	if len(redactedCfg.Models) != 1 {
		t.Fatalf("expected 1 model in redacted config, got %d", len(redactedCfg.Models))
	}
	if redactedCfg.Models[0].APIKey != "[REDACTED]" || redactedCfg.Models[0].GCPServiceAccountKey != "[REDACTED]" {
		t.Errorf("GetConfigRedacted leaked secrets: %+v", redactedCfg.Models[0])
	}

	// 4. Update model sending [REDACTED] as APIKey and empty GCPServiceAccountKey
	updateReq := model
	updateReq.APIKey = "[REDACTED]"
	updateReq.GCPServiceAccountKey = ""
	updateReq.DisplayName = "GPT-4o Updated Title"

	if err := store.SaveModel(updateReq); err != nil {
		t.Fatalf("SaveModel update with [REDACTED] failed: %v", err)
	}

	// 5. Verify the underlying stored secret was NOT overwritten with "[REDACTED]" or ""
	reloaded, err := store.GetModel("prod-model-1")
	if err != nil || reloaded == nil {
		t.Fatalf("GetModel after update failed: %v", err)
	}
	if reloaded.APIKey != rawKey {
		t.Errorf("stored APIKey was overwritten! Expected %q, got %q", rawKey, reloaded.APIKey)
	}
	if reloaded.GCPServiceAccountKey != rawGCPKey {
		t.Errorf("stored GCPServiceAccountKey was overwritten! Expected raw, got %q", reloaded.GCPServiceAccountKey)
	}
	if reloaded.DisplayName != "GPT-4o Updated Title" {
		t.Errorf("other fields failed to update, got %q", reloaded.DisplayName)
	}
}

// TestChallenger_SanitizeURL_QueryParamScrubbing tests SanitizeURL across diverse query structures
// and secret parameter names.
func TestChallenger_SanitizeURL_QueryParamScrubbing(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		contains string
		omits    string
	}{
		{
			"GoogleGeminiKeyParam",
			"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=AIzaSySecretKey123&alt=json",
			"key=%5BREDACTED%5D",
			"AIzaSySecretKey123",
		},
		{
			"ApiKeyParam",
			"https://api.example.com/v1/chat?api_key=sk-secret-456&client=swiss",
			"api_key=%5BREDACTED%5D",
			"sk-secret-456",
		},
		{
			"TokenParam",
			"https://api.example.com/v1/user?token=my-bearer-token&page=1",
			"token=%5BREDACTED%5D",
			"my-bearer-token",
		},
		{
			"AccessTokenParam",
			"https://api.example.com/oauth/data?access_token=ya29.secret&debug=false",
			"access_token=%5BREDACTED%5D",
			"ya29.secret",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sanitized := SanitizeURL(tc.input)
			if !strings.Contains(sanitized, tc.contains) {
				t.Errorf("SanitizeURL(%q) = %q; expected to contain %q", tc.input, sanitized, tc.contains)
			}
			if strings.Contains(sanitized, tc.omits) {
				t.Errorf("SanitizeURL(%q) = %q; leaked secret %q!", tc.input, sanitized, tc.omits)
			}
		})
	}
}
