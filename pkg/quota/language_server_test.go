package quota

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseLanguageServerCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCSRF   string
		wantPort   int
		wantAccept bool
	}{
		{
			name: "separate arguments",
			args: []string{
				"/opt/Antigravity/resources/bin/language_server",
				"--override_ide_name", "antigravity",
				"--https_server_port", "44901",
				"--csrf_token", "local-token",
			},
			wantCSRF:   "local-token",
			wantPort:   44901,
			wantAccept: true,
		},
		{
			name: "equals arguments",
			args: []string{
				"language_server",
				"--subclient_type=hub",
				"--https_server_port=0",
				"--csrf_token=another-token",
			},
			wantCSRF:   "another-token",
			wantPort:   0,
			wantAccept: true,
		},
		{
			name: "not antigravity",
			args: []string{"language_server", "--csrf_token", "token"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csrf, port, accepted := parseLanguageServerCommand(tt.args)
			if accepted != tt.wantAccept || csrf != tt.wantCSRF || port != tt.wantPort {
				t.Fatalf("parseLanguageServerCommand() = (%q, %d, %v), want (%q, %d, %v)",
					csrf, port, accepted, tt.wantCSRF, tt.wantPort, tt.wantAccept)
			}
		})
	}
}

func TestDecodeGRPCWebMessageSkipsTrailer(t *testing.T) {
	payload := []byte(`{"groups":[]}`)
	frame := grpcWebFrame(payload)
	trailer := []byte{0x80, 0, 0, 0, 2, 'O', 'K'}
	got, err := decodeGRPCWebMessage(append(frame, trailer...))
	if err != nil {
		t.Fatalf("decodeGRPCWebMessage() error = %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("decodeGRPCWebMessage() = %q, want %q", got, payload)
	}
}

func TestFetchArgonQuotaFromLanguageServer(t *testing.T) {
	const csrfToken = "test-local-csrf-token"
	resetTime := "2026-10-28T07:58:15Z"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-codeium-csrf-token"); got != csrfToken {
			t.Errorf("csrf header = %q, want %q", got, csrfToken)
		}
		requestPayload := readGRPCWebRequest(t, r)
		if string(requestPayload) != `{}` {
			t.Errorf("request payload = %q, want {}", requestPayload)
		}

		w.Header().Set("Content-Type", "application/grpc-web+json")
		switch r.URL.Path {
		case languageServerUserStatusMethod:
			writeGRPCWebJSON(t, w, map[string]any{
				"userStatus": map[string]any{"email": "active@example.com"},
			})
		case languageServerQuotaSummaryMethod:
			writeGRPCWebJSON(t, w, map[string]any{
				"response": map[string]any{
					"groups": []any{
						map[string]any{
							"displayName": "Gemini 4 Argon",
							"buckets": []any{
								map[string]any{
									"bucketId":          "gemini-argon-5h",
									"displayName":       "Gemini 4 Argon Five Hour Limit",
									"window":            "5h",
									"remainingFraction": 0.64,
									"resetTime":         "2026-10-21T16:00:00Z",
								},
								map[string]any{
									"bucketId":          "gemini-argon-wtu-weekly",
									"displayName":       "Gemini 4 Argon Weekly Limit",
									"window":            "weekly",
									"remainingFraction": 0.7717351,
									"resetTime":         resetTime,
								},
							},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	got, err := fetchArgonQuotaFromEndpoint(languageServerEndpoint{
		origin:    server.URL,
		csrfToken: csrfToken,
	}, "active@example.com", time.Now())
	if err != nil {
		t.Fatalf("fetchArgonQuotaFromEndpoint() error = %v", err)
	}
	if got.FiveHour == nil || *got.FiveHour != 0.64 {
		t.Errorf("5H fraction = %v, want 0.64", got.FiveHour)
	}
	if got.Weekly == nil || *got.Weekly != 0.7717351 {
		t.Errorf("weekly fraction = %v, want 0.7717351", got.Weekly)
	}
	if got.WeeklyResetTime.Format(time.RFC3339) != resetTime {
		t.Errorf("weekly reset time = %v, want %s", got.WeeklyResetTime, resetTime)
	}
}

func TestApplyLanguageServerArgon(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(4 * time.Hour)
	reset := now.Add(48 * time.Hour)
	remaining5h := 0.64
	remaining := 0.77
	breakdown := &LiveQuotaBreakdown{
		QuotaWeeklyGemini: 0.89,
		Models: []ModelQuota{{
			ModelName: "Weekly Limit Remaining",
			Fraction:  0.89,
		}},
	}

	applyLanguageServerArgon(breakdown, &languageServerArgonQuota{
		FiveHour:          &remaining5h,
		FiveHourResetTime: reset5h,
		Weekly:            &remaining,
		WeeklyResetTime:   reset,
	}, now)

	if breakdown.Quota5hArgon == nil || *breakdown.Quota5hArgon != remaining5h {
		t.Fatalf("5H argon = %v, want %v", breakdown.Quota5hArgon, remaining5h)
	}
	if breakdown.QuotaWeeklyArgon == nil || *breakdown.QuotaWeeklyArgon != remaining {
		t.Fatalf("weekly argon = %v, want %v", breakdown.QuotaWeeklyArgon, remaining)
	}
	if breakdown.QuotaWeeklyGemini != 0.89 {
		t.Fatalf("Gemini weekly quota was overwritten: %v", breakdown.QuotaWeeklyGemini)
	}
	if len(breakdown.Models) != 3 {
		t.Fatalf("expected separate Argon 5H and weekly models, got %+v", breakdown.Models)
	}
	if breakdown.Models[1].ModelName != "Gemini 4 Argon 5H" || breakdown.Models[2].ModelName != "Gemini 4 Argon Weekly" {
		t.Fatalf("Argon model rows were not appended correctly: %+v", breakdown.Models)
	}
}

func readGRPCWebRequest(t *testing.T, r *http.Request) []byte {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	if len(body) < 5 {
		t.Fatalf("request body too short: %d", len(body))
	}
	size := int(binary.BigEndian.Uint32(body[1:5]))
	if body[0] != 0 || size > len(body)-5 {
		t.Fatalf("invalid request frame: flag=%d size=%d body=%d", body[0], size, len(body))
	}
	return body[5 : 5+size]
}

func writeGRPCWebJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	_, _ = w.Write(grpcWebFrame(data))
}

func TestIssue293LanguageServerEndpointsCoverHTTPAndHTTPS(t *testing.T) {
	endpoints := languageServerEndpointsForPorts("csrf", []int{43123, 43123})
	if len(endpoints) != 2 {
		t.Fatalf("endpoint count = %d, want 2", len(endpoints))
	}
	if endpoints[0].origin != "https://127.0.0.1:43123" || endpoints[1].origin != "http://127.0.0.1:43123" {
		t.Fatalf("endpoint schemes = %q, %q", endpoints[0].origin, endpoints[1].origin)
	}
	if endpoints[0].csrfToken != "csrf" || endpoints[1].csrfToken != "csrf" {
		t.Fatal("language server CSRF token was not retained")
	}
}
