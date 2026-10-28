package keyring

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint"
)

func setupTestSettingsEnv(t *testing.T) (agHostConfigDir, geminiAgDir, geminiConfigDir, swissConfigDir string) {
	t.Helper()
	tmpDir := t.TempDir()
	agHostConfigDir = filepath.Join(tmpDir, ".config", "Antigravity")
	geminiAgDir = filepath.Join(tmpDir, ".gemini", "antigravity")
	geminiConfigDir = filepath.Join(tmpDir, ".gemini", "config")
	swissConfigDir = filepath.Join(tmpDir, ".config", "antigravity-swiss")

	if err := os.MkdirAll(agHostConfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(geminiAgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(geminiConfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(swissConfigDir, 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANTIGRAVITY_HOST_CONFIG_DIR", agHostConfigDir)
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", geminiAgDir)
	t.Setenv("GEMINI_CONFIG_DIR", geminiConfigDir)
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", swissConfigDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_HOME", tmpDir)
	return
}

func initTestVscdb(t *testing.T, vscdbPath string, initialKeys map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(vscdbPath), 0755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", vscdbPath)
	if err != nil {
		t.Fatalf("failed to open test vscdb: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS ItemTable (key TEXT PRIMARY KEY, value TEXT)")
	if err != nil {
		t.Fatalf("failed to create ItemTable: %v", err)
	}

	for k, v := range initialKeys {
		_, err = db.Exec("INSERT OR REPLACE INTO ItemTable(key, value) VALUES(?, ?)", k, v)
		if err != nil {
			t.Fatalf("failed to insert initial vscdb key %s: %v", k, err)
		}
	}
}

func TestSettingsSync_SnapshotAndPersistence(t *testing.T) {
	agHostConfigDir, geminiAgDir, geminiConfigDir, swissConfigDir := setupTestSettingsEnv(t)

	// 1. Create app_storage.json with preferences
	storagePath := filepath.Join(agHostConfigDir, "app_storage.json")
	initialStorage := map[string]interface{}{
		"jetski.onboarding.lastLoginUsername": "user1@enterprise.org",
		"keepComputerAwake":                   true,
		"runInBackground":                     true,
		"didAskForNotificationPermission":     true,
		"ide-install-wizard-shown":            true,
	}
	sBytes, _ := json.MarshalIndent(initialStorage, "", "  ")
	if err := os.WriteFile(storagePath, sBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Create config.json with userSettings
	configPath := filepath.Join(geminiConfigDir, "config.json")
	initialConfig := map[string]interface{}{
		"plugins": map[string]interface{}{"kdense-suite": true},
		"userSettings": map[string]interface{}{
			"permissionPreset":         "CUSTOM",
			"browserJsExecutionPolicy": "BROWSER_JS_EXECUTION_POLICY_TURBO",
			"themeMode":                "dark",
			"useAiCredits":             true,
			"verboseAgentChat":         true,
			"autoExecutionPolicy": map[string]interface{}{
				"allowedCommands": []string{"git", "go"},
				"toolPermissions": map[string]interface{}{"terminal": "auto"},
			},
		},
	}
	cBytes, _ := json.MarshalIndent(initialConfig, "", "  ")
	if err := os.WriteFile(configPath, cBytes, 0600); err != nil {
		t.Fatal(err)
	}

	// 3. Create state.vscdb with unified state sync items
	vscdbPath := filepath.Join(agHostConfigDir, "User", "globalStorage", "state.vscdb")
	initTestVscdb(t, vscdbPath, map[string]string{
		"antigravityUnifiedStateSync.userStatus":         "SENTINEL_USER_STATUS_1",
		"antigravityUnifiedStateSync.agentPreferences":   "SENTINEL_AGENT_PREF_123",
		"antigravityUnifiedStateSync.browserPreferences": "SENTINEL_BROWSER_PREF_456",
		"antigravityUnifiedStateSync.modelPreferences":   "SENTINEL_MODEL_PREF_789",
		"antigravityUnifiedStateSync.overrideStore":      "SENTINEL_OVERRIDE_ABC",
		"antigravityUnifiedStateSync.theme":              "SENTINEL_THEME_DEF",
		"colorThemeData":                                 `{"id":"theme-dark"}`,
	})

	// 4. Create antigravity_state.pbtxt with onboarding and NUXs
	pbtxtPath := filepath.Join(geminiAgDir, "antigravity_state.pbtxt")
	initialPbtxt := `agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED
installation_uuid: "uuid-user-1-original"
seen_nuxs: {
  uids: 27
  uids: 26
}
post_onboarding: {
  completed_steps: POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME
  completed_steps: POST_ONBOARDING_STEP_TYPE_USAGE_MODE
}
`
	if err := os.WriteFile(pbtxtPath, []byte(initialPbtxt), 0600); err != nil {
		t.Fatal(err)
	}

	// Run snapshot
	snapshot, err := SnapshotLiveAntigravitySettings()
	if err != nil {
		t.Fatalf("SnapshotLiveAntigravitySettings failed: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}

	// Verify persistence in Swiss Knife config directory
	sharedPath := filepath.Join(swissConfigDir, "shared_antigravity_settings.json")
	if _, err := os.Stat(sharedPath); os.IsNotExist(err) {
		t.Fatalf("canonical shared settings file %s was not created", sharedPath)
	}

	loaded, err := LoadSharedAntigravitySettings()
	if err != nil || loaded == nil {
		t.Fatalf("failed to load shared settings: %v", err)
	}

	// Verify AppStorage preferences
	if loaded.AppStorage["keepComputerAwake"] != true {
		t.Errorf("expected keepComputerAwake=true, got %v", loaded.AppStorage["keepComputerAwake"])
	}
	if loaded.AppStorage["runInBackground"] != true {
		t.Errorf("expected runInBackground=true, got %v", loaded.AppStorage["runInBackground"])
	}

	// Verify config.json userSettings
	if loaded.UserSettings["permissionPreset"] != "CUSTOM" {
		t.Errorf("expected permissionPreset=CUSTOM, got %v", loaded.UserSettings["permissionPreset"])
	}
	if loaded.UserSettings["themeMode"] != "dark" {
		t.Errorf("expected themeMode=dark, got %v", loaded.UserSettings["themeMode"])
	}
	if loaded.UserSettings["browserJsExecutionPolicy"] != "BROWSER_JS_EXECUTION_POLICY_TURBO" {
		t.Errorf("expected browserJsExecutionPolicy=TURBO, got %v", loaded.UserSettings["browserJsExecutionPolicy"])
	}

	// Verify Vscdb preference items (account-specific userStatus must NOT be stored in shared items)
	if loaded.VscdbItems["antigravityUnifiedStateSync.agentPreferences"] != "SENTINEL_AGENT_PREF_123" {
		t.Errorf("agentPreferences mismatch: got %s", loaded.VscdbItems["antigravityUnifiedStateSync.agentPreferences"])
	}
	if loaded.VscdbItems["antigravityUnifiedStateSync.browserPreferences"] != "SENTINEL_BROWSER_PREF_456" {
		t.Errorf("browserPreferences mismatch: got %s", loaded.VscdbItems["antigravityUnifiedStateSync.browserPreferences"])
	}
	if loaded.VscdbItems["antigravityUnifiedStateSync.modelPreferences"] != "SENTINEL_MODEL_PREF_789" {
		t.Errorf("modelPreferences mismatch: got %s", loaded.VscdbItems["antigravityUnifiedStateSync.modelPreferences"])
	}
	if loaded.VscdbItems["antigravityUnifiedStateSync.theme"] != "SENTINEL_THEME_DEF" {
		t.Errorf("theme mismatch: got %s", loaded.VscdbItems["antigravityUnifiedStateSync.theme"])
	}
	if _, hasUserStatus := loaded.VscdbItems["antigravityUnifiedStateSync.userStatus"]; hasUserStatus {
		t.Errorf("userStatus should NOT be stored in shared settings, found: %v", loaded.VscdbItems["antigravityUnifiedStateSync.userStatus"])
	}

	// Verify pbtxt onboarding states
	if loaded.PbtxtOnboardingCompleted != "AGENT_ONBOARDING_STATE_COMPLETED" {
		t.Errorf("expected onboarding completed, got %s", loaded.PbtxtOnboardingCompleted)
	}
	if len(loaded.PbtxtSeenNuxs) < 2 {
		t.Errorf("expected at least 2 seen nuxs, got %v", loaded.PbtxtSeenNuxs)
	}
	if len(loaded.PbtxtCompletedSteps) < 2 {
		t.Errorf("expected at least 2 completed steps, got %v", loaded.PbtxtCompletedSteps)
	}
}

func TestSettingsSync_PropagateSettingsAcrossAccounts(t *testing.T) {
	agHostConfigDir, geminiAgDir, geminiConfigDir, _ := setupTestSettingsEnv(t)

	// Setup Account 1 files
	storagePath := filepath.Join(agHostConfigDir, "app_storage.json")
	acc1Storage := map[string]interface{}{
		"jetski.onboarding.lastLoginUsername": "account1@corp.org",
		"keepComputerAwake":                   true,
		"runInBackground":                     true,
	}
	sBytes, _ := json.MarshalIndent(acc1Storage, "", "  ")
	_ = os.WriteFile(storagePath, sBytes, 0644)

	configPath := filepath.Join(geminiConfigDir, "config.json")
	acc1Config := map[string]interface{}{
		"userSettings": map[string]interface{}{
			"permissionPreset":         "CUSTOM",
			"browserJsExecutionPolicy": "BROWSER_JS_EXECUTION_POLICY_TURBO",
			"themeMode":                "dark",
			"verboseAgentChat":         true,
			"autoExecutionPolicy": map[string]interface{}{
				"allowedCommands": []string{"npm", "docker"},
				"toolPermissions": map[string]interface{}{"browser": "auto"},
			},
		},
	}
	cBytes, _ := json.MarshalIndent(acc1Config, "", "  ")
	_ = os.WriteFile(configPath, cBytes, 0600)

	vscdbPath := filepath.Join(agHostConfigDir, "User", "globalStorage", "state.vscdb")
	initTestVscdb(t, vscdbPath, map[string]string{
		"antigravityUnifiedStateSync.agentPreferences":   "AGENT_PREF_ACC1",
		"antigravityUnifiedStateSync.browserPreferences": "BROWSER_PREF_ACC1",
		"antigravityUnifiedStateSync.modelPreferences":   "MODEL_PREF_ACC1",
		"antigravityUnifiedStateSync.overrideStore":      "OVERRIDE_STORE_ACC1",
		"antigravityUnifiedStateSync.theme":              "THEME_ACC1",
	})

	pbtxtPath := filepath.Join(geminiAgDir, "antigravity_state.pbtxt")
	acc1Pbtxt := `agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED
installation_uuid: "uuid-account1"
seen_nuxs: {
  uids: 38
  uids: 43
}
post_onboarding: {
  completed_steps: POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME
}
`
	_ = os.WriteFile(pbtxtPath, []byte(acc1Pbtxt), 0600)

	// Snapshot live settings
	_, err := SnapshotLiveAntigravitySettings()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}

	// Now switch to Account 2
	acc2 := &Account{
		Email:        "account2@corp.org",
		AccessToken:  "ya29.acc2_fresh_token",
		RefreshToken: "1//refresh_token_acc2",
	}

	profStore, err := fingerprint.NewStore(filepath.Join(t.TempDir(), "device_profiles.json"))
	if err != nil {
		t.Fatal(err)
	}

	if err := SyncAllSurfaces(acc2, []string{"account1@corp.org", acc2.Email}, profStore); err != nil {
		t.Fatalf("SyncAllSurfaces failed: %v", err)
	}

	// 1. Verify app_storage.json has new login username BUT preserved preferences
	data, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatal(err)
	}
	var resStorage map[string]interface{}
	_ = json.Unmarshal(data, &resStorage)
	if resStorage["jetski.onboarding.lastLoginUsername"] != "account2@corp.org" {
		t.Errorf("lastLoginUsername not updated to account2: %v", resStorage["jetski.onboarding.lastLoginUsername"])
	}
	if resStorage["keepComputerAwake"] != true {
		t.Errorf("keepComputerAwake was wiped: %v", resStorage["keepComputerAwake"])
	}
	if resStorage["runInBackground"] != true {
		t.Errorf("runInBackground was wiped: %v", resStorage["runInBackground"])
	}

	// 2. Verify config.json userSettings retained Account 1's customizations
	cData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var resConfig map[string]interface{}
	_ = json.Unmarshal(cData, &resConfig)
	userSettings := resConfig["userSettings"].(map[string]interface{})
	if userSettings["permissionPreset"] != "CUSTOM" {
		t.Errorf("permissionPreset was reset: %v", userSettings["permissionPreset"])
	}
	if userSettings["browserJsExecutionPolicy"] != "BROWSER_JS_EXECUTION_POLICY_TURBO" {
		t.Errorf("browserJsExecutionPolicy was reset: %v", userSettings["browserJsExecutionPolicy"])
	}
	if userSettings["themeMode"] != "dark" {
		t.Errorf("themeMode was reset: %v", userSettings["themeMode"])
	}

	// 3. Verify state.vscdb ItemTable has Account 2's user status AND Account 1's preferences
	db, err := sql.Open("sqlite", vscdbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var agentPref, browserPref, modelPref, overrideStore, themeVal string
	_ = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityUnifiedStateSync.agentPreferences'").Scan(&agentPref)
	_ = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityUnifiedStateSync.browserPreferences'").Scan(&browserPref)
	_ = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityUnifiedStateSync.modelPreferences'").Scan(&modelPref)
	_ = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityUnifiedStateSync.overrideStore'").Scan(&overrideStore)
	_ = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityUnifiedStateSync.theme'").Scan(&themeVal)

	if agentPref != "AGENT_PREF_ACC1" {
		t.Errorf("agentPreferences not propagated: got %q", agentPref)
	}
	if browserPref != "BROWSER_PREF_ACC1" {
		t.Errorf("browserPreferences not propagated: got %q", browserPref)
	}
	if modelPref != "MODEL_PREF_ACC1" {
		t.Errorf("modelPreferences not propagated: got %q", modelPref)
	}
	if overrideStore != "OVERRIDE_STORE_ACC1" {
		t.Errorf("overrideStore not propagated: got %q", overrideStore)
	}
	if themeVal != "THEME_ACC1" {
		t.Errorf("theme not propagated: got %q", themeVal)
	}

	// 4. Verify antigravity_state.pbtxt retained onboarding status and seen NUXs
	pData, err := os.ReadFile(pbtxtPath)
	if err != nil {
		t.Fatal(err)
	}
	pContent := string(pData)
	if !strings.Contains(pContent, "agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED") {
		t.Errorf("onboarding status was lost from pbtxt:\n%s", pContent)
	}
	if !strings.Contains(pContent, "uids: 38") || !strings.Contains(pContent, "uids: 43") {
		t.Errorf("seen NUXs were lost from pbtxt:\n%s", pContent)
	}
	if !strings.Contains(pContent, "POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME") {
		t.Errorf("post onboarding step was lost from pbtxt:\n%s", pContent)
	}
}

func TestSettingsSync_FreshEmptyProfileInheritsSettings(t *testing.T) {
	agHostConfigDir, geminiAgDir, geminiConfigDir, _ := setupTestSettingsEnv(t)

	// 1. Seed canonical shared settings in Swiss Knife
	shared := NewSharedAntigravitySettings()
	shared.AppStorage["keepComputerAwake"] = true
	shared.AppStorage["runInBackground"] = true
	shared.UserSettings["permissionPreset"] = "STRICT"
	shared.UserSettings["themeMode"] = "light"
	shared.UserSettings["useAiCredits"] = false
	shared.VscdbItems["antigravityUnifiedStateSync.agentPreferences"] = "AGENT_PREF_SHARED"
	shared.VscdbItems["antigravityUnifiedStateSync.browserPreferences"] = "BROWSER_PREF_SHARED"
	shared.VscdbItems["antigravityUnifiedStateSync.modelPreferences"] = "MODEL_PREF_SHARED"
	shared.VscdbItems["antigravityUnifiedStateSync.overrideStore"] = "OVERRIDE_SHARED"
	shared.VscdbItems["antigravityUnifiedStateSync.theme"] = "THEME_SHARED"
	shared.PbtxtOnboardingCompleted = "AGENT_ONBOARDING_STATE_COMPLETED"
	shared.PbtxtSeenNuxs = []string{"24", "26", "27"}
	shared.PbtxtCompletedSteps = []string{"POST_ONBOARDING_STEP_TYPE_USAGE_MODE"}
	if err := SaveSharedAntigravitySettings(shared); err != nil {
		t.Fatal(err)
	}

	// 2. Fresh profile with no existing config.json, no app_storage.json, and fresh state.vscdb
	vscdbPath := filepath.Join(agHostConfigDir, "User", "globalStorage", "state.vscdb")
	initTestVscdb(t, vscdbPath, nil) // empty ItemTable

	accFresh := &Account{
		Email:        "fresh_user@corp.org",
		AccessToken:  "ya29.fresh_token_xyz",
		RefreshToken: "1//refresh_fresh",
	}

	prof, err := fingerprint.GenerateRandom()
	if err != nil {
		t.Fatal(err)
	}

	// 1. Sync hardware profile to empty dirs
	if err := SyncHardwareProfileToDirs(prof, agHostConfigDir, geminiAgDir); err != nil {
		t.Fatalf("SyncHardwareProfileToDirs failed: %v", err)
	}

	// 2. Apply shared settings to ensure empty profile inherits all canonical settings
	if err := ApplySharedAntigravitySettings(shared); err != nil {
		t.Fatalf("ApplySharedAntigravitySettings failed: %v", err)
	}

	// 3. Sync app storage login user for the fresh account
	if err := SyncAppStorageLoginUser(accFresh.Email); err != nil {
		t.Fatalf("SyncAppStorageLoginUser failed: %v", err)
	}

	// 4. Sync state.vscdb
	if err := SyncStateVscdb(accFresh); err != nil {
		t.Fatalf("SyncStateVscdb failed: %v", err)
	}

	// 5. Sync gemini config.json
	if err := SyncGeminiConfigUserSettings(); err != nil {
		t.Fatalf("SyncGeminiConfigUserSettings failed: %v", err)
	}

	// Assertions:
	// 1. app_storage.json inherits keepComputerAwake and runInBackground
	appStorageData, err := os.ReadFile(filepath.Join(agHostConfigDir, "app_storage.json"))
	if err != nil {
		t.Fatalf("failed to read app_storage.json: %v", err)
	}
	var appStorageMap map[string]interface{}
	_ = json.Unmarshal(appStorageData, &appStorageMap)
	if appStorageMap["jetski.onboarding.lastLoginUsername"] != accFresh.Email {
		t.Errorf("lastLoginUsername mismatch: got %v", appStorageMap["jetski.onboarding.lastLoginUsername"])
	}
	if appStorageMap["keepComputerAwake"] != true {
		t.Errorf("fresh profile did not inherit keepComputerAwake=true")
	}
	if appStorageMap["runInBackground"] != true {
		t.Errorf("fresh profile did not inherit runInBackground=true")
	}

	// 2. config.json was created and inherited userSettings
	configData, err := os.ReadFile(filepath.Join(geminiConfigDir, "config.json"))
	if err != nil {
		t.Fatalf("config.json was not created for fresh profile: %v", err)
	}
	var configMap map[string]interface{}
	_ = json.Unmarshal(configData, &configMap)
	uSettings := configMap["userSettings"].(map[string]interface{})
	if uSettings["permissionPreset"] != "STRICT" {
		t.Errorf("permissionPreset mismatch: got %v", uSettings["permissionPreset"])
	}
	if uSettings["themeMode"] != "light" {
		t.Errorf("themeMode mismatch: got %v", uSettings["themeMode"])
	}

	// 3. state.vscdb ItemTable was populated with shared preferences
	db, err := sql.Open("sqlite", vscdbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var agentPref, modelPref string
	_ = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityUnifiedStateSync.agentPreferences'").Scan(&agentPref)
	_ = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityUnifiedStateSync.modelPreferences'").Scan(&modelPref)
	if agentPref != "AGENT_PREF_SHARED" {
		t.Errorf("fresh vscdb did not inherit agentPreferences: got %q", agentPref)
	}
	if modelPref != "MODEL_PREF_SHARED" {
		t.Errorf("fresh vscdb did not inherit modelPreferences: got %q", modelPref)
	}

	// 4. antigravity_state.pbtxt has onboarding completed, seen NUXs, and completed steps
	pbtxtData, err := os.ReadFile(filepath.Join(geminiAgDir, "antigravity_state.pbtxt"))
	if err != nil {
		t.Fatalf("failed to read antigravity_state.pbtxt: %v", err)
	}
	pbtxtContent := string(pbtxtData)
	if !strings.Contains(pbtxtContent, "agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED") {
		t.Errorf("fresh pbtxt missing agent_onboarding_completed:\n%s", pbtxtContent)
	}
	if !strings.Contains(pbtxtContent, "uids: 24") || !strings.Contains(pbtxtContent, "uids: 26") {
		t.Errorf("fresh pbtxt missing seen NUXs:\n%s", pbtxtContent)
	}
	if !strings.Contains(pbtxtContent, "POST_ONBOARDING_STEP_TYPE_USAGE_MODE") {
		t.Errorf("fresh pbtxt missing completed step:\n%s", pbtxtContent)
	}
	if !strings.Contains(pbtxtContent, prof.InstallationUUID) {
		t.Errorf("fresh pbtxt missing installation_uuid %s:\n%s", prof.InstallationUUID, pbtxtContent)
	}
}

func TestSettingsSync_OnboardingStatusPreservedAcrossMultipleSwitches(t *testing.T) {
	agHostConfigDir, geminiAgDir, _, _ := setupTestSettingsEnv(t)

	pbtxtPath := filepath.Join(geminiAgDir, "antigravity_state.pbtxt")
	initialContent := `agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED
installation_uuid: "uuid-original"
seen_nuxs: {
  uids: 23
  uids: 29
  uids: 31
}
post_onboarding: {
  completed_steps: POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME
  completed_steps: POST_ONBOARDING_STEP_TYPE_USAGE_MODE
  completed_steps: POST_ONBOARDING_STEP_TYPE_AGENT_CONFIGURATION
  completed_steps: POST_ONBOARDING_STEP_TYPE_ADD_WORKSPACE
}
`
	if err := os.WriteFile(pbtxtPath, []byte(initialContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Snapshot to record initial onboarding state
	_, _ = SnapshotLiveAntigravitySettings()

	// Switch 1: Update to Profile A
	profA, err := fingerprint.GenerateRandom()
	if err != nil {
		t.Fatal(err)
	}
	if err := SyncHardwareProfileToDirs(profA, agHostConfigDir, geminiAgDir); err != nil {
		t.Fatalf("Switch 1 failed: %v", err)
	}

	contentA, _ := os.ReadFile(pbtxtPath)
	if !strings.Contains(string(contentA), "agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED") {
		t.Fatalf("Switch 1 dropped onboarding completed:\n%s", string(contentA))
	}
	if !strings.Contains(string(contentA), profA.InstallationUUID) {
		t.Fatalf("Switch 1 did not write UUID A:\n%s", string(contentA))
	}
	if !strings.Contains(string(contentA), "POST_ONBOARDING_STEP_TYPE_ADD_WORKSPACE") {
		t.Fatalf("Switch 1 dropped completed steps:\n%s", string(contentA))
	}

	// Switch 2: Update to Profile B
	profB, err := fingerprint.GenerateRandom()
	if err != nil {
		t.Fatal(err)
	}
	if err := SyncHardwareProfileToDirs(profB, agHostConfigDir, geminiAgDir); err != nil {
		t.Fatalf("Switch 2 failed: %v", err)
	}

	contentB, _ := os.ReadFile(pbtxtPath)
	if !strings.Contains(string(contentB), "agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED") {
		t.Fatalf("Switch 2 dropped onboarding completed:\n%s", string(contentB))
	}
	if !strings.Contains(string(contentB), profB.InstallationUUID) {
		t.Fatalf("Switch 2 did not write UUID B:\n%s", string(contentB))
	}
	if strings.Contains(string(contentB), profA.InstallationUUID) {
		t.Fatalf("Switch 2 still contains UUID A:\n%s", string(contentB))
	}
	if !strings.Contains(string(contentB), "uids: 31") {
		t.Fatalf("Switch 2 dropped seen NUXs:\n%s", string(contentB))
	}
}

func TestSettingsSync_ExternalWipeRestoration(t *testing.T) {
	agHostConfigDir, geminiAgDir, geminiConfigDir, _ := setupTestSettingsEnv(t)

	// Save canonical settings in Swiss Knife
	shared := NewSharedAntigravitySettings()
	shared.AppStorage["keepComputerAwake"] = true
	shared.AppStorage["runInBackground"] = true
	shared.UserSettings["themeMode"] = "dark"
	shared.UserSettings["permissionPreset"] = "CUSTOM"
	shared.VscdbItems["antigravityUnifiedStateSync.theme"] = "THEME_RESTORED"
	shared.PbtxtOnboardingCompleted = "AGENT_ONBOARDING_STATE_COMPLETED"
	if err := SaveSharedAntigravitySettings(shared); err != nil {
		t.Fatal(err)
	}

	// Prepare wiped state on disk
	storagePath := filepath.Join(agHostConfigDir, "app_storage.json")
	_ = os.WriteFile(storagePath, []byte(`{"jetski.onboarding.lastLoginUsername":"user@corp.org"}`), 0644)

	configPath := filepath.Join(geminiConfigDir, "config.json")
	_ = os.Remove(configPath)

	vscdbPath := filepath.Join(agHostConfigDir, "User", "globalStorage", "state.vscdb")
	initTestVscdb(t, vscdbPath, nil)

	pbtxtPath := filepath.Join(geminiAgDir, "antigravity_state.pbtxt")
	_ = os.WriteFile(pbtxtPath, []byte(`installation_uuid: "uuid-alone"`), 0600)

	// Trigger restore
	if err := ApplySharedAntigravitySettings(nil); err != nil {
		t.Fatalf("ApplySharedAntigravitySettings failed: %v", err)
	}

	// Verify app_storage.json restored
	sData, _ := os.ReadFile(storagePath)
	var sMap map[string]interface{}
	_ = json.Unmarshal(sData, &sMap)
	if sMap["keepComputerAwake"] != true || sMap["runInBackground"] != true {
		t.Errorf("app_storage preferences not restored: %v", sMap)
	}

	// Verify config.json restored
	cData, _ := os.ReadFile(configPath)
	var cMap map[string]interface{}
	_ = json.Unmarshal(cData, &cMap)
	uMap := cMap["userSettings"].(map[string]interface{})
	if uMap["themeMode"] != "dark" || uMap["permissionPreset"] != "CUSTOM" {
		t.Errorf("config.json userSettings not restored: %v", uMap)
	}

	// Verify state.vscdb restored
	db, err := sql.Open("sqlite", vscdbPath)
	if err != nil {
		t.Fatal(err)
	}
	var themeVal string
	_ = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityUnifiedStateSync.theme'").Scan(&themeVal)
	_ = db.Close()
	if themeVal != "THEME_RESTORED" {
		t.Errorf("vscdb theme not restored: got %s", themeVal)
	}

	// Verify pbtxt restored
	pData, _ := os.ReadFile(pbtxtPath)
	if !strings.Contains(string(pData), "agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED") {
		t.Errorf("pbtxt onboarding not restored:\n%s", string(pData))
	}
}

func TestSettingsSync_DeepMergeUserSettings(t *testing.T) {
	_, _, geminiConfigDir, _ := setupTestSettingsEnv(t)
	configPath := filepath.Join(geminiConfigDir, "config.json")

	initial := map[string]interface{}{
		"plugins": map[string]interface{}{
			"customPlugin": true,
		},
		"userSettings": map[string]interface{}{
			"existingKey": "keepValue",
			"autoExecutionPolicy": map[string]interface{}{
				"allowedCommands": []string{"cmd1"},
				"preserveSubKey":  "subValue",
			},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	_ = os.WriteFile(configPath, data, 0600)

	incoming := map[string]interface{}{
		"themeMode": "dark",
		"autoExecutionPolicy": map[string]interface{}{
			"allowedCommands": []string{"cmd1", "cmd2"},
			"newSubKey":       "newVal",
		},
	}

	if err := applyGeminiConfigUserSettings(configPath, incoming); err != nil {
		t.Fatalf("applyGeminiConfigUserSettings failed: %v", err)
	}

	resData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var res map[string]interface{}
	_ = json.Unmarshal(resData, &res)

	// Top level plugins preserved
	if plugins, ok := res["plugins"].(map[string]interface{}); !ok || plugins["customPlugin"] != true {
		t.Errorf("plugins was clobbered: %v", res["plugins"])
	}

	uSettings := res["userSettings"].(map[string]interface{})
	if uSettings["existingKey"] != "keepValue" {
		t.Errorf("existingKey was clobbered: %v", uSettings["existingKey"])
	}
	if uSettings["themeMode"] != "dark" {
		t.Errorf("themeMode was not applied: %v", uSettings["themeMode"])
	}

	policy := uSettings["autoExecutionPolicy"].(map[string]interface{})
	if policy["preserveSubKey"] != "subValue" {
		t.Errorf("preserveSubKey was clobbered: %v", policy["preserveSubKey"])
	}
	if policy["newSubKey"] != "newVal" {
		t.Errorf("newSubKey was not applied: %v", policy["newSubKey"])
	}
}

func TestSettingsSync_RollbackOnSyncFailure(t *testing.T) {
	agHostConfigDir, _, geminiConfigDir, _ := setupTestSettingsEnv(t)

	// Setup initial Account 1 files
	storagePath := filepath.Join(agHostConfigDir, "app_storage.json")
	initialStorage := `{"jetski.onboarding.lastLoginUsername":"original@corp.org","keepComputerAwake":false}`
	_ = os.WriteFile(storagePath, []byte(initialStorage), 0644)

	configPath := filepath.Join(geminiConfigDir, "config.json")
	initialConfig := `{"userSettings":{"themeMode":"light"}}`
	_ = os.WriteFile(configPath, []byte(initialConfig), 0600)

	vscdbPath := filepath.Join(agHostConfigDir, "User", "globalStorage", "state.vscdb")
	initTestVscdb(t, vscdbPath, map[string]string{
		"antigravityUnifiedStateSync.userStatus": "ORIGINAL_STATUS",
		"antigravityUnifiedStateSync.theme":      "THEME_LIGHT",
	})

	// Inject error on cli oauth token step (after app_storage, hardware profile, state.vscdb)
	syncStepHookForTest = func(step string) error {
		if step == "cli oauth token" {
			return os.ErrPermission
		}
		return nil
	}
	defer func() { syncStepHookForTest = nil }()

	targetAcc := &Account{
		Email:        "switched@corp.org",
		AccessToken:  "ya29.target_token",
		RefreshToken: "1//target_refresh",
	}

	err := SyncAllSurfaces(targetAcc, []string{"original@corp.org", targetAcc.Email}, nil)
	if err == nil {
		t.Fatal("expected SyncAllSurfaces to fail on injected step")
	}

	// Verify rollback restored app_storage.json
	restoredStorage, _ := os.ReadFile(storagePath)
	if string(restoredStorage) != initialStorage {
		t.Errorf("app_storage not rolled back: got %s, want %s", string(restoredStorage), initialStorage)
	}

	// Verify rollback restored state.vscdb
	db, err := sql.Open("sqlite", vscdbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var restoredStatus, restoredTheme string
	_ = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityUnifiedStateSync.userStatus'").Scan(&restoredStatus)
	_ = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityUnifiedStateSync.theme'").Scan(&restoredTheme)
	if restoredStatus != "ORIGINAL_STATUS" {
		t.Errorf("userStatus not rolled back: got %s", restoredStatus)
	}
	if restoredTheme != "THEME_LIGHT" {
		t.Errorf("theme not rolled back: got %s", restoredTheme)
	}
}
