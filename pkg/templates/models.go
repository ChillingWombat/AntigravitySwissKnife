package templates

// TemplateSchedule defines the recurring cron timing for a scheduled task.
type TemplateSchedule struct {
	Frequency      string `json:"frequency"`       // "hourly", "daily", "weekly", "custom"
	TimeOfDay      string `json:"time_of_day"`     // "08:00", "17:00", "23:00"
	DaysOfWeek     []int  `json:"days_of_week"`    // 1=Mon ... 7=Sun or 0=Sun
	CronExpression string `json:"cron_expression"` // "0 8 * * *"
}

// TemplateParameter defines a configurable field in the template settings modal.
type TemplateParameter struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Type         string `json:"type"` // "text", "textarea", "list", "select"
	DefaultValue string `json:"default_value"`
	Description  string `json:"description"`
}

// ScheduledTemplate represents a pre-configured automation template.
type ScheduledTemplate struct {
	ID              string              `json:"id"`
	Title           string              `json:"title"`
	Subtitle        string              `json:"subtitle"`
	Category        string              `json:"category"` // "Personal Assistant", "CI/CD & Development", "Security & Quality", "Research & Market"
	Icon            string              `json:"icon"`     // Lucide icon name
	Description     string              `json:"description"`
	DefaultSchedule TemplateSchedule    `json:"default_schedule"`
	RequiredTools   []string            `json:"required_tools"`  // Required MCP servers
	RequiredSkills  []string            `json:"required_skills"` // Required skills
	Parameters      []TemplateParameter `json:"parameters"`
	PromptTemplate  string              `json:"prompt_template"`
}

// SidecarPayload defines the JSON structure of an Antigravity ~/.gemini/config/sidecars/<id>/sidecar.json file.
type SidecarPayload struct {
	Builtin       string   `json:"builtin"`                 // "schedule"
	RestartPolicy string   `json:"restart_policy,omitempty"` // "always"
	Args          []string `json:"args"`
	DisplayName   string   `json:"display_name"`
}

// DeployTaskRequest holds parameters sent from the modal to instantiate a template.
type DeployTaskRequest struct {
	TemplateID     string            `json:"template_id"`
	DisplayName    string            `json:"display_name"`
	CronExpression string            `json:"cron_expression"`
	TargetProject  string            `json:"target_project"`
	CustomPrompt   string            `json:"custom_prompt"`
	Parameters     map[string]string `json:"parameters"`
}

// SidecarTaskInfo describes an active sidecar task detected on disk.
type SidecarTaskInfo struct {
	ID             string `json:"id"`
	DisplayName    string `json:"display_name"`
	CronExpression string `json:"cron_expression"`
	PromptPreview  string `json:"prompt_preview"`
	Path           string `json:"path"`
}
