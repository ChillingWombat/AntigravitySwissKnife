package gui

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	_ "modernc.org/sqlite"
)

// Store manages GUI improvement configuration and project discovery.
type Store struct {
	mu             sync.RWMutex
	configPath     string
	config         *Config
	injector       *Injector
	desktopManager *DesktopManager
}

// NewStore initializes or loads the GUI configuration from the specified directory.
func NewStore(configDir string) (*Store, error) {
	if configDir == "" {
		configDir = core.GetConfigDir()
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "gui_improvements.json")
	inj := NewInjector(0)
	s := &Store{
		configPath:     configPath,
		config:         DefaultConfig(),
		injector:       inj,
		desktopManager: NewDesktopManager("", configPath, inj),
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	_ = s.SyncPersistentFiles()
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err == nil {
		if _, ok := raw["active_conversation_bold"]; !ok {
			cfg.ActiveConversationBold = false
		}
		if _, ok := raw["auto_archive_conversations"]; !ok {
			cfg.AutoArchiveConversations = true
		}
		if _, ok := raw["replace_see_all_triangle"]; !ok {
			cfg.ReplaceSeeAllTriangle = true
		}
		if _, ok := raw["consistent_project_spacing"]; !ok {
			cfg.ConsistentProjectSpacing = true
		}
		if _, ok := raw["consistent_project_spacing_line"]; !ok {
			cfg.ConsistentProjectSpacingLine = true
		}
	}
	if cfg.ActiveConversationIndicator == "" {
		cfg.ActiveConversationIndicator = "background"
	}
	if cfg.ProjectColors == nil {
		cfg.ProjectColors = make(map[string]string)
	}
	if cfg.TintOpacity <= 0 {
		cfg.TintOpacity = 0.15
	}
	if cfg.ArchivedProjects == nil {
		cfg.ArchivedProjects = []string{}
	}
	if cfg.ConversationTabsMode == "" {
		cfg.ConversationTabsMode = "dynamic"
	}
	if cfg.ConversationTabsFixedLimit <= 0 || cfg.ConversationTabsFixedLimit > 10 {
		cfg.ConversationTabsFixedLimit = 6
	}
	if cfg.ConversationTabsAgeThreshold == "" {
		cfg.ConversationTabsAgeThreshold = "14d"
	}
	if cfg.ConversationTabsMin <= 0 {
		cfg.ConversationTabsMin = 3
	}
	if cfg.ConversationTabsMax <= 0 {
		cfg.ConversationTabsMax = 6
	}
	if cfg.AutoArchiveHorizon == "" {
		cfg.AutoArchiveHorizon = "14d"
	}
	s.config = &cfg
	return nil
}

// Save writes the current configuration to disk and updates persistent styles and scripts.
func (s *Store) Save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		s.mu.RUnlock()
		return err
	}
	cfgCopy := *s.config
	s.mu.RUnlock()

	if err := os.WriteFile(s.configPath, data, 0600); err != nil {
		return err
	}

	_ = s.writePersistentFiles(&cfgCopy)
	return nil
}

func (s *Store) writePersistentFiles(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	configDir := filepath.Dir(s.configPath)
	if configDir == "" {
		configDir = core.GetConfigDir()
	}
	_ = os.MkdirAll(configDir, 0755)

	cssPath := filepath.Join(configDir, "persistent_styles.css")
	jsPath := filepath.Join(configDir, "persistent_script.js")

	if rawCore, err := os.ReadFile(filepath.Join(configDir, "config.json")); err == nil {
		var parsed map[string]interface{}
		if json.Unmarshal(rawCore, &parsed) == nil {
			if pve, ok := parsed["persistent_visual_effects"].(bool); ok && !pve {
				_ = os.Remove(cssPath)
				_ = os.Remove(jsPath)
				return nil
			}
		}
	}

	css := GenerateCSS(cfg)
	script := GenerateScriptWithCustomModels(cfg, nil)

	if err := os.WriteFile(cssPath, []byte(css), 0644); err != nil {
		return err
	}
	return os.WriteFile(jsPath, []byte(script), 0644)
}

// SyncPersistentFiles writes persistent_styles.css and persistent_script.js to disk.
func (s *Store) SyncPersistentFiles() error {
	cfg := s.GetConfig()
	return s.writePersistentFiles(&cfg)
}

// GetConfig returns a copy of the current configuration.
func (s *Store) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	colorsCopy := make(map[string]string)
	for k, v := range s.config.ProjectColors {
		colorsCopy[k] = v
	}

	orderCopy := make([]string, len(s.config.ProjectOrder))
	copy(orderCopy, s.config.ProjectOrder)

	archivedCopy := make([]string, len(s.config.ArchivedProjects))
	copy(archivedCopy, s.config.ArchivedProjects)

	return Config{
		Enabled:                     s.config.Enabled,
		ColorStylingEnabled:         s.config.ColorStylingEnabled,
		SolidLeftEdge:               s.config.SolidLeftEdge,
		DragRearrangeEnabled:        s.config.DragRearrangeEnabled,
		ActiveConversationIndicator:   s.config.ActiveConversationIndicator,
		ActiveConversationBorderWidth: s.config.ActiveConversationBorderWidth,
		ActiveConversationBold:        s.config.ActiveConversationBold,
		ProjectColors:               colorsCopy,
		ProjectOrder:                orderCopy,
		ArchivedProjects:            archivedCopy,
		TintOpacity:                 s.config.TintOpacity,
		ConversationTabsMode:        s.config.ConversationTabsMode,
		ConversationTabsFixedLimit:  s.config.ConversationTabsFixedLimit,
		ConversationTabsAgeThreshold: s.config.ConversationTabsAgeThreshold,
		ConversationTabsMin:         s.config.ConversationTabsMin,
		ConversationTabsMax:         s.config.ConversationTabsMax,
		ReplaceSeeAllTriangle:       s.config.ReplaceSeeAllTriangle,
		ConsistentProjectSpacing:    s.config.ConsistentProjectSpacing,
		ConsistentProjectSpacingLine: s.config.ConsistentProjectSpacingLine,
		AutoArchiveConversations:    s.config.AutoArchiveConversations,
		AutoArchiveHorizon:          s.config.AutoArchiveHorizon,
		AutoInject:                  s.config.AutoInject,
	}
}

// UpdateConfig updates the full configuration and saves it.
func (s *Store) UpdateConfig(cfg *Config) error {
	s.mu.Lock()
	if cfg.ActiveConversationIndicator == "" {
		cfg.ActiveConversationIndicator = "background"
	}
	if cfg.ProjectColors == nil {
		cfg.ProjectColors = make(map[string]string)
	}
	if cfg.ArchivedProjects == nil {
		cfg.ArchivedProjects = []string{}
	}
	if cfg.TintOpacity <= 0 {
		cfg.TintOpacity = 0.15
	}
	if cfg.ConversationTabsMode == "" {
		cfg.ConversationTabsMode = "dynamic"
	}
	if cfg.ConversationTabsFixedLimit <= 0 || cfg.ConversationTabsFixedLimit > 10 {
		cfg.ConversationTabsFixedLimit = 6
	}
	if cfg.ConversationTabsAgeThreshold == "" {
		cfg.ConversationTabsAgeThreshold = "14d"
	}
	if cfg.ConversationTabsMin <= 0 {
		cfg.ConversationTabsMin = 3
	}
	if cfg.ConversationTabsMax <= 0 {
		cfg.ConversationTabsMax = 6
	}
	if cfg.AutoArchiveHorizon == "" {
		cfg.AutoArchiveHorizon = "14d"
	}
	s.config = cfg
	s.mu.Unlock()

	if err := s.Save(); err != nil {
		return err
	}

	if cfg.AutoInject {
		_, _ = s.Apply()
	}

	return nil
}

// SetProjectColor sets or updates the color for a specific project.
func (s *Store) SetProjectColor(projectName, hexColor string) error {
	projectName = strings.TrimSpace(projectName)
	hexColor = strings.TrimSpace(hexColor)
	if projectName == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if !strings.HasPrefix(hexColor, "#") {
		hexColor = "#" + hexColor
	}

	s.mu.Lock()
	if s.config.ProjectColors == nil {
		s.config.ProjectColors = make(map[string]string)
	}
	s.config.ProjectColors[projectName] = hexColor
	s.mu.Unlock()

	if err := s.Save(); err != nil {
		return err
	}

	if s.config.AutoInject {
		_, _ = s.Apply()
	}

	return nil
}

// RemoveProjectColor resets the project color to factory default if it is a pre-configured project,
// or removes the custom color if it was user-defined.
func (s *Store) RemoveProjectColor(projectName string) error {
	s.mu.Lock()
	if s.config.ProjectColors == nil {
		s.config.ProjectColors = make(map[string]string)
	}
	factoryColors := FactoryProjectColors()
	if defaultColor, ok := factoryColors[projectName]; ok {
		s.config.ProjectColors[projectName] = defaultColor
	} else {
		delete(s.config.ProjectColors, projectName)
	}
	s.mu.Unlock()

	if err := s.Save(); err != nil {
		return err
	}

	if s.config.AutoInject {
		_, _ = s.Apply()
	}

	return nil
}

// ResetProjectColor resets the project color to factory default if it is a pre-configured project,
// or removes the custom color if it was user-defined.
func (s *Store) ResetProjectColor(projectName string) error {
	return s.RemoveProjectColor(projectName)
}

// UpdateProjectOrder sets the custom order of projects.
func (s *Store) UpdateProjectOrder(order []string) error {
	s.mu.Lock()
	s.config.ProjectOrder = order
	s.mu.Unlock()

	if err := s.Save(); err != nil {
		return err
	}

	if s.config.AutoInject {
		_, _ = s.Apply()
	}

	return nil
}

// ReorderProject moves sourceProject before or after targetProject.
func (s *Store) ReorderProject(source, target string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	order := s.config.ProjectOrder
	var srcIdx, tgtIdx = -1, -1
	for i, name := range order {
		if name == source {
			srcIdx = i
		}
		if name == target {
			tgtIdx = i
		}
	}

	// If source is not in the order list, add it
	if srcIdx == -1 {
		order = append([]string{source}, order...)
		srcIdx = 0
		if tgtIdx != -1 {
			tgtIdx++
		}
	}
	// If target is not in order list, append target
	if tgtIdx == -1 {
		order = append(order, target)
		tgtIdx = len(order) - 1
	}

	// Move source to target's position
	item := order[srcIdx]
	order = append(order[:srcIdx], order[srcIdx+1:]...)
	if srcIdx < tgtIdx {
		tgtIdx--
	}
	newOrder := make([]string, 0, len(order)+1)
	newOrder = append(newOrder, order[:tgtIdx]...)
	newOrder = append(newOrder, item)
	newOrder = append(newOrder, order[tgtIdx:]...)

	s.config.ProjectOrder = newOrder

	data, err := json.MarshalIndent(s.config, "", "  ")
	if err == nil {
		_ = os.WriteFile(s.configPath, data, 0600)
	}

	if s.config.AutoInject {
		colorsCopy := make(map[string]string, len(s.config.ProjectColors))
		for k, v := range s.config.ProjectColors {
			colorsCopy[k] = v
		}
		orderCopy := make([]string, len(s.config.ProjectOrder))
		copy(orderCopy, s.config.ProjectOrder)
		archivedCopy := make([]string, len(s.config.ArchivedProjects))
		copy(archivedCopy, s.config.ArchivedProjects)
		cfg := *s.config
		cfg.ProjectColors = colorsCopy
		cfg.ProjectOrder = orderCopy
		cfg.ArchivedProjects = archivedCopy
		go func() {
			_, _ = s.injector.ApplyConfig(&cfg)
		}()
	}

	return nil
}

// DetectProjects discovers all available projects across Antigravity and combines with configured colors & order.
func (s *Store) DetectProjects() ([]ProjectItem, error) {
	detectedSet := make(map[string]string) // name -> source

	// 1. Try detecting live projects from running Antigravity via CDP
	if port, err := s.injector.FindDevToolsPort(); err == nil {
		if pages, err := s.injector.GetPageTargets(port); err == nil && len(pages) > 0 {
			queryScript := `(() => {
				const names = new Set();
				const container = document.querySelector(".w-full.relative > [data-index]")?.parentElement;
				if (container) {
					const key = Object.keys(container).find(k => k.startsWith("__reactFiber"));
					let fiber = container[key];
					while (fiber) {
						if (fiber.memoizedProps?.items) {
							fiber.memoizedProps.items.forEach(it => {
								if (it.type === "header" && it.label) {
									const l = it.label.trim();
									if (l && l !== "Today" && l !== "Yesterday" && l !== "Previous 7 Days" && l !== "Previous 30 Days") {
										names.add(l);
									}
								}
							});
							break;
						}
						fiber = fiber.return;
					}
				}
				return Array.from(names);
			})()`

			res, err := s.injector.ExecuteScript(pages[0].WebSocketDebuggerURL, queryScript)
			if err == nil && res != nil {
				if valList, ok := res["value"].([]interface{}); ok {
					for _, v := range valList {
						if str, ok := v.(string); ok && strings.TrimSpace(str) != "" {
							detectedSet[strings.TrimSpace(str)] = "live_antigravity"
						}
					}
				}
			}
		}
	}

	// 2. Discover from SQLite history ONLY if live Antigravity is not running or returned no projects
	if len(detectedSet) == 0 {
		home, err := os.UserHomeDir()
		if err == nil {
			dbPath := filepath.Join(home, ".gemini", "antigravity", "conversation_summaries.db")
			if _, statErr := os.Stat(dbPath); statErr == nil {
				if sdb, errOpen := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath)); errOpen == nil {
					// 2a. Query conversations_fts table which stores canonical Antigravity project names
					rowsFTS, errFTS := sdb.Query("SELECT DISTINCT project FROM conversations_fts WHERE project IS NOT NULL AND project != ''")
					if errFTS == nil {
						for rowsFTS.Next() {
							var pName string
							if errScan := rowsFTS.Scan(&pName); errScan == nil && pName != "" {
								pName = strings.TrimSpace(pName)
								if pName != "" {
									detectedSet[pName] = "sqlite_history"
								}
							}
						}
						rowsFTS.Close()
					}

					// 2b. Only fallback to workspace URIs if no projects were found in FTS
					if len(detectedSet) == 0 {
						rows, errQuery := sdb.Query("SELECT DISTINCT workspace_uris FROM conversation_summaries WHERE workspace_uris IS NOT NULL AND workspace_uris != ''")
						if errQuery == nil {
							for rows.Next() {
								var raw string
								if errScan := rows.Scan(&raw); errScan == nil && raw != "" {
									var uris []string
									if errJSON := json.Unmarshal([]byte(raw), &uris); errJSON == nil {
										for _, uStr := range uris {
											u, errParse := url.Parse(uStr)
											if errParse == nil && u.Path != "" {
												unescapedPath, unescErr := url.PathUnescape(u.Path)
												if unescErr == nil {
													cleanPath := filepath.Clean(unescapedPath)
													base := filepath.Base(cleanPath)
													lower := strings.ToLower(base)
													if lower == "projects" || lower == "documents" || lower == "desktop" || lower == "home" || lower == "workspace" {
														continue
													}
													if base != "" && base != "." && base != "/" && !strings.Contains(base, "gemini") {
														detectedSet[base] = "sqlite_history"
													}
												}
											}
										}
									}
								}
							}
							rows.Close()
						}
					}
					sdb.Close()
				}
			}
		}
	}

	// 3. Merge with user-configured project colors & order
	s.mu.RLock()
	configured := make(map[string]string)
	for k, v := range s.config.ProjectColors {
		configured[k] = v
	}
	orderMap := make(map[string]int)
	for i, name := range s.config.ProjectOrder {
		orderMap[name] = i
	}
	s.mu.RUnlock()

	for k := range configured {
		if _, exists := detectedSet[k]; !exists {
			detectedSet[k] = "configured"
		}
	}

	var result []ProjectItem
	for name, src := range detectedSet {
		color := configured[name]
		orderIdx, hasOrder := orderMap[name]
		if !hasOrder {
			orderIdx = 9999
		}
		result = append(result, ProjectItem{
			Name:         name,
			Color:        color,
			IsConfigured: color != "",
			Source:       src,
			OrderIndex:   orderIdx,
		})
	}

	// Sort according to custom project order, then alphabetically
	sort.Slice(result, func(i, j int) bool {
		if result[i].OrderIndex != result[j].OrderIndex {
			return result[i].OrderIndex < result[j].OrderIndex
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

// Apply sends the current configuration into running Antigravity instances.
func (s *Store) Apply() (*ApplyResult, error) {
	_ = s.SyncPersistentFiles()
	cfg := s.GetConfig()
	return s.injector.ApplyConfig(&cfg)
}

// DeactivateOnDaemonStop removes injected extensions from live Antigravity windows via CDP,
// and if keepVisualEffects is false, also removes all live visual styling.
func (s *Store) DeactivateOnDaemonStop(keepVisualEffects bool) error {
	if s.injector == nil {
		return nil
	}
	port, err := s.injector.FindDevToolsPort()
	if err != nil {
		return nil // Antigravity is not running
	}

	pages, err := s.injector.GetPageTargets(port)
	if err != nil || len(pages) == 0 {
		return nil
	}

	deactScript := `(() => {
		try {
			if (typeof window.setSwissDaemonOnline === "function") {
				window.setSwissDaemonOnline(false);
			}
			const g = document.getElementById("swiss-left-nav-group");
			if (g) g.remove();
			document.querySelectorAll(".swiss-aux-btn-group, .swiss-aux-tabs-divider, .swiss-aux-tabs-divider-left, .swiss-aux-tabs-divider-right").forEach(el => el.remove());
			const st = document.getElementById("swiss-main-stage-container");
			if (st) st.style.display = "none";
			const aux = document.getElementById("swiss-aux-container");
			if (aux) aux.style.display = "none";
		} catch (_) {}
		return true;
	})()`

	if !keepVisualEffects {
		deactScript += `
		;(() => {
			try {
				if (window.__swissObserverInstance) {
					window.__swissObserverInstance.disconnect();
					window.__swissObserverInstance = null;
				}
				if (window.__swissIntervalId) {
					clearInterval(window.__swissIntervalId);
					window.__swissIntervalId = null;
				}
				const s = document.getElementById("antigravity-swiss-styles");
				if (s) s.remove();
				const dc = document.getElementById("antigravity-swiss-dynamic-colors");
				if (dc) dc.remove();
				document.querySelectorAll("[data-swiss-project]").forEach(el => el.removeAttribute("data-swiss-project"));
				document.querySelectorAll(".swiss-convo-tabs-divider, .swiss-project-bottom-spacer, .swiss-project-spacer-line").forEach(el => el.remove());
			} catch (_) {}
			return true;
		})()`
	}

	for _, page := range pages {
		_, _ = s.injector.ExecuteScript(page.WebSocketDebuggerURL, deactScript)
	}
	return nil
}

// InstallDesktopLoader installs the permanent desktop loader in Antigravity resources.
func (s *Store) InstallDesktopLoader() (*ApplyResult, error) {
	_ = s.SyncPersistentFiles()
	return s.desktopManager.InstallDesktopLoader()
}

// UninstallDesktopLoader removes the permanent desktop loader from Antigravity resources without wiping settings.
func (s *Store) UninstallDesktopLoader() (*ApplyResult, error) {
	configDir := filepath.Dir(s.configPath)
	if configDir != "" {
		_ = os.Remove(filepath.Join(configDir, "persistent_styles.css"))
		_ = os.Remove(filepath.Join(configDir, "persistent_script.js"))
	}
	return s.desktopManager.UninstallDesktopLoader()
}

// RestoreFactoryDefaults restores the factory original app.asar and resets configuration.
func (s *Store) RestoreFactoryDefaults() (*ApplyResult, error) {
	return s.desktopManager.RestoreFactoryDefaults(s)
}

// GetDesktopStatus returns current installation and backup status of desktop loader.
func (s *Store) GetDesktopStatus() DesktopStatus {
	return s.desktopManager.GetStatus()
}

// SetDesktopManager sets a custom DesktopManager (useful for unit testing).
func (s *Store) SetDesktopManager(dm *DesktopManager) {
	s.desktopManager = dm
}

// ArchiveProject marks a project as hidden/archived.
func (s *Store) ArchiveProject(nameOrID string) error {
	s.mu.Lock()
	nameOrID = strings.TrimSpace(nameOrID)
	if nameOrID == "" {
		s.mu.Unlock()
		return fmt.Errorf("project name or id cannot be empty")
	}

	for _, p := range s.config.ArchivedProjects {
		if strings.EqualFold(p, nameOrID) {
			s.mu.Unlock()
			return nil // already archived
		}
	}
	s.config.ArchivedProjects = append(s.config.ArchivedProjects, nameOrID)
	s.mu.Unlock()

	if err := s.Save(); err != nil {
		return err
	}
	if s.config.AutoInject {
		_, _ = s.Apply()
	}
	return nil
}

// RestoreProject restores an archived project back to active sidebar visibility.
func (s *Store) RestoreProject(nameOrID string) error {
	s.mu.Lock()
	nameOrID = strings.TrimSpace(nameOrID)
	newArchived := make([]string, 0, len(s.config.ArchivedProjects))
	for _, p := range s.config.ArchivedProjects {
		if !strings.EqualFold(p, nameOrID) {
			newArchived = append(newArchived, p)
		}
	}
	s.config.ArchivedProjects = newArchived
	s.mu.Unlock()

	if err := s.Save(); err != nil {
		return err
	}
	if s.config.AutoInject {
		_, _ = s.Apply()
	}
	return nil
}

// DeleteProject deletes a project from configuration and archived tracking.
func (s *Store) DeleteProject(nameOrID string) error {
	s.mu.Lock()
	nameOrID = strings.TrimSpace(nameOrID)

	// Remove from archived projects
	newArchived := make([]string, 0, len(s.config.ArchivedProjects))
	for _, p := range s.config.ArchivedProjects {
		if !strings.EqualFold(p, nameOrID) {
			newArchived = append(newArchived, p)
		}
	}
	s.config.ArchivedProjects = newArchived

	// Remove from colors & order
	delete(s.config.ProjectColors, nameOrID)
	newOrder := make([]string, 0, len(s.config.ProjectOrder))
	for _, p := range s.config.ProjectOrder {
		if !strings.EqualFold(p, nameOrID) {
			newOrder = append(newOrder, p)
		}
	}
	s.config.ProjectOrder = newOrder
	s.mu.Unlock()

	_ = s.Save()

	// If project config exists in ~/.gemini/config/projects/, move to .deleted
	projectsDir := filepath.Join(os.Getenv("HOME"), ".gemini", "config", "projects")
	jsonPath := filepath.Join(projectsDir, nameOrID+".json")
	if _, err := os.Stat(jsonPath); err == nil {
		_ = os.Rename(jsonPath, jsonPath+".deleted")
	}

	if s.config.AutoInject {
		_, _ = s.Apply()
	}
	return nil
}

// GetArchivedProjects returns metadata and activity stats for all archived projects.
func (s *Store) GetArchivedProjects() ([]ArchivedProjectItem, error) {
	s.mu.RLock()
	archivedNames := make([]string, len(s.config.ArchivedProjects))
	copy(archivedNames, s.config.ArchivedProjects)
	projectColors := make(map[string]string)
	for k, v := range s.config.ProjectColors {
		projectColors[k] = v
	}
	s.mu.RUnlock()

	// 1. Load project definitions from ~/.gemini/config/projects/
	projectsDir := filepath.Join(os.Getenv("HOME"), ".gemini", "config", "projects")
	projectMeta := make(map[string]struct {
		ID        string
		Name      string
		FolderURI string
	})

	if entries, err := os.ReadDir(projectsDir); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".json") && !strings.HasSuffix(entry.Name(), ".deleted") {
				filePath := filepath.Join(projectsDir, entry.Name())
				if data, err := os.ReadFile(filePath); err == nil {
					var raw struct {
						ID               string `json:"id"`
						Name             string `json:"name"`
						ProjectResources struct {
							Resources []struct {
								FolderURI string `json:"folderUri"`
							} `json:"resources"`
						} `json:"projectResources"`
					}
					if err := json.Unmarshal(data, &raw); err == nil && raw.Name != "" {
						uri := ""
						if len(raw.ProjectResources.Resources) > 0 {
							uri = raw.ProjectResources.Resources[0].FolderURI
						}
						m := struct {
							ID        string
							Name      string
							FolderURI string
						}{
							ID:        raw.ID,
							Name:      raw.Name,
							FolderURI: uri,
						}
						projectMeta[raw.ID] = m
						projectMeta[raw.Name] = m
					}
				}
			}
		}
	}

	// 2. Query activity stats from conversation_summaries.db
	dbStats := make(map[string]struct {
		Count      int
		LastActive time.Time
	})

	dbPath := filepath.Join(os.Getenv("HOME"), ".gemini", "antigravity", "conversation_summaries.db")
	if _, err := os.Stat(dbPath); err == nil {
		pyScript := `
import sqlite3, json, sys
try:
    conn = sqlite3.connect(sys.argv[1])
    c = conn.cursor()
    c.execute("SELECT project_id, COUNT(*), MAX(last_modified_time) FROM conversation_summaries GROUP BY project_id")
    res = {}
    for r in c.fetchall():
        if r[0]:
            res[r[0]] = {"count": r[1], "last_active": r[2]}
    print(json.dumps(res))
except Exception:
    print("{}")
`
		cmd := exec.Command("python3", "-c", pyScript, dbPath)
		if out, err := cmd.Output(); err == nil {
			var parsed map[string]struct {
				Count      int    `json:"count"`
				LastActive string `json:"last_active"`
			}
			if err := json.Unmarshal(out, &parsed); err == nil {
				for pID, stat := range parsed {
					var t time.Time
					if stat.LastActive != "" {
						t, _ = time.Parse(time.RFC3339, stat.LastActive)
						if t.IsZero() {
							t, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", stat.LastActive)
						}
						if t.IsZero() {
							t, _ = time.Parse("2006-01-02 15:04:05.999999999+00:00", stat.LastActive)
						}
					}
					dbStats[pID] = struct {
						Count      int
						LastActive time.Time
					}{
						Count:      stat.Count,
						LastActive: t,
					}
				}
			}
		}
	}

	now := time.Now().UTC()
	var result []ArchivedProjectItem

	for _, nameOrID := range archivedNames {
		pName := nameOrID
		pID := nameOrID
		folderURI := ""

		if meta, exists := projectMeta[nameOrID]; exists {
			pName = meta.Name
			pID = meta.ID
			folderURI = meta.FolderURI
		}

		color := projectColors[pName]
		if color == "" {
			color = "#64748b"
		}

		count := 0
		var lastActive time.Time

		if stat, ok := dbStats[pID]; ok {
			count = stat.Count
			lastActive = stat.LastActive
		} else if stat, ok := dbStats[pName]; ok {
			count = stat.Count
			lastActive = stat.LastActive
		}

		relTime := FormatRelativeTime(lastActive, now)

		result = append(result, ArchivedProjectItem{
			ID:                 pID,
			Name:               pName,
			Color:              color,
			ConversationCount:  count,
			LastActiveTime:     lastActive,
			LastActiveRelative: relTime,
			FolderURI:          folderURI,
			ArchivedAt:         now,
		})
	}

	return result, nil
}

// OpenProjectSettings signals Antigravity via CDP to open the native settings modal for target project.
func (s *Store) OpenProjectSettings(nameOrID string) error {
	nameOrID = strings.TrimSpace(nameOrID)
	if nameOrID == "" {
		return fmt.Errorf("project name or id required")
	}

	targetName := nameOrID
	projectsDir := filepath.Join(os.Getenv("HOME"), ".gemini", "config", "projects")
	if data, err := os.ReadFile(filepath.Join(projectsDir, nameOrID+".json")); err == nil {
		var raw struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &raw) == nil && raw.Name != "" {
			targetName = raw.Name
		}
	}

	port, err := s.injector.FindDevToolsPort()
	if err != nil {
		return fmt.Errorf("antigravity desktop app not running: %w", err)
	}

	pages, err := s.injector.GetPageTargets(port)
	if err != nil || len(pages) == 0 {
		return fmt.Errorf("no antigravity window target found: %w", err)
	}

	target := pages[0]
	safeName := strings.ReplaceAll(targetName, `\`, `\\`)
	safeName = strings.ReplaceAll(safeName, `"`, `\"`)

	jsExpr := fmt.Sprintf(`(async () => {
		let modal = document.querySelector('[role="dialog"]');
		if (!modal) {
			const sBtn = document.querySelector('[data-testid="settings-button"]');
			if (sBtn) sBtn.click();
			await new Promise(r => setTimeout(r, 400));
			modal = document.querySelector('[role="dialog"]');
		}
		if (!modal) return false;
		let projBtn = Array.from(modal.querySelectorAll('button')).find(b => b.textContent && b.textContent.trim() === "%s");
		if (!projBtn) {
			const showAllBtn = Array.from(modal.querySelectorAll('button')).find(b => b.textContent && b.textContent.trim().toLowerCase().includes("show all"));
			if (showAllBtn) {
				showAllBtn.click();
				await new Promise(r => setTimeout(r, 200));
			}
			projBtn = Array.from(modal.querySelectorAll('button')).find(b => b.textContent && b.textContent.trim() === "%s");
		}
		if (projBtn) {
			projBtn.click();
			return true;
		}
		return false;
	})()`, safeName, safeName)

	_, err = s.injector.ExecuteScript(target.WebSocketDebuggerURL, jsExpr)
	return err
}


