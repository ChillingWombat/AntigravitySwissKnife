package custommodels

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInspectAndSanitizeCloudCodeRequest_NeutralizesRFC2119AndPinsAccount(t *testing.T) {
	rawBody := `{
		"model": "gemini-4-hydrogen-high",
		"request": {
			"systemInstruction": {
				"parts": [
					{"text": "RFC 2119: The key words MUST and SHALL are to be interpreted as described in RFC 2119."}
				]
			},
			"contents": [
				{
					"role": "user",
					"parts": [
						{"text": "Please follow rfc-2119: strict rules from RFC 2119 in responses."}
					]
				}
			]
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1internal:generateContent", strings.NewReader(rawBody))
	req.Header.Set("User-Agent", "antigravity/2.32.1 linux/amd64")

	res := InspectAndSanitizeCloudCodeRequest(req, []byte(rawBody), ProxyScopeAntigravity)
	if res.Block {
		t.Fatalf("unexpected block for legitimate IDE request: %+v", res)
	}
	if !res.Sanitized {
		t.Fatalf("expected RFC 2119 markers to trigger sanitization")
	}
	if !res.PinSingleAccount {
		t.Fatalf("expected sanitized request to set PinSingleAccount=true")
	}
	if len(res.SanitizedMarkers) == 0 || res.SanitizedMarkers[0] != "rfc_2119" {
		t.Fatalf("unexpected SanitizedMarkers: %v", res.SanitizedMarkers)
	}

	out := string(res.Body)
	if strings.Contains(strings.ToLower(out), "rfc 2119") || strings.Contains(strings.ToLower(out), "rfc-2119") {
		t.Fatalf("expected all RFC 2119 markers to be neutralized, got: %s", out)
	}
	if !json.Valid(res.Body) {
		t.Fatalf("sanitized body must remain valid JSON: %s", out)
	}
}

func TestInspectAndSanitizeCloudCodeRequest_BlocksThirdPartyClients(t *testing.T) {
	t.Run("BlocksThirdPartyUserAgentOnV1Internal", func(t *testing.T) {
		for _, ua := range []string{"cline/1.2.0", "opencode/0.1", "cpa-proxy/1.0", "Roo-Code/3.0"} {
			req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent", nil)
			req.Header.Set("User-Agent", ua)
			res := InspectAndSanitizeCloudCodeRequest(req, []byte(`{"model":"gemini-3.8-flash"}`), ProxyScopeAntigravity)
			if !res.Block || res.BlockStatus != http.StatusForbidden || res.BlockReason != "third_party_client_blocked" {
				t.Fatalf("UA %q expected 403 third_party_client_blocked, got %+v", ua, res)
			}
		}
	})

	t.Run("BlocksCPAAndOneAPIHeaders", func(t *testing.T) {
		for _, hdr := range []string{"X-CPA-Pool", "X-OneAPI-Request-Id", "X-New-API-Token"} {
			req := httptest.NewRequest(http.MethodPost, "/v1internal:generateContent", nil)
			req.Header.Set(hdr, "active")
			res := InspectAndSanitizeCloudCodeRequest(req, []byte(`{"model":"gemini-3.8-flash"}`), ProxyScopeAntigravity)
			if !res.Block || res.BlockStatus != http.StatusForbidden {
				t.Fatalf("header %q expected 403 block, got %+v", hdr, res)
			}
		}
	})

	t.Run("AllowsAntigravityIDEAndEmptyUA", func(t *testing.T) {
		for _, ua := range []string{"", "antigravity/2.32.1 linux/amd64"} {
			req := httptest.NewRequest(http.MethodPost, "/v1internal:generateContent", nil)
			if ua != "" {
				req.Header.Set("User-Agent", ua)
			}
			res := InspectAndSanitizeCloudCodeRequest(req, []byte(`{"model":"gemini-3.8-flash"}`), ProxyScopeAntigravity)
			if res.Block {
				t.Fatalf("UA %q should not be blocked, got %+v", ua, res)
			}
		}
	})
}

func TestClassifyUpstream429(t *testing.T) {
	t.Run("Headerless429WithoutQuotaDetailsIsAnomalousTrap", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{},
		}
		cls := ClassifyUpstream429(resp, []byte(`{"error":{"code":429,"message":"Too many requests"}}`))
		if cls.Kind != RateLimit429AnomalousTrap {
			t.Fatalf("expected RateLimit429AnomalousTrap, got %+v", cls)
		}
	})

	t.Run("RetryAfter45HeaderIsGenuineQuota", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"45"}},
		}
		cls := ClassifyUpstream429(resp, []byte(`{"error":"rate limited on b"}`))
		if cls.Kind != RateLimit429GenuineQuota || !cls.HasRetryAfter || cls.RetryAfter.Seconds() != 45 {
			t.Fatalf("expected genuine quota 429 with 45s RetryAfter, got %+v", cls)
		}
	})

	t.Run("RFC2119PolicyMessageEvenWithRetryAfterIsAnomalousTrap", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"30"}},
		}
		cls := ClassifyUpstream429(resp, []byte(`{"error":{"code":429,"message":"Rejected due to RFC 2119 policy violation"}}`))
		if cls.Kind != RateLimit429AnomalousTrap {
			t.Fatalf("expected RateLimit429AnomalousTrap for policy message, got %+v", cls)
		}
	})

	t.Run("StructuredQuotaDetailsWithoutHeaderIsGenuineQuota", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{},
		}
		body := `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"RATE_LIMIT_EXCEEDED"}]}}`
		cls := ClassifyUpstream429(resp, []byte(body))
		if cls.Kind != RateLimit429GenuineQuota || !cls.HasQuotaDetails {
			t.Fatalf("expected genuine quota when ErrorInfo reason=RATE_LIMIT_EXCEEDED, got %+v", cls)
		}
	})
}
