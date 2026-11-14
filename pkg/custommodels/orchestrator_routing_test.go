package custommodels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type stubAccountProvider struct {
	accounts    []*FleetAccount
	activeEmail string
	quotas      map[string]*FleetQuotaSummary
	tokenErrFor map[string]error
	refreshSeq  int
}

func (s *stubAccountProvider) ListAccounts() []*FleetAccount {
	return s.accounts
}

func (s *stubAccountProvider) ActiveEmail() string {
	return s.activeEmail
}

func (s *stubAccountProvider) LoadQuotaSummaries() map[string]*FleetQuotaSummary {
	return s.quotas
}

func (s *stubAccountProvider) EnsureFreshToken(acc *FleetAccount) (string, error) {
	if acc == nil {
		return "", context.DeadlineExceeded
	}
	if s.tokenErrFor != nil {
		if err, ok := s.tokenErrFor[strings.ToLower(acc.Email)]; ok {
			return "", err
		}
	}
	return acc.AccessToken, nil
}

type stubProjectResolver struct {
	projects map[string]string
	errFor   map[string]error
	calls    map[string]int
}

func (s *stubProjectResolver) ResolveProject(_ context.Context, acc *FleetAccount) (string, error) {
	if s.calls == nil {
		s.calls = make(map[string]int)
	}
	email := strings.ToLower(acc.Email)
	s.calls[email]++
	if s.errFor != nil {
		if err, ok := s.errFor[email]; ok {
			return "", err
		}
	}
	return s.projects[email], nil
}

func testFloatPtr(v float64) *float64 {
	return &v
}

func TestResolveModelPlaceholderMappings(t *testing.T) {
	cases := map[string]string{
		"MODEL_PLACEHOLDER_M318":        "gemini-3.8-flash-high",
		"MODEL_PLACEHOLDER_M319":        "gemini-3.8-flash-medium",
		"MODEL_PLACEHOLDER_M320":        "gemini-3.8-flash-low",
		"MODEL_PLACEHOLDER_M400":        "claude-opus-5-5-low",
		"MODEL_PLACEHOLDER_M401":        "claude-opus-5-5-medium",
		"MODEL_PLACEHOLDER_M402":        "claude-opus-5-5-high",
		"MODEL_PLACEHOLDER_M403":        "claude-sonnet-5-5-low",
		"MODEL_PLACEHOLDER_M404":        "claude-sonnet-5-5-medium",
		"MODEL_PLACEHOLDER_M405":        "claude-sonnet-5-5-high",
		"MODEL_PLACEHOLDER_M417":        "gemini-4-argon-high",
		"MODEL_PLACEHOLDER_M418":        "gemini-4-argon-medium",
		"MODEL_PLACEHOLDER_M419":        "gemini-4-argon-low",
		"MODEL_PLACEHOLDER_M525":        "gemini-4-hydrogen-high",
		"MODEL_PLACEHOLDER_M526":        "gemini-4-hydrogen-medium",
		"MODEL_PLACEHOLDER_M527":        "gemini-4-hydrogen-low",
		"MODEL_PLACEHOLDER_M528":        "gemini-4-hydrogen-xhigh",
		"MODEL_PLACEHOLDER_M668":        "gemini-pro-orchestrator-g1-pro",
		"models/MODEL_PLACEHOLDER_M668": "gemini-pro-orchestrator-g1-pro",
		"model_placeholder_m528":        "gemini-4-hydrogen-xhigh",
		"models/gemini-4-argon-high":    "gemini-4-argon-high",
	}
	for input, want := range cases {
		if got := ResolveModelPlaceholder(input); got != want {
			t.Errorf("ResolveModelPlaceholder(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStripEffortSuffix(t *testing.T) {
	cases := map[string]string{
		"Gemini 4 Hydrogen (XHigh)":  "Gemini 4 Hydrogen",
		"Gemini 4 Argon (High)":      "Gemini 4 Argon",
		"Claude Opus 5.5 (Extra)":    "Claude Opus 5.5",
		"Orchestrator (Argon)":       "Orchestrator (Argon)",
		"gemini-4-hydrogen-xhigh":    "gemini-4-hydrogen",
		"gemini-4-argon-medium":      "gemini-4-argon",
		"claude-sonnet-5-5-thinking": "claude-sonnet-5-5",
	}
	for input, want := range cases {
		if got := stripEffortSuffix(input); got != want {
			t.Errorf("stripEffortSuffix(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeCanonicalAndPricingGemini4Claude55(t *testing.T) {
	normCases := []struct {
		id       string
		name     string
		wantID   string
		wantName string
	}{
		{"MODEL_PLACEHOLDER_M528", "Gemini 4 Hydrogen (XHigh)", "gemini-4-hydrogen", "Gemini 4 Hydrogen"},
		{"gemini-4-hydrogen-high", "", "gemini-4-hydrogen", "Gemini 4 Hydrogen"},
		{"MODEL_PLACEHOLDER_M417", "Gemini 4 Argon (High)", "gemini-4-argon", "Gemini 4 Argon"},
		{"MODEL_PLACEHOLDER_M668", "Orchestrator (Argon)", "gemini-pro-orchestrator-g1-pro", "Orchestrator (Argon)"},
		{"gemini-pro-orchestrator-g1-pro", "", "gemini-pro-orchestrator-g1-pro", "Orchestrator (Argon)"},
		{"MODEL_PLACEHOLDER_M402", "Claude Opus 5.5 (High)", "claude-opus-5-5", "Claude Opus 5.5"},
		{"MODEL_PLACEHOLDER_M405", "Claude Sonnet 5.5 (High)", "claude-sonnet-5-5", "Claude Sonnet 5.5"},
	}
	for _, tc := range normCases {
		gotID, gotName := NormalizeCanonicalNativeModel(tc.id, tc.name)
		if gotID != tc.wantID || gotName != tc.wantName {
			t.Errorf("NormalizeCanonicalNativeModel(%q, %q) = (%q, %q), want (%q, %q)",
				tc.id, tc.name, gotID, gotName, tc.wantID, tc.wantName)
		}
	}

	priceCases := []struct {
		id         string
		wantIn     float64
		wantCached float64
		wantOut    float64
	}{
		{"gemini-4-argon-high", 2.00, 0.10, 10.00},
		{"MODEL_PLACEHOLDER_M418", 2.00, 0.10, 10.00},
		{"gemini-pro-orchestrator-g1-pro", 2.00, 0.10, 10.00},
		{"MODEL_PLACEHOLDER_M668", 2.00, 0.10, 10.00},
		{"gemini-4-hydrogen-xhigh", 0.75, 0.075, 3.75},
		{"MODEL_PLACEHOLDER_M525", 0.75, 0.075, 3.75},
		{"claude-opus-5-5-high", 15.00, 1.50, 75.00},
		{"MODEL_PLACEHOLDER_M400", 15.00, 1.50, 75.00},
		{"claude-sonnet-5-5-medium", 3.00, 0.30, 15.00},
		{"MODEL_PLACEHOLDER_M405", 3.00, 0.30, 15.00},
	}
	for _, tc := range priceCases {
		in, cached, out, ok := lookupOfficialProviderPricing(tc.id, "")
		if !ok || in == nil || cached == nil || out == nil {
			t.Fatalf("lookupOfficialProviderPricing(%q) failed: ok=%v", tc.id, ok)
		}
		if *in != tc.wantIn || *cached != tc.wantCached || *out != tc.wantOut {
			t.Errorf("lookupOfficialProviderPricing(%q) = (%.3f, %.3f, %.3f), want (%.3f, %.3f, %.3f)",
				tc.id, *in, *cached, *out, tc.wantIn, tc.wantCached, tc.wantOut)
		}
	}
}

func TestClassifyCloudCodeTurn(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantRole string
	}{
		{
			name: "root orchestrator with exit_plan_mode tool",
			body: `{
				"model": "MODEL_PLACEHOLDER_M668",
				"request": {
					"cascade_id": "root-1",
					"root_cascade_id": "root-1",
					"tools": [{"functionDeclarations": [{"name": "exit_plan_mode"}]}],
					"contents": [{"role": "user", "parts": [{"text": "plan feature"}]}]
				}
			}`,
			wantRole: TurnRoleRootOrchestrator,
		},
		{
			name: "root orchestrator via argon placeholder and plan_execute_protocol",
			body: `{
				"model": "MODEL_PLACEHOLDER_M417",
				"request": {
					"cascadeId": "root-2",
					"systemInstruction": {"parts": [{"text": "<plan_execute_protocol>rule</plan_execute_protocol>"}]},
					"contents": [{"role": "user", "parts": [{"text": "go"}]}]
				}
			}`,
			wantRole: TurnRoleRootOrchestrator,
		},
		{
			name: "subagent via cascade_id != root_cascade_id",
			body: `{
				"model": "gemini-4-hydrogen-high",
				"request": {
					"cascade_id": "child-1",
					"root_cascade_id": "root-1",
					"contents": [{"role": "user", "parts": [{"text": "implement slice"}]}]
				}
			}`,
			wantRole: TurnRoleSubagentOrSwarm,
		},
		{
			name: "subagent via subagent_reminder in contents",
			body: `{
				"model": "gemini-4-hydrogen-xhigh",
				"request": {
					"cascade_id": "sub-msg-1",
					"contents": [{"role": "user", "parts": [{"text": "<subagent_reminder>\nWork autonomously.\n</subagent_reminder>"}]}]
				}
			}`,
			wantRole: TurnRoleSubagentOrSwarm,
		},
		{
			name: "subagent via dispatched banner in systemInstruction",
			body: `{
				"model": "MODEL_PLACEHOLDER_M528",
				"request": {
					"systemInstruction": {"parts": [{"text": "You are dispatched by an agent, not the user."}]},
					"contents": [{"role": "user", "parts": [{"text": "check"}]}]
				}
			}`,
			wantRole: TurnRoleSubagentOrSwarm,
		},
		{
			name: "subagent via SYSTEM_MESSAGE sender tag",
			body: `{
				"model": "gemini-3.8-flash-high",
				"request": {
					"contents": [{"role": "user", "parts": [{"text": "<SYSTEM_MESSAGE>\n[Message] timestamp=2026 sender=parent-agent content=do work"}]}]
				}
			}`,
			wantRole: TurnRoleSubagentOrSwarm,
		},
		{
			name: "native swarm worker via read_board and swarm_wait tools",
			body: `{
				"model": "MODEL_PLACEHOLDER_M405",
				"request": {
					"cascade_id": "swarm-w1",
					"tools": [{"function_declarations": [{"name": "read_board"}, {"name": "swarm_wait"}]}],
					"contents": [{"role": "user", "parts": [{"text": "wait for board"}]}]
				}
			}`,
			wantRole: TurnRoleSubagentOrSwarm,
		},
		{
			name: "post plan execution via exit_plan_mode functionCall/functionResponse",
			body: `{
				"model": "MODEL_PLACEHOLDER_M668",
				"request": {
					"cascade_id": "root-1",
					"root_cascade_id": "root-1",
					"tools": [{"functionDeclarations": [{"name": "exit_plan_mode"}]}],
					"contents": [
						{"role": "model", "parts": [{"functionCall": {"name": "exit_plan_mode", "args": {}}}]},
						{"role": "user", "parts": [{"functionResponse": {"name": "exit_plan_mode", "response": {"approved": true}}}]}
					]
				}
			}`,
			wantRole: TurnRolePostPlanExecution,
		},
		{
			name: "standard turn without orchestrator or subagent markers",
			body: `{
				"model": "MODEL_PLACEHOLDER_M525",
				"request": {
					"cascade_id": "chat-1",
					"root_cascade_id": "chat-1",
					"contents": [{"role": "user", "parts": [{"text": "explain go interfaces"}]}]
				}
			}`,
			wantRole: TurnRoleStandard,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotRole, _, _, _ := ClassifyCloudCodeTurn([]byte(tc.body))
			if gotRole != tc.wantRole {
				t.Errorf("ClassifyCloudCodeTurn role = %q, want %q", gotRole, tc.wantRole)
			}
		})
	}
}

func TestModelQuotaBucket(t *testing.T) {
	cases := map[string]string{
		"gemini-4-argon-high":            "argon",
		"gemini-pro-orchestrator-g1-pro": "argon",
		"MODEL_PLACEHOLDER_M417":         "argon",
		"MODEL_PLACEHOLDER_M419":         "argon",
		"MODEL_PLACEHOLDER_M668":         "argon",
		"claude-opus-5-5-high":           "claude_gpt",
		"claude-sonnet-5-5-medium":       "claude_gpt",
		"gpt-5.4":                        "claude_gpt",
		"MODEL_PLACEHOLDER_M400":         "claude_gpt",
		"MODEL_PLACEHOLDER_M405":         "claude_gpt",
		"MODEL_OPENAI_GPT_5":             "claude_gpt",
		"gemini-4-hydrogen-xhigh":        "gemini",
		"gemini-3.8-flash-high":          "gemini",
		"MODEL_PLACEHOLDER_M525":         "gemini",
		"MODEL_PLACEHOLDER_M528":         "gemini",
		"MODEL_PLACEHOLDER_M318":         "gemini",
	}
	for model, want := range cases {
		if got := modelQuotaBucket(model); got != want {
			t.Errorf("modelQuotaBucket(%q) = %q, want %q", model, got, want)
		}
	}
}

func TestFleetSubagentRouterSelectionAndAffinity(t *testing.T) {
	now := time.Now()
	provider := &stubAccountProvider{
		activeEmail: "a@example.com",
		accounts: []*FleetAccount{
			{Email: "a@example.com", AccessToken: "tok-a", RefreshToken: "ref-a", IsActive: true},
			{Email: "b@example.com", AccessToken: "tok-b", RefreshToken: "ref-b"},
			{Email: "c@example.com", AccessToken: "tok-c", RefreshToken: "ref-c"},
		},
		quotas: map[string]*FleetQuotaSummary{
			"a@example.com": {Quota5hArgon: testFloatPtr(0.95), Quota5hFraction: 0.90, Quota5hClaudeGPT: 0.90, LastPolled: now},
			"b@example.com": {Quota5hArgon: testFloatPtr(0.90), Quota5hFraction: 0.90, Quota5hClaudeGPT: 0.90, LastPolled: now},
			"c@example.com": {Quota5hArgon: testFloatPtr(0.90), Quota5hFraction: 0.90, Quota5hClaudeGPT: 0.90, LastPolled: now},
		},
	}
	projects := &stubProjectResolver{
		projects: map[string]string{
			"a@example.com": "proj-a",
			"b@example.com": "proj-b",
			"c@example.com": "proj-c",
		},
	}
	router := NewFleetSubagentRouter(provider, projects)

	// 1. Root orchestrator stays on active account A.
	rootAcc, err := router.SelectAccountForTurn(TurnRoleRootOrchestrator, "root-1", "root-1", "MODEL_PLACEHOLDER_M668", "a@example.com")
	if err != nil {
		t.Fatalf("root SelectAccountForTurn: %v", err)
	}
	if rootAcc.Email != "a@example.com" {
		t.Fatalf("root expected a@example.com, got %s", rootAcc.Email)
	}
	if got := router.GetRootAccount("root-1"); got != "a@example.com" {
		t.Fatalf("GetRootAccount = %q, want a@example.com", got)
	}

	// 2. Concurrent subagents fan out across B and C (never A while B & C are healthy).
	sub1, err := router.SelectAccountForTurn(TurnRoleSubagentOrSwarm, "sub-1", "root-1", "gemini-4-hydrogen-high", "a@example.com")
	if err != nil {
		t.Fatalf("sub-1 SelectAccountForTurn: %v", err)
	}
	sub2, err := router.SelectAccountForTurn(TurnRoleSubagentOrSwarm, "sub-2", "root-1", "gemini-4-hydrogen-high", "a@example.com")
	if err != nil {
		t.Fatalf("sub-2 SelectAccountForTurn: %v", err)
	}
	if sub1.Email == "a@example.com" || sub2.Email == "a@example.com" {
		t.Fatalf("subagents should offload away from root a@example.com, got %s and %s", sub1.Email, sub2.Email)
	}
	if sub1.Email == sub2.Email {
		t.Fatalf("subagents should spread round-robin across B and C, both got %s", sub1.Email)
	}

	// 3. Sticky cascade_id affinity holds on Turn 2.
	sub1Turn2, err := router.SelectAccountForTurn(TurnRoleSubagentOrSwarm, "sub-1", "root-1", "gemini-4-hydrogen-high", "a@example.com")
	if err != nil {
		t.Fatalf("sub-1 turn 2: %v", err)
	}
	if sub1Turn2.Email != sub1.Email {
		t.Fatalf("sticky affinity violated for sub-1: turn1=%s, turn2=%s", sub1.Email, sub1Turn2.Email)
	}
	if router.GetConvAccount("sub-1") != sub1.Email {
		t.Fatalf("GetConvAccount(sub-1) = %q, want %q", router.GetConvAccount("sub-1"), sub1.Email)
	}

	// 4. ResolveCredentials fetches project once and caches it.
	tok, proj, err := router.ResolveCredentials(context.Background(), sub1)
	if err != nil || tok != sub1.AccessToken || proj != projects.projects[strings.ToLower(sub1.Email)] {
		t.Fatalf("ResolveCredentials = (%q, %q, %v)", tok, proj, err)
	}
	_, proj2, err := router.ResolveCredentials(context.Background(), sub1)
	if err != nil || proj2 != proj {
		t.Fatalf("cached ResolveCredentials = (%q, %v)", proj2, err)
	}
	if calls := projects.calls[strings.ToLower(sub1.Email)]; calls != 1 {
		t.Fatalf("expected ResolveProject called once (cached), got %d", calls)
	}

	// 5. Post-plan execution honors OffloadPostPlanExecution toggle.
	router.OffloadPostPlanExecution = false
	postStay, err := router.SelectAccountForTurn(TurnRolePostPlanExecution, "root-1", "root-1", "MODEL_PLACEHOLDER_M668", "a@example.com")
	if err != nil || postStay.Email != "a@example.com" {
		t.Fatalf("OffloadPostPlanExecution=false expected a@example.com, got %v, %v", postStay, err)
	}
	router.OffloadPostPlanExecution = true
	postOffload, err := router.SelectAccountForTurn(TurnRolePostPlanExecution, "post-exec-1", "root-1", "MODEL_PLACEHOLDER_M668", "a@example.com")
	if err != nil || postOffload.Email == "a@example.com" {
		t.Fatalf("OffloadPostPlanExecution=true expected non-A account, got %v, %v", postOffload, err)
	}

	// 6. Cooldown on sub1 triggers failover to the other worker on next sub1 call.
	router.MarkCooldown(sub1.Email, "gemini-4-hydrogen-high", time.Minute)
	sub1Failover, err := router.SelectAccountForTurn(TurnRoleSubagentOrSwarm, "sub-1", "root-1", "gemini-4-hydrogen-high", "a@example.com")
	if err != nil {
		t.Fatalf("failover SelectAccountForTurn: %v", err)
	}
	if sub1Failover.Email != sub2.Email {
		t.Fatalf("expected failover from %s to %s, got %s", sub1.Email, sub2.Email, sub1Failover.Email)
	}

	// 7. Cooldown/exhausted C leaves only A -> gracefully falls back to A.
	router.MarkCooldown(sub2.Email, "gemini-4-hydrogen-high", time.Minute)
	fallbackA, err := router.SelectAccountForTurn(TurnRoleSubagentOrSwarm, "sub-3", "root-1", "gemini-4-hydrogen-high", "a@example.com")
	if err != nil || fallbackA.Email != "a@example.com" {
		t.Fatalf("expected fallback to a@example.com when workers on cooldown, got %v, %v", fallbackA, err)
	}

	// 8. Root fails over if A's argon bucket is exhausted <= 0.02 (and ignores BANNED accounts).
	provider.accounts[1].Status = "BANNED"
	provider.quotas["a@example.com"].Quota5hArgon = testFloatPtr(0.01)
	rootFailover, err := router.SelectAccountForTurn(TurnRoleRootOrchestrator, "root-2", "root-2", "MODEL_PLACEHOLDER_M668", "a@example.com")
	if err != nil {
		t.Fatalf("root failover SelectAccountForTurn: %v", err)
	}
	if rootFailover.Email != "c@example.com" {
		t.Fatalf("expected root failover to c@example.com (ignoring exhausted A and BANNED B), got %s", rootFailover.Email)
	}
}

func TestRewriteCloudCodeProject(t *testing.T) {
	in := []byte(`{"model":"gemini-4-argon-high","project":"old-proj","request":{"project":"old-req-proj","contents":[]}}`)
	out := rewriteCloudCodeProject(in, "proj-fleet-b")

	var parsed struct {
		Project string `json:"project"`
		Request struct {
			Project string `json:"project"`
		} `json:"request"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal rewritten body: %v", err)
	}
	if parsed.Project != "proj-fleet-b" || parsed.Request.Project != "proj-fleet-b" {
		t.Fatalf("unexpected rewritten projects: top=%q req=%q", parsed.Project, parsed.Request.Project)
	}

	// When request.project is absent, rewriteCloudCodeProject only writes top-level project.
	noNested := []byte(`{"model":"gemini-4-hydrogen-high","project":"old","request":{"contents":[]}}`)
	out2 := rewriteCloudCodeProject(noNested, "proj-fleet-c")
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out2, &raw); err != nil {
		t.Fatalf("unmarshal noNested: %v", err)
	}
	var reqMap map[string]interface{}
	if err := json.Unmarshal(raw["request"], &reqMap); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if _, exists := reqMap["project"]; exists {
		t.Fatalf("did not expect request.project to be injected when absent")
	}

	// Verify defaultProjectResolver parses loadCodeAssist via HTTP.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1internal:loadCodeAssist" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok-z" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"cloudaicompanionProject":"projects/real-companion-42"}`))
	}))
	defer srv.Close()

	resolver := newDefaultProjectResolver(func() string { return srv.URL })
	gotProj, err := resolver.ResolveProject(context.Background(), &FleetAccount{
		Email:       "z@example.com",
		AccessToken: "tok-z",
	})
	if err != nil {
		t.Fatalf("defaultProjectResolver.ResolveProject: %v", err)
	}
	if gotProj != "projects/real-companion-42" {
		t.Fatalf("ResolveProject = %q, want projects/real-companion-42", gotProj)
	}
}
