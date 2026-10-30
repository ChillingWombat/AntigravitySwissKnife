package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// A2A 0.3.0 JSON-RPC 2.0 transport method names.
const (
	A2AMethodMessageSend      = "message/send"
	A2AMethodMessageStream    = "message/stream"
	A2AMethodTasksGet         = "tasks/get"
	A2AMethodTasksCancel      = "tasks/cancel"
	A2AMethodTasksResubscribe = "tasks/resubscribe"
)

// A2A protocol task states (0.3.0 wire values).
const (
	A2AStateSubmitted     = "submitted"
	A2AStateWorking       = "working"
	A2AStateInputRequired = "input-required"
	A2AStateCompleted     = "completed"
	A2AStateFailed        = "failed"
	A2AStateCanceled      = "canceled"
	A2AStateUnknown       = "unknown"
)

// JSON-RPC 2.0 and A2A specific error codes.
const (
	A2AErrParseError                   = -32700
	A2AErrInvalidRequest               = -32600
	A2AErrMethodNotFound               = -32601
	A2AErrInvalidParams                = -32602
	A2AErrInternal                     = -32603
	A2AErrTaskNotFound                 = -32001
	A2AErrTaskNotCancelable            = -32002
	A2AErrPushNotificationNotSupported = -32003
	A2AErrUnsupportedOperation         = -32004
	A2AErrContentTypeNotSupported      = -32005
)

// A2ATransportJSONRPC is the transport name advertised in the Agent Card.
const A2ATransportJSONRPC = "JSONRPC"

// A2ARPCRequest is an inbound JSON-RPC 2.0 request envelope.
type A2ARPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// A2ARPCError is a JSON-RPC 2.0 error object.
type A2ARPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// A2ARPCResponse is a JSON-RPC 2.0 response envelope.
type A2ARPCResponse struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      interface{}  `json:"id"`
	Result  interface{}  `json:"result,omitempty"`
	Error   *A2ARPCError `json:"error,omitempty"`
}

// A2ARPCResult carries either a unary response or a stream of responses.
type A2ARPCResult struct {
	Response *A2ARPCResponse
	Stream   <-chan A2ARPCResponse
}

// A2APart is one content part of an A2A message.
type A2APart struct {
	Kind string      `json:"kind"`
	Text string      `json:"text,omitempty"`
	File *A2AFileRef `json:"file,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// A2AFileRef references a file by URI or inline bytes.
type A2AFileRef struct {
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	URI      string `json:"uri,omitempty"`
	Bytes    string `json:"bytes,omitempty"`
}

// A2AMessage is an A2A protocol message exchanged with a peer agent.
type A2AMessage struct {
	Kind      string                 `json:"kind"`
	MessageID string                 `json:"messageId"`
	Role      string                 `json:"role"`
	Parts     []A2APart              `json:"parts"`
	ContextID string                 `json:"contextId,omitempty"`
	TaskID    string                 `json:"taskId,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// A2ASendConfiguration mirrors the protocol's SendMessageConfiguration.
type A2ASendConfiguration struct {
	Blocking            bool     `json:"blocking,omitempty"`
	HistoryLength       int      `json:"historyLength,omitempty"`
	AcceptedOutputModes []string `json:"acceptedOutputModes,omitempty"`
}

// A2ASendMessageParams is the params object of message/send and message/stream.
type A2ASendMessageParams struct {
	Message       A2AMessage             `json:"message"`
	Configuration *A2ASendConfiguration  `json:"configuration,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// A2ATaskQueryParams is the params object of tasks/get and tasks/resubscribe.
type A2ATaskQueryParams struct {
	ID            string `json:"id"`
	HistoryLength int    `json:"historyLength,omitempty"`
}

// A2AArtifact is a deliverable produced by a task.
type A2AArtifact struct {
	ArtifactID string                 `json:"artifactId"`
	Name       string                 `json:"name,omitempty"`
	Parts      []A2APart              `json:"parts"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// A2ATaskStatus is the lifecycle status of a protocol task.
type A2ATaskStatus struct {
	State     string      `json:"state"`
	Message   *A2AMessage `json:"message,omitempty"`
	Timestamp string      `json:"timestamp"`
}

// A2AProtocolTask is an A2A protocol task projection of an internal A2ATask.
type A2AProtocolTask struct {
	Kind      string                 `json:"kind"`
	ID        string                 `json:"id"`
	ContextID string                 `json:"contextId"`
	Status    A2ATaskStatus          `json:"status"`
	Artifacts []A2AArtifact          `json:"artifacts,omitempty"`
	History   []A2AMessage           `json:"history,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// A2AStatusUpdateEvent is streamed by message/stream and tasks/resubscribe.
type A2AStatusUpdateEvent struct {
	Kind      string        `json:"kind"`
	TaskID    string        `json:"taskId"`
	ContextID string        `json:"contextId"`
	Status    A2ATaskStatus `json:"status"`
	Final     bool          `json:"final"`
}

// A2AArtifactUpdateEvent delivers a produced artifact over a stream.
type A2AArtifactUpdateEvent struct {
	Kind      string      `json:"kind"`
	TaskID    string      `json:"taskId"`
	ContextID string      `json:"contextId"`
	Artifact  A2AArtifact `json:"artifact"`
	Append    bool        `json:"append"`
	LastChunk bool        `json:"lastChunk"`
}

// A2ASpecInterface describes one advertised transport binding.
type A2ASpecInterface struct {
	URL       string `json:"url"`
	Transport string `json:"transport"`
}

// A2ASpecCapabilities is the protocol's capabilities block.
type A2ASpecCapabilities struct {
	Streaming              bool `json:"streaming"`
	PushNotifications      bool `json:"pushNotifications"`
	StateTransitionHistory bool `json:"stateTransitionHistory"`
}

// A2ASpecSkill is the protocol's skill descriptor.
type A2ASpecSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

// A2ASecurityScheme describes one auth scheme. Only HTTP bearer is advertised.
type A2ASecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Description  string `json:"description,omitempty"`
}

// A2ASpecAgentCard is the canonical A2A 0.3.0 Agent Card served at
// /.well-known/agent-card.json. It is derived from DefaultA2AAgentCard, so the
// identity, version, and skills of the two card shapes cannot drift apart.
type A2ASpecAgentCard struct {
	ProtocolVersion      string                       `json:"protocolVersion"`
	Name                 string                       `json:"name"`
	Description          string                       `json:"description"`
	URL                  string                       `json:"url"`
	PreferredTransport   string                       `json:"preferredTransport"`
	AdditionalInterfaces []A2ASpecInterface           `json:"additionalInterfaces,omitempty"`
	Version              string                       `json:"version"`
	Capabilities         A2ASpecCapabilities          `json:"capabilities"`
	DefaultInputModes    []string                     `json:"defaultInputModes"`
	DefaultOutputModes   []string                     `json:"defaultOutputModes"`
	Skills               []A2ASpecSkill               `json:"skills"`
	SecuritySchemes      map[string]A2ASecurityScheme `json:"securitySchemes,omitempty"`
	Security             []map[string][]string        `json:"security,omitempty"`
}

// A2ARPCRequestPath is the JSON-RPC endpoint path advertised by the Agent Card.
const A2ARPCRequestPath = "/a2a"

// SpecAgentCard builds the A2A 0.3.0 Agent Card from the legacy descriptor.
func SpecAgentCard(baseURL string) A2ASpecAgentCard {
	legacy := DefaultA2AAgentCard(baseURL)
	base := strings.TrimRight(legacy.URL, "/")
	rpcURL := base + A2ARPCRequestPath

	inputModes := []string{"text/plain", "application/json"}
	outputModes := []string{"text/plain", "application/json"}

	skills := make([]A2ASpecSkill, 0, len(legacy.Skills))
	for _, sk := range legacy.Skills {
		skills = append(skills, A2ASpecSkill{
			ID:          sk.ID,
			Name:        sk.Name,
			Description: sk.Description,
			Tags:        sk.Tags,
			InputModes:  inputModes,
			OutputModes: outputModes,
		})
	}

	return A2ASpecAgentCard{
		ProtocolVersion:    legacy.ProtocolVersion,
		Name:               legacy.Name,
		Description:        legacy.Description,
		URL:                rpcURL,
		PreferredTransport: A2ATransportJSONRPC,
		AdditionalInterfaces: []A2ASpecInterface{
			{URL: rpcURL, Transport: A2ATransportJSONRPC},
		},
		Version: legacy.Version,
		Capabilities: A2ASpecCapabilities{
			Streaming:              legacy.Capabilities.Streaming,
			PushNotifications:      false,
			StateTransitionHistory: true,
		},
		DefaultInputModes:  inputModes,
		DefaultOutputModes: outputModes,
		Skills:             skills,
		SecuritySchemes: map[string]A2ASecurityScheme{
			"bearer": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "opaque",
				Description:  "Bridge bearer token or pairing PIN issued by the Swiss Knife desktop app.",
			},
		},
		Security: []map[string][]string{
			{"bearer": {}},
		},
	}
}

func a2aRPCError(id interface{}, code int, message string, data interface{}) *A2ARPCResponse {
	return &A2ARPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &A2ARPCError{Code: code, Message: message, Data: data},
	}
}

func a2aRPCResult(id interface{}, result interface{}) *A2ARPCResponse {
	return &A2ARPCResponse{JSONRPC: "2.0", ID: id, Result: result}
}

// HandleA2AJSONRPC dispatches one A2A JSON-RPC 2.0 call against the bridge.
// Protocol-level failures come back as JSON-RPC error objects, never as a
// transport error, so callers can always answer with HTTP 200.
func (s *Service) HandleA2AJSONRPC(ctx context.Context, raw []byte) A2ARPCResult {
	body := strings.TrimSpace(string(raw))
	if body == "" {
		return A2ARPCResult{Response: a2aRPCError(nil, A2AErrParseError, "empty request body", nil)}
	}
	if strings.HasPrefix(body, "[") {
		return A2ARPCResult{Response: a2aRPCError(nil, A2AErrInvalidRequest, "batch requests are not supported", nil)}
	}

	var req A2ARPCRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		return A2ARPCResult{Response: a2aRPCError(nil, A2AErrParseError, "invalid JSON: "+err.Error(), nil)}
	}
	if req.ID == nil {
		return A2ARPCResult{Response: a2aRPCError(nil, A2AErrInvalidRequest, "request id is required", nil)}
	}
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrInvalidRequest, "unsupported jsonrpc version: "+req.JSONRPC, nil)}
	}

	switch req.Method {
	case A2AMethodMessageSend:
		return A2ARPCResult{Response: s.a2aSendMessage(ctx, &req)}
	case A2AMethodMessageStream:
		return s.a2aSendMessageStream(ctx, &req)
	case A2AMethodTasksGet:
		return A2ARPCResult{Response: s.a2aGetTask(&req)}
	case A2AMethodTasksCancel:
		return A2ARPCResult{Response: s.a2aCancelTask(&req)}
	case A2AMethodTasksResubscribe:
		return s.a2aResubscribe(ctx, &req)
	case "tasks/pushNotificationConfig/set", "tasks/pushNotificationConfig/get",
		"tasks/pushNotificationConfig/list", "tasks/pushNotificationConfig/delete":
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrPushNotificationNotSupported,
			"push notifications are not supported by this agent", nil)}
	default:
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrMethodNotFound,
			"method not found: "+req.Method, nil)}
	}
}

// a2aTaskManager returns the task manager or nil when the bridge is unstarted.
func (s *Service) a2aTaskManager() *A2ATaskManager { return s.a2aMgr }

func (s *Service) a2aSendMessage(ctx context.Context, req *A2ARPCRequest) *A2ARPCResponse {
	var params A2ASendMessageParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return a2aRPCError(req.ID, A2AErrInvalidParams, "invalid params: "+err.Error(), nil)
	}
	prompt, err := a2aPromptFromMessage(params.Message)
	if err != nil {
		return a2aRPCError(req.ID, A2AErrInvalidParams, err.Error(), nil)
	}

	skill, sErr := a2aSkillFromMetadata(params.Message.Metadata, params.Metadata)
	if sErr != nil {
		return a2aRPCError(req.ID, A2AErrInvalidParams, sErr.Error(), nil)
	}

	mgr := s.a2aTaskManager()
	if mgr == nil {
		return a2aRPCError(req.ID, A2AErrInternal, "A2A task manager is not initialized", nil)
	}

	blocking := params.Configuration != nil && params.Configuration.Blocking
	historyLen := 0
	if params.Configuration != nil {
		historyLen = params.Configuration.HistoryLength
	}

	task, err := s.a2aAcceptMessage(mgr, params.Message, prompt, skill)
	if err != nil {
		return a2aRPCError(req.ID, A2AErrInvalidParams, err.Error(), nil)
	}

	if blocking && !isTerminalStatus(task.Status) {
		waitCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
		defer cancel()
		if resolved, awaitErr := mgr.AwaitTask(waitCtx, task.ID); awaitErr == nil && resolved != nil {
			task = resolved
		}
	}

	return a2aRPCResult(req.ID, a2aProtocolTask(task, historyLen))
}

// a2aAcceptMessage creates a task, or continues the task named by the inbound
// message's taskId/contextId.
func (s *Service) a2aAcceptMessage(mgr *A2ATaskManager, msg A2AMessage, prompt, skill string) (*A2ATask, error) {
	if existing := mgr.TaskForContext(msg.TaskID, msg.ContextID); existing != nil {
		if existing.Status == TaskStatusCanceled {
			return nil, fmt.Errorf("cannot send message to canceled task %s", existing.ID)
		}
		updated, err := mgr.SendTaskMessage(existing.ID, prompt)
		if err != nil {
			return nil, err
		}
		return updated, nil
	}

	title := strings.TrimSpace(a2aMetadataString(msg.Metadata, "title"))
	if title == "" {
		title = a2aTitleFromPrompt(prompt)
	}
	return mgr.CreateTask(skill, prompt, title)
}

// a2aSendMessageStream handles message/stream.
func (s *Service) a2aSendMessageStream(ctx context.Context, req *A2ARPCRequest) A2ARPCResult {
	var params A2ASendMessageParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrInvalidParams, "invalid params: "+err.Error(), nil)}
	}
	prompt, err := a2aPromptFromMessage(params.Message)
	if err != nil {
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrInvalidParams, err.Error(), nil)}
	}
	skill, sErr := a2aSkillFromMetadata(params.Message.Metadata, params.Metadata)
	if sErr != nil {
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrInvalidParams, sErr.Error(), nil)}
	}

	mgr := s.a2aTaskManager()
	if mgr == nil {
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrInternal, "A2A task manager is not initialized", nil)}
	}

	task, err := s.a2aAcceptMessage(mgr, params.Message, prompt, skill)
	if err != nil {
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrInvalidParams, err.Error(), nil)}
	}
	return s.a2aStreamTask(ctx, req.ID, mgr, task.ID)
}

// a2aResubscribe re-attaches a stream to an existing task.
func (s *Service) a2aResubscribe(ctx context.Context, req *A2ARPCRequest) A2ARPCResult {
	var params A2ATaskQueryParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrInvalidParams, "invalid params: "+err.Error(), nil)}
	}
	if params.ID == "" {
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrInvalidParams, "id is required", nil)}
	}
	mgr := s.a2aTaskManager()
	if mgr == nil {
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrInternal, "A2A task manager is not initialized", nil)}
	}
	if _, err := mgr.GetTask(params.ID); err != nil {
		return A2ARPCResult{Response: a2aRPCError(req.ID, A2AErrTaskNotFound, err.Error(), nil)}
	}
	return s.a2aStreamTask(ctx, req.ID, mgr, params.ID)
}

// a2aStreamTask projects task lifecycle events onto protocol events.
func (s *Service) a2aStreamTask(ctx context.Context, id interface{}, mgr *A2ATaskManager, taskID string) A2ARPCResult {
	events, unsub := mgr.SubscribeTaskEvents(taskID)
	out := make(chan A2ARPCResponse, 16)

	go func() {
		defer close(out)
		defer unsub()

		send := func(res A2ARPCResponse) bool {
			select {
			case out <- res:
				return true
			case <-ctx.Done():
				return false
			}
		}

		// Snapshot first: a fast backend can finish before the subscription
		// above is registered, which would otherwise leave the client hanging.
		snapshot, err := mgr.GetTask(taskID)
		if err != nil {
			send(*a2aRPCError(id, A2AErrTaskNotFound, err.Error(), nil))
			return
		}
		final := isTerminalStatus(snapshot.Status)
		if final {
			if artifactEv, ok := a2aArtifactEvent(snapshot); ok {
				if !send(*a2aRPCResult(id, artifactEv)) {
					return
				}
			}
		}
		if !send(*a2aRPCResult(id, a2aStatusEvent(snapshot, final))) || final {
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-events:
				if !ok {
					return
				}
				current, gErr := mgr.GetTask(taskID)
				if gErr != nil {
					return
				}
				done := isTerminalStatus(current.Status)
				if done {
					if artifactEv, aOK := a2aArtifactEvent(current); aOK {
						if !send(*a2aRPCResult(id, artifactEv)) {
							return
						}
					}
				}
				if !send(*a2aRPCResult(id, a2aStatusEvent(current, done))) {
					return
				}
				if done {
					return
				}
			}
		}
	}()

	return A2ARPCResult{Stream: out}
}

func (s *Service) a2aGetTask(req *A2ARPCRequest) *A2ARPCResponse {
	var params A2ATaskQueryParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return a2aRPCError(req.ID, A2AErrInvalidParams, "invalid params: "+err.Error(), nil)
	}
	if params.ID == "" {
		return a2aRPCError(req.ID, A2AErrInvalidParams, "id is required", nil)
	}
	mgr := s.a2aTaskManager()
	if mgr == nil {
		return a2aRPCError(req.ID, A2AErrInternal, "A2A task manager is not initialized", nil)
	}
	task, err := mgr.GetTask(params.ID)
	if err != nil {
		return a2aRPCError(req.ID, A2AErrTaskNotFound, err.Error(), nil)
	}
	return a2aRPCResult(req.ID, a2aProtocolTask(task, params.HistoryLength))
}

func (s *Service) a2aCancelTask(req *A2ARPCRequest) *A2ARPCResponse {
	var params A2ATaskQueryParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return a2aRPCError(req.ID, A2AErrInvalidParams, "invalid params: "+err.Error(), nil)
	}
	if params.ID == "" {
		return a2aRPCError(req.ID, A2AErrInvalidParams, "id is required", nil)
	}
	mgr := s.a2aTaskManager()
	if mgr == nil {
		return a2aRPCError(req.ID, A2AErrInternal, "A2A task manager is not initialized", nil)
	}
	if _, err := mgr.GetTask(params.ID); err != nil {
		return a2aRPCError(req.ID, A2AErrTaskNotFound, err.Error(), nil)
	}
	task, err := mgr.CancelTask(params.ID)
	if err != nil {
		return a2aRPCError(req.ID, A2AErrTaskNotCancelable, err.Error(), nil)
	}
	return a2aRPCResult(req.ID, a2aProtocolTask(task, 0))
}

// a2aPromptFromMessage flattens message parts into a single prompt.
func a2aPromptFromMessage(msg A2AMessage) (string, error) {
	if len(msg.Parts) == 0 {
		return "", errors.New("message.parts must contain at least one part")
	}
	lines := make([]string, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch strings.ToLower(strings.TrimSpace(part.Kind)) {
		case "text", "":
			if txt := strings.TrimSpace(part.Text); txt != "" {
				lines = append(lines, txt)
			}
		case "file":
			if part.File == nil {
				return "", errors.New("file part requires a file object")
			}
			ref := part.File.URI
			if ref == "" {
				ref = part.File.Name
			}
			if ref == "" {
				return "", errors.New("file part requires a uri, name, or inline bytes")
			}
			lines = append(lines, fmt.Sprintf("[file] %s", ref))
		case "data":
			if part.Data == nil {
				continue
			}
			encoded, err := json.Marshal(part.Data)
			if err != nil {
				return "", fmt.Errorf("data part is not serializable: %w", err)
			}
			lines = append(lines, string(encoded))
		default:
			return "", fmt.Errorf("unsupported part kind: %s", part.Kind)
		}
	}
	if len(lines) == 0 {
		return "", errors.New("message.parts produced no usable content")
	}
	return strings.Join(lines, "\n"), nil
}

// a2aSkillFromMetadata resolves the requested skill and rejects unknown ones.
func a2aSkillFromMetadata(messageMeta, paramsMeta map[string]interface{}) (string, error) {
	skill := a2aMetadataString(messageMeta, "skillId")
	if skill == "" {
		skill = a2aMetadataString(paramsMeta, "skillId")
	}
	if skill == "" {
		skill = a2aMetadataString(paramsMeta, "skill")
	}
	if skill == "" {
		return SkillSoftwareEngineering, nil
	}
	switch skill {
	case SkillSoftwareEngineering, SkillCodeReview, SkillGeneralAutomation:
		return skill, nil
	}
	return "", fmt.Errorf("unknown skill: %s", skill)
}

func a2aMetadataString(meta map[string]interface{}, key string) string {
	if meta == nil {
		return ""
	}
	raw, ok := meta[key]
	if !ok {
		return ""
	}
	str, _ := raw.(string)
	return strings.TrimSpace(str)
}

func a2aTitleFromPrompt(prompt string) string {
	line := prompt
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)
	if len(line) > 80 {
		line = line[:80]
	}
	if line == "" {
		return "Autonomous A2A Task"
	}
	return line
}

func a2aStateFromStatus(status string) string {
	switch status {
	case TaskStatusSubmitted:
		return A2AStateSubmitted
	case TaskStatusWorking:
		return A2AStateWorking
	case TaskStatusInputRequired:
		return A2AStateInputRequired
	case TaskStatusCompleted:
		return A2AStateCompleted
	case TaskStatusFailed:
		return A2AStateFailed
	case TaskStatusCanceled:
		return A2AStateCanceled
	}
	return A2AStateUnknown
}

func a2aContextID(task *A2ATask) string {
	if task.ConversationID != "" {
		return task.ConversationID
	}
	return task.ID
}

func a2aTimestamp(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// a2aProtocolTask projects an internal task onto the protocol shape.
func a2aProtocolTask(task *A2ATask, historyLength int) *A2AProtocolTask {
	if task == nil {
		return nil
	}
	statusTime := task.UpdatedAt
	if statusTime.IsZero() {
		statusTime = task.CreatedAt
	}

	status := A2ATaskStatus{
		State:     a2aStateFromStatus(task.Status),
		Timestamp: a2aTimestamp(statusTime),
	}

	var history []A2AMessage
	history = append(history, A2AMessage{
		Kind:      "message",
		MessageID: "msg-" + task.ID + "-user",
		Role:      "user",
		Parts:     []A2APart{{Kind: "text", Text: task.Prompt}},
		ContextID: a2aContextID(task),
		TaskID:    task.ID,
	})

	if task.Error != "" {
		status.Message = &A2AMessage{
			Kind:      "message",
			MessageID: "msg-" + task.ID + "-error",
			Role:      "agent",
			Parts:     []A2APart{{Kind: "text", Text: task.Error}},
			ContextID: a2aContextID(task),
			TaskID:    task.ID,
		}
	} else if task.Result != nil && (task.Result.Text != "" || task.Status == TaskStatusCompleted) {
		text := task.Result.Text
		if text == "" {
			text = "Task completed."
		}
		agentMsg := A2AMessage{
			Kind:      "message",
			MessageID: "msg-" + task.ID + "-agent",
			Role:      "agent",
			Parts:     []A2APart{{Kind: "text", Text: text}},
			ContextID: a2aContextID(task),
			TaskID:    task.ID,
		}
		if task.Status == TaskStatusCompleted {
			status.Message = &agentMsg
		}
		history = append(history, agentMsg)
	}

	if historyLength > 0 && len(history) > historyLength {
		history = history[len(history)-historyLength:]
	}

	metadata := map[string]interface{}{
		"skill":      task.Skill,
		"created_at": a2aTimestamp(task.CreatedAt),
		"updated_at": a2aTimestamp(task.UpdatedAt),
		"status":     task.Status,
	}
	if task.ConversationID != "" {
		metadata["conversation_id"] = task.ConversationID
	}
	if task.Result != nil && task.Result.Backend != "" {
		metadata["backend"] = task.Result.Backend
	}

	return &A2AProtocolTask{
		Kind:      "task",
		ID:        task.ID,
		ContextID: a2aContextID(task),
		Status:    status,
		Artifacts: a2aArtifacts(task),
		History:   history,
		Metadata:  metadata,
	}
}

func a2aArtifacts(task *A2ATask) []A2AArtifact {
	if task == nil || task.Result == nil || len(task.Result.Artifacts) == 0 {
		return nil
	}
	out := make([]A2AArtifact, 0, len(task.Result.Artifacts))
	for i, path := range task.Result.Artifacts {
		uri := path
		if !strings.Contains(uri, "://") {
			uri = "file://" + strings.TrimPrefix(path, "/")
		}
		out = append(out, A2AArtifact{
			ArtifactID: fmt.Sprintf("%s-artifact-%d", task.ID, i+1),
			Name:       a2aBaseName(path),
			Parts: []A2APart{{
				Kind: "file",
				File: &A2AFileRef{Name: a2aBaseName(path), URI: uri},
			}},
		})
	}
	return out
}

func a2aBaseName(path string) string {
	trimmed := strings.TrimRight(path, "/")
	if idx := strings.LastIndexByte(trimmed, '/'); idx >= 0 && idx+1 < len(trimmed) {
		return trimmed[idx+1:]
	}
	return trimmed
}

func a2aStatusEvent(task *A2ATask, final bool) *A2AStatusUpdateEvent {
	projected := a2aProtocolTask(task, 0)
	return &A2AStatusUpdateEvent{
		Kind:      "status-update",
		TaskID:    projected.ID,
		ContextID: projected.ContextID,
		Status:    projected.Status,
		Final:     final,
	}
}

func a2aArtifactEvent(task *A2ATask) (*A2AArtifactUpdateEvent, bool) {
	artifacts := a2aArtifacts(task)
	if len(artifacts) == 0 {
		return nil, false
	}
	projected := a2aProtocolTask(task, 0)
	return &A2AArtifactUpdateEvent{
		Kind:      "artifact-update",
		TaskID:    projected.ID,
		ContextID: projected.ContextID,
		Artifact:  artifacts[len(artifacts)-1],
		Append:    false,
		LastChunk: true,
	}, true
}
