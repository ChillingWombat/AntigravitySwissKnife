package custommodels

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSSRF_CloudMetadataBlocked(t *testing.T) {
	metadataTargets := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://169.254.169.254:80/latest/meta-data/",
		"http://metadata.google.internal/computeMetadata/v1/",
		"http://metadata.google.internal:8080/computeMetadata/v1/",
		"http://sub.metadata.google.internal/test",
		"http://instance-data/latest/meta-data/",
		"http://100.100.100.200/latest/meta-data/",
		"http://[fd00:ec2::254]/latest/meta-data/",
	}

	for _, target := range metadataTargets {
		// Test across multiple provider types - cloud metadata must NEVER be allowed
		for _, provider := range []ProviderType{ProviderOpenAI, ProviderAnthropic, ProviderGemini, ProviderCustom, ProviderLocal} {
			_, _, err := ValidateEndpointSSRF(target, provider, false)
			if err == nil {
				t.Errorf("expected SSRF block for metadata target %q under provider %s, but got nil error", target, provider)
			}
			if !strings.Contains(err.Error(), "ssrf blocked") {
				t.Errorf("expected 'ssrf blocked' error message for %q, got: %v", target, err)
			}
		}
	}
}

func TestSSRF_ExternalProviderBlocksLoopback(t *testing.T) {
	loopbackTargets := []string{
		"http://127.0.0.1:8765/v1",
		"http://127.0.0.1:11434/v1",
		"http://localhost:8765/v1",
		"http://127.0.0.2:8000/v1",
		"http://[::1]:8080/v1",
	}

	externalProviders := []ProviderType{
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderGemini,
		ProviderCustom,
	}

	for _, target := range loopbackTargets {
		for _, provider := range externalProviders {
			_, _, err := ValidateEndpointSSRF(target, provider, false)
			if err == nil {
				t.Errorf("expected loopback to be blocked for external provider %s targeting %q", provider, target)
			}
			if !strings.Contains(err.Error(), "ssrf blocked") || !strings.Contains(err.Error(), "loopback") {
				t.Errorf("expected loopback block error for %q, got: %v", target, err)
			}
		}
	}
}

func TestSSRF_LocalProviderAllowsLoopback(t *testing.T) {
	loopbackTargets := []string{
		"http://127.0.0.1:11434",
		"http://localhost:11434",
		"http://127.0.0.1:1234/v1",
	}

	for _, target := range loopbackTargets {
		_, _, err := ValidateEndpointSSRF(target, ProviderLocal, false)
		if err != nil {
			t.Errorf("expected loopback to be allowed for ProviderLocal on %q, got error: %v", target, err)
		}
	}
}

func TestSSRF_TesterIntegration(t *testing.T) {
	tester := NewTester() // AllowLoopback is false by default

	// 1. External provider attempting loopback must be rejected
	resLoopback, errLoopback := tester.TestEndpoint(CustomModel{
		ID:           "test-ext-loopback",
		Name:         "gpt-4o",
		ProviderType: ProviderOpenAI,
		BaseURL:      "http://127.0.0.1:8765",
	})
	if errLoopback == nil || resLoopback.Success {
		t.Fatalf("expected TestEndpoint to fail on loopback for ProviderOpenAI, got res=%+v, err=%v", resLoopback, errLoopback)
	}
	if !strings.Contains(resLoopback.Message, "SSRF validation blocked") {
		t.Errorf("expected message to mention 'SSRF validation blocked', got: %s", resLoopback.Message)
	}

	// 2. Cloud metadata IP must be rejected immediately
	resMeta, errMeta := tester.TestEndpoint(CustomModel{
		ID:           "test-cloud-meta",
		Name:         "claude-3-5-sonnet",
		ProviderType: ProviderAnthropic,
		BaseURL:      "http://169.254.169.254/latest/meta-data",
	})
	if errMeta == nil || resMeta.Success {
		t.Fatalf("expected TestEndpoint to fail on metadata IP, got res=%+v, err=%v", resMeta, errMeta)
	}
	if !strings.Contains(resMeta.Message, "SSRF validation blocked") {
		t.Errorf("expected message to mention 'SSRF validation blocked', got: %s", resMeta.Message)
	}

	// 3. ProviderLocal on mock loopback server must be permitted through SSRF
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"local pong"}}]}`))
	}))
	defer mockServer.Close()

	resLocal, errLocal := tester.TestEndpoint(CustomModel{
		ID:           "test-local-ollama",
		Name:         "llama3.1",
		ProviderType: ProviderLocal,
		BaseURL:      mockServer.URL,
	})
	if errLocal != nil || !resLocal.Success {
		t.Errorf("expected TestEndpoint to succeed for ProviderLocal, got res=%+v, err=%v", resLocal, errLocal)
	}
}
