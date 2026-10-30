package bridge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func a2aTestRPC(t *testing.T, svc *Service, body string) *A2ARPCResponse {
	t.Helper()
	res := svc.HandleA2AJSONRPC(context.Background(), []byte(body))
	if res.Response == nil {
		t.Fatalf("expected unary response for %s", body)
	}
	return res.Response
}

func newA2ATestService(t *testing.T) (*Service, *mockAgentBackend) {
	t.Helper()
	mock := newMockBackend()
	svc := &Service{}
	svc.a2aMgr = NewA2ATaskManager(func() agentBackend { return mock })
	return svc, mock
}

func TestA2ASpecAgentCard_Shape(t *testing.T) {
	card := SpecAgentCard("https://bridge.example.test/")

	if card.ProtocolVersion != "0.3.0" {
		t.Errorf("expected protocolVersion 0.3.0, got %s", card.ProtocolVersion)
	}
	if card.PreferredTransport != A2ATransportJSONRPC {
		t.Errorf("expected preferredTransport %s, got %s", A2ATransportJSONRPC, card.PreferredTransport)
	}
	if !strings.HasSuffix(card.URL, A2ARPCRequestPath) {
		t.Errorf("expected card URL to end with %s, got %s", A2ARPCRequestPath, card.URL)
	}
	if strings.HasSuffix(card.URL, "//"+A2ARPCRequestPath) {
		t.Errorf("expected trailing slash to be trimmed, got %s", card.URL)
	}
	if len(card.AdditionalInterfaces) == 0 || card.AdditionalInterfaces[0].Transport != A2ATransportJSONRPC {
		t.Errorf("expected a JSONRPC additional interface, got %+v", card.AdditionalInterfaces)
	}
	if !card.Capabilities.Streaming {
		t.Errorf("expected streaming capability advertised")
	}
	if len(card.DefaultInputModes) == 0 || len(card.DefaultOutputModes) == 0 {
		t.Errorf("expected default input and output modes")
	}
	if len(card.Skills) != len(DefaultA2AAgentCard("").Skills) {
		t.Errorf("expected spec skills to mirror legacy card skills")
	}
	if _, ok := card.SecuritySchemes["bearer"]; !ok {
		t.Errorf("expected bearer security scheme")
	}

	// Card must serialize to snake-free camelCase protocol keys.
	encoded, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("failed to marshal spec card: %v", err)
	}
	for _, key := range []string{"protocolVersion", "preferredTransport", "defaultInputModes", "securitySchemes"} {
		if !strings.Contains(string(encoded), `"`+key+`"`) {
			t.Errorf("expected serialized card to contain key %q", key)
		}
	}
}

func TestA2ARPC_RejectsMalformedEnvelopes(t *testing.T) {
	svc, _ := newA2ATestService(t)

	cases := []struct {
		name string
		body string
		code int
	}{
		{"empty", "", A2AErrParseError},
		{"invalid json", "{oops", A2AErrParseError},
		{"batch", `[{"jsonrpc":"2.0","id":1,"method":"tasks/get"}]`, A2AErrInvalidRequest},
		{"missing id", `{"jsonrpc":"2.0","method":"tasks/get"}`, A2AErrInvalidRequest},
		{"bad version", `{"jsonrpc":"1.0","id":1,"method":"tasks/get"}`, A2AErrInvalidRequest},
		{"unknown method", `{"jsonrpc":"2.0","id":1,"method":"nope/nope"}`, A2AErrMethodNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := a2aTestRPC(t, svc, tc.body)
			if resp.Error == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if resp.Error.Code != tc.code {
				t.Errorf("expected code %d, got %d (%s)", tc.code, resp.Error.Code, resp.Error.Message)
			}
			if resp.JSONRPC != "2.0" {
				t.Errorf("expected jsonrpc 2.0 envelope")
			}
		})
	}
}

func TestA2ARPC_MessageSendCreatesTask(t *testing.T) {
	svc, mock := newA2ATestService(t)

	body := `{"jsonrpc":"2.0","id":"req-1","method":"message/send","params":{"message":{
		"kind":"message","messageId":"msg-1","role":"user",
		"parts":[{"kind":"text","text":"Implement the A2A bridge endpoint"}],
		"metadata":{"skillId":"software_engineering","title":"A2A transport"}}}}`

	resp := a2aTestRPC(t, svc, body)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if resp.ID != "req-1" {
		t.Errorf("expected id echoed as req-1, got %v", resp.ID)
	}

	raw, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}
	var task A2AProtocolTask
	if err := json.Unmarshal(raw, &task); err != nil {
		t.Fatalf("failed to parse task result: %v", err)
	}
	if task.Kind != "task" {
		t.Errorf("expected kind task, got %s", task.Kind)
	}
	if task.ID == "" || task.ContextID == "" {
		t.Fatalf("expected task id and contextId, got %+v", task)
	}
	if task.Status.State != A2AStateSubmitted && task.Status.State != A2AStateWorking && task.Status.State != A2AStateCompleted {
		t.Errorf("unexpected state %s", task.Status.State)
	}
	if len(task.History) == 0 || task.History[0].Role != "user" {
		t.Errorf("expected user history entry, got %+v", task.History)
	}

	// Dispatch happens on a background goroutine, so poll for the call.
	deadline := time.Now().Add(2 * time.Second)
	created := 0
	for time.Now().Before(deadline) {
		mock.mu.Lock()
		created = len(mock.created)
		mock.mu.Unlock()
		if created > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mock.mu.Lock()
	prompt := ""
	if created > 0 {
		prompt, _ = mock.created[0]["prompt"].(string)
	}
	mock.mu.Unlock()

	if created == 0 {
		t.Fatalf("expected backend CreateConversation to be called")
	}
	if prompt != "Implement the A2A bridge endpoint" {
		t.Errorf("expected prompt forwarded, got %q", prompt)
	}
}

func TestA2ARPC_MessageSendRejectsBadParts(t *testing.T) {
	svc, _ := newA2ATestService(t)

	cases := []struct{ name, body string }{
		{"no parts", `{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"kind":"message","messageId":"m","role":"user","parts":[]}}}`},
		{"unsupported kind", `{"jsonrpc":"2.0","id":2,"method":"message/send","params":{"message":{"kind":"message","messageId":"m","role":"user","parts":[{"kind":"video","text":"x"}]}}}`},
		{"file without ref", `{"jsonrpc":"2.0","id":3,"method":"message/send","params":{"message":{"kind":"message","messageId":"m","role":"user","parts":[{"kind":"file","file":{}}]}}}`},
		{"unknown skill", `{"jsonrpc":"2.0","id":4,"method":"message/send","params":{"message":{"kind":"message","messageId":"m","role":"user","parts":[{"kind":"text","text":"x"}],"metadata":{"skillId":"astrology"}}}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := a2aTestRPC(t, svc, tc.body)
			if resp.Error == nil {
				t.Fatalf("expected invalid params error for %s", tc.name)
			}
			if resp.Error.Code != A2AErrInvalidParams {
				t.Errorf("expected code %d, got %d", A2AErrInvalidParams, resp.Error.Code)
			}
		})
	}
}

func TestA2ARPC_BlockingSendCompletes(t *testing.T) {
	svc, mock := newA2ATestService(t)
	mock.mockResp = "Implemented the endpoint."

	body := `{"jsonrpc":"2.0","id":7,"method":"message/send","params":{
		"message":{"kind":"message","messageId":"m1","role":"user","parts":[{"kind":"text","text":"Do the thing"}]},
		"configuration":{"blocking":true,"historyLength":5}}}`

	resp := a2aTestRPC(t, svc, body)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var task A2AProtocolTask
	if err := json.Unmarshal(raw, &task); err != nil {
		t.Fatalf("failed to parse task: %v", err)
	}
	if task.Status.State != A2AStateCompleted {
		t.Fatalf("expected completed state, got %s", task.Status.State)
	}
	if task.Status.Message == nil || !strings.Contains(task.Status.Message.Parts[0].Text, "Implemented the endpoint.") {
		t.Errorf("expected agent response text in status message, got %+v", task.Status.Message)
	}
	if len(task.Artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(task.Artifacts))
	}
	if task.Artifacts[0].Parts[0].File == nil || task.Artifacts[0].Parts[0].File.URI == "" {
		t.Errorf("expected artifact file part with uri, got %+v", task.Artifacts[0])
	}
}

func TestA2ARPC_TasksGetAndCancel(t *testing.T) {
	svc, mock := newA2ATestService(t)
	mock.delay = 2 * time.Second

	created, err := svc.a2aMgr.CreateTask(SkillSoftwareEngineering, "long running", "Long")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	getResp := a2aTestRPC(t, svc, `{"jsonrpc":"2.0","id":1,"method":"tasks/get","params":{"id":"`+created.ID+`"}}`)
	if getResp.Error != nil {
		t.Fatalf("unexpected tasks/get error: %+v", getResp.Error)
	}

	missing := a2aTestRPC(t, svc, `{"jsonrpc":"2.0","id":2,"method":"tasks/get","params":{"id":"does-not-exist"}}`)
	if missing.Error == nil || missing.Error.Code != A2AErrTaskNotFound {
		t.Fatalf("expected task not found error, got %+v", missing.Error)
	}

	cancelResp := a2aTestRPC(t, svc, `{"jsonrpc":"2.0","id":3,"method":"tasks/cancel","params":{"id":"`+created.ID+`"}}`)
	if cancelResp.Error != nil {
		t.Fatalf("unexpected tasks/cancel error: %+v", cancelResp.Error)
	}
	raw, _ := json.Marshal(cancelResp.Result)
	var canceled A2AProtocolTask
	_ = json.Unmarshal(raw, &canceled)
	if canceled.Status.State != A2AStateCanceled {
		t.Errorf("expected canceled state, got %s", canceled.Status.State)
	}
}

func TestA2ARPC_MessageStreamEmitsStatusAndArtifacts(t *testing.T) {
	svc, _ := newA2ATestService(t)

	body := `{"jsonrpc":"2.0","id":"stream-1","method":"message/stream","params":{
		"message":{"kind":"message","messageId":"m1","role":"user","parts":[{"kind":"text","text":"stream me"}]}}}`

	res := svc.HandleA2AJSONRPC(context.Background(), []byte(body))
	if res.Response != nil {
		t.Fatalf("expected stream, got error response: %+v", res.Response.Error)
	}
	if res.Stream == nil {
		t.Fatalf("expected stream channel")
	}

	var kinds []string
	sawFinal := false
	timeout := time.After(5 * time.Second)

	for !sawFinal {
		select {
		case resp, ok := <-res.Stream:
			if !ok {
				if !sawFinal {
					t.Fatalf("stream closed before a final event; saw %v", kinds)
				}
			} else {
				if resp.Error != nil {
					t.Fatalf("stream emitted error: %+v", resp.Error)
				}
				raw, _ := json.Marshal(resp.Result)
				var probe struct {
					Kind  string `json:"kind"`
					Final bool   `json:"final"`
				}
				_ = json.Unmarshal(raw, &probe)
				kinds = append(kinds, probe.Kind)
				if probe.Kind == "status-update" && probe.Final {
					sawFinal = true
				}
			}
		case <-timeout:
			t.Fatalf("timed out waiting for final status event; saw %v", kinds)
		}
	}

	if len(kinds) == 0 {
		t.Fatalf("expected streamed events")
	}
	for _, k := range kinds {
		if k != "status-update" && k != "artifact-update" {
			t.Errorf("unexpected streamed event kind: %s", k)
		}
	}
}

func TestA2ARPC_ResubscribeUnknownTask(t *testing.T) {
	svc, _ := newA2ATestService(t)

	res := svc.HandleA2AJSONRPC(context.Background(), []byte(`{"jsonrpc":"2.0","id":9,"method":"tasks/resubscribe","params":{"id":"nope"}}`))
	if res.Response == nil || res.Response.Error == nil {
		t.Fatalf("expected error response for unknown task")
	}
	if res.Response.Error.Code != A2AErrTaskNotFound {
		t.Errorf("expected task not found, got %d", res.Response.Error.Code)
	}
}

func TestA2ARPC_PushNotificationUnsupported(t *testing.T) {
	svc, _ := newA2ATestService(t)

	resp := a2aTestRPC(t, svc, `{"jsonrpc":"2.0","id":1,"method":"tasks/pushNotificationConfig/set","params":{}}`)
	if resp.Error == nil || resp.Error.Code != A2AErrPushNotificationNotSupported {
		t.Fatalf("expected push notification unsupported error, got %+v", resp.Error)
	}
}

func TestA2ATaskManager_TaskForContext(t *testing.T) {
	svc, _ := newA2ATestService(t)
	mgr := svc.A2ATaskManager()

	task, err := mgr.CreateTask(SkillCodeReview, "review it", "Review")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if got := mgr.TaskForContext(task.ID, ""); got == nil || got.ID != task.ID {
		t.Errorf("expected lookup by task id to resolve")
	}
	if got := mgr.TaskForContext("", task.ID); got == nil || got.ID != task.ID {
		t.Errorf("expected lookup by id-as-context to resolve")
	}
	if got := mgr.TaskForContext("", "unknown-context"); got != nil {
		t.Errorf("expected nil for unknown context, got %+v", got)
	}
	if got := mgr.TaskForContext("", ""); got != nil {
		t.Errorf("expected nil for empty lookup keys")
	}
}
