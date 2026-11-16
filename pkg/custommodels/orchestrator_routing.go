package custommodels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// Cloud Code turn classification roles for multi-account orchestrator routing.
const (
	TurnRoleRootOrchestrator  = "root_orchestrator"
	TurnRoleSubagentOrSwarm   = "subagent_or_swarm"
	TurnRolePostPlanExecution = "post_plan_execution"
	TurnRoleStandard          = "standard"
)

const (
	minHealthyQuotaFraction       = 0.02
	topHealthyPoolFloor           = 0.30
	defaultAccountCooldown        = 30 * time.Second
	defaultCircuitBreakerDuration = 10 * time.Minute
	defaultCorrelationWindow      = 15 * time.Second
)

// ClassifyCloudCodeTurn inspects a Cloud Code generateContent/streamGenerateContent
// JSON body and determines the turn's orchestration role along with its cascade
// identifiers and target model ID.
func ClassifyCloudCodeTurn(body []byte) (role string, cascadeID string, rootCascadeID string, modelID string) {
	if len(bytes.TrimSpace(body)) == 0 {
		return TurnRoleStandard, "", "", ""
	}

	var envelope struct {
		Model         string          `json:"model"`
		RequestID     string          `json:"requestId"`
		RequestIDS    string          `json:"request_id"`
		CascadeID     string          `json:"cascade_id"`
		CascadeIDC    string          `json:"cascadeId"`
		RootCascadeID string          `json:"root_cascade_id"`
		RootCascadeC  string          `json:"rootCascadeId"`
		Request       json.RawMessage `json:"request"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return TurnRoleStandard, "", "", ""
	}

	var req struct {
		Model             string            `json:"model"`
		CascadeID         string            `json:"cascade_id"`
		CascadeIDC        string            `json:"cascadeId"`
		RootCascadeID     string            `json:"root_cascade_id"`
		RootCascadeC      string            `json:"rootCascadeId"`
		SystemInstruction json.RawMessage   `json:"systemInstruction"`
		SystemInstrSnake  json.RawMessage   `json:"system_instruction"`
		Contents          []json.RawMessage `json:"contents"`
		Tools             []json.RawMessage `json:"tools"`
	}
	if len(bytes.TrimSpace(envelope.Request)) > 0 {
		_ = json.Unmarshal(envelope.Request, &req)
	}

	modelID = firstNonEmpty(envelope.Model, req.Model)
	cascadeID = firstNonEmpty(req.CascadeID, req.CascadeIDC, envelope.CascadeID, envelope.CascadeIDC)
	rootCascadeID = firstNonEmpty(req.RootCascadeID, req.RootCascadeC, envelope.RootCascadeID, envelope.RootCascadeC)
	if cascadeID == "" {
		reqID := firstNonEmpty(envelope.RequestID, envelope.RequestIDS)
		if m := uuidRegex.FindString(reqID); m != "" {
			cascadeID = m
		}
	}

	sysRaw := req.SystemInstruction
	if len(bytes.TrimSpace(sysRaw)) == 0 {
		sysRaw = req.SystemInstrSnake
	}
	sysText := extractInstructionText(sysRaw)
	contentTexts, hasExitPlanCallOrResp := inspectCloudCodeContents(req.Contents)
	declaredTools := collectDeclaredToolNames(req.Tools)

	isSubagent := false
	if cascadeID != "" && rootCascadeID != "" && cascadeID != rootCascadeID {
		isSubagent = true
	}
	if !isSubagent && isSubagentMessageText(sysText) {
		isSubagent = true
	}
	if !isSubagent {
		for _, text := range contentTexts {
			if isSubagentMessageText(text) {
				isSubagent = true
				break
			}
		}
	}
	if !isSubagent && (declaredTools["read_board"] || declaredTools["swarm_wait"]) {
		isSubagent = true
	}
	if isSubagent {
		return TurnRoleSubagentOrSwarm, cascadeID, rootCascadeID, modelID
	}

	if hasExitPlanCallOrResp {
		return TurnRolePostPlanExecution, cascadeID, rootCascadeID, modelID
	}

	if isOrchestratorModel(modelID) || declaredTools["exit_plan_mode"] || strings.Contains(sysText, "<plan_execute_protocol>") {
		return TurnRoleRootOrchestrator, cascadeID, rootCascadeID, modelID
	}

	return TurnRoleStandard, cascadeID, rootCascadeID, modelID
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func extractInstructionText(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ""
	}
	var asString string
	if json.Unmarshal(trimmed, &asString) == nil && asString != "" {
		return asString
	}
	var content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}
	if json.Unmarshal(trimmed, &content) != nil {
		return ""
	}
	var sb strings.Builder
	for _, part := range content.Parts {
		if part.Text != "" {
			sb.WriteString(part.Text)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func inspectCloudCodeContents(contents []json.RawMessage) (texts []string, hasExitPlan bool) {
	type partWire struct {
		Text                  string `json:"text"`
		FunctionCall          *struct{ Name string `json:"name"` } `json:"functionCall"`
		FunctionCallSnake     *struct{ Name string `json:"name"` } `json:"function_call"`
		FunctionResponse      *struct{ Name string `json:"name"` } `json:"functionResponse"`
		FunctionResponseSnake *struct{ Name string `json:"name"` } `json:"function_response"`
	}
	for _, rawContent := range contents {
		var item struct {
			Parts []partWire `json:"parts"`
		}
		if json.Unmarshal(rawContent, &item) != nil {
			continue
		}
		for _, p := range item.Parts {
			if p.Text != "" {
				texts = append(texts, p.Text)
			}
			for _, fn := range []*struct{ Name string `json:"name"` }{
				p.FunctionCall,
				p.FunctionCallSnake,
				p.FunctionResponse,
				p.FunctionResponseSnake,
			} {
				if fn != nil && strings.EqualFold(strings.TrimSpace(fn.Name), "exit_plan_mode") {
					hasExitPlan = true
				}
			}
		}
	}
	return texts, hasExitPlan
}

func collectDeclaredToolNames(tools []json.RawMessage) map[string]bool {
	names := make(map[string]bool)
	for _, rawTool := range tools {
		var decl struct {
			Name                 string `json:"name"`
			FunctionDeclarations []struct {
				Name string `json:"name"`
			} `json:"functionDeclarations"`
			FunctionDeclsSnake []struct {
				Name string `json:"name"`
			} `json:"function_declarations"`
		}
		if json.Unmarshal(rawTool, &decl) != nil {
			continue
		}
		if name := strings.TrimSpace(decl.Name); name != "" {
			names[name] = true
		}
		for _, fn := range decl.FunctionDeclarations {
			if name := strings.TrimSpace(fn.Name); name != "" {
				names[name] = true
			}
		}
		for _, fn := range decl.FunctionDeclsSnake {
			if name := strings.TrimSpace(fn.Name); name != "" {
				names[name] = true
			}
		}
	}
	return names
}

func isSubagentMessageText(text string) bool {
	if text == "" {
		return false
	}
	norm := strings.ReplaceAll(text, "\r\n", "\n")
	if strings.Contains(norm, "<subagent_reminder>") ||
		strings.Contains(norm, "You are running as a subagent") ||
		strings.Contains(norm, "You are dispatched by an agent, not the user") {
		return true
	}
	if strings.Contains(norm, "<SYSTEM_MESSAGE>\n[Message]") && strings.Contains(norm, "sender=") {
		return true
	}
	return false
}

func isOrchestratorModel(modelID string) bool {
	resolved := strings.ToLower(ResolveModelPlaceholder(modelID))
	raw := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(modelID), "models/"))
	return strings.HasPrefix(resolved, "gemini-pro-orchestrator") ||
		strings.HasPrefix(resolved, "gemini-4-argon") ||
		raw == "model_placeholder_m668" ||
		raw == "model_placeholder_m417" ||
		raw == "model_placeholder_m418" ||
		raw == "model_placeholder_m419"
}

func modelQuotaBucket(modelID string) string {
	resolved := strings.ToLower(ResolveModelPlaceholder(modelID))
	raw := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(modelID), "models/"))

	if strings.HasPrefix(resolved, "gemini-4-argon") ||
		strings.HasPrefix(resolved, "gemini-pro-orchestrator") ||
		strings.Contains(resolved, "argon") ||
		strings.Contains(raw, "m417") ||
		strings.Contains(raw, "m418") ||
		strings.Contains(raw, "m419") ||
		strings.Contains(raw, "m668") {
		return "argon"
	}

	if strings.HasPrefix(resolved, "claude-") ||
		strings.HasPrefix(resolved, "gpt-") ||
		strings.HasPrefix(resolved, "o1") ||
		strings.HasPrefix(resolved, "o3") ||
		strings.HasPrefix(raw, "model_openai_") ||
		strings.Contains(raw, "m400") ||
		strings.Contains(raw, "m401") ||
		strings.Contains(raw, "m402") ||
		strings.Contains(raw, "m403") ||
		strings.Contains(raw, "m404") ||
		strings.Contains(raw, "m405") {
		return "claude_gpt"
	}

	return "gemini"
}

const (
	fleetOAuthTokenURL     = "https://oauth2.googleapis.com/token"
	fleetOAuthClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	fleetOAuthClientSecret = "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"
)

var cloudCodeLoadProjectURLs = []string{
	"https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
	"https://daily-cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
	"https://cloudcodeassist-pa.googleapis.com/v1internal:loadCodeAssist",
	"https://cloudaicompanion.googleapis.com/v1internal:loadCodeAssist",
	"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:loadCodeAssist",
}

// FleetAccount holds the routing metadata and OAuth credentials for one fleet account.
type FleetAccount struct {
	Email        string    `json:"email"`
	Status       string    `json:"status,omitempty"`
	IsActive     bool      `json:"is_active"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenExpiry  time.Time `json:"token_expiry,omitempty"`
}

// FleetModelQuota mirrors per-model quota metrics persisted in quota_cache.json.
type FleetModelQuota struct {
	ModelName    string    `json:"model_name"`
	Fraction     float64   `json:"fraction"`
	ResetTime    time.Time `json:"reset_time"`
	ResetText    string    `json:"reset_text"`
	HealthStatus string    `json:"health_status"`
}

// FleetQuotaSummary mirrors cached per-account quota buckets in quota_cache.json.
type FleetQuotaSummary struct {
	AccountEmail              string            `json:"account_email"`
	Quota5hFraction           float64           `json:"quota_5h_fraction,omitempty"`
	Quota5hArgon              *float64          `json:"quota_5h_argon,omitempty"`
	QuotaWeeklyFraction       float64           `json:"quota_weekly_fraction,omitempty"`
	QuotaWeeklyArgon          *float64          `json:"quota_weekly_argon,omitempty"`
	Quota5hClaudeGPT          float64           `json:"quota_5h_claude_gpt,omitempty"`
	QuotaWeeklyClaudeGPT      float64           `json:"quota_weekly_claude_gpt,omitempty"`
	ClaudeWeeklyBucketPresent bool              `json:"claude_weekly_bucket_present,omitempty"`
	Models                    []FleetModelQuota `json:"models"`
	MinFraction               float64           `json:"min_fraction"`
	OverallHealth             string            `json:"overall_health"`
	LastPolled                time.Time         `json:"last_polled"`
}

// AccountProvider abstracts fleet account listing, active account lookup, quota cache access,
// and OAuth token refresh for FleetSubagentRouter.
type AccountProvider interface {
	ListAccounts() []*FleetAccount
	ActiveEmail() string
	LoadQuotaSummaries() map[string]*FleetQuotaSummary
	EnsureFreshToken(acc *FleetAccount) (string, error)
}

// ProjectResolver resolves the Cloud Code companion project ID for an account.
type ProjectResolver interface {
	ResolveProject(ctx context.Context, acc *FleetAccount) (string, error)
}

type cachedOAuthToken struct {
	accessToken string
	expiry      time.Time
}

type defaultAccountProvider struct {
	mu           sync.Mutex
	cachedTokens map[string]cachedOAuthToken
	httpClient   *http.Client
}

func newDefaultAccountProvider() *defaultAccountProvider {
	return &defaultAccountProvider{
		cachedTokens: make(map[string]cachedOAuthToken),
		httpClient:   &http.Client{Timeout: 8 * time.Second},
	}
}

func (p *defaultAccountProvider) loadAccountsSnapshot() (activeEmail string, accounts []*FleetAccount) {
	accountsPath := filepath.Join(core.GetConfigDir(), "accounts.json")
	data, err := os.ReadFile(accountsPath)
	if err != nil {
		return "", nil
	}

	var ff struct {
		ActiveAccount string          `json:"active_account"`
		Accounts      json.RawMessage `json:"accounts"`
	}
	if json.Unmarshal(data, &ff) != nil {
		return "", nil
	}
	activeEmail = strings.TrimSpace(ff.ActiveAccount)

	type rawCred struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Expiry       string `json:"expiry,omitempty"`
	}
	type rawAcc struct {
		Email        string   `json:"email"`
		Status       string   `json:"status,omitempty"`
		Notes        string   `json:"notes,omitempty"`
		AccessToken  string   `json:"access_token,omitempty"`
		RefreshToken string   `json:"refresh_token,omitempty"`
		TokenExpiry  string   `json:"token_expiry,omitempty"`
		Credential   *rawCred `json:"credential,omitempty"`
	}

	var rawItems []rawAcc
	trimmed := bytes.TrimSpace(ff.Accounts)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var accMap map[string]rawAcc
		if json.Unmarshal(ff.Accounts, &accMap) == nil {
			for email, item := range accMap {
				if item.Email == "" {
					item.Email = email
				}
				rawItems = append(rawItems, item)
			}
		}
	} else if len(trimmed) > 0 && trimmed[0] == '[' {
		_ = json.Unmarshal(ff.Accounts, &rawItems)
	}

	accounts = make([]*FleetAccount, 0, len(rawItems))
	for _, item := range rawItems {
		em := strings.TrimSpace(item.Email)
		if em == "" {
			continue
		}
		accessToken := core.DecryptCredential(item.AccessToken)
		refreshToken := core.DecryptCredential(item.RefreshToken)
		tokenExpiry := parseFleetTokenExpiry(item.TokenExpiry)
		if item.Credential != nil {
			if accessToken == "" && item.Credential.AccessToken != "" {
				accessToken = core.DecryptCredential(item.Credential.AccessToken)
			}
			if refreshToken == "" && item.Credential.RefreshToken != "" {
				refreshToken = core.DecryptCredential(item.Credential.RefreshToken)
			}
			if tokenExpiry.IsZero() && item.Credential.Expiry != "" {
				tokenExpiry = parseFleetTokenExpiry(item.Credential.Expiry)
			}
		}
		if strings.TrimSpace(refreshToken) == "" && item.Notes != "" {
			if rf, at, exp, ok := extractOAuthFromNotes(item.Notes); ok {
				refreshToken = rf
				if accessToken == "" && at != "" {
					accessToken = at
				}
				if tokenExpiry.IsZero() && !exp.IsZero() {
					tokenExpiry = exp
				}
			}
		}

		status := strings.ToUpper(strings.TrimSpace(item.Status))
		isActive := activeEmail != "" && strings.EqualFold(em, activeEmail)
		if status == "" {
			if isActive {
				status = "ACTIVE"
			} else {
				status = "STANDBY"
			}
		} else if status == "ACTIVE" && activeEmail == "" {
			isActive = true
		}

		acc := &FleetAccount{
			Email:        em,
			Status:       status,
			IsActive:     isActive,
			AccessToken:  strings.TrimSpace(accessToken),
			RefreshToken: strings.TrimSpace(refreshToken),
			TokenExpiry:  tokenExpiry,
		}
		accounts = append(accounts, acc)
	}
	return activeEmail, accounts
}

func parseFleetTokenExpiry(raw string) time.Time {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

func extractOAuthFromNotes(notes string) (refreshToken, accessToken string, expiry time.Time, ok bool) {
	trimmed := strings.TrimSpace(notes)
	if trimmed == "" || !strings.Contains(trimmed, "refresh_token") {
		return "", "", time.Time{}, false
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != '{' {
			continue
		}
		var m map[string]interface{}
		if json.Unmarshal([]byte(trimmed[i:]), &m) == nil {
			rf, _ := m["refresh_token"].(string)
			if strings.TrimSpace(rf) != "" {
				at, _ := m["access_token"].(string)
				if strings.TrimSpace(at) == "" {
					at, _ = m["token"].(string)
				}
				if expStr, _ := m["expiry"].(string); expStr != "" {
					expiry = parseFleetTokenExpiry(expStr)
				}
				return strings.TrimSpace(rf), strings.TrimSpace(at), expiry, true
			}
		}
	}
	return "", "", time.Time{}, false
}

func (p *defaultAccountProvider) ListAccounts() []*FleetAccount {
	_, accounts := p.loadAccountsSnapshot()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, acc := range accounts {
		key := strings.ToLower(strings.TrimSpace(acc.Email))
		if cached, ok := p.cachedTokens[key]; ok && cached.accessToken != "" {
			if acc.AccessToken == "" || (!cached.expiry.IsZero() && cached.expiry.After(acc.TokenExpiry)) {
				acc.AccessToken = cached.accessToken
				acc.TokenExpiry = cached.expiry
			}
		}
	}
	return accounts
}

func (p *defaultAccountProvider) ActiveEmail() string {
	active, _ := p.loadAccountsSnapshot()
	if active != "" {
		return active
	}
	if !core.IsRunningTests() {
		if cfg, err := core.LoadConfig(); err == nil && cfg != nil {
			if activeCfg := strings.TrimSpace(cfg.GetActiveAppAccount("antigravity")); activeCfg != "" {
				return activeCfg
			}
		}
	}
	return ""
}

func (p *defaultAccountProvider) LoadQuotaSummaries() map[string]*FleetQuotaSummary {
	path := filepath.Join(core.GetConfigDir(), "quota_cache.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]*FleetQuotaSummary)
	}
	var res map[string]*FleetQuotaSummary
	if json.Unmarshal(data, &res) != nil || res == nil {
		return make(map[string]*FleetQuotaSummary)
	}
	return res
}

func (p *defaultAccountProvider) EnsureFreshToken(acc *FleetAccount) (string, error) {
	if acc == nil {
		return "", fmt.Errorf("nil account")
	}
	key := strings.ToLower(strings.TrimSpace(acc.Email))

	p.mu.Lock()
	if cached, ok := p.cachedTokens[key]; ok && cached.accessToken != "" && !cached.expiry.IsZero() && time.Until(cached.expiry) > 5*time.Minute {
		p.mu.Unlock()
		return cached.accessToken, nil
	}
	p.mu.Unlock()

	if acc.AccessToken != "" && !acc.TokenExpiry.IsZero() && time.Until(acc.TokenExpiry) > 15*time.Minute {
		return acc.AccessToken, nil
	}

	rt := strings.TrimSpace(acc.RefreshToken)
	if rt != "" && !core.IsRunningTests() && strings.HasPrefix(rt, "1//") {
		form := url.Values{
			"client_id":     {fleetOAuthClientID},
			"client_secret": {fleetOAuthClientSecret},
			"refresh_token": {rt},
			"grant_type":    {"refresh_token"},
		}
		if resp, err := p.httpClient.PostForm(fleetOAuthTokenURL, form); err == nil {
			var tokResp struct {
				AccessToken string `json:"access_token"`
				ExpiresIn   int    `json:"expires_in"`
			}
			decErr := json.NewDecoder(resp.Body).Decode(&tokResp)
			_ = resp.Body.Close()
			if decErr == nil && strings.TrimSpace(tokResp.AccessToken) != "" {
				exp := time.Now().Add(55 * time.Minute)
				if tokResp.ExpiresIn > 120 {
					exp = time.Now().Add(time.Duration(tokResp.ExpiresIn-60) * time.Second)
				}
				fresh := strings.TrimSpace(tokResp.AccessToken)
				p.mu.Lock()
				p.cachedTokens[key] = cachedOAuthToken{accessToken: fresh, expiry: exp}
				p.mu.Unlock()
				return fresh, nil
			}
		}
	}

	if strings.TrimSpace(acc.AccessToken) == "" {
		return "", fmt.Errorf("no access token available for %s", acc.Email)
	}
	return acc.AccessToken, nil
}

type defaultProjectResolver struct {
	upstreamFn func() string
	client     *http.Client
}

func newDefaultProjectResolver(upstreamFn func() string) *defaultProjectResolver {
	return &defaultProjectResolver{
		upstreamFn: upstreamFn,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *defaultProjectResolver) ResolveProject(ctx context.Context, acc *FleetAccount) (string, error) {
	if acc == nil {
		return "", fmt.Errorf("nil account")
	}
	token := strings.TrimSpace(acc.AccessToken)
	if token == "" {
		return "", fmt.Errorf("empty access token for %s", acc.Email)
	}

	upstream := proxyUpstreamDefault
	if r.upstreamFn != nil {
		if u := strings.TrimSpace(r.upstreamFn()); u != "" {
			upstream = u
		}
	}

	endpoints := []string{strings.TrimRight(upstream, "/") + "/v1internal:loadCodeAssist"}
	if upstream == proxyUpstreamDefault {
		for _, ep := range cloudCodeLoadProjectURLs {
			if ep != endpoints[0] {
				endpoints = append(endpoints, ep)
			}
		}
	}

	payload := []byte(`{"metadata":{"ideType":"ANTIGRAVITY"}}`)
	var lastErr error
	for _, ep := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep, bytes.NewReader(payload))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "antigravity/2.32.1 linux/amd64")

		resp, err := r.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("loadCodeAssist HTTP %d for %s", resp.StatusCode, acc.Email)
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("loadCodeAssist HTTP %d", resp.StatusCode)
			continue
		}
		var parsed struct {
			Project      string `json:"cloudaicompanionProject"`
			ProjectSnake string `json:"cloudaicompanion_project"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			lastErr = err
			continue
		}
		if proj := firstNonEmpty(parsed.Project, parsed.ProjectSnake); proj != "" {
			return proj, nil
		}
		return "", nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("unable to resolve Cloud Code project for %s", acc.Email)
}

// FleetProtectionStatus reports anti-429 circuit-breaker state and request guard counters.
type FleetProtectionStatus struct {
	CircuitOpen            bool      `json:"circuit_open"`
	CircuitReason          string    `json:"circuit_reason,omitempty"`
	CircuitOpenUntil       string    `json:"circuit_open_until,omitempty"`
	Anomalous429Blocked    int       `json:"anomalous_429_blocked"`
	Correlated429Aborts    int       `json:"correlated_429_aborts"`
	SanitizedRequests      int       `json:"sanitized_requests"`
	BlockedExternalClients int       `json:"blocked_external_clients"`
	LastTrippedAt          time.Time `json:"-"`
}

// FleetSubagentRouter distributes Antigravity orchestrator subagent, swarm,
// and post-plan execution turns across healthy fleet accounts while keeping
// the root planning conversation on the active account.
type FleetSubagentRouter struct {
	mu              sync.RWMutex
	rrCounter       atomic.Uint64
	accounts        AccountProvider
	projects        ProjectResolver
	convAccount     map[string]string
	rootAccount     map[string]string
	accountProjects map[string]string
	cooldownUntil   map[string]time.Time

	OffloadPostPlanExecution bool
	DefaultCooldown          time.Duration
	CircuitBreakerDuration   time.Duration
	CorrelationWindow        time.Duration
	nowFn                    func() time.Time

	circuitOpenUntil       time.Time
	circuitReason          string
	last429At              time.Time
	last429Email           string
	anomalous429Blocked    int
	correlated429Aborts    int
	sanitizedRequests      int
	blockedExternalClients int
	lastTrippedAt          time.Time
}

// NewFleetSubagentRouter creates a FleetSubagentRouter backed by custom AccountProvider and ProjectResolver.
func NewFleetSubagentRouter(accounts AccountProvider, projects ProjectResolver) *FleetSubagentRouter {
	return &FleetSubagentRouter{
		accounts:                 accounts,
		projects:                 projects,
		convAccount:              make(map[string]string),
		rootAccount:              make(map[string]string),
		accountProjects:          make(map[string]string),
		cooldownUntil:            make(map[string]time.Time),
		OffloadPostPlanExecution: true,
		DefaultCooldown:          defaultAccountCooldown,
		CircuitBreakerDuration:   defaultCircuitBreakerDuration,
		CorrelationWindow:        defaultCorrelationWindow,
		nowFn:                    time.Now,
	}
}

// NewDefaultFleetSubagentRouter creates a FleetSubagentRouter backed by the local keyring store,
// quota cache, and /v1internal:loadCodeAssist on upstreamFn.
func NewDefaultFleetSubagentRouter(upstreamFn func() string) *FleetSubagentRouter {
	return NewFleetSubagentRouter(newDefaultAccountProvider(), newDefaultProjectResolver(upstreamFn))
}

func (r *FleetSubagentRouter) now() time.Time {
	if r.nowFn != nil {
		return r.nowFn()
	}
	return time.Now()
}

func cooldownKey(email, bucket string) string {
	return strings.ToLower(strings.TrimSpace(email)) + "|" + bucket
}

// SetAccountProject caches a pre-resolved Cloud Code companion project for email.
func (r *FleetSubagentRouter) SetAccountProject(email, project string) {
	key := strings.ToLower(strings.TrimSpace(email))
	if key == "" || strings.TrimSpace(project) == "" {
		return
	}
	r.mu.Lock()
	r.accountProjects[key] = strings.TrimSpace(project)
	r.mu.Unlock()
}

// GetAccountProject returns any cached Cloud Code companion project for email.
func (r *FleetSubagentRouter) GetAccountProject(email string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(email))
	r.mu.RLock()
	proj, ok := r.accountProjects[key]
	r.mu.RUnlock()
	return proj, ok
}

// GetConvAccount returns the sticky account email bound to cascadeID, if any.
func (r *FleetSubagentRouter) GetConvAccount(cascadeID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.convAccount[cascadeID]
}

// GetRootAccount returns the account email bound to rootCascadeID, if any.
func (r *FleetSubagentRouter) GetRootAccount(rootCascadeID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.rootAccount[rootCascadeID]
}

// MarkCooldown places email on temporary cooldown for the quota bucket of modelID.
func (r *FleetSubagentRouter) MarkCooldown(email, modelID string, d time.Duration) {
	if strings.TrimSpace(email) == "" {
		return
	}
	if d <= 0 {
		d = r.DefaultCooldown
		if d <= 0 {
			d = defaultAccountCooldown
		}
	}
	bucket := modelQuotaBucket(modelID)
	r.mu.Lock()
	r.cooldownUntil[cooldownKey(email, bucket)] = r.now().Add(d)
	r.mu.Unlock()
}

// IsOnCooldown reports whether email is currently on cooldown for modelID's quota bucket.
func (r *FleetSubagentRouter) IsOnCooldown(email, modelID string) bool {
	bucket := modelQuotaBucket(modelID)
	now := r.now()
	r.mu.RLock()
	exp, ok := r.cooldownUntil[cooldownKey(email, bucket)]
	r.mu.RUnlock()
	return ok && now.Before(exp)
}

// IsCircuitOpen reports whether the anti-429 circuit breaker is active, along
// with its trip reason and expiration timestamp.
func (r *FleetSubagentRouter) IsCircuitOpen() (bool, string, time.Time) {
	if r == nil {
		return false, "", time.Time{}
	}
	now := r.now()
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.circuitOpenUntil.IsZero() && now.Before(r.circuitOpenUntil) {
		return true, r.circuitReason, r.circuitOpenUntil
	}
	return false, "", time.Time{}
}

// TripCircuitBreaker opens the anti-429 circuit breaker for duration d (or the
// router's configured CircuitBreakerDuration when d <= 0).
func (r *FleetSubagentRouter) TripCircuitBreaker(reason string, d time.Duration) {
	if r == nil {
		return
	}
	if d <= 0 {
		d = r.CircuitBreakerDuration
		if d <= 0 {
			d = defaultCircuitBreakerDuration
		}
	}
	now := r.now()
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "anti_proxy_429_circuit_open"
	}
	r.mu.Lock()
	r.circuitOpenUntil = now.Add(d)
	r.circuitReason = reason
	r.lastTrippedAt = now
	r.mu.Unlock()
}

// ResetCircuitBreaker closes the anti-429 circuit breaker and clears correlation state.
func (r *FleetSubagentRouter) ResetCircuitBreaker() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.circuitOpenUntil = time.Time{}
	r.circuitReason = ""
	r.last429At = time.Time{}
	r.last429Email = ""
	r.mu.Unlock()
}

// RecordSanitizedRequest increments the counter of requests whose bait markers were sanitized.
func (r *FleetSubagentRouter) RecordSanitizedRequest() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.sanitizedRequests++
	r.mu.Unlock()
}

// RecordBlockedClient increments the counter of blocked third-party client requests.
func (r *FleetSubagentRouter) RecordBlockedClient() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.blockedExternalClients++
	r.mu.Unlock()
}

// Record429Outcome records an upstream HTTP 429 response for email/modelID,
// places the account on cooldown, and trips the circuit breaker when an
// anomalous trap or cross-account 429 correlation is detected.
func (r *FleetSubagentRouter) Record429Outcome(email, modelID string, classification Upstream429Classification, request429Count int) (abortFailover bool, tripReason string) {
	if r == nil {
		if classification.Kind == RateLimit429AnomalousTrap {
			return true, "anomalous_429_trap"
		}
		if request429Count >= 2 {
			return true, "correlated_429_across_accounts"
		}
		return false, ""
	}

	now := r.now()
	bucket := modelQuotaBucket(modelID)
	normEmail := strings.ToLower(strings.TrimSpace(email))

	cooldownDur := classification.RetryAfter
	if cooldownDur <= 0 {
		cooldownDur = r.DefaultCooldown
		if cooldownDur <= 0 {
			cooldownDur = defaultAccountCooldown
		}
	}

	cbDur := r.CircuitBreakerDuration
	if cbDur <= 0 {
		cbDur = defaultCircuitBreakerDuration
	}
	corrWin := r.CorrelationWindow
	if corrWin <= 0 {
		corrWin = defaultCorrelationWindow
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if normEmail != "" {
		r.cooldownUntil[cooldownKey(normEmail, bucket)] = now.Add(cooldownDur)
	}

	switch {
	case classification.Kind == RateLimit429AnomalousTrap:
		r.anomalous429Blocked++
		abortFailover = true
		if classification.Reason != "" {
			tripReason = "anomalous_429_trap: " + classification.Reason
		} else {
			tripReason = "anomalous_429_trap"
		}
	case request429Count >= 2:
		r.correlated429Aborts++
		abortFailover = true
		tripReason = fmt.Sprintf("correlated_429_across_accounts (count=%d)", request429Count)
	case !r.last429At.IsZero() && now.Sub(r.last429At) >= 0 && now.Sub(r.last429At) <= corrWin && r.last429Email != "" && normEmail != "" && !strings.EqualFold(r.last429Email, normEmail):
		r.correlated429Aborts++
		abortFailover = true
		tripReason = fmt.Sprintf("correlated_429_window (%s -> %s)", r.last429Email, normEmail)
	}

	if normEmail != "" {
		r.last429At = now
		r.last429Email = normEmail
	}

	if abortFailover {
		r.circuitOpenUntil = now.Add(cbDur)
		r.circuitReason = tripReason
		r.lastTrippedAt = now
	}

	return abortFailover, tripReason
}

// ProtectionStatus returns a snapshot of circuit-breaker and guard telemetry.
func (r *FleetSubagentRouter) ProtectionStatus() FleetProtectionStatus {
	if r == nil {
		return FleetProtectionStatus{}
	}
	now := r.now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	open := !r.circuitOpenUntil.IsZero() && now.Before(r.circuitOpenUntil)
	status := FleetProtectionStatus{
		CircuitOpen:            open,
		Anomalous429Blocked:    r.anomalous429Blocked,
		Correlated429Aborts:    r.correlated429Aborts,
		SanitizedRequests:      r.sanitizedRequests,
		BlockedExternalClients: r.blockedExternalClients,
		LastTrippedAt:          r.lastTrippedAt,
	}
	if open {
		status.CircuitReason = r.circuitReason
		status.CircuitOpenUntil = r.circuitOpenUntil.UTC().Format(time.RFC3339)
	}
	return status
}

// CredentialedAccountCount returns the number of non-banned accounts with tokens.
func (r *FleetSubagentRouter) CredentialedAccountCount() int {
	if r == nil || r.accounts == nil {
		return 0
	}
	count := 0
	for _, acc := range r.accounts.ListAccounts() {
		if acc == nil || strings.TrimSpace(acc.Email) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(acc.Status), "BANNED") {
			continue
		}
		if strings.TrimSpace(acc.AccessToken) == "" && strings.TrimSpace(acc.RefreshToken) == "" {
			continue
		}
		count++
	}
	return count
}

// ResolveActiveEmail determines the active account email from the incoming Bearer
// token or the AccountProvider's active account metadata.
func (r *FleetSubagentRouter) ResolveActiveEmail(authHeader string) string {
	if r == nil || r.accounts == nil {
		return ""
	}
	accounts := r.accounts.ListAccounts()
	if token := extractBearerToken(authHeader); token != "" {
		for _, acc := range accounts {
			if acc != nil && acc.AccessToken != "" && acc.AccessToken == token {
				return acc.Email
			}
		}
	}
	if active := strings.TrimSpace(r.accounts.ActiveEmail()); active != "" {
		return active
	}
	for _, acc := range accounts {
		if acc != nil && acc.IsActive && strings.TrimSpace(acc.Email) != "" {
			return acc.Email
		}
	}
	return ""
}

func extractBearerToken(header string) string {
	trimmed := strings.TrimSpace(header)
	if len(trimmed) >= 7 && strings.EqualFold(trimmed[:7], "Bearer ") {
		return strings.TrimSpace(trimmed[7:])
	}
	return ""
}

type fleetCandidate struct {
	acc          *FleetAccount
	score        float64
	hasQuota     bool
	polledNoData bool
}

// SelectAccountForTurn chooses the best fleet account for a classified Cloud Code turn.
func (r *FleetSubagentRouter) SelectAccountForTurn(role, cascadeID, rootCascadeID, modelID, activeEmail string) (*FleetAccount, error) {
	if r == nil || r.accounts == nil {
		return nil, fmt.Errorf("fleet account provider is not configured")
	}

	accounts := r.accounts.ListAccounts()
	quotaMap := r.accounts.LoadQuotaSummaries()
	bucket := modelQuotaBucket(modelID)
	now := r.now()

	if strings.TrimSpace(activeEmail) == "" {
		activeEmail = strings.TrimSpace(r.accounts.ActiveEmail())
		if activeEmail == "" {
			for _, acc := range accounts {
				if acc != nil && acc.IsActive {
					activeEmail = acc.Email
					break
				}
			}
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var healthy []fleetCandidate
	for _, acc := range accounts {
		if acc == nil || strings.TrimSpace(acc.Email) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(acc.Status), "BANNED") {
			continue
		}
		if strings.TrimSpace(acc.AccessToken) == "" && strings.TrimSpace(acc.RefreshToken) == "" {
			continue
		}
		ck := cooldownKey(acc.Email, bucket)
		if exp, ok := r.cooldownUntil[ck]; ok {
			if now.Before(exp) {
				continue
			}
			delete(r.cooldownUntil, ck)
		}

		summary := lookupQuotaSummary(quotaMap, acc.Email)
		frac, hasQuota, polledNoData := bucketQuotaFraction(summary, bucket)
		if hasQuota && frac <= minHealthyQuotaFraction {
			continue
		}
		score := frac
		if !hasQuota {
			if polledNoData {
				score = 0.05
			} else {
				score = 0.50
			}
		}
		healthy = append(healthy, fleetCandidate{
			acc:          cloneKeyringAccount(acc),
			score:        score,
			hasQuota:     hasQuota,
			polledNoData: polledNoData,
		})
	}

	if len(healthy) == 0 {
		return nil, fmt.Errorf("no healthy fleet accounts available for bucket %s", bucket)
	}

	sortFleetCandidates(healthy)

	if !r.circuitOpenUntil.IsZero() && now.Before(r.circuitOpenUntil) {
		if activeEmail != "" {
			for _, c := range healthy {
				if strings.EqualFold(c.acc.Email, activeEmail) {
					return cloneKeyringAccount(c.acc), nil
				}
			}
		}
		return cloneKeyringAccount(healthy[0].acc), nil
	}

	switch role {
	case TurnRoleRootOrchestrator:
		var chosen *FleetAccount
		if activeEmail != "" {
			for _, c := range healthy {
				if strings.EqualFold(c.acc.Email, activeEmail) {
					chosen = c.acc
					break
				}
			}
		}
		if chosen == nil {
			chosen = healthy[0].acc
		}
		if rootCascadeID != "" {
			r.rootAccount[rootCascadeID] = chosen.Email
		}
		if cascadeID != "" {
			r.rootAccount[cascadeID] = chosen.Email
		}
		return cloneKeyringAccount(chosen), nil

	case TurnRoleSubagentOrSwarm, TurnRolePostPlanExecution:
		if role == TurnRolePostPlanExecution && !r.OffloadPostPlanExecution {
			return r.selectStickyOrActiveLocked(healthy, cascadeID, activeEmail), nil
		}

		// Reuse sticky conversation assignment while that account remains healthy.
		if cascadeID != "" {
			if boundEmail := r.convAccount[cascadeID]; boundEmail != "" {
				for _, c := range healthy {
					if strings.EqualFold(c.acc.Email, boundEmail) {
						return cloneKeyringAccount(c.acc), nil
					}
				}
			}
		}

		rootEmail := ""
		if rootCascadeID != "" {
			rootEmail = r.rootAccount[rootCascadeID]
		}
		if rootEmail == "" && cascadeID != "" {
			rootEmail = r.rootAccount[cascadeID]
		}
		if rootEmail == "" {
			rootEmail = activeEmail
		}

		pool := make([]fleetCandidate, 0, len(healthy))
		if len(healthy) >= 2 && rootEmail != "" {
			for _, c := range healthy {
				if !strings.EqualFold(c.acc.Email, rootEmail) {
					pool = append(pool, c)
				}
			}
		}
		if len(pool) == 0 {
			pool = append(pool, healthy...)
		}

		pool = filterTopHealthyCandidates(pool)
		sortFleetCandidates(pool)

		idx := int((r.rrCounter.Add(1) - 1) % uint64(len(pool)))
		chosen := pool[idx].acc
		if cascadeID != "" {
			r.convAccount[cascadeID] = chosen.Email
		}
		return cloneKeyringAccount(chosen), nil

	default:
		return r.selectStickyOrActiveLocked(healthy, cascadeID, activeEmail), nil
	}
}

func (r *FleetSubagentRouter) selectStickyOrActiveLocked(healthy []fleetCandidate, cascadeID, activeEmail string) *FleetAccount {
	if cascadeID != "" {
		if boundEmail := r.convAccount[cascadeID]; boundEmail != "" {
			for _, c := range healthy {
				if strings.EqualFold(c.acc.Email, boundEmail) {
					return cloneKeyringAccount(c.acc)
				}
			}
		}
	}
	if activeEmail != "" {
		for _, c := range healthy {
			if strings.EqualFold(c.acc.Email, activeEmail) {
				return cloneKeyringAccount(c.acc)
			}
		}
	}
	chosen := healthy[0].acc
	if cascadeID != "" {
		r.convAccount[cascadeID] = chosen.Email
	}
	return cloneKeyringAccount(chosen)
}

func filterTopHealthyCandidates(pool []fleetCandidate) []fleetCandidate {
	hasData := false
	for _, c := range pool {
		if !c.polledNoData {
			hasData = true
			break
		}
	}
	if hasData {
		filtered := make([]fleetCandidate, 0, len(pool))
		for _, c := range pool {
			if !c.polledNoData {
				filtered = append(filtered, c)
			}
		}
		pool = filtered
	}

	topHealthy := make([]fleetCandidate, 0, len(pool))
	for _, c := range pool {
		if !c.hasQuota || c.score > topHealthyPoolFloor {
			topHealthy = append(topHealthy, c)
		}
	}
	if len(topHealthy) > 0 {
		return topHealthy
	}
	return pool
}

func sortFleetCandidates(cands []fleetCandidate) {
	sort.Slice(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.polledNoData != b.polledNoData {
			return !a.polledNoData
		}
		if a.hasQuota != b.hasQuota {
			return a.hasQuota
		}
		if a.score != b.score {
			return a.score > b.score
		}
		if a.acc.IsActive != b.acc.IsActive {
			return a.acc.IsActive
		}
		return strings.ToLower(a.acc.Email) < strings.ToLower(b.acc.Email)
	})
}

func lookupQuotaSummary(quotaMap map[string]*FleetQuotaSummary, email string) *FleetQuotaSummary {
	if len(quotaMap) == 0 || strings.TrimSpace(email) == "" {
		return nil
	}
	if s, ok := quotaMap[email]; ok && s != nil {
		return s
	}
	low := strings.ToLower(strings.TrimSpace(email))
	if s, ok := quotaMap[low]; ok && s != nil {
		return s
	}
	for k, s := range quotaMap {
		if s != nil && strings.EqualFold(strings.TrimSpace(k), low) {
			return s
		}
	}
	return nil
}

func bucketQuotaFraction(summary *FleetQuotaSummary, bucket string) (fraction float64, known bool, polledNoData bool) {
	if summary == nil {
		return 0, false, false
	}
	switch bucket {
	case "argon":
		var vals []float64
		if summary.Quota5hArgon != nil {
			vals = append(vals, *summary.Quota5hArgon)
		}
		if summary.QuotaWeeklyArgon != nil {
			vals = append(vals, *summary.QuotaWeeklyArgon)
		}
		if len(vals) > 0 {
			return minFloat(vals), true, false
		}
		if frac, ok := minMatchingModelQuota(summary.Models, "argon", "orchestrator"); ok {
			return frac, true, false
		}
		if strings.EqualFold(summary.OverallHealth, core.StatusExhausted) {
			return 0, true, false
		}
		if !summary.LastPolled.IsZero() {
			return 0, false, true
		}
		return 0, false, false

	case "claude_gpt":
		if summary.Quota5hClaudeGPT > 0 || summary.QuotaWeeklyClaudeGPT > 0 || summary.ClaudeWeeklyBucketPresent {
			frac := summary.Quota5hClaudeGPT
			if (summary.ClaudeWeeklyBucketPresent || summary.QuotaWeeklyClaudeGPT > 0) && summary.QuotaWeeklyClaudeGPT < frac {
				frac = summary.QuotaWeeklyClaudeGPT
			}
			return frac, true, false
		}
		if frac, ok := minMatchingModelQuota(summary.Models, "claude", "gpt"); ok {
			return frac, true, false
		}
		if !summary.LastPolled.IsZero() || strings.EqualFold(summary.OverallHealth, core.StatusExhausted) {
			return 0, true, false
		}
		if summary.Quota5hFraction == 0 && summary.QuotaWeeklyFraction == 0 && summary.Quota5hArgon == nil && summary.QuotaWeeklyArgon == nil {
			return 0, true, false
		}
		return 0, false, false

	default:
		if summary.Quota5hFraction > 0 && summary.QuotaWeeklyFraction > 0 {
			return minFloat([]float64{summary.Quota5hFraction, summary.QuotaWeeklyFraction}), true, false
		}
		if summary.Quota5hFraction > 0 {
			return summary.Quota5hFraction, true, false
		}
		if summary.QuotaWeeklyFraction > 0 {
			return summary.QuotaWeeklyFraction, true, false
		}
		if frac, ok := minMatchingModelQuota(summary.Models, "gemini", "hydrogen", "flash"); ok {
			return frac, true, false
		}
		if !summary.LastPolled.IsZero() || summary.OverallHealth != "" ||
			(summary.Quota5hArgon == nil && summary.QuotaWeeklyArgon == nil && summary.Quota5hClaudeGPT == 0 && summary.QuotaWeeklyClaudeGPT == 0) {
			return 0, true, false
		}
		return 0, false, false
	}
}

func minMatchingModelQuota(models []FleetModelQuota, needles ...string) (float64, bool) {
	found := false
	minVal := 1.0
	for _, m := range models {
		low := strings.ToLower(m.ModelName)
		matched := false
		for _, needle := range needles {
			if strings.Contains(low, needle) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if !found || m.Fraction < minVal {
			minVal = m.Fraction
			found = true
		}
	}
	return minVal, found
}

func minFloat(vals []float64) float64 {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func cloneKeyringAccount(acc *FleetAccount) *FleetAccount {
	if acc == nil {
		return nil
	}
	cp := *acc
	return &cp
}

// ResolveCredentials ensures a fresh OAuth access token and resolves the companion
// Cloud Code project ID for acc (caching the project in accountProjects).
func (r *FleetSubagentRouter) ResolveCredentials(ctx context.Context, acc *FleetAccount) (token string, project string, err error) {
	if acc == nil {
		return "", "", fmt.Errorf("nil account")
	}
	if r.accounts != nil {
		token, err = r.accounts.EnsureFreshToken(acc)
		if err != nil {
			return "", "", err
		}
	} else {
		token = strings.TrimSpace(acc.AccessToken)
	}
	if token == "" {
		return "", "", fmt.Errorf("empty access token for %s", acc.Email)
	}
	acc.AccessToken = token

	if proj, ok := r.GetAccountProject(acc.Email); ok && proj != "" {
		return token, proj, nil
	}
	if r.projects != nil {
		proj, projErr := r.projects.ResolveProject(ctx, acc)
		if projErr != nil {
			return token, "", projErr
		}
		if proj != "" {
			r.SetAccountProject(acc.Email, proj)
		}
		return token, proj, nil
	}
	return token, "", nil
}

func rewriteCloudCodeProject(body []byte, project string) []byte {
	project = strings.TrimSpace(project)
	if project == "" || len(bytes.TrimSpace(body)) == 0 {
		return body
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil || root == nil {
		return body
	}
	projRaw, err := json.Marshal(project)
	if err != nil {
		return body
	}
	root["project"] = projRaw
	if reqRaw, ok := root["request"]; ok && len(bytes.TrimSpace(reqRaw)) > 0 {
		var reqMap map[string]json.RawMessage
		if err := json.Unmarshal(reqRaw, &reqMap); err == nil && reqMap != nil {
			if _, hasProj := reqMap["project"]; hasProj {
				reqMap["project"] = projRaw
				if updatedReq, err := json.Marshal(reqMap); err == nil {
					root["request"] = updatedReq
				}
			}
		}
	}
	updated, err := json.Marshal(root)
	if err != nil {
		return body
	}
	return updated
}
