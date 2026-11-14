package custommodels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newProxyFixture seeds a store with one custom model and returns a proxy whose
// upstream is the given test server.
func newProxyFixture(t *testing.T, upstream string) (*Store, *Proxy) {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SaveModel(CustomModel{
		ID:           "deepseek-proxy",
		Name:         "deepseek-chat",
		DisplayName:  "DeepSeek Proxy",
		ProviderType: ProviderOpenAI,
		BaseURL:      upstream,
		APIKey:       "sk-up",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}
	proxy := NewProxy(store)
	proxy.Upstream = upstream
	return store, proxy
}

// Native traffic must reach Google verbatim: same method, path, query, headers,
// body and status.
func TestProxyPassesNativeTrafficThrough(t *testing.T) {
	var seen struct {
		method, path, query, auth, body string
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen.method = r.Method
		seen.path = r.URL.Path
		seen.query = r.URL.RawQuery
		seen.auth = r.Header.Get("Authorization")
		seen.body = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	body := `{"model":"gemini-4-argon-medium","request":{"contents":[]}}`
	req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer ya29.native")
	rr := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if seen.path != "/v1internal:streamGenerateContent" || seen.query != "alt=sse" {
		t.Errorf("path/query must be preserved, got %q?%q", seen.path, seen.query)
	}
	if seen.auth != "Bearer ya29.native" {
		t.Errorf("the caller's bearer token must be forwarded, got %q", seen.auth)
	}
	if seen.body != body {
		t.Errorf("body must be forwarded verbatim, got %q", seen.body)
	}
	if rr.Body.String() != `{"ok":true}` {
		t.Errorf("response must pass through, got %q", rr.Body.String())
	}
}

// A custom model on the app path must answer with the Cloud Code envelope; bare
// Gemini frames are rejected by the language server.
func TestProxyWrapsAppSSEFrames(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"pong"}}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	body := `{"project":"aicode-consumers","requestId":"agent/req-1","model":"deepseek-proxy","request":{"contents":[{"role":"user","parts":[{"text":"say pong"}]}]},"requestType":"agent"}`
	req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", strings.NewReader(body))
	rr := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr, req)

	stream := rr.Body.String()
	if !strings.Contains(stream, `"response"`) {
		t.Fatalf("expected the Cloud Code envelope, got %s", stream)
	}
	if !strings.Contains(stream, `"traceId"`) || !strings.Contains(stream, `"metadata"`) {
		t.Errorf("envelope must carry traceId and metadata, got %s", stream)
	}
	if !strings.Contains(stream, `"text":"pong"`) {
		t.Errorf("expected the provider text inside the envelope, got %s", stream)
	}
	// Every frame must parse as the envelope shape.
	for _, line := range strings.Split(stream, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var frame map[string]json.RawMessage
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &frame); err != nil {
			t.Fatalf("frame is not JSON: %v", err)
		}
		if _, ok := frame["response"]; !ok {
			t.Errorf("frame missing the response envelope: %s", line)
		}
	}
}

// The same translation on the CLI gateway path must NOT wrap: the CLI wants bare
// Gemini frames.
func TestProxyLeavesGatewayFramesUnwrapped(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"pong"}}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	store, _ := newProxyFixture(t, upstream.URL)
	gateway := NewGateway(store)
	body := `{"contents":[{"role":"user","parts":[{"text":"say pong"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/deepseek-proxy:streamGenerateContent?alt=sse", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+gateway.Token())
	rr := httptest.NewRecorder()
	gateway.Handler().ServeHTTP(rr, req)

	stream := rr.Body.String()
	if strings.Contains(stream, `"traceId"`) {
		t.Errorf("CLI frames must stay bare, got %s", stream)
	}
	if !strings.Contains(stream, `"candidates"`) {
		t.Errorf("expected bare Gemini candidates, got %s", stream)
	}
}

// An app model override routes a native model name to a custom provider.
func TestProxyNeverInterceptsOtherEndpoints(t *testing.T) {
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"models":{}}`))
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	for _, path := range []string{"/v1internal:fetchAvailableModels", "/v1internal:loadCodeAssist", "/v1internal:listExperiments"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		rr := httptest.NewRecorder()
		proxy.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s: expected passthrough 200, got %d", path, rr.Code)
		}
	}
	if hits != 3 {
		t.Errorf("expected all three calls upstream, got %d", hits)
	}
}

func TestProxyAppModelOverrideRoutesNativeToCustomModel(t *testing.T) {
	upstreamHit := false
	customProviderHit := false
	var customBody map[string]any

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		customProviderHit = true
		_ = json.NewDecoder(r.Body).Decode(&customBody)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello from custom model\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer providerServer.Close()

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":{"candidates":[{"content":{"parts":[{"text":"native gemini"}]}}]}}`))
	}))
	defer upstreamServer.Close()

	store, proxy := newProxyFixture(t, upstreamServer.URL)
	if err := store.SaveModel(CustomModel{
		ID:               "deepseek-active",
		Name:             "deepseek-chat",
		DisplayName:      "DeepSeek Active",
		ProviderType:     ProviderOpenAI,
		BaseURL:          providerServer.URL,
		APIKey:           "sk-test",
		Enabled:          true,
		SupportsThinking: true,
		ThinkingLevel:    "high",
	}); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}

	// 1. With ActiveModelID set to deepseek-active, native model name "gemini-3.8-flash" routes to custom model
	if err := store.SetActiveModel("deepseek-active"); err != nil {
		t.Fatalf("SetActiveModel: %v", err)
	}

	body := `{"model":"gemini-3.8-flash","requestId":"req-active-test","request":{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}}`
	req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer ya29.native")
	rr := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr, req)

	if !customProviderHit {
		t.Errorf("expected custom provider to receive request when ActiveModelID is set")
	}
	if upstreamHit {
		t.Errorf("upstream Google Cloud Code should NOT have been called when custom model is active")
	}
	if !strings.Contains(rr.Body.String(), "hello from custom model") {
		t.Errorf("expected custom model response in stream, got %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"traceId":"req-active-test"`) {
		t.Errorf("expected wrapped SSE envelope with traceId, got %s", rr.Body.String())
	}
	if effort, ok := customBody["reasoning_effort"].(string); !ok || effort != "high" {
		t.Errorf("expected reasoning_effort 'high' in custom payload, got %v", customBody["reasoning_effort"])
	}

	// 2. Clear ActiveModelID: native model requests must now pass through to upstream verbatim
	upstreamHit = false
	customProviderHit = false
	if err := store.SetActiveModel(""); err != nil {
		t.Fatalf("SetActiveModel clear: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", strings.NewReader(body))
	req2.Header.Set("Authorization", "Bearer ya29.native")
	rr2 := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr2, req2)

	if !upstreamHit {
		t.Errorf("expected upstream Google Cloud Code to receive request when ActiveModelID is cleared")
	}
	if customProviderHit {
		t.Errorf("custom provider should NOT have been called after clearing ActiveModelID")
	}
}

func TestProxyActiveModelDoesNotHijackSubagentNativeRequests(t *testing.T) {
	var customHits, upstreamHits int32

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&customHits, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"custom\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer providerServer.Close()

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":{"candidates":[{"content":{"parts":[{"text":"native"}]}}]}}`))
	}))
	defer upstreamServer.Close()

	store, proxy := newProxyFixture(t, upstreamServer.URL)
	if err := store.SaveModel(CustomModel{
		ID:           "deepseek-active",
		Name:         "deepseek-chat",
		DisplayName:  "DeepSeek Active",
		ProviderType: ProviderOpenAI,
		BaseURL:      providerServer.URL,
		APIKey:       "sk-test",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}
	if err := store.SetActiveModel("deepseek-active"); err != nil {
		t.Fatalf("SetActiveModel: %v", err)
	}

	subagentConv := "11111111-1111-4111-8111-111111111111"
	proxy.IsSubagentConversation = func(convID string) bool { return convID == subagentConv }

	subagentBody := fmt.Sprintf(`{"model":"gemini-3.8-flash","requestId":"agent/%s/1","request":{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}}`, subagentConv)
	req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", strings.NewReader(subagentBody))
	rr := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr, req)

	if got := atomic.LoadInt32(&customHits); got != 0 {
		t.Fatalf("subagent native request must not hit the active custom model, got %d custom calls", got)
	}
	if got := atomic.LoadInt32(&upstreamHits); got != 1 {
		t.Fatalf("subagent native request must reach Cloud Code, got %d upstream calls", got)
	}
	if got := store.GetConversationModel(subagentConv); got != "" {
		t.Fatalf("subagent conversation must not be mapped to the main custom model, got %q", got)
	}

	mainConv := "22222222-2222-4222-8222-222222222222"
	mainBody := fmt.Sprintf(`{"model":"gemini-3.8-flash","requestId":"agent/%s/1","request":{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}}`, mainConv)
	mainReq := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", strings.NewReader(mainBody))
	mainRR := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(mainRR, mainReq)

	if got := atomic.LoadInt32(&customHits); got != 1 {
		t.Fatalf("main conversation must still use the active custom model, got %d custom calls", got)
	}
	if got := store.GetConversationModel(mainConv); got != "deepseek-active" {
		t.Fatalf("main conversation mapping = %q, want deepseek-active", got)
	}
}

func TestProxyLegacyActiveMappingDoesNotPinConversationAfterNativeSelection(t *testing.T) {
	var customHits, upstreamHits int32

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&customHits, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"custom\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer providerServer.Close()

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":{"candidates":[{"content":{"parts":[{"text":"native"}]}}]}}`))
	}))
	defer upstreamServer.Close()

	store, proxy := newProxyFixture(t, upstreamServer.URL)
	if err := store.SaveModel(CustomModel{
		ID:           "deepseek-active",
		Name:         "deepseek-chat",
		DisplayName:  "DeepSeek Active",
		ProviderType: ProviderOpenAI,
		BaseURL:      providerServer.URL,
		APIKey:       "sk-test",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}

	convID := "44444444-4444-4444-8444-444444444444"
	if err := store.RecordConversationModelWithSource(convID, "deepseek-active", ""); err != nil {
		t.Fatalf("RecordConversationModelWithSource: %v", err)
	}
	if err := store.SetActiveModel(""); err != nil {
		t.Fatalf("SetActiveModel clear: %v", err)
	}

	body := fmt.Sprintf(`{"model":"gemini-3.8-flash","requestId":"agent/%s/1","request":{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}}`, convID)
	req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", strings.NewReader(body))
	rr := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr, req)

	if got := atomic.LoadInt32(&customHits); got != 0 {
		t.Fatalf("legacy active mapping must not pin a native selection, got %d custom calls", got)
	}
	if got := atomic.LoadInt32(&upstreamHits); got != 1 {
		t.Fatalf("native selection must reach Cloud Code, got %d upstream calls", got)
	}
}

const proxyValidCatalog = `{"models":{"gemini-3.8-flash":{"displayName":"Gemini 3.8 Flash"}}}`

func proxyCatalogRequest(proxy *Proxy, ctx context.Context, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1internal:fetchAvailableModels", strings.NewReader(`{"project":"test-project"}`)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr, req)
	return rr
}

func TestProxyCatalogSurvivesFirstCallerCancellation(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			_, _ = w.Write([]byte(proxyValidCatalog))
			return
		}
		http.Error(w, "temporary catalog failure", http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	ctx, cancel := context.WithCancel(context.Background())
	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		firstDone <- proxyCatalogRequest(proxy, ctx, "account-a")
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("upstream catalog refresh did not start")
	}
	cancel()
	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled catalog caller did not return")
	}
	releaseOnce.Do(func() { close(release) })

	rr := proxyCatalogRequest(proxy, context.Background(), "account-a")
	if rr.Code != http.StatusOK || rr.Body.String() != proxyValidCatalog {
		t.Fatalf("later caller did not get the completed catalog: status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestProxyCatalogSharesRefreshAcrossConcurrentCallers(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(proxyValidCatalog))
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	const callerCount = 8
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, callerCount)
	var wg sync.WaitGroup
	for range callerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- proxyCatalogRequest(proxy, context.Background(), "account-a")
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	if got := calls.Load(); got != 1 {
		t.Fatalf("concurrent callers triggered %d upstream refreshes, want 1", got)
	}
	for rr := range results {
		if rr.Code != http.StatusOK || rr.Body.String() != proxyValidCatalog {
			t.Errorf("concurrent caller got status=%d body=%s", rr.Code, rr.Body.String())
		}
	}
}

func TestProxyCatalogSeparatesAccounts(t *testing.T) {
	const accountACatalog = `{"models":{"account-a-model":{"displayName":"Account A"}}}`
	var accountACalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer account-a":
			if accountACalls.Add(1) == 1 {
				_, _ = w.Write([]byte(accountACatalog))
				return
			}
			http.Error(w, "catalog unavailable", http.StatusServiceUnavailable)
		case "Bearer account-b":
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			http.Error(w, "missing auth", http.StatusUnauthorized)
		}
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	if rr := proxyCatalogRequest(proxy, context.Background(), "account-a"); rr.Code != http.StatusOK || rr.Body.String() != accountACatalog {
		t.Fatalf("account A seed failed: status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr := proxyCatalogRequest(proxy, context.Background(), "account-b"); rr.Code != http.StatusForbidden {
		t.Fatalf("account B must not receive account A's catalog: status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr := proxyCatalogRequest(proxy, context.Background(), "account-a"); rr.Code != http.StatusOK || rr.Body.String() != accountACatalog {
		t.Fatalf("account A cache was lost: status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestProxyCatalogRejectsInvalidResponses(t *testing.T) {
	cases := map[string]string{
		"malformed":        `{"models":`,
		"empty object":     `{}`,
		"empty models":     `{"models":{}}`,
		"empty model list": `{"models":[]}`,
	}
	for name, invalid := range cases {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					_, _ = w.Write([]byte(invalid))
					return
				}
				_, _ = w.Write([]byte(proxyValidCatalog))
			}))
			defer upstream.Close()

			_, proxy := newProxyFixture(t, upstream.URL)
			_ = proxyCatalogRequest(proxy, context.Background(), "account-a")
			rr := proxyCatalogRequest(proxy, context.Background(), "account-a")
			if rr.Code != http.StatusOK || rr.Body.String() != proxyValidCatalog {
				t.Fatalf("invalid catalog was cached: status=%d body=%s", rr.Code, rr.Body.String())
			}
			if got := calls.Load(); got != 2 {
				t.Fatalf("upstream calls=%d, want a retry and no cache", got)
			}
		})
	}
}

func TestProxyCatalogDoesNotCacheHTTPFailures(t *testing.T) {
	for _, status := range []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					http.Error(w, "catalog rejected", status)
					return
				}
				_, _ = w.Write([]byte(proxyValidCatalog))
			}))
			defer upstream.Close()

			_, proxy := newProxyFixture(t, upstream.URL)
			_ = proxyCatalogRequest(proxy, context.Background(), "account-a")
			rr := proxyCatalogRequest(proxy, context.Background(), "account-a")
			if rr.Code != http.StatusOK || rr.Body.String() != proxyValidCatalog {
				t.Fatalf("HTTP %d was cached: status=%d body=%s", status, rr.Code, rr.Body.String())
			}
			if got := calls.Load(); got != 2 {
				t.Fatalf("upstream calls=%d, want a retry and no cache", got)
			}
		})
	}
}

func TestProxyCatalogServesLastKnownGoodOnRefreshFailure(t *testing.T) {
	var calls atomic.Int32
	refreshStarted := make(chan struct{})
	var refreshOnce sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			_, _ = w.Write([]byte(proxyValidCatalog))
			return
		}
		refreshOnce.Do(func() { close(refreshStarted) })
		http.Error(w, "upstream overloaded", http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	if rr := proxyCatalogRequest(proxy, context.Background(), "account-a"); rr.Code != http.StatusOK || rr.Body.String() != proxyValidCatalog {
		t.Fatalf("seed catalog failed: status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr := proxyCatalogRequest(proxy, context.Background(), "account-a")
	if rr.Code != http.StatusOK || rr.Body.String() != proxyValidCatalog {
		t.Fatalf("last-known-good catalog not served: status=%d body=%s", rr.Code, rr.Body.String())
	}
	select {
	case <-refreshStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("background last-known-good refresh did not start")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("upstream calls=%d, want one successful seed and one failed refresh", got)
	}
}

func TestSyncModelIdentityInRequest(t *testing.T) {
	model := &CustomModel{
		ID:               "openai-deepseek-v4-flash-1",
		Name:             "deepseek-v4.1-flash",
		DisplayName:      "DeepSeek V4.1 Flash (OpenCode)",
		ThinkingLevel:    "high",
		SupportsThinking: true,
	}

	originalText := "<USER_REQUEST>\nwhat model are u\n</USER_REQUEST>\n<ADDITIONAL_METADATA>\nThe current local time is: 2026-10-29T17:16:43+11:00.\n</ADDITIONAL_METADATA>\n<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to Gemini 3.8 Flash (High). No need to comment on this change if the user doesn't ask about it. If reporting what model you are, please use a human readable name instead of the exact string.\n</USER_SETTINGS_CHANGE>"

	req := &geminiRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: originalText},
				},
			},
		},
		SystemInstruction: &geminiContent{
			Role: "system",
			Parts: []geminiPart{
				{Text: "You are Antigravity, a powerful agentic AI coding assistant designed by Google."},
			},
		},
	}

	syncModelIdentityInRequest(req, model)

	resultText := req.Contents[0].Parts[0].Text
	if !strings.Contains(resultText, "DeepSeek V4.1 Flash (OpenCode) (High)") {
		t.Errorf("expected rewritten USER_SETTINGS_CHANGE to contain DeepSeek V4.1 Flash (OpenCode) (High), got: %s", resultText)
	}
	if strings.Contains(resultText, "Gemini 3.8 Flash") {
		t.Errorf("result text should no longer contain Gemini 3.8 Flash, got: %s", resultText)
	}
	if !strings.Contains(resultText, "No need to comment on this change") {
		t.Errorf("expected trailing guidance in USER_SETTINGS_CHANGE to be preserved, got: %s", resultText)
	}

	sysText := req.SystemInstruction.Parts[len(req.SystemInstruction.Parts)-1].Text
	if !strings.Contains(sysText, "DeepSeek V4.1 Flash (OpenCode)") {
		t.Errorf("expected system instruction to include active model environment, got: %s", sysText)
	}
}

func TestExtractConversationID(t *testing.T) {
	reqID := "agent/e05a681a-1e3c-47f0-b217-aaadc7625295/1793254603527/3362b57d-514b-4be0-a7ab-aab12cf4c165/1"
	got := extractConversationID(reqID, nil)
	if got != "e05a681a-1e3c-47f0-b217-aaadc7625295" {
		t.Errorf("extractConversationID from reqID = %q, want e05a681a-1e3c-47f0-b217-aaadc7625295", got)
	}

	raw := json.RawMessage(`{"cascade_id":"11223344-5566-7788-99aa-bbccddeeff00"}`)
	got2 := extractConversationID("no-uuid", raw)
	if got2 != "11223344-5566-7788-99aa-bbccddeeff00" {
		t.Errorf("extractConversationID from cascade_id = %q, want 11223344-5566-7788-99aa-bbccddeeff00", got2)
	}
}

func TestStore_RecordAndGetConversationModel(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.RecordConversationModel("conv-1234", "custom-model-abc")
	if got := store.GetConversationModel("conv-1234"); got != "custom-model-abc" {
		t.Errorf("GetConversationModel = %q, want custom-model-abc", got)
	}

	// Verify persistence
	store2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore 2: %v", err)
	}
	if got := store2.GetConversationModel("conv-1234"); got != "custom-model-abc" {
		t.Errorf("GetConversationModel after reload = %q, want custom-model-abc", got)
	}
}

func TestStore_ConversationModelSourcePersists(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.RecordConversationModelWithSource("conv-source", "custom-model-abc", ConversationModelSourceActive); err != nil {
		t.Fatalf("RecordConversationModelWithSource: %v", err)
	}
	if got := store.GetConversationModelSource("conv-source"); got != ConversationModelSourceActive {
		t.Fatalf("source = %q, want %q", got, ConversationModelSourceActive)
	}

	store2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore 2: %v", err)
	}
	if got := store2.GetConversationModelSource("conv-source"); got != ConversationModelSourceActive {
		t.Fatalf("source after reload = %q, want %q", got, ConversationModelSourceActive)
	}

	if err := store2.RecordConversationModel("conv-source", ""); err != nil {
		t.Fatalf("clear mapping: %v", err)
	}
	if got := store2.GetConversationModelSource("conv-source"); got != "" {
		t.Fatalf("source after clear = %q, want empty", got)
	}
}

func TestProxyCatalogAugmentsForAgyCLI(t *testing.T) {
	const rawUpstreamCatalog = `{"agentModelSorts":[{"displayName":"Recommended","groups":[{"modelIds":["gemini-3.8-flash"]}]}],"models":{"gemini-3.8-flash":{"displayName":"Gemini 3.8 Flash"}}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(rawUpstreamCatalog))
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)

	// 1. Non-CLI caller: receives upstream catalog unchanged
	reqNonCLI := httptest.NewRequest(http.MethodPost, "/v1internal:fetchAvailableModels", strings.NewReader(`{}`)).WithContext(context.Background())
	reqNonCLI.Header.Set("Authorization", "Bearer account-test")
	rrNonCLI := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rrNonCLI, reqNonCLI)
	if rrNonCLI.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrNonCLI.Code)
	}
	if rrNonCLI.Body.String() != rawUpstreamCatalog {
		t.Fatalf("expected raw catalog for non-CLI caller, got: %s", rrNonCLI.Body.String())
	}

	// 2. Agy CLI caller: receives augmented catalog containing both native and custom models
	reqCLI := httptest.NewRequest(http.MethodPost, "/v1internal:fetchAvailableModels", strings.NewReader(`{}`)).WithContext(context.Background())
	reqCLI.Header.Set("Authorization", "Bearer account-test")
	reqCLI.Header.Set("User-Agent", "antigravity/cli/1.4.10 (aidev_client; os_type=linux; arch=amd64)")
	rrCLI := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rrCLI, reqCLI)
	if rrCLI.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrCLI.Code)
	}

	body := rrCLI.Body.String()
	// Must contain native model
	if !strings.Contains(body, "gemini-3.8-flash") {
		t.Errorf("augmented catalog missing native model gemini-3.8-flash: %s", body)
	}
	// Must contain custom model configured in newProxyFixture ("deepseek-chat" / "deepseek-proxy")
	if !strings.Contains(body, "deepseek-chat") && !strings.Contains(body, "deepseek-proxy") {
		t.Errorf("augmented catalog missing custom model: %s", body)
	}
}

func TestProxy_InternalAppRouting_ActiveModelPrecedence(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	gcpModel := CustomModel{
		ID:           "gcp-argon-1",
		Name:         "gemini-4-argon",
		DisplayName:  "Gemini 4 Argon (GCP)",
		ProviderType: ProviderGCP,
		BaseURL:      "https://asia-southeast1-aiplatform.googleapis.com",
		Enabled:      true,
	}
	apiModel := CustomModel{
		ID:               "gemini-api-argon-2",
		Name:             "gemini-4-argon",
		DisplayName:      "Gemini 4 Argon (Gemini API)",
		ProviderType:     ProviderGemini,
		BaseURL:          "https://generativelanguage.googleapis.com",
		Enabled:          true,
		SupportsThinking: true,
		ThinkingLevels:   []string{"low", "medium", "high"},
	}
	_ = store.SaveModel(gcpModel)
	_ = store.SaveModel(apiModel)

	proxy := NewProxy(store)

	// Case 1: active_model_id is gemini-api-argon-2.
	_ = store.SetActiveModel("gemini-api-argon-2")
	reqApp := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent", nil)
	envelope := customModelEnvelope{
		Model: "gemini-4-argon-high",
	}
	resolved := proxy.resolveCustomModel(reqApp, "gemini-4-argon-high", &envelope)
	if resolved == nil {
		t.Fatalf("expected resolved custom model, got nil")
	}
	if resolved.ID != "gemini-api-argon-2" {
		t.Fatalf("expected active model gemini-api-argon-2, got %q", resolved.ID)
	}
	if resolved.ThinkingLevel != "high" {
		t.Fatalf("expected thinking level high, got %q", resolved.ThinkingLevel)
	}

	// Case 2: active_model_id is empty (native mode).
	_ = store.SetActiveModel("")
	resolvedNative := proxy.resolveCustomModel(reqApp, "gemini-4-argon-high", &envelope)
	if resolvedNative != nil {
		t.Fatalf("expected native model request not to be hijacked by custom model name, got %q", resolvedNative.ID)
	}
}

func TestProxyFleetRoutingRootAndParallelSubagents(t *testing.T) {
	now := time.Now()
	type capturedCall struct {
		auth       string
		project    string
		reqProject string
	}
	var mu sync.Mutex
	var calls []capturedCall

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var parsed struct {
			Project string `json:"project"`
			Request struct {
				Project string `json:"project"`
			} `json:"request"`
		}
		_ = json.Unmarshal(raw, &parsed)
		mu.Lock()
		calls = append(calls, capturedCall{
			auth:       r.Header.Get("Authorization"),
			project:    parsed.Project,
			reqProject: parsed.Request.Project,
		})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response":{"candidates":[]}}`))
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	proxy.FleetRouter = NewFleetSubagentRouter(
		&stubAccountProvider{
			activeEmail: "a@example.com",
			accounts: []*FleetAccount{
				{Email: "a@example.com", AccessToken: "tok-a", RefreshToken: "ref-a", IsActive: true},
				{Email: "b@example.com", AccessToken: "tok-b", RefreshToken: "ref-b"},
				{Email: "c@example.com", AccessToken: "tok-c", RefreshToken: "ref-c"},
			},
			quotas: map[string]*FleetQuotaSummary{
				"a@example.com": {Quota5hArgon: testFloatPtr(0.95), Quota5hFraction: 0.90, LastPolled: now},
				"b@example.com": {Quota5hArgon: testFloatPtr(0.90), Quota5hFraction: 0.90, LastPolled: now},
				"c@example.com": {Quota5hArgon: testFloatPtr(0.90), Quota5hFraction: 0.90, LastPolled: now},
			},
		},
		&stubProjectResolver{
			projects: map[string]string{
				"a@example.com": "proj-a",
				"b@example.com": "proj-b",
				"c@example.com": "proj-c",
			},
		},
	)

	sendTurn := func(body, auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1internal:generateContent", strings.NewReader(body))
		req.Header.Set("Authorization", auth)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		proxy.Handler().ServeHTTP(rr, req)
		return rr
	}

	// 1. Root orchestrator stays on A with proj-a.
	rootBody := `{
		"model": "MODEL_PLACEHOLDER_M668",
		"project": "orig-proj",
		"request": {
			"project": "orig-req-proj",
			"cascade_id": "root-conv",
			"root_cascade_id": "root-conv",
			"tools": [{"functionDeclarations": [{"name": "exit_plan_mode"}]}],
			"contents": [{"role": "user", "parts": [{"text": "plan work"}]}]
		}
	}`
	rrRoot := sendTurn(rootBody, "Bearer tok-a")
	if rrRoot.Code != http.StatusOK {
		t.Fatalf("root HTTP %d: %s", rrRoot.Code, rrRoot.Body.String())
	}
	if got := rrRoot.Header().Get("X-Swiss-Routed-Account"); got != "a@example.com" {
		t.Fatalf("root X-Swiss-Routed-Account = %q, want a@example.com", got)
	}
	if got := rrRoot.Header().Get("X-Swiss-Turn-Role"); got != TurnRoleRootOrchestrator {
		t.Fatalf("root X-Swiss-Turn-Role = %q, want %q", got, TurnRoleRootOrchestrator)
	}

	// 2. Parallel subagents fan out across B and C with rewritten Bearer + project.
	subPayload := func(cid string) string {
		return fmt.Sprintf(`{
			"model": "MODEL_PLACEHOLDER_M525",
			"project": "orig-proj",
			"request": {
				"project": "orig-req-proj",
				"cascade_id": %q,
				"root_cascade_id": "root-conv",
				"contents": [{"role": "user", "parts": [{"text": "<subagent_reminder>worker</subagent_reminder>"}]}]
			}
		}`, cid)
	}

	var wg sync.WaitGroup
	var rrSub1, rrSub2 *httptest.ResponseRecorder
	wg.Add(2)
	go func() {
		defer wg.Done()
		rrSub1 = sendTurn(subPayload("sub-1"), "Bearer tok-a")
	}()
	go func() {
		defer wg.Done()
		rrSub2 = sendTurn(subPayload("sub-2"), "Bearer tok-a")
	}()
	wg.Wait()

	acc1 := rrSub1.Header().Get("X-Swiss-Routed-Account")
	acc2 := rrSub2.Header().Get("X-Swiss-Routed-Account")
	if acc1 == "" || acc2 == "" || acc1 == "a@example.com" || acc2 == "a@example.com" {
		t.Fatalf("expected subagents on B and C (not A), got acc1=%q acc2=%q", acc1, acc2)
	}
	if acc1 == acc2 {
		t.Fatalf("expected parallel subagents to spread across B and C, both hit %q", acc1)
	}
	if rrSub1.Header().Get("X-Swiss-Turn-Role") != TurnRoleSubagentOrSwarm {
		t.Fatalf("subagent X-Swiss-Turn-Role = %q", rrSub1.Header().Get("X-Swiss-Turn-Role"))
	}

	// 3. Turn 2 for sub-1 retains sticky affinity.
	rrSub1Turn2 := sendTurn(subPayload("sub-1"), "Bearer tok-a")
	if got := rrSub1Turn2.Header().Get("X-Swiss-Routed-Account"); got != acc1 {
		t.Fatalf("sub-1 turn 2 = %q, want sticky %q", got, acc1)
	}

	// Verify upstream saw exact rewritten tokens and project IDs.
	mu.Lock()
	if len(calls) != 4 {
		mu.Unlock()
		t.Fatalf("expected 4 upstream calls, got %d", len(calls))
	}
	rootCall := calls[0]
	if rootCall.auth != "Bearer tok-a" || rootCall.project != "proj-a" || rootCall.reqProject != "proj-a" {
		mu.Unlock()
		t.Fatalf("unexpected root upstream rewrite: %+v", rootCall)
	}
	seenWorkerAuth := map[string]string{}
	for _, c := range calls[1:] {
		seenWorkerAuth[c.auth] = c.project
		if c.project != c.reqProject {
			mu.Unlock()
			t.Fatalf("nested request.project mismatch: %+v", c)
		}
	}
	mu.Unlock()
	if seenWorkerAuth["Bearer tok-b"] != "proj-b" || seenWorkerAuth["Bearer tok-c"] != "proj-c" {
		t.Fatalf("expected worker calls with tok-b/proj-b and tok-c/proj-c, got %+v", seenWorkerAuth)
	}

	// 4. DisableFleetRouting bypasses fleet routing cleanly.
	proxy.DisableFleetRouting = true
	rrDisabled := sendTurn(subPayload("sub-disabled"), "Bearer tok-a")
	if got := rrDisabled.Header().Get("X-Swiss-Routed-Account"); got != "" {
		t.Fatalf("expected no X-Swiss-Routed-Account when disabled, got %q", got)
	}
	mu.Lock()
	lastCall := calls[len(calls)-1]
	mu.Unlock()
	if lastCall.auth != "Bearer tok-a" || lastCall.project != "orig-proj" {
		t.Fatalf("expected passthrough when fleet routing disabled, got %+v", lastCall)
	}
}

func TestProxyFleetTransparent429And503Failover(t *testing.T) {
	now := time.Now()
	var authLog []string
	var mu sync.Mutex

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		mu.Lock()
		authLog = append(authLog, auth)
		mu.Unlock()

		if auth == "Bearer tok-b" {
			w.Header().Set("Retry-After", "45")
			http.Error(w, `{"error":"rate limited on b"}`, http.StatusTooManyRequests)
			return
		}
		if auth == "Bearer tok-c" {
			http.Error(w, `{"error":"overloaded on c"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"response\":{\"candidates\":[]}}\n\n"))
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	proxy.FleetRouter = NewFleetSubagentRouter(
		&stubAccountProvider{
			activeEmail: "a@example.com",
			accounts: []*FleetAccount{
				{Email: "a@example.com", AccessToken: "tok-a", RefreshToken: "ref-a", IsActive: true},
				{Email: "b@example.com", AccessToken: "tok-b", RefreshToken: "ref-b"},
				{Email: "c@example.com", AccessToken: "tok-c", RefreshToken: "ref-c"},
			},
			quotas: map[string]*FleetQuotaSummary{
				"a@example.com": {Quota5hFraction: 0.90, LastPolled: now},
				"b@example.com": {Quota5hFraction: 0.90, LastPolled: now},
				"c@example.com": {Quota5hFraction: 0.90, LastPolled: now},
			},
		},
		&stubProjectResolver{
			projects: map[string]string{
				"a@example.com": "proj-a",
				"b@example.com": "proj-b",
				"c@example.com": "proj-c",
			},
		},
	)

	// Subagent hits B -> 429, retries C -> 503, retries A -> 200 OK SSE without client ever seeing 429/503!
	subReqBody := `{
		"model": "gemini-4-hydrogen-high",
		"project": "orig-proj",
		"request": {
			"cascade_id": "sub-failover",
			"root_cascade_id": "root-1",
			"contents": [{"role": "user", "parts": [{"text": "You are running as a subagent."}]}]
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", strings.NewReader(subReqBody))
	req.Header.Set("Authorization", "Bearer tok-a")
	rr := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected transparent failover to 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("X-Swiss-Routed-Account"); got != "a@example.com" {
		t.Fatalf("expected final routed account a@example.com, got %q", got)
	}
	if !strings.Contains(rr.Body.String(), `"candidates"`) {
		t.Fatalf("unexpected SSE body after failover: %q", rr.Body.String())
	}
	if !proxy.FleetRouter.IsOnCooldown("b@example.com", "gemini-4-hydrogen-high") {
		t.Errorf("expected b@example.com to be placed on cooldown after 429")
	}
	if !proxy.FleetRouter.IsOnCooldown("c@example.com", "gemini-4-hydrogen-high") {
		t.Errorf("expected c@example.com to be placed on cooldown after 503")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(authLog) != 3 {
		t.Fatalf("expected 3 upstream attempts (B->C->A), got %d: %v", len(authLog), authLog)
	}
}

func TestFindCustomModelResolvesPlaceholder(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SaveModel(CustomModel{
		ID:           "gemini-4-hydrogen",
		Name:         "gemini-4-hydrogen",
		DisplayName:  "Gemini 4 Hydrogen",
		ProviderType: ProviderOpenAI,
		BaseURL:      "http://127.0.0.1:9999/v1",
		APIKey:       "k",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}
	proxy := NewProxy(store)
	m := proxy.findCustomModel("MODEL_PLACEHOLDER_M528")
	if m == nil || m.ID != "gemini-4-hydrogen" || m.ThinkingLevel != "xhigh" {
		t.Fatalf("findCustomModel(MODEL_PLACEHOLDER_M528) = %+v", m)
	}
}
