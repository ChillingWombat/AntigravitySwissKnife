package templates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Store manages scheduled templates and Antigravity sidecar tasks.
type Store struct {
	mu          sync.RWMutex
	sidecarsDir string
	templates   []ScheduledTemplate
}

// NewStore initializes a new Store with optional custom sidecars directory.
func NewStore(customSidecarsDir string) (*Store, error) {
	dir := customSidecarsDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		dir = filepath.Join(home, ".gemini", "config", "sidecars")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sidecars directory: %w", err)
	}

	return &Store{
		sidecarsDir: dir,
		templates:   GetDefaultTemplates(),
	}, nil
}

// GetTemplates returns all available scheduled templates.
func (s *Store) GetTemplates() []ScheduledTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.templates
}

func (s *Store) getTemplateByIDLocked(id string) *ScheduledTemplate {
	for _, t := range s.templates {
		if t.ID == id {
			return &t
		}
	}
	return nil
}

// GetTemplateByID finds a template by its unique ID.
func (s *Store) GetTemplateByID(id string) (*ScheduledTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t := s.getTemplateByIDLocked(id)
	if t != nil {
		return t, nil
	}
	return nil, fmt.Errorf("template not found: %s", id)
}

// slugify converts a string into a safe directory and filename slug.
func slugify(input string) string {
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")
	slug := reg.ReplaceAllString(strings.ToLower(input), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "scheduled-task"
	}
	return slug
}

// RenderPrompt substitutes parameters into a template's prompt string.
func RenderPrompt(templatePrompt string, params map[string]string, defaults []TemplateParameter) string {
	result := templatePrompt

	// Fill with parameter defaults first
	for _, p := range defaults {
		placeholder := fmt.Sprintf("{{%s}}", p.Key)
		val := p.DefaultValue
		if userVal, ok := params[p.Key]; ok && userVal != "" {
			val = userVal
		}
		result = strings.ReplaceAll(result, placeholder, val)
	}

	// Substitute any additional custom params passed
	for k, v := range params {
		placeholder := fmt.Sprintf("{{%s}}", k)
		result = strings.ReplaceAll(result, placeholder, v)
	}

	return result
}

// DeploySidecar writes an Antigravity ~/.gemini/config/sidecars/<slug>/sidecar.json configuration.
func (s *Store) DeploySidecar(req DeployTaskRequest) (*SidecarTaskInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmpl := s.getTemplateByIDLocked(req.TemplateID)

	displayName := req.DisplayName
	if displayName == "" {
		if tmpl != nil {
			displayName = tmpl.Title
		} else {
			displayName = "Scheduled Task"
		}
	}

	cronExpr := req.CronExpression
	if cronExpr == "" && tmpl != nil {
		cronExpr = tmpl.DefaultSchedule.CronExpression
	}
	if cronExpr == "" {
		cronExpr = "0 8 * * *"
	}

	finalPrompt := req.CustomPrompt
	if finalPrompt == "" && tmpl != nil {
		finalPrompt = RenderPrompt(tmpl.PromptTemplate, req.Parameters, tmpl.Parameters)
	}
	if finalPrompt == "" {
		finalPrompt = "Execute scheduled task briefing."
	}

	// If a target project is specified, prepend context
	if req.TargetProject != "" {
		finalPrompt = fmt.Sprintf("Target Project: %s\n\n%s", req.TargetProject, finalPrompt)
	}

	slug := slugify(displayName)
	targetDir := filepath.Join(s.sidecarsDir, slug)

	// Ensure unique directory name if existing
	if _, err := os.Stat(targetDir); err == nil {
		slug = fmt.Sprintf("%s-%d", slug, os.Getpid())
		targetDir = filepath.Join(s.sidecarsDir, slug)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create task sidecar directory: %w", err)
	}

	payload := SidecarPayload{
		Builtin:       "schedule",
		RestartPolicy: "always",
		Args: []string{
			cronExpr,
			"agentapi",
			"new-conversation",
			"--",
			finalPrompt,
		},
		DisplayName: displayName,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to encode sidecar payload: %w", err)
	}

	filePath := filepath.Join(targetDir, "sidecar.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write sidecar.json: %w", err)
	}

	promptPreview := finalPrompt
	if len(promptPreview) > 120 {
		promptPreview = promptPreview[:120] + "..."
	}

	return &SidecarTaskInfo{
		ID:             slug,
		DisplayName:    displayName,
		CronExpression: cronExpr,
		ScheduleText:   CronToHuman(cronExpr),
		PromptPreview:  promptPreview,
		Path:           filePath,
	}, nil
}

// ListSidecars enumerates all active sidecar tasks configured on disk.
func (s *Store) ListSidecars() ([]SidecarTaskInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.sidecarsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SidecarTaskInfo{}, nil
		}
		return nil, err
	}

	var results []SidecarTaskInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		cfgPath := filepath.Join(s.sidecarsDir, entry.Name(), "sidecar.json")
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			continue
		}

		var payload SidecarPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			continue
		}

		cronExpr := ""
		fullPrompt := ""
		if len(payload.Args) > 0 {
			cronExpr = payload.Args[0]
		}
		if len(payload.Args) > 4 {
			fullPrompt = payload.Args[4]
		} else if len(payload.Args) > 3 {
			fullPrompt = payload.Args[3]
		}
		promptPreview := fullPrompt
		if len(promptPreview) > 120 {
			promptPreview = promptPreview[:120] + "..."
		}

		disp := payload.DisplayName
		if disp == "" {
			disp = entry.Name()
		}

		results = append(results, SidecarTaskInfo{
			ID:             entry.Name(),
			DisplayName:    disp,
			CronExpression: cronExpr,
			ScheduleText:   CronToHuman(cronExpr),
			PromptPreview:  promptPreview,
			Prompt:         fullPrompt,
			Path:           cfgPath,
		})
	}

	return results, nil
}

// UpdateSidecar updates an existing sidecar task on disk.
func (s *Store) UpdateSidecar(req UpdateSidecarRequest) (*SidecarTaskInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.Contains(req.ID, "..") || strings.Contains(req.ID, "/") || strings.Contains(req.ID, "\\") {
		return nil, fmt.Errorf("invalid sidecar task ID: %s", req.ID)
	}

	targetDir := filepath.Join(s.sidecarsDir, req.ID)
	cfgPath := filepath.Join(targetDir, "sidecar.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("sidecar task not found: %s", req.ID)
	}

	var payload SidecarPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("invalid sidecar configuration: %w", err)
	}

	if req.DisplayName != "" {
		payload.DisplayName = req.DisplayName
	}

	cronExpr := req.CronExpression
	if cronExpr == "" && len(payload.Args) > 0 {
		cronExpr = payload.Args[0]
	}
	if cronExpr == "" {
		cronExpr = "0 8 * * *"
	}

	finalPrompt := req.Prompt
	if finalPrompt == "" && len(payload.Args) > 4 {
		finalPrompt = payload.Args[4]
	} else if finalPrompt == "" && len(payload.Args) > 3 {
		finalPrompt = payload.Args[3]
	}

	payload.Args = []string{
		cronExpr,
		"agentapi",
		"new-conversation",
		"--",
		finalPrompt,
	}

	newData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to encode sidecar payload: %w", err)
	}

	if err := os.WriteFile(cfgPath, newData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write sidecar.json: %w", err)
	}

	promptPreview := finalPrompt
	if len(promptPreview) > 120 {
		promptPreview = promptPreview[:120] + "..."
	}

	return &SidecarTaskInfo{
		ID:             req.ID,
		DisplayName:    payload.DisplayName,
		CronExpression: cronExpr,
		ScheduleText:   CronToHuman(cronExpr),
		PromptPreview:  promptPreview,
		Prompt:         finalPrompt,
		Path:           cfgPath,
	}, nil
}

// DeleteSidecar removes a sidecar directory and stops the scheduled task.
func (s *Store) DeleteSidecar(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		return fmt.Errorf("invalid sidecar task ID: %s", id)
	}

	targetDir := filepath.Join(s.sidecarsDir, id)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return fmt.Errorf("sidecar task not found: %s", id)
	}

	return os.RemoveAll(targetDir)
}
