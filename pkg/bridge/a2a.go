package bridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// A2A Task Status constants matching the Agent-to-Agent protocol specification.
const (
	TaskStatusSubmitted     = "submitted"
	TaskStatusWorking       = "working"
	TaskStatusInputRequired = "input_required"
	TaskStatusCompleted     = "completed"
	TaskStatusFailed        = "failed"
	TaskStatusCanceled      = "canceled"
)

// Default skills supported by Antigravity Swiss Knife A2A Agent Card.
const (
	SkillSoftwareEngineering = "software_engineering"
	SkillCodeReview          = "code_review"
	SkillGeneralAutomation   = "general_automation"
)

// A2ATaskResult contains the final output response and created artifacts.
type A2ATaskResult struct {
	Text      string   `json:"text"`
	Artifacts []string `json:"artifacts,omitempty"`
	StepCount int      `json:"step_count,omitempty"`
	Backend   string   `json:"backend,omitempty"`
}

// A2ATask represents an autonomous high-level task dispatched via the A2A protocol.
type A2ATask struct {
	ID             string         `json:"id"`
	Skill          string         `json:"skill"`
	Status         string         `json:"status"` // submitted | working | input_required | completed | failed | canceled
	Prompt         string         `json:"prompt"`
	Title          string         `json:"title,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	Result         *A2ATaskResult `json:"result,omitempty"`
	Error          string         `json:"error,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`

	cancelFunc context.CancelFunc `json:"-"`
}

// A2AEvent represents a real-time event for SSE streaming.
type A2AEvent struct {
	Type      string      `json:"type"` // task.created | task.status | task.progress | task.completed | task.failed | task.canceled
	TaskID    string      `json:"task_id"`
	Status    string      `json:"status"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// A2ASkill defines one skill exposed in the Agent Card.
type A2ASkill struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

// A2ACapabilities defines supported features in the Agent Card.
type A2ACapabilities struct {
	Streaming        bool `json:"streaming"`
	AsyncTasks       bool `json:"async_tasks"`
	TaskCancellation bool `json:"task_cancellation"`
	HumanInTheLoop   bool `json:"human_in_the_loop"`
}

// A2AAgentCard is the discoverable descriptor for /.well-known/agent.json per A2A spec.
type A2AAgentCard struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	URL             string                 `json:"url"`
	Version         string                 `json:"version"`
	ProtocolVersion string                 `json:"protocol_version"`
	Capabilities    A2ACapabilities        `json:"capabilities"`
	Skills          []A2ASkill             `json:"skills"`
	Authentication  map[string]interface{} `json:"authentication,omitempty"`
}

// DefaultA2AAgentCard constructs an RFC/A2A spec-compliant Agent Card.
func DefaultA2AAgentCard(baseURL string) A2AAgentCard {
	trimmedURL := strings.TrimRight(baseURL, "/")
	if trimmedURL == "" {
		trimmedURL = "http://127.0.0.1:8765"
	}

	return A2AAgentCard{
		Name:            "Antigravity Swiss Knife",
		Description:     "Autonomous full-lifecycle coding, refactoring, and agent execution companion for Google Antigravity 2.0",
		URL:             trimmedURL,
		Version:         "0.2.0",
		ProtocolVersion: "0.3.0",
		Capabilities: A2ACapabilities{
			Streaming:        true,
			AsyncTasks:       true,
			TaskCancellation: true,
			HumanInTheLoop:   true,
		},
		Skills: []A2ASkill{
			{
				ID:          SkillSoftwareEngineering,
				Name:        "Software Engineering & Architecture",
				Description: "Autonomous end-to-end coding, refactoring, test-driven development, and bug fixing in local repositories.",
				Tags:        []string{"coding", "architecture", "tdd", "antigravity", "refactoring"},
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prompt": map[string]interface{}{
							"type":        "string",
							"description": "High-level objective or feature specification.",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Optional short title for the task.",
						},
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "Optional conversation ID to resume or extend.",
						},
					},
					"required": []string{"prompt"},
				},
			},
			{
				ID:          SkillCodeReview,
				Name:        "Code Review & Quality Audit",
				Description: "Autonomous multi-axis code review checking correctness, regressions, standards, and anti-cruft.",
				Tags:        []string{"review", "audit", "security", "quality"},
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prompt": map[string]interface{}{
							"type":        "string",
							"description": "Code review instructions or commit/diff range to audit.",
						},
					},
					"required": []string{"prompt"},
				},
			},
			{
				ID:          SkillGeneralAutomation,
				Name:        "General Task Automation",
				Description: "Workstation automation, command execution, and file transformations.",
				Tags:        []string{"automation", "system", "workspace"},
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prompt": map[string]interface{}{
							"type":        "string",
							"description": "Automation task or workflow instructions.",
						},
					},
					"required": []string{"prompt"},
				},
			},
		},
		Authentication: map[string]interface{}{
			"type": "bearer",
			"oauth2": map[string]interface{}{
				"authorization_endpoint": fmt.Sprintf("%s/oauth/authorize", trimmedURL),
				"token_endpoint":         fmt.Sprintf("%s/oauth/token", trimmedURL),
				"scopes":                 []string{"a2a", "tasks", "mcp"},
			},
		},
	}
}

// A2ATaskManager manages lifecycle and real-time execution of A2A tasks.
type A2ATaskManager struct {
	mu             sync.RWMutex
	tasks          map[string]*A2ATask
	order          []string
	subscribers    map[string][]chan A2AEvent
	allSubscribers []chan A2AEvent
	getBackend     func() agentBackend
}

// NewA2ATaskManager initializes an A2A Task Manager.
func NewA2ATaskManager(getBackend func() agentBackend) *A2ATaskManager {
	return &A2ATaskManager{
		tasks:       make(map[string]*A2ATask),
		order:       make([]string, 0),
		subscribers: make(map[string][]chan A2AEvent),
		getBackend:  getBackend,
	}
}

func generateA2ATaskID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("task-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

// CreateTask registers a high-level task and launches autonomous background execution.
func (m *A2ATaskManager) CreateTask(skill, prompt, title string) (*A2ATask, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if skill == "" {
		skill = SkillSoftwareEngineering
	}
	if title == "" {
		title = "Autonomous A2A Task"
	}

	taskID := generateA2ATaskID()
	now := time.Now()

	ctx, cancel := context.WithCancel(context.Background())

	task := &A2ATask{
		ID:         taskID,
		Skill:      skill,
		Status:     TaskStatusSubmitted,
		Prompt:     prompt,
		Title:      title,
		CreatedAt:  now,
		UpdatedAt:  now,
		cancelFunc: cancel,
	}

	m.mu.Lock()
	m.tasks[taskID] = task
	m.order = append([]string{taskID}, m.order...)
	m.mu.Unlock()

	m.broadcastEvent(A2AEvent{
		Type:      "task.created",
		TaskID:    taskID,
		Status:    TaskStatusSubmitted,
		Message:   "Task received and queued for execution",
		Timestamp: now,
	})

	// Launch background autonomous execution loop
	go m.executeTask(ctx, task)

	return m.cloneTask(task), nil
}

func (m *A2ATaskManager) executeTask(ctx context.Context, task *A2ATask) {
	m.updateTaskStatus(task.ID, TaskStatusWorking, "Agent actively working on task", nil)

	backend := m.getBackend()
	if backend == nil {
		m.failTask(task.ID, errors.New("no active agent backend configured"))
		return
	}

	// 1. Dispatch high-level task to Antigravity
	createArgs := map[string]interface{}{
		"prompt": task.Prompt,
		"title":  task.Title,
	}

	respStr, err := backend.CreateConversation(createArgs)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		m.failTask(task.ID, fmt.Errorf("create conversation: %w", err))
		return
	}

	var parsed struct {
		ConversationID string   `json:"conversation_id"`
		Status         string   `json:"status"`
		Response       string   `json:"response"`
		Artifacts      []string `json:"artifacts"`
	}
	_ = json.Unmarshal([]byte(respStr), &parsed)

	convID := parsed.ConversationID
	if convID != "" {
		m.mu.Lock()
		task.ConversationID = convID
		task.UpdatedAt = time.Now()
		m.mu.Unlock()
	}

	// If the backend already completed in one turn (e.g. single-turn CLI)
	if parsed.Status == TaskStatusCompleted && parsed.Response != "" {
		result := &A2ATaskResult{
			Text:      parsed.Response,
			Artifacts: parsed.Artifacts,
			Backend:   backend.Name(),
		}
		m.completeTask(task.ID, result)
		return
	}

	// 2. Poll or await response until Antigravity finishes generating
	m.broadcastEvent(A2AEvent{
		Type:      "task.progress",
		TaskID:    task.ID,
		Status:    TaskStatusWorking,
		Message:   "Waiting for Antigravity autonomous execution turn",
		Timestamp: time.Now(),
	})

	awaitArgs := map[string]interface{}{
		"conversation_id": convID,
		"timeout_seconds": 180,
	}

	awaitRespStr, err := backend.AwaitResponse(awaitArgs)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		m.failTask(task.ID, fmt.Errorf("await response: %w", err))
		return
	}

	var awaitParsed struct {
		Status    string   `json:"status"`
		Response  string   `json:"response"`
		Artifacts []string `json:"artifacts"`
	}
	_ = json.Unmarshal([]byte(awaitRespStr), &awaitParsed)

	resultText := awaitParsed.Response
	if resultText == "" {
		resultText = "Task completed by Antigravity."
	}

	result := &A2ATaskResult{
		Text:      resultText,
		Artifacts: awaitParsed.Artifacts,
		Backend:   backend.Name(),
	}

	m.completeTask(task.ID, result)
}

// TaskForContext resolves a task by protocol task ID or context ID (the
// Antigravity conversation ID). The most recent match wins, so a client that
// echoes back a contextId continues the same task. Returns nil when unknown.
func (m *A2ATaskManager) TaskForContext(taskID, contextID string) *A2ATask {
	taskID = strings.TrimSpace(taskID)
	contextID = strings.TrimSpace(contextID)
	if taskID == "" && contextID == "" {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, id := range m.order {
		task := m.tasks[id]
		if task == nil {
			continue
		}
		if taskID != "" && task.ID == taskID {
			return m.cloneTask(task)
		}
		if contextID != "" && (task.ConversationID == contextID || task.ID == contextID) {
			return m.cloneTask(task)
		}
	}
	return nil
}

// GetTask returns a copy of the task by ID.
func (m *A2ATaskManager) GetTask(taskID string) (*A2ATask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return m.cloneTask(task), nil
}

// ListTasks returns tasks filtered by optional status, ordered most-recent first.
func (m *A2ATaskManager) ListTasks(status string, limit int) []*A2ATask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*A2ATask, 0)
	for _, id := range m.order {
		task := m.tasks[id]
		if task == nil {
			continue
		}
		if status != "" && task.Status != status {
			continue
		}
		out = append(out, m.cloneTask(task))
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// SendTaskMessage delivers follow-up instructions into the running task.
func (m *A2ATaskManager) SendTaskMessage(taskID, message string) (*A2ATask, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, errors.New("message is required")
	}

	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status == TaskStatusCanceled {
		m.mu.Unlock()
		return nil, errors.New("cannot send message to canceled task")
	}

	ctx, cancel := context.WithCancel(context.Background())
	task.cancelFunc = cancel
	task.Status = TaskStatusWorking
	task.UpdatedAt = time.Now()
	convID := task.ConversationID
	m.mu.Unlock()

	backend := m.getBackend()
	if backend != nil {
		_, _ = backend.SendMessage(map[string]interface{}{
			"message":         message,
			"conversation_id": convID,
		})
	}

	m.broadcastEvent(A2AEvent{
		Type:      "task.status",
		TaskID:    taskID,
		Status:    TaskStatusWorking,
		Message:   "Follow-up message sent; waiting for response",
		Timestamp: time.Now(),
	})

	go func() {
		if backend == nil {
			return
		}
		awaitArgs := map[string]interface{}{
			"conversation_id": convID,
			"timeout_seconds": 180,
		}
		respStr, err := backend.AwaitResponse(awaitArgs)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			m.failTask(taskID, err)
			return
		}
		var parsed struct {
			Response  string   `json:"response"`
			Artifacts []string `json:"artifacts"`
		}
		_ = json.Unmarshal([]byte(respStr), &parsed)
		res := &A2ATaskResult{
			Text:      parsed.Response,
			Artifacts: parsed.Artifacts,
			Backend:   backend.Name(),
		}
		m.completeTask(taskID, res)
	}()

	return m.GetTask(taskID)
}

// CancelTask terminates active execution of a task.
func (m *A2ATaskManager) CancelTask(taskID string) (*A2ATask, error) {
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	if task.cancelFunc != nil {
		task.cancelFunc()
	}

	now := time.Now()
	task.Status = TaskStatusCanceled
	task.UpdatedAt = now
	task.CompletedAt = &now
	convID := task.ConversationID
	m.mu.Unlock()

	backend := m.getBackend()
	if backend != nil && convID != "" {
		_, _ = backend.StopConversation(map[string]interface{}{
			"conversation_id": convID,
		})
	}

	m.broadcastEvent(A2AEvent{
		Type:      "task.canceled",
		TaskID:    taskID,
		Status:    TaskStatusCanceled,
		Message:   "Task execution canceled by caller",
		Timestamp: now,
	})

	return m.GetTask(taskID)
}

// AwaitTask blocks until the task reaches completed, failed, or canceled status.
func (m *A2ATaskManager) AwaitTask(ctx context.Context, taskID string) (*A2ATask, error) {
	events, unsub := m.SubscribeTaskEvents(taskID)
	defer unsub()

	// Initial check in case it's already done
	task, err := m.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	if isTerminalStatus(task.Status) {
		return task, nil
	}

	for {
		select {
		case <-ctx.Done():
			return m.GetTask(taskID)
		case ev := <-events:
			if isTerminalStatus(ev.Status) {
				return m.GetTask(taskID)
			}
		}
	}
}

func isTerminalStatus(st string) bool {
	return st == TaskStatusCompleted || st == TaskStatusFailed || st == TaskStatusCanceled
}

func (m *A2ATaskManager) updateTaskStatus(taskID, status, msg string, data interface{}) {
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return
	}
	task.Status = status
	task.UpdatedAt = time.Now()
	m.mu.Unlock()

	m.broadcastEvent(A2AEvent{
		Type:      "task.status",
		TaskID:    taskID,
		Status:    status,
		Message:   msg,
		Data:      data,
		Timestamp: time.Now(),
	})
}

func (m *A2ATaskManager) completeTask(taskID string, res *A2ATaskResult) {
	now := time.Now()
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return
	}
	task.Status = TaskStatusCompleted
	task.Result = res
	task.UpdatedAt = now
	task.CompletedAt = &now
	m.mu.Unlock()

	m.broadcastEvent(A2AEvent{
		Type:      "task.completed",
		TaskID:    taskID,
		Status:    TaskStatusCompleted,
		Message:   "Task successfully completed",
		Data:      res,
		Timestamp: now,
	})
}

func (m *A2ATaskManager) failTask(taskID string, err error) {
	now := time.Now()
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return
	}
	task.Status = TaskStatusFailed
	task.Error = err.Error()
	task.UpdatedAt = now
	task.CompletedAt = &now
	m.mu.Unlock()

	m.broadcastEvent(A2AEvent{
		Type:      "task.failed",
		TaskID:    taskID,
		Status:    TaskStatusFailed,
		Message:   err.Error(),
		Timestamp: now,
	})
}

// SubscribeTaskEvents returns an event channel for a specific task.
func (m *A2ATaskManager) SubscribeTaskEvents(taskID string) (<-chan A2AEvent, func()) {
	ch := make(chan A2AEvent, 32)
	m.mu.Lock()
	m.subscribers[taskID] = append(m.subscribers[taskID], ch)
	m.mu.Unlock()

	unsub := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		subs := m.subscribers[taskID]
		for i, s := range subs {
			if s == ch {
				m.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
	return ch, unsub
}

// SubscribeAllEvents returns an event channel for all tasks.
func (m *A2ATaskManager) SubscribeAllEvents() (<-chan A2AEvent, func()) {
	ch := make(chan A2AEvent, 64)
	m.mu.Lock()
	m.allSubscribers = append(m.allSubscribers, ch)
	m.mu.Unlock()

	unsub := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for i, s := range m.allSubscribers {
			if s == ch {
				m.allSubscribers = append(m.allSubscribers[:i], m.allSubscribers[i+1:]...)
				break
			}
		}
	}
	return ch, unsub
}

func (m *A2ATaskManager) broadcastEvent(ev A2AEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.allSubscribers {
		select {
		case ch <- ev:
		default:
		}
	}

	for _, ch := range m.subscribers[ev.TaskID] {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (m *A2ATaskManager) cloneTask(t *A2ATask) *A2ATask {
	if t == nil {
		return nil
	}
	cpy := *t
	if t.Result != nil {
		resCpy := *t.Result
		if t.Result.Artifacts != nil {
			resCpy.Artifacts = append([]string{}, t.Result.Artifacts...)
		}
		cpy.Result = &resCpy
	}
	return &cpy
}
