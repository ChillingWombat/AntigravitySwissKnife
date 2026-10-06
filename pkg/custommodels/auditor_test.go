package custommodels

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuditorRiskAssessmentGrades(t *testing.T) {
	aud := NewAuditor()

	tests := []struct {
		probes        []SecurityAuditProbe
		expectedGrade string
		expectedLevel string
	}{
		{
			probes: []SecurityAuditProbe{
				{Status: "passed"},
				{Status: "passed"},
			},
			expectedGrade: "A+",
			expectedLevel: "low",
		},
		{
			probes: []SecurityAuditProbe{
				{Status: "warning"},
			},
			expectedGrade: "A",
			expectedLevel: "medium",
		},
		{
			probes: []SecurityAuditProbe{
				{Status: "warning"},
				{Status: "warning"},
			},
			expectedGrade: "B",
			expectedLevel: "medium",
		},
		{
			probes: []SecurityAuditProbe{
				{Status: "failed"},
			},
			expectedGrade: "C",
			expectedLevel: "high",
		},
		{
			probes: []SecurityAuditProbe{
				{Status: "failed"},
				{Status: "failed"},
			},
			expectedGrade: "D",
			expectedLevel: "critical",
		},
		{
			probes: []SecurityAuditProbe{
				{Status: "failed"},
				{Status: "failed"},
				{Status: "failed"},
			},
			expectedGrade: "F",
			expectedLevel: "critical",
		},
	}

	for i, tt := range tests {
		report := &SecurityAuditReport{
			Probes: tt.probes,
		}
		aud.computeRiskAssessment(report)

		if report.SecurityGrade != tt.expectedGrade {
			t.Errorf("test %d: expected grade %q, got %q (score=%d)", i, tt.expectedGrade, report.SecurityGrade, report.RiskScore)
		}
		if report.RiskLevel != tt.expectedLevel {
			t.Errorf("test %d: expected level %q, got %q", i, tt.expectedLevel, report.RiskLevel)
		}
	}
}

func TestAuditorOriginLineage(t *testing.T) {
	aud := NewAuditor()

	// 1. Mock proxy server with CF-Ray and Via headers
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("CF-Ray", "8923749823749-SJC")
		w.Header().Set("Via", "1.1 cloudflare")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer ts.Close()

	modelRelay := CustomModel{
		BaseURL: ts.URL,
	}

	probeRelay, _, reachable := aud.probeOriginLineage(modelRelay, ts.URL)
	if !reachable {
		t.Errorf("expected test server to be reachable")
	}
	if probeRelay.Status != "warning" {
		t.Errorf("expected proxy headers to trigger warning, got %s", probeRelay.Status)
	}
}

func TestAuditorCredentialHeaderLeakage(t *testing.T) {
	aud := NewAuditor()
	model := CustomModel{
		BaseURL: "https://mock.example.com/v1",
		APIKey:  "sk-test-secret-key-1234567890",
	}

	// Leaking header
	leakingHeaders := http.Header{}
	leakingHeaders.Set("X-Forwarded-Token", "Bearer sk-test-secret-key-1234567890")

	probe := aud.probeCredentialLeakage(model, leakingHeaders)
	if probe.Status != "failed" {
		t.Errorf("expected leaked token in header to fail probe, got %s", probe.Status)
	}

	// Clean headers
	cleanHeaders := http.Header{}
	cleanHeaders.Set("Content-Type", "application/json")
	cleanProbe := aud.probeCredentialLeakage(model, cleanHeaders)
	if cleanProbe.Status != "passed" {
		t.Errorf("expected clean headers to pass, got %s", cleanProbe.Status)
	}
}
