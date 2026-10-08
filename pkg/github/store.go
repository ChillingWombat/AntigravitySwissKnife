package github

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// AgentTaskData stores persistent mappings between conversations, agents, and GitHub issues.
type AgentTaskData struct {
	AgentLabels   map[string]string `json:"agent_labels"`   // conversation_id -> label (e.g. "Lead Orchestrator")
	IssueBindings map[string]int    `json:"issue_bindings"` // conversation_id -> issue_number
	PRBindings    map[string]int    `json:"pr_bindings"`    // conversation_id -> pr_number
}

// Store provides thread-safe access to agent task bindings.
type Store struct {
	mu       sync.RWMutex
	filePath string
	data     AgentTaskData
}

// NewStore initializes a new Store instance.
func NewStore(customPath string) (*Store, error) {
	target := customPath
	if target == "" {
		target = filepath.Join(core.GetConfigDir(), "github_agent_tasks.json")
	}

	s := &Store{
		filePath: target,
		data: AgentTaskData{
			AgentLabels:   make(map[string]string),
			IssueBindings: make(map[string]int),
			PRBindings:    make(map[string]int),
		},
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load agent tasks store: %w", err)
	}

	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var loaded AgentTaskData
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	if loaded.AgentLabels == nil {
		loaded.AgentLabels = make(map[string]string)
	}
	if loaded.IssueBindings == nil {
		loaded.IssueBindings = make(map[string]int)
	}
	if loaded.PRBindings == nil {
		loaded.PRBindings = make(map[string]int)
	}

	s.data = loaded
	return nil
}

func (s *Store) save() error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, bytes, 0644)
}

// GetAgentLabel retrieves the custom label for a conversation.
func (s *Store) GetAgentLabel(conversationID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.AgentLabels[conversationID]
}

// SetAgentLabel assigns a custom label to a conversation.
func (s *Store) SetAgentLabel(conversationID string, label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if label == "" {
		delete(s.data.AgentLabels, conversationID)
	} else {
		s.data.AgentLabels[conversationID] = label
	}
	return s.save()
}

// GetBoundIssue returns the issue number bound to a conversation, or 0 if none.
func (s *Store) GetBoundIssue(conversationID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.IssueBindings[conversationID]
}

// BindIssue associates a conversation with an issue number.
func (s *Store) BindIssue(conversationID string, issueNumber int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if issueNumber <= 0 {
		delete(s.data.IssueBindings, conversationID)
	} else {
		s.data.IssueBindings[conversationID] = issueNumber
	}
	return s.save()
}

// GetConversationsForIssue returns all conversation IDs bound to a given issue number.
func (s *Store) GetConversationsForIssue(issueNumber int) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var convs []string
	for convID, num := range s.data.IssueBindings {
		if num == issueNumber {
			convs = append(convs, convID)
		}
	}
	return convs
}

// GetAllLabels returns a copy of all agent labels.
func (s *Store) GetAllLabels() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make(map[string]string, len(s.data.AgentLabels))
	for k, v := range s.data.AgentLabels {
		res[k] = v
	}
	return res
}

// GetAllBindings returns a copy of all issue bindings.
func (s *Store) GetAllBindings() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make(map[string]int, len(s.data.IssueBindings))
	for k, v := range s.data.IssueBindings {
		res[k] = v
	}
	return res
}
