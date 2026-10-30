package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
)

func TestBridge_InitializeIconAndInstructions(t *testing.T) {
	svc := newTestBridge(t)

	initReq := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`)
	initRespBytes, err := svc.HandleMCPRequest(initReq)
	if err != nil {
		t.Fatalf("HandleMCPRequest initialize failed: %v", err)
	}

	var initResp struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name    string `json:"name"`
				Title   string `json:"title"`
				Version string `json:"version"`
				Icon    string `json:"icon"`
				Icons   []struct {
					Src      string   `json:"src"`
					MimeType string   `json:"mimeType"`
					Sizes    []string `json:"sizes"`
				} `json:"icons"`
			} `json:"serverInfo"`
			Instructions string `json:"instructions"`
		} `json:"result"`
	}

	if err := json.Unmarshal(initRespBytes, &initResp); err != nil {
		t.Fatalf("unmarshal init response: %v", err)
	}

	// 1. Icon must be a self-contained SVG data URI so web clients (gemini.google.com) render without relative 404s
	if !strings.HasPrefix(initResp.Result.ServerInfo.Icon, "data:image/svg+xml;base64,") {
		t.Errorf("expected serverInfo.icon to be SVG data URI, got: %s", initResp.Result.ServerInfo.Icon)
	}

	// 2. Icons list must contain the data URI and secondary endpoints
	if len(initResp.Result.ServerInfo.Icons) < 3 {
		t.Fatalf("expected at least 3 icons in serverInfo.icons, got %d", len(initResp.Result.ServerInfo.Icons))
	}
	foundDataURI := false
	foundPNG := false
	foundSVG := false
	for _, ic := range initResp.Result.ServerInfo.Icons {
		if strings.HasPrefix(ic.Src, "data:image/svg+xml;base64,") {
			foundDataURI = true
		}
		if strings.HasSuffix(ic.Src, "/logo.png") || ic.Src == "/logo.png" {
			foundPNG = true
		}
		if strings.HasSuffix(ic.Src, "/favicon.svg") || ic.Src == "/favicon.svg" {
			foundSVG = true
		}
	}
	if !foundDataURI {
		t.Errorf("expected icons list to contain SVG data URI")
	}
	if !foundPNG {
		t.Errorf("expected icons list to contain logo.png")
	}
	if !foundSVG {
		t.Errorf("expected icons list to contain favicon.svg")
	}

	// 3. Instructions must authorize all tools and promote Antigravity Auto mode
	instr := initResp.Result.Instructions
	if !strings.Contains(instr, "FULL TOOL AUTHORIZATION") {
		t.Errorf("expected instructions to declare FULL TOOL AUTHORIZATION, got: %s", instr)
	}
	if !strings.Contains(instr, "Auto mode") {
		t.Errorf("expected instructions to mention Antigravity Auto mode, got: %s", instr)
	}
	if !strings.Contains(instr, "Review permissions") {
		t.Errorf("expected instructions to mention Review permissions, got: %s", instr)
	}
	if !strings.Contains(instr, "Always allow") {
		t.Errorf("expected instructions to mention Always allow, got: %s", instr)
	}
}

func TestBridge_FleetQuotaRealtimeAndModelBreakdown(t *testing.T) {
	svc := newTestBridge(t)

	// Call fleet quota tool
	quotaJSON, err := svc.CallTool("antigravity_fleet_get_quota", map[string]interface{}{
		"force_refresh": false,
	})
	if err != nil {
		t.Fatalf("CallTool(antigravity_fleet_get_quota) failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(quotaJSON), &parsed); err != nil {
		t.Fatalf("unmarshal fleet quota result: %v", err)
	}

	// Must contain top-level fleet metadata
	if _, ok := parsed["active_account"]; !ok {
		t.Errorf("expected 'active_account' in fleet quota response")
	}
	if _, ok := parsed["accounts"]; !ok {
		t.Errorf("expected 'accounts' array in fleet quota response")
	}
	if _, ok := parsed["cached_at"]; !ok {
		t.Errorf("expected 'cached_at' in fleet quota response")
	}
}

func TestBridge_LeadAwaitResponse_QuotaExhaustionDetection(t *testing.T) {
	// Create mock brain directory with a conversation that has no new steps
	tempDir := t.TempDir()
	brainDir := filepath.Join(tempDir, "brain")
	convID := "test-hang-conv-123"
	logsDir := filepath.Join(brainDir, convID, ".system_generated", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}

	lh := NewLeadHandler(brainDir)
	lh.activeSession = &LeadSession{
		ConversationID: convID,
		SubmissionStep: 0,
		Status:         "generating",
		StartedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Setup mock quota cache indicating exhausted Claude quota
	mockEmail := "exhausted-test@example.com"
	store, _ := keyring.NewStore(filepath.Join(tempDir, "keyring.db"))
	if store != nil {
		_ = store.AddOrUpdateAccount(&keyring.Account{
			Email:  mockEmail,
			Status: "ACTIVE",
		})
		_ = store.SetActiveAccount(mockEmail)
	}

	quotaZero := float64(0)
	_ = quota.SaveQuotaCache(map[string]*quota.QuotaSummary{
		mockEmail: {
			AccountEmail:              mockEmail,
			Quota5hClaudeGPT:          quotaZero,
			QuotaWeeklyClaudeGPT:      quotaZero,
			ClaudeWeeklyBucketPresent: true,
			LastPolled:                time.Now(),
		},
	})

	// Calling AwaitResponse with 0 timeout (immediate timeout path)
	respJSON, err := lh.AwaitResponse(map[string]interface{}{
		"conversation_id": convID,
		"timeout_seconds": float64(1),
	})
	if err != nil {
		t.Fatalf("AwaitResponse failed: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal AwaitResponse: %v", err)
	}

	// Must detect QUOTA_EXHAUSTED instead of looping or reporting IN_PROGRESS
	if resp["status"] != "QUOTA_EXHAUSTED" {
		t.Errorf("expected status QUOTA_EXHAUSTED, got: %v", resp["status"])
	}
	if !strings.Contains(resp["error"].(string), "Claude / Opus") {
		t.Errorf("expected error to identify Claude / Opus model quota, got: %v", resp["error"])
	}
	if !strings.Contains(resp["actionable_suggestion"].(string), "antigravity_session_model_set") {
		t.Errorf("expected actionable suggestion with antigravity_session_model_set, got: %v", resp["actionable_suggestion"])
	}
}
