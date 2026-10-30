package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/bridge"
)

func TestA2ASpecCard_WellKnownEndpoint(t *testing.T) {
	srv, _, _ := setupTestA2AServer(t)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	w := httptest.NewRecorder()
	srv.handleA2AAgentCardSpec(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON content type, got %s", ct)
	}

	var card bridge.A2ASpecAgentCard
	if err := json.Unmarshal(w.Body.Bytes(), &card); err != nil {
		t.Fatalf("failed to parse spec card: %v", err)
	}
	if card.ProtocolVersion != "0.3.0" {
		t.Errorf("expected protocolVersion 0.3.0, got %s", card.ProtocolVersion)
	}
	if card.PreferredTransport != bridge.A2ATransportJSONRPC {
		t.Errorf("expected JSONRPC transport, got %s", card.PreferredTransport)
	}
	if !strings.HasSuffix(card.URL, "/a2a") {
		t.Errorf("expected card URL to point at /a2a, got %s", card.URL)
	}
	if len(card.Skills) == 0 {
		t.Errorf("expected skills advertised")
	}

	// OPTIONS must be answered for browser preflight.
	opt := httptest.NewRequest(http.MethodOptions, "/.well-known/agent-card.json", nil)
	wOpt := httptest.NewRecorder()
	srv.handleA2AAgentCardSpec(wOpt, opt)
	if wOpt.Code != http.StatusOK {
		t.Errorf("expected 200 for OPTIONS, got %d", wOpt.Code)
	}
}

func TestA2AJSONRPC_RequiresAuth(t *testing.T) {
	srv, _, _ := setupTestA2AServer(t)

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tasks/get","params":{"id":"x"}}`)
	req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleA2AJSONRPC(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if challenge := w.Header().Get("WWW-Authenticate"); !strings.Contains(challenge, "agent-card.json") {
		t.Errorf("expected challenge pointing at agent-card.json, got %q", challenge)
	}
	var payload struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON-RPC error body, got %s", w.Body.String())
	}
	if payload.Error.Message == "" {
		t.Errorf("expected unauthorized message")
	}
}

func TestA2AJSONRPC_MethodNotAllowed(t *testing.T) {
	srv, _, token := setupTestA2AServer(t)

	req := httptest.NewRequest(http.MethodGet, "/a2a", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.handleA2AJSONRPC(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
	var resp bridge.A2ARPCResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected JSON-RPC error envelope, got %s", w.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != bridge.A2AErrInvalidRequest {
		t.Errorf("expected invalid request error, got %+v", resp.Error)
	}
}

func TestA2AJSONRPC_MessageSendAndGet(t *testing.T) {
	srv, _, token := setupTestA2AServer(t)

	sendBody := []byte(`{"jsonrpc":"2.0","id":"r1","method":"message/send","params":{
		"message":{"kind":"message","messageId":"m1","role":"user",
		"parts":[{"kind":"text","text":"Build A2A bridge endpoint"}]}}}`)

	sendReq := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(sendBody))
	sendReq.Header.Set("Authorization", "Bearer "+token)
	wSend := httptest.NewRecorder()
	srv.handleA2AJSONRPC(wSend, sendReq)

	if wSend.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", wSend.Code, wSend.Body.String())
	}

	var sendResp bridge.A2ARPCResponse
	if err := json.Unmarshal(wSend.Body.Bytes(), &sendResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if sendResp.Error != nil {
		t.Fatalf("unexpected protocol error: %+v", sendResp.Error)
	}
	raw, _ := json.Marshal(sendResp.Result)
	var task bridge.A2AProtocolTask
	if err := json.Unmarshal(raw, &task); err != nil {
		t.Fatalf("failed to parse task: %v", err)
	}
	if task.ID == "" {
		t.Fatalf("expected task id")
	}

	getBody := []byte(`{"jsonrpc":"2.0","id":"r2","method":"tasks/get","params":{"id":"` + task.ID + `"}}`)
	getReq := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(getBody))
	getReq.Header.Set("Authorization", "Bearer "+token)
	wGet := httptest.NewRecorder()
	srv.handleA2AJSONRPC(wGet, getReq)

	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200 for tasks/get, got %d", wGet.Code)
	}
	var getResp bridge.A2ARPCResponse
	_ = json.Unmarshal(wGet.Body.Bytes(), &getResp)
	if getResp.Error != nil {
		t.Fatalf("unexpected tasks/get error: %+v", getResp.Error)
	}
}

func TestA2AJSONRPC_StreamUsesSSE(t *testing.T) {
	srv, _, token := setupTestA2AServer(t)

	body := []byte(`{"jsonrpc":"2.0","id":"s1","method":"message/stream","params":{
		"message":{"kind":"message","messageId":"m1","role":"user",
		"parts":[{"kind":"text","text":"stream this"}]}}}`)

	req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.handleA2AJSONRPC(w, req)

	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}
	out := w.Body.String()
	if !strings.Contains(out, "data: ") {
		t.Fatalf("expected SSE data frames, got %q", out)
	}

	// Every frame must be a valid JSON-RPC response carrying the request id.
	frames := 0
	sawFinal := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		frames++
		var resp bridge.A2ARPCResponse
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &resp); err != nil {
			t.Fatalf("invalid SSE frame %q: %v", line, err)
		}
		if resp.ID != "s1" {
			t.Errorf("expected id s1 in frame, got %v", resp.ID)
		}
		if resp.Error != nil {
			t.Fatalf("stream frame carried error: %+v", resp.Error)
		}
		raw, _ := json.Marshal(resp.Result)
		var probe struct {
			Final bool `json:"final"`
		}
		_ = json.Unmarshal(raw, &probe)
		if probe.Final {
			sawFinal = true
		}
	}
	if frames == 0 {
		t.Fatalf("expected at least one SSE frame")
	}
	if !sawFinal {
		t.Errorf("expected a terminal status-update frame, body was %q", out)
	}
}

func TestA2AJSONRPC_LegacyRoutesStillWork(t *testing.T) {
	srv, _, token := setupTestA2AServer(t)

	// The pre-existing REST surface must keep answering after the JSON-RPC
	// routes were added on the same prefix.
	body := []byte(`{"prompt":"legacy path check"}`)
	req := httptest.NewRequest(http.MethodPost, "/a2a/tasks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.handleA2ATasks(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 from legacy /a2a/tasks, got %d: %s", w.Code, w.Body.String())
	}
}

func TestA2APublicPaths(t *testing.T) {
	for _, p := range []string{"/a2a", "/a2a/rpc", "/api/bridge/a2a", "/api/bridge/a2a/rpc", "/a2a/tasks", "/a2a/tasks/abc", "/.well-known/agent-card.json"} {
		if !isPublicPath(p) {
			t.Errorf("expected %s to bypass the GUI session gate", p)
		}
		if !isExternalOriginAllowedPath(p) {
			t.Errorf("expected %s to be reachable from an external origin", p)
		}
	}
}
