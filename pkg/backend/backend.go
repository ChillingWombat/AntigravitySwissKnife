package backend

import (
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
)

// BackendService owns the daemon-backed state used by the web UI.
type BackendService interface {
	Accounts() AccountService
	Quota() QuotaService
	GUI() GUIService
}

type AccountImport struct {
	Email        string `json:"email"`
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	Label        string `json:"label"`
	TOTPSecret   string `json:"totp_secret"`
}

type AccountUpdate struct {
	Email                string  `json:"email"`
	Label                string  `json:"label"`
	PlanTier             string  `json:"plan_tier"`
	Status               string  `json:"status"`
	Priority             string  `json:"priority"`
	Notes                string  `json:"notes"`
	Password             string  `json:"password"`
	TOTPSecret           string  `json:"totp_secret"`
	RefreshToken         string  `json:"refresh_token"`
	AccessToken          string  `json:"access_token"`
	Credits              float64 `json:"credits"`
	EnableCreditOverages bool    `json:"enable_credit_overages"`
	AllowClaudeGPT       bool    `json:"allow_claude_gpt"`
	SetActive            bool    `json:"set_active"`
}

type AccountSwitch struct {
	Email       string `json:"email"`
	RelaunchIDE *bool  `json:"relaunch_ide,omitempty"`
	TargetApp   string `json:"target_app,omitempty"`
}

type AccountSwitchResult struct {
	Switched           bool              `json:"switched"`
	Account            string            `json:"account"`
	Success            bool              `json:"success"`
	ActiveAccount      string            `json:"active_account"`
	RelaunchIDE        bool              `json:"relaunch_ide"`
	TargetApp          string            `json:"target_app"`
	MultiAppSyncMode   string            `json:"multi_app_sync_mode"`
	ActiveAppAccounts  map[string]string `json:"active_app_accounts"`
	AutoSwitchDisabled bool              `json:"auto_switch_disabled,omitempty"`
}

type AccountService interface {
	ListAccounts() []*keyring.Account
	GetAccount(email string) (*keyring.Account, error)
	ScanAccounts() ([]keyring.DiscoveredAccount, error)
	ImportAccount(req AccountImport) (*keyring.Account, error)
	UpdateAccount(req AccountUpdate) (map[string]interface{}, error)
	RemoveAccount(email string) (map[string]interface{}, error)
	SwitchAccount(req AccountSwitch) (AccountSwitchResult, error)
	SetTOTPSecret(email, secret string) error
	ExportAccounts() []keyring.AccountExport
	BatchImportAccounts(items []keyring.BatchImportItem) (int, error)
}

type QuotaService interface {
	GetSummary(email string, refresh bool) (*quota.QuotaSummary, error)
	GetFleet() *quota.FleetQuotaSummary
	RefreshFleet() error
}

type GUIService interface {
	GetConfig() gui.Config
	UpdateConfigPartial(rawJSON []byte) (*gui.GUIConfig, error)
	DetectProjects() ([]gui.ProjectItem, error)
	SetProjectColor(projectName, hexColor string) error
	DeleteProject(projectName string) error
	RemoveProjectColor(projectName string) error
	UpdateProjectOrder(order []string) error
	Apply() (*gui.ApplyResult, error)
	GetArchivedProjects() ([]gui.ArchivedProjectItem, error)
	ArchiveProject(projectName string) error
	RestoreProject(projectName string) error
	OpenProjectSettings(projectName string) error
}
