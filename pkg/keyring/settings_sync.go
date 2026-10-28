package keyring

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

var (
	sharedSettingsMu sync.RWMutex

	// AppStoragePreferenceKeys defines known app preferences stored in app_storage.json.
	AppStoragePreferenceKeys = []string{
		"keepComputerAwake",
		"runInBackground",
		"autoCheckForUpdates",
		"didAskForNotificationPermission",
		"ide-install-wizard-shown",
		"new-convo-magic-workspace-selected",
	}

	// UnifiedSyncPreferenceKeys defines settings keys stored in state.vscdb ItemTable.
	UnifiedSyncPreferenceKeys = []string{
		"antigravityUnifiedStateSync.agentPreferences",
		"antigravityUnifiedStateSync.browserPreferences",
		"antigravityUnifiedStateSync.modelPreferences",
		"antigravityUnifiedStateSync.overrideStore",
		"antigravityUnifiedStateSync.theme",
		"antigravityUnifiedStateSync.editorPreferences",
		"antigravityUnifiedStateSync.tabPreferences",
		"antigravityUnifiedStateSync.windowPreferences",
		"antigravityUnifiedStateSync.seenNuxIds",
		"antigravityUnifiedStateSync.sidebarWorkspaces",
		"antigravityUnifiedStateSync.artifactReview",
		"antigravityUnifiedStateSync.enterprisePreferences",
		"antigravityOnboarding",
		"colorThemeData",
		"iconThemeData",
		"productIconThemeData",
	}
)

// SharedAntigravitySettings encapsulates all shared Antigravity application,
// agent, security, and onboarding preferences that persist across account switches.
type SharedAntigravitySettings struct {
	// AppStorage contains preferences from app_storage.json (e.g. keepComputerAwake, runInBackground)
	AppStorage map[string]interface{} `json:"app_storage,omitempty"`

	// UserSettings contains preferences from ~/.gemini/config/config.json -> userSettings
	// (e.g. permissionPreset, autoExecutionPolicy, browserJsExecutionPolicy, nonWorkspaceFileAccessPolicy, themeMode, useAiCredits, verboseAgentChat)
	UserSettings map[string]interface{} `json:"user_settings,omitempty"`

	// VscdbItems contains ItemTable key/values from state.vscdb
	// (e.g. antigravityUnifiedStateSync.agentPreferences, browserPreferences, modelPreferences, overrideStore, theme)
	VscdbItems map[string]string `json:"vscdb_items,omitempty"`

	// PbtxtOnboardingCompleted stores agent_onboarding_completed from antigravity_state.pbtxt
	PbtxtOnboardingCompleted string `json:"pbtxt_onboarding_completed,omitempty"`

	// PbtxtSeenNuxs stores seen NUX uids
	PbtxtSeenNuxs []string `json:"pbtxt_seen_nuxs,omitempty"`

	// PbtxtCompletedSteps stores post-onboarding completed steps
	PbtxtCompletedSteps []string `json:"pbtxt_completed_steps,omitempty"`

	// PbtxtRawSections stores raw protobuf text blocks (post_onboarding, seen_nuxs)
	PbtxtRawSections map[string]string `json:"pbtxt_raw_sections,omitempty"`

	LastUpdated time.Time `json:"last_updated"`
}

// NewSharedAntigravitySettings creates an initialized SharedAntigravitySettings struct.
func NewSharedAntigravitySettings() *SharedAntigravitySettings {
	return &SharedAntigravitySettings{
		AppStorage:          make(map[string]interface{}),
		UserSettings:        make(map[string]interface{}),
		VscdbItems:          make(map[string]string),
		PbtxtSeenNuxs:       make([]string, 0),
		PbtxtCompletedSteps: make([]string, 0),
		PbtxtRawSections:    make(map[string]string),
		LastUpdated:         time.Now(),
	}
}

// GetSharedSettingsFilePath returns the canonical persistence path for shared settings
// in Swiss Knife's configuration directory (~/.config/antigravity-swiss/shared_antigravity_settings.json).
func GetSharedSettingsFilePath() string {
	return filepath.Join(core.GetConfigDir(), "shared_antigravity_settings.json")
}

// LoadSharedAntigravitySettings loads the canonical settings from Swiss Knife's config directory.
func LoadSharedAntigravitySettings() (*SharedAntigravitySettings, error) {
	sharedSettingsMu.RLock()
	defer sharedSettingsMu.RUnlock()

	path := GetSharedSettingsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var s SharedAntigravitySettings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if s.AppStorage == nil {
		s.AppStorage = make(map[string]interface{})
	}
	if s.UserSettings == nil {
		s.UserSettings = make(map[string]interface{})
	}
	if s.VscdbItems == nil {
		s.VscdbItems = make(map[string]string)
	}
	if s.PbtxtRawSections == nil {
		s.PbtxtRawSections = make(map[string]string)
	}
	return &s, nil
}

// SaveSharedAntigravitySettings saves the canonical settings to Swiss Knife's config directory.
func SaveSharedAntigravitySettings(s *SharedAntigravitySettings) error {
	if s == nil {
		return nil
	}
	sharedSettingsMu.Lock()
	defer sharedSettingsMu.Unlock()

	s.LastUpdated = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	path := GetSharedSettingsFilePath()
	if !isSafeTestWritePath(path) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return writeAtomic(path, data, 0600)
}

// getGeminiConfigDir resolves the ~/.gemini/config directory per host and test environment.
func getGeminiConfigDir() string {
	if d := os.Getenv("GEMINI_CONFIG_DIR"); d != "" {
		return d
	}
	antiDir := core.GetAntigravityDir()
	parent := filepath.Dir(antiDir)
	if filepath.Base(parent) == ".gemini" {
		return filepath.Join(parent, "config")
	}
	home := getUserHomeDir()
	if home != "" {
		return filepath.Join(home, ".gemini", "config")
	}
	return filepath.Join(antiDir, "config")
}

// getGeminiConfigPath returns the absolute path to ~/.gemini/config/config.json.
func getGeminiConfigPath() string {
	return filepath.Join(getGeminiConfigDir(), "config.json")
}

// isAppStoragePreference returns true if the key represents an application preference
// rather than conversation metadata, user login email, or session layouts.
func isAppStoragePreference(key string) bool {
	if key == "jetski.onboarding.lastLoginUsername" ||
		key == "jetski.onboarding.lastLoginIsGcpTos" ||
		strings.HasPrefix(key, "antigravity-multi-conversation-layout") ||
		strings.HasPrefix(key, "aux-pane") ||
		key == "projectsOrder" ||
		key == "comments" ||
		key == "diffs" ||
		key == "drafts" ||
		key == "antigravity_swiss_last_conversation_path" {
		return false
	}
	return true
}

// mergeMaps recursively merges src map into dest map, preserving nested objects.
func mergeMaps(dest, src map[string]interface{}) map[string]interface{} {
	if dest == nil {
		dest = make(map[string]interface{})
	}
	for k, v := range src {
		if srcMap, ok := v.(map[string]interface{}); ok {
			if destMap, ok := dest[k].(map[string]interface{}); ok {
				dest[k] = mergeMaps(destMap, srcMap)
				continue
			}
		}
		dest[k] = v
	}
	return dest
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// SnapshotLiveAntigravitySettings scans live host surfaces (app_storage.json, config.json,
// state.vscdb, antigravity_state.pbtxt) and merges detected preferences into the
// canonical shared settings file.
func SnapshotLiveAntigravitySettings() (*SharedAntigravitySettings, error) {
	shared, _ := LoadSharedAntigravitySettings()
	if shared == nil {
		shared = NewSharedAntigravitySettings()
	}

	// 1. Snapshot app_storage.json
	storagePath := filepath.Join(core.GetAntigravityHostConfigDir(), "app_storage.json")
	snapshotAppStorage(storagePath, shared)

	// 2. Snapshot ~/.gemini/config/config.json
	configPath := getGeminiConfigPath()
	snapshotGeminiConfig(configPath, shared)

	// 3. Snapshot state.vscdb from Antigravity and VS Code
	vscdbAntigravity := filepath.Join(core.GetAntigravityHostConfigDir(), "User", "globalStorage", "state.vscdb")
	snapshotVscdb(vscdbAntigravity, shared)
	vscdbCode := filepath.Join(core.GetVSCodeHostConfigDir(), "User", "globalStorage", "state.vscdb")
	snapshotVscdb(vscdbCode, shared)

	// 4. Snapshot antigravity_state.pbtxt
	pbtxtPath := filepath.Join(core.GetAntigravityDir(), "antigravity_state.pbtxt")
	snapshotPbtxt(pbtxtPath, shared)

	_ = SaveSharedAntigravitySettings(shared)
	return shared, nil
}

func snapshotAppStorage(storagePath string, shared *SharedAntigravitySettings) {
	if storagePath == "" {
		return
	}
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return
	}
	if shared.AppStorage == nil {
		shared.AppStorage = make(map[string]interface{})
	}
	for k, v := range rawMap {
		if isAppStoragePreference(k) {
			shared.AppStorage[k] = v
		}
	}
}

func snapshotGeminiConfig(configPath string, shared *SharedAntigravitySettings) {
	if configPath == "" {
		return
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return
	}
	if userSettings, ok := rawMap["userSettings"].(map[string]interface{}); ok {
		if shared.UserSettings == nil {
			shared.UserSettings = make(map[string]interface{})
		}
		shared.UserSettings = mergeMaps(shared.UserSettings, userSettings)
	}
}

func snapshotVscdb(vscdbPath string, shared *SharedAntigravitySettings) {
	if vscdbPath == "" || !isSafeTestWritePath(vscdbPath) {
		return
	}
	if _, err := os.Stat(vscdbPath); err != nil {
		return
	}
	db, err := sql.Open("sqlite", vscdbPath+"?_busy_timeout=5000")
	if err != nil {
		return
	}
	defer db.Close()

	var tableCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='ItemTable'").Scan(&tableCount)
	if err != nil || tableCount == 0 {
		return
	}

	rows, err := db.Query("SELECT key, value FROM ItemTable WHERE key LIKE 'antigravityUnifiedStateSync.%' OR key IN ('colorThemeData', 'iconThemeData', 'productIconThemeData')")
	if err != nil {
		return
	}
	defer rows.Close()

	if shared.VscdbItems == nil {
		shared.VscdbItems = make(map[string]string)
	}

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil && v != "" {
			if k == "antigravityUnifiedStateSync.userStatus" || k == "antigravityUnifiedStateSync.oauthToken" {
				continue
			}
			shared.VscdbItems[k] = v
		}
	}
}

func snapshotPbtxt(pbtxtPath string, shared *SharedAntigravitySettings) {
	if pbtxtPath == "" {
		return
	}
	data, err := os.ReadFile(pbtxtPath)
	if err != nil {
		return
	}
	content := string(data)

	// 1. agent_onboarding_completed
	reOnboarding := regexp.MustCompile(`(?m)^\s*agent_onboarding_completed\s*:\s*([^\r\n#]+)`)
	if m := reOnboarding.FindStringSubmatch(content); len(m) > 1 {
		shared.PbtxtOnboardingCompleted = strings.TrimSpace(m[1])
	}

	// 2. post_onboarding block
	rePostBlock := regexp.MustCompile(`(?s)post_onboarding\s*:\s*\{([^}]*)\}`)
	if m := rePostBlock.FindStringSubmatch(content); len(m) > 1 {
		if shared.PbtxtRawSections == nil {
			shared.PbtxtRawSections = make(map[string]string)
		}
		shared.PbtxtRawSections["post_onboarding"] = strings.TrimSpace(m[0])
		reStep := regexp.MustCompile(`(?m)completed_steps\s*:\s*([^\r\n#]+)`)
		for _, sm := range reStep.FindAllStringSubmatch(m[1], -1) {
			if len(sm) > 1 {
				step := strings.TrimSpace(sm[1])
				if !containsString(shared.PbtxtCompletedSteps, step) {
					shared.PbtxtCompletedSteps = append(shared.PbtxtCompletedSteps, step)
				}
			}
		}
	}

	// 3. seen_nuxs block
	reNuxBlock := regexp.MustCompile(`(?s)seen_nuxs\s*:\s*\{([^}]*)\}`)
	if m := reNuxBlock.FindStringSubmatch(content); len(m) > 1 {
		if shared.PbtxtRawSections == nil {
			shared.PbtxtRawSections = make(map[string]string)
		}
		shared.PbtxtRawSections["seen_nuxs"] = strings.TrimSpace(m[0])
		reUID := regexp.MustCompile(`(?m)uids\s*:\s*([^\r\n#]+)`)
		for _, um := range reUID.FindAllStringSubmatch(m[1], -1) {
			if len(um) > 1 {
				uid := strings.TrimSpace(um[1])
				if !containsString(shared.PbtxtSeenNuxs, uid) {
					shared.PbtxtSeenNuxs = append(shared.PbtxtSeenNuxs, uid)
				}
			}
		}
	}
}

// PreserveAndApplyPbtxtSettings ensures onboarding state, seen NUXs, and completed steps
// are preserved and never dropped from antigravity_state.pbtxt.
func PreserveAndApplyPbtxtSettings(content string, shared *SharedAntigravitySettings) string {
	if shared == nil {
		return content
	}

	result := content

	// 1. Preserve or apply agent_onboarding_completed
	reOnboarding := regexp.MustCompile(`(?m)^\s*agent_onboarding_completed\s*:`)
	if !reOnboarding.MatchString(result) {
		val := shared.PbtxtOnboardingCompleted
		if val == "" {
			val = "AGENT_ONBOARDING_STATE_COMPLETED"
		}
		trimmed := strings.TrimRight(result, "\n")
		if trimmed == "" {
			result = fmt.Sprintf("agent_onboarding_completed: %s\n", val)
		} else {
			result = trimmed + fmt.Sprintf("\nagent_onboarding_completed: %s\n", val)
		}
	}

	// 2. Preserve or apply post_onboarding
	rePost := regexp.MustCompile(`(?m)^\s*post_onboarding\s*:`)
	if !rePost.MatchString(result) {
		if raw, ok := shared.PbtxtRawSections["post_onboarding"]; ok && strings.TrimSpace(raw) != "" {
			trimmed := strings.TrimRight(result, "\n")
			result = trimmed + "\n" + strings.TrimSpace(raw) + "\n"
		} else if len(shared.PbtxtCompletedSteps) > 0 {
			var b strings.Builder
			b.WriteString("post_onboarding: {\n")
			for _, step := range shared.PbtxtCompletedSteps {
				b.WriteString(fmt.Sprintf("  completed_steps: %s\n", step))
			}
			b.WriteString("}\n")
			trimmed := strings.TrimRight(result, "\n")
			result = trimmed + "\n" + b.String()
		}
	}

	// 3. Preserve or apply seen_nuxs
	reNuxs := regexp.MustCompile(`(?m)^\s*seen_nuxs\s*:`)
	if !reNuxs.MatchString(result) {
		if raw, ok := shared.PbtxtRawSections["seen_nuxs"]; ok && strings.TrimSpace(raw) != "" {
			trimmed := strings.TrimRight(result, "\n")
			result = trimmed + "\n" + strings.TrimSpace(raw) + "\n"
		} else if len(shared.PbtxtSeenNuxs) > 0 {
			var b strings.Builder
			b.WriteString("seen_nuxs: {\n")
			for _, uid := range shared.PbtxtSeenNuxs {
				b.WriteString(fmt.Sprintf("  uids: %s\n", uid))
			}
			b.WriteString("}\n")
			trimmed := strings.TrimRight(result, "\n")
			result = trimmed + "\n" + b.String()
		}
	}

	return result
}

// SyncGeminiConfigUserSettings applies shared user settings into ~/.gemini/config/config.json.
func SyncGeminiConfigUserSettings() error {
	shared, err := LoadSharedAntigravitySettings()
	if err != nil || shared == nil || len(shared.UserSettings) == 0 {
		return nil
	}
	return applyGeminiConfigUserSettings(getGeminiConfigPath(), shared.UserSettings)
}

func applyGeminiConfigUserSettings(configPath string, settings map[string]interface{}) error {
	if configPath == "" || !isSafeTestWritePath(configPath) {
		return nil
	}

	var rawMap map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(data, &rawMap); err != nil {
			rawMap = make(map[string]interface{})
		}
	} else if os.IsNotExist(err) {
		rawMap = make(map[string]interface{})
	} else {
		return err
	}

	var userSettings map[string]interface{}
	if existing, ok := rawMap["userSettings"].(map[string]interface{}); ok {
		userSettings = existing
	} else {
		userSettings = make(map[string]interface{})
	}

	userSettings = mergeMaps(userSettings, settings)
	rawMap["userSettings"] = userSettings

	encoded, err := json.MarshalIndent(rawMap, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	return writeAtomic(configPath, encoded, 0600)
}

// syncVscdbPreferences reads existing preferences if shared settings are empty,
// and re-asserts canonical preferences into the ItemTable.
func syncVscdbPreferences(db *sql.DB, execWithRetry func(string, ...interface{}) error) error {
	if db == nil {
		return nil
	}

	shared, _ := LoadSharedAntigravitySettings()
	if shared == nil {
		shared = NewSharedAntigravitySettings()
	}

	// 1. Snapshot any live preferences from ItemTable if not yet in shared settings
	rows, err := db.Query("SELECT key, value FROM ItemTable WHERE key LIKE 'antigravityUnifiedStateSync.%' OR key IN ('colorThemeData', 'iconThemeData', 'productIconThemeData')")
	if err == nil {
		updated := false
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err == nil && v != "" {
				if k == "antigravityUnifiedStateSync.userStatus" || k == "antigravityUnifiedStateSync.oauthToken" {
					continue
				}
				if _, exists := shared.VscdbItems[k]; !exists {
					shared.VscdbItems[k] = v
					updated = true
				}
			}
		}
		rows.Close()
		if updated {
			_ = SaveSharedAntigravitySettings(shared)
		}
	}

	// 2. Re-assert shared unified state sync preferences into ItemTable
	if shared != nil && len(shared.VscdbItems) > 0 {
		for k, v := range shared.VscdbItems {
			if k == "antigravityUnifiedStateSync.userStatus" || k == "antigravityUnifiedStateSync.oauthToken" {
				continue
			}
			if err := execWithRetry("INSERT OR REPLACE INTO ItemTable(key, value) VALUES(?, ?)", k, v); err != nil {
				return err
			}
		}
	}

	return nil
}

// ApplySharedAntigravitySettings restores and propagates all canonical shared settings
// across host surfaces: app_storage.json, config.json, state.vscdb, and antigravity_state.pbtxt.
func ApplySharedAntigravitySettings(shared *SharedAntigravitySettings) error {
	if shared == nil {
		var err error
		shared, err = LoadSharedAntigravitySettings()
		if err != nil || shared == nil {
			return nil
		}
	}

	// 1. Apply to app_storage.json
	applyAppStoragePreferences(shared)

	// 2. Apply to config.json
	if len(shared.UserSettings) > 0 {
		_ = applyGeminiConfigUserSettings(getGeminiConfigPath(), shared.UserSettings)
	}

	// 3. Apply to state.vscdb
	syncSharedVscdbPreferences(shared)

	// 4. Apply to antigravity_state.pbtxt
	applyPbtxtSettings(shared)

	return nil
}

func applyAppStoragePreferences(shared *SharedAntigravitySettings) {
	if shared == nil || len(shared.AppStorage) == 0 {
		return
	}
	storagePath := filepath.Join(core.GetAntigravityHostConfigDir(), "app_storage.json")
	if !isSafeTestWritePath(storagePath) {
		return
	}

	lockFile, err := lockAppStorage(storagePath)
	if err == nil {
		defer unlockAppStorage(lockFile)
	}

	data, err := os.ReadFile(storagePath)
	var rawMap map[string]interface{}
	if err == nil {
		if err := json.Unmarshal(data, &rawMap); err != nil {
			rawMap = make(map[string]interface{})
		}
	} else if os.IsNotExist(err) {
		if _, statErr := os.Stat(filepath.Dir(storagePath)); os.IsNotExist(statErr) {
			return
		}
		rawMap = make(map[string]interface{})
	} else {
		return
	}

	for k, v := range shared.AppStorage {
		rawMap[k] = v
	}

	encoded, err := json.MarshalIndent(rawMap, "", "  ")
	if err == nil {
		_ = writeAtomic(storagePath, encoded, 0644)
	}
}

func syncSharedVscdbPreferences(shared *SharedAntigravitySettings) {
	if shared == nil || len(shared.VscdbItems) == 0 {
		return
	}
	paths := []string{
		filepath.Join(core.GetAntigravityHostConfigDir(), "User", "globalStorage", "state.vscdb"),
		filepath.Join(core.GetVSCodeHostConfigDir(), "User", "globalStorage", "state.vscdb"),
	}
	for _, p := range paths {
		if !isSafeTestWritePath(p) {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		db, err := sql.Open("sqlite", p+"?_busy_timeout=5000")
		if err != nil {
			continue
		}
		var tableCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='ItemTable'").Scan(&tableCount)
		if tableCount > 0 {
			for k, v := range shared.VscdbItems {
				if k == "antigravityUnifiedStateSync.userStatus" || k == "antigravityUnifiedStateSync.oauthToken" {
					continue
				}
				_, _ = db.Exec("INSERT OR REPLACE INTO ItemTable(key, value) VALUES(?, ?)", k, v)
			}
		}
		_ = db.Close()
	}
}

func applyPbtxtSettings(shared *SharedAntigravitySettings) {
	if shared == nil {
		return
	}
	statePath := filepath.Join(core.GetAntigravityDir(), "antigravity_state.pbtxt")
	if !isSafeTestWritePath(statePath) {
		return
	}

	existing, err := os.ReadFile(statePath)
	var content string
	if err == nil {
		content = string(existing)
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
			return
		}
		content = ""
	} else {
		return
	}

	updated := PreserveAndApplyPbtxtSettings(content, shared)
	if updated != "" && updated != content {
		_ = writeAtomic(statePath, []byte(updated), 0600)
	}
}
