package webgui

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/backend"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/bridge"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/cache"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/github"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/importer"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/process"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/revival"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/system"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/totp"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/vault"
)

//go:embed all:dist
var distFS embed.FS

var (
	newGUIConfigStore    = gui.NewStore
	newCustomModelsStore = custommodels.NewStore
	newEnhancementsStore = enhancements.NewStore
	newGitHubStore       = github.NewStore
	newVaultManager      = vault.NewManager
	newRevivalEngine     = revival.NewEngine
)

// RPCClient is the explicit daemon transport used by standalone web mode.
type RPCClient interface {
	Call(method string, params interface{}, out interface{}) error
}

type githubHandlerService interface {
	DetectRepository(workspacePath string) (*github.RepoInfo, error)
	DetectRepositoryStrict(workspacePath string) (*github.RepoInfo, error)
	ListIssues(repo *github.RepoInfo, state string, search string) ([]github.Issue, error)
	GetIssue(repo *github.RepoInfo, number int) (*github.IssueDetail, error)
	UpdateIssue(repo *github.RepoInfo, number int, req github.UpdateIssueRequest) (*github.Issue, error)
	AddIssueComment(repo *github.RepoInfo, number int, body string) error
	CreateIssue(repo *github.RepoInfo, req github.CreateIssueRequest) (*github.Issue, error)
	ListPullRequests(repo *github.RepoInfo, state string) ([]github.PullRequest, error)
	GetPullRequest(repo *github.RepoInfo, number int) (*github.PullRequestDetail, error)
	ListProjects(repo *github.RepoInfo) ([]github.ProjectBoard, error)
	GetKanbanBoard(repo *github.RepoInfo, projectNumber int) (*github.KanbanBoard, error)
	MoveKanbanCard(repo *github.RepoInfo, req *github.MoveKanbanCardRequest) error
	GetAgentTasks(workspacePath string) ([]github.AgentTaskSummary, error)
	BindTask(convID string, issueNumber int, agentLabel string) error
	SetAgentLabel(convID string, label string) error
	ReportAgentTask(workspacePath string, task github.SelfReportedTask) error
	FormatContextForChat(itemType string, number int, repo *github.RepoInfo) (string, error)
}

// Server serves the minimalist light theme web UI and REST API.
type Server struct {
	client                    RPCClient
	backend                   backend.BackendService
	guiStore                  *gui.Store
	customModelsStore         *custommodels.Store
	customModelsGateway       *custommodels.Gateway
	cloudCodeProxy            *custommodels.Proxy
	customModelsTester        *custommodels.Tester
	systemDetector            *system.Detector
	appReleaseManager         *system.AppReleaseManager
	enhancementsStore         *enhancements.Store
	oauthMgr                  *keyring.GoogleOAuthManager
	githubService             githubHandlerService
	vaultManager              *vault.Manager
	revivalEngine             *revival.Engine
	bridgeService             *bridge.Service
	httpServer                *http.Server
	addr                      string
	memoMu                    sync.RWMutex
	globalMemosPath           string
	antigravityDataDir        string
	catalogFetcher            func(force bool) []custommodels.CatalogModelInput
	knownWorkspaces           map[string]struct{}
	quotaPollMu               sync.Mutex
	quotaPolling              bool
	testIDEsMu                sync.Mutex
	testIDEs                  []int
	lazyMu                    sync.Mutex
	authMu                    sync.RWMutex
	sessionTokens             map[string]time.Time
	daemonSessionToken        string
	legacyAppPasswordAttempts map[string]appPasswordAttemptWindow
	desktopSettingsMu         sync.Mutex
	isSwitching               atomic.Bool
	lifetimeCostsMu           sync.RWMutex
	lifetimeCostsCache        map[string]float64
	lifetimeCostsExp          time.Time
	syncLSURLStop             chan struct{}
	restoreNativeDeps         restoreNativeDeps
}

type restoreNativeDeps struct {
	syncURL     func(string) error
	restartApps func() error
}

type restoreNativeOutcome struct {
	SyncErr    error
	RestartErr error
}

func (o restoreNativeOutcome) err() error {
	return errors.Join(o.SyncErr, o.RestartErr)
}

// NewServer initializes the daemon-backed web UI server.
func NewServer(addr string, service backend.BackendService, rpc RPCClient) (*Server, error) {
	if service == nil || service.Accounts() == nil {
		return nil, errors.New("account backend is required")
	}
	if service.Quota() == nil {
		return nil, errors.New("quota backend is required")
	}
	if service.GUI() == nil {
		return nil, errors.New("GUI backend is required")
	}
	if service.CustomModels() == nil {
		return nil, errors.New("custom-model backend is required")
	}
	if service.Enhancements() == nil {
		return nil, errors.New("enhancement backend is required")
	}
	if service.GitHub() == nil {
		return nil, errors.New("GitHub backend is required")
	}
	if service.Vault() == nil {
		return nil, errors.New("vault backend is required")
	}
	if service.Revival() == nil {
		return nil, errors.New("revival backend is required")
	}
	return newServer(addr, service, rpc)
}

// NewRPCServer initializes standalone web mode with an explicit daemon RPC client.
func NewRPCServer(addr string, socketPath string) (*Server, error) {
	return newServer(addr, nil, ipc.NewClient(socketPath))
}

func newServer(addr string, service backend.BackendService, rpc RPCClient) (*Server, error) {
	if rpc == nil {
		return nil, errors.New("daemon RPC client is required")
	}
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	var (
		guiStore  *gui.Store
		cmStore   *custommodels.Store
		enhStore  *enhancements.Store
		ghService githubHandlerService
		vaultMgr  *vault.Manager
		revEngine *revival.Engine
		err       error
	)
	if service == nil {
		guiStore, err = newGUIConfigStore("")
		if err != nil {
			return nil, fmt.Errorf("initialize GUI store: %w", err)
		}
		cmStore, err = newCustomModelsStore("")
		if err != nil {
			return nil, fmt.Errorf("initialize custom-model store: %w", err)
		}
		enhStore, err = newEnhancementsStore("")
		if err != nil {
			return nil, fmt.Errorf("initialize enhancement store: %w", err)
		}
		ghStore, err := newGitHubStore("")
		if err != nil {
			return nil, fmt.Errorf("initialize GitHub store: %w", err)
		}
		ghService = github.NewService(github.NewAgentTracker("", ghStore), ghStore)
		vaultMgr = newVaultManager("", "")
		revEngine = newRevivalEngine("", 0)
	} else {
		cmStore = service.CustomModels()
		enhStore = service.Enhancements()
		ghService = service.GitHub()
		vaultMgr = service.Vault()
		revEngine = service.Revival()
	}
	cmTester := custommodels.NewTester()
	custommodels.AutoEnsureCLIGatewayIntegration(cmStore)
	sysDetector := system.NewDetector()
	appRelMgr := system.NewAppReleaseManager()
	oauthMgr := keyring.NewGoogleOAuthManager("", "")
	bridgeSvc, err := bridge.NewService("")
	if err != nil {
		return nil, fmt.Errorf("initialize bridge service: %w", err)
	}
	localTok, err := GenerateLocalSessionToken()
	if err != nil {
		return nil, fmt.Errorf("initialize local session token: %w", err)
	}
	sessMap := make(map[string]time.Time)
	if localTok != "" {
		sessMap[localTok] = time.Now().Add(365 * 24 * time.Hour)
	}
	return &Server{
		client:             rpc,
		backend:            service,
		guiStore:           guiStore,
		customModelsStore:  cmStore,
		customModelsTester: cmTester,
		systemDetector:     sysDetector,
		appReleaseManager:  appRelMgr,
		enhancementsStore:  enhStore,
		oauthMgr:           oauthMgr,
		githubService:      ghService,
		vaultManager:       vaultMgr,
		revivalEngine:      revEngine,
		bridgeService:      bridgeSvc,
		addr:               addr,
		knownWorkspaces:    make(map[string]struct{}),
		sessionTokens:      sessMap,
		daemonSessionToken: localTok,
	}, nil
}

func (s *Server) createSessionToken() (string, error) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	if s.sessionTokens == nil {
		s.sessionTokens = make(map[string]time.Time)
	}
	now := time.Now()
	for tok, exp := range s.sessionTokens {
		if !now.Before(exp) {
			delete(s.sessionTokens, tok)
		}
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}
	tok := hex.EncodeToString(b)
	s.sessionTokens[tok] = now.Add(24 * time.Hour)
	return tok, nil
}

func (s *Server) isAuthorized(r *http.Request) bool {
	return s.validateRequestAuth(r)
}

// GetSessionToken returns the startup local session token.
func (s *Server) GetSessionToken() string {
	s.authMu.RLock()
	defer s.authMu.RUnlock()
	return s.daemonSessionToken
}

// SetSessionToken configures or overrides the active session token (useful for test suites).
func (s *Server) SetSessionToken(token string) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	s.daemonSessionToken = token
	if s.sessionTokens == nil {
		s.sessionTokens = make(map[string]time.Time)
	}
	if token != "" {
		s.sessionTokens[token] = time.Now().Add(365 * 24 * time.Hour)
	}
	if onServerStartHook != nil && s.addr != "" {
		onServerStartHook(s.addr, token)
	}
}

func (s *Server) getBridgeService() *bridge.Service {
	s.lazyMu.Lock()
	defer s.lazyMu.Unlock()
	if s.bridgeService == nil {
		s.bridgeService, _ = bridge.NewService("")
	}
	return s.bridgeService
}

func (s *Server) getSystemDetector() *system.Detector {
	s.lazyMu.Lock()
	defer s.lazyMu.Unlock()
	if s.systemDetector == nil {
		s.systemDetector = system.NewDetector()
	}
	return s.systemDetector
}

func (s *Server) getAppReleaseManager() *system.AppReleaseManager {
	s.lazyMu.Lock()
	defer s.lazyMu.Unlock()
	if s.appReleaseManager == nil {
		s.appReleaseManager = system.NewAppReleaseManager()
	}
	return s.appReleaseManager
}

func (s *Server) getVaultManager() *vault.Manager {
	s.lazyMu.Lock()
	defer s.lazyMu.Unlock()
	if s.vaultManager == nil {
		if s.backend != nil {
			s.vaultManager = s.backend.Vault()
		} else {
			s.vaultManager = newVaultManager("", "")
		}
	}
	return s.vaultManager
}

func (s *Server) getRevivalEngine() *revival.Engine {
	if s.revivalEngine != nil {
		return s.revivalEngine
	}
	if s.backend != nil {
		return s.backend.Revival()
	}
	return nil
}

// getCustomModelsGateway returns the shared LLM gateway for custom models.
// getCloudCodeProxy returns the shared Cloud Code pass-through proxy.
func (s *Server) getCloudCodeProxy() *custommodels.Proxy {
	store := s.getCustomModelsStore()
	if store == nil {
		return nil
	}
	s.lazyMu.Lock()
	defer s.lazyMu.Unlock()
	if s.cloudCodeProxy == nil {
		s.cloudCodeProxy = custommodels.NewProxy(store)
	}
	return s.cloudCodeProxy
}

func (s *Server) getCustomModelsGateway() *custommodels.Gateway {
	store := s.getCustomModelsStore()
	if store == nil {
		return nil
	}
	s.lazyMu.Lock()
	defer s.lazyMu.Unlock()
	if s.customModelsGateway == nil {
		s.customModelsGateway = custommodels.NewGateway(store)
	}
	return s.customModelsGateway
}

func (s *Server) getCustomModelsStore() *custommodels.Store {
	s.lazyMu.Lock()
	defer s.lazyMu.Unlock()
	return s.customModelsStore
}

func (s *Server) getCustomModelsTester() *custommodels.Tester {
	s.lazyMu.Lock()
	defer s.lazyMu.Unlock()
	if s.customModelsTester == nil {
		s.customModelsTester = custommodels.NewTester()
	}
	return s.customModelsTester
}

func (s *Server) getGUIStore() *gui.Store {
	s.lazyMu.Lock()
	defer s.lazyMu.Unlock()
	if s.guiStore == nil && s.backend == nil {
		s.guiStore, _ = gui.NewStore("")
	}
	return s.guiStore
}

func (s *Server) getEnhancementsStore() *enhancements.Store {
	s.lazyMu.Lock()
	defer s.lazyMu.Unlock()
	return s.enhancementsStore
}

// SetRevivalEngine replaces the revivalEngine on the server (useful for tests).
func (s *Server) SetRevivalEngine(engine *revival.Engine) {
	s.revivalEngine = engine
}

// SetVaultManager replaces the vaultManager on the server (useful for tests).
func (s *Server) SetVaultManager(mgr *vault.Manager) {
	s.vaultManager = mgr
}

// SetAppReleaseManager replaces the appReleaseManager on the server (useful for tests).
func (s *Server) SetAppReleaseManager(mgr *system.AppReleaseManager) {
	s.appReleaseManager = mgr
}

// SetGUIStore replaces the guiStore on the server (useful for testing with isolated temporary config directories).
func (s *Server) SetGUIStore(store *gui.Store) {
	s.guiStore = store
	if s.backend != nil {
		if setter, ok := s.backend.GUI().(interface{ SetStore(*gui.Store) }); ok {
			setter.SetStore(store)
		}
	}
}

// SetGitHubService replaces the githubService on the server (useful for tests).
func (s *Server) SetGitHubService(svc githubHandlerService) {
	s.githubService = svc
}

// SetGlobalMemosPath replaces the global memos storage path (useful for testing with isolated temporary config directories).
func (s *Server) SetGlobalMemosPath(path string) {
	s.memoMu.Lock()
	defer s.memoMu.Unlock()
	s.globalMemosPath = path
}

// AddKnownWorkspace registers a project workspace directory for aggregated queries.
func (s *Server) AddKnownWorkspace(ws string) {
	clean := cleanUserPath(ws)
	if clean == "" {
		return
	}
	s.memoMu.Lock()
	defer s.memoMu.Unlock()
	if s.knownWorkspaces == nil {
		s.knownWorkspaces = make(map[string]struct{})
	}
	s.knownWorkspaces[clean] = struct{}{}
}

// Hooks used by test suites to automatically register and deregister active server addresses.
var (
	onServerStartHook       func(addr, token string)
	onServerStopHook        func(addr string)
	languageServerURLSyncer = quota.SetLanguageServerCloudCodeURL
)

// setupRoutes registers all application endpoint handlers onto a fresh ServeMux.
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	distSub, errDist := fs.Sub(distFS, "dist")
	var fileServer http.Handler
	if errDist == nil {
		fileServer = http.FileServer(http.FS(distSub))
		mux.Handle("/assets/", fileServer)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			if distSub != nil {
				data, err := fs.ReadFile(distSub, "index.html")
				if err == nil {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
					return
				}
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(indexHTML))
			return
		}
		if fileServer != nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		daemonRunning := false
		daemonPID := 0
		if s.client != nil {
			var status map[string]interface{}
			if err := s.client.Call("swiss.getStatus", nil, &status); err == nil {
				daemonRunning = true
				if p, ok := status["daemon_pid"].(float64); ok {
					daemonPID = int(p)
				} else if p, ok := status["pid"].(float64); ok {
					daemonPID = int(p)
				}
			}
		}
		if daemonPID == 0 {
			daemonPID = os.Getpid()
		}
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"status":         "ok",
			"version":        core.AppVersion,
			"daemon_running": daemonRunning,
			"daemon_pid":     daemonPID,
			"pid":            daemonPID,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/api/health", healthHandler)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/accounts", s.handleAccounts)
	mux.HandleFunc("/api/accounts/scan", s.handleAccountsScan)
	mux.HandleFunc("/api/accounts/import", s.handleAccountsImport)
	mux.HandleFunc("/api/accounts/update", s.handleAccountUpdate)
	mux.HandleFunc("/api/accounts/delete", s.handleAccountDelete)
	mux.HandleFunc("/api/accounts/export", s.handleAccountsExport)
	mux.HandleFunc("/api/accounts/batch-import", s.handleAccountsBatchImport)
	mux.HandleFunc("/api/switch", s.handleSwitch)
	mux.HandleFunc("/api/totp", s.handleTOTP)
	mux.HandleFunc("/api/fingerprint", s.handleFingerprint)
	mux.HandleFunc("/api/cache/scan", s.handleCacheScan)
	mux.HandleFunc("/api/cache/prune", s.handleCachePrune)
	mux.HandleFunc("/api/cache/config", s.handleCacheConfig)
	mux.HandleFunc("/api/quota", s.handleQuota)
	mux.HandleFunc("/api/quota/fleet", s.handleFleetQuota)
	mux.HandleFunc("/api/quota/refresh", s.handleQuotaRefresh)
	mux.HandleFunc("/api/rules", s.handleRules)
	mux.HandleFunc("/api/rules/auto_switch", s.handleAutoSwitch)
	mux.HandleFunc("/api/gui/config", s.handleGUIConfig)
	mux.HandleFunc("/api/gui/projects", s.handleGUIProjects)
	mux.HandleFunc("/api/gui/projects/color", s.handleGUIProjectColor)
	mux.HandleFunc("/api/gui/projects/delete", s.handleGUIProjectDelete)
	mux.HandleFunc("/api/gui/projects/order", s.handleGUIProjectOrder)
	mux.HandleFunc("/api/gui/projects/archived", s.handleGUIProjectsArchived)
	mux.HandleFunc("/api/gui/projects/archive", s.handleGUIProjectsArchive)
	mux.HandleFunc("/api/gui/projects/restore", s.handleGUIProjectsRestore)
	mux.HandleFunc("/api/gui/projects/open_settings", s.handleGUIProjectsOpenSettings)
	mux.HandleFunc("/api/gui/conversations/auto-archive", s.handleGUIConversationsAutoArchive)
	mux.HandleFunc("/api/gui/conversations/pruned", s.handleGUIConversationsPruned)
	mux.HandleFunc("/api/vault/status", s.handleVaultStatus)
	mux.HandleFunc("/api/vault/sync", s.handleVaultSync)
	mux.HandleFunc("/api/vault/toggle", s.handleVaultToggle)
	mux.HandleFunc("/api/gui/color", s.handleGUIProjectColor)
	mux.HandleFunc("/api/gui/color/delete", s.handleGUIProjectColorDelete)
	mux.HandleFunc("/api/gui/projects/color/delete", s.handleGUIProjectColorDelete)
	mux.HandleFunc("/api/gui/color/reset", s.handleGUIProjectColorDelete)
	mux.HandleFunc("/api/gui/projects/color/reset", s.handleGUIProjectColorDelete)
	mux.HandleFunc("/api/gui/reorder", s.handleGUIProjectOrder)
	mux.HandleFunc("/api/gui/apply", s.handleGUIApply)
	mux.HandleFunc("/api/gui/inject", s.handleGUIApply)
	mux.HandleFunc("/api/gui/desktop/install", s.handleGUIDesktopInstall)
	mux.HandleFunc("/api/gui/desktop/restore", s.handleGUIDesktopRestore)
	mux.HandleFunc("/api/gui/desktop/status", s.handleGUIDesktopStatus)
	mux.HandleFunc("/api/gui/desktop-status", s.handleGUIDesktopStatus)
	mux.HandleFunc("/api/gui/desktop/persistence", s.handleGUIDesktopPersistence)
	mux.HandleFunc("/api/desktop/relaunch", s.handleDesktopRelaunch)
	mux.HandleFunc("/api/gui/desktop/relaunch", s.handleDesktopRelaunch)
	mux.HandleFunc("/api/desktop/launch", s.handleDesktopLaunch)
	mux.HandleFunc("/api/gui/desktop/launch", s.handleDesktopLaunch)
	mux.HandleFunc("/api/vscode/launch", s.handleVSCodeLaunch)
	mux.HandleFunc("/api/gui/vscode/launch", s.handleVSCodeLaunch)

	// Active Conversations & Subagent Revival
	mux.HandleFunc("/api/conversations/active", s.handleConversationsActive)
	mux.HandleFunc("/api/conversations/revive", s.handleConversationsRevive)
	mux.HandleFunc("/api/conversations/status", s.handleConversationsStatus)
	mux.HandleFunc("/api/conversations/ack", s.handleConversationsAck)
	mux.HandleFunc("/api/session/continuation", s.handleConversationsStatus)
	mux.HandleFunc("/api/session/continuation/ack", s.handleConversationsAck)

	// System installations & updates
	mux.HandleFunc("/api/system/installations", s.handleSystemInstallations)
	mux.HandleFunc("/api/system/check_updates", s.handleSystemCheckUpdates)

	// App releases & upgrade
	mux.HandleFunc("/api/system/app_release", s.handleAppRelease)
	mux.HandleFunc("/api/system/check_app_release", s.handleCheckAppRelease)
	mux.HandleFunc("/api/system/app_release/settings", s.handleAppReleaseSettings)
	mux.HandleFunc("/api/system/app_release/upgrade", s.handleAppReleaseUpgrade)

	// Available models catalog
	mux.HandleFunc("/api/models/available", s.handleAvailableModels)
	mux.HandleFunc("/api/available_models", s.handleAvailableModels)

	// Custom models provider
	mux.HandleFunc("/api/custom_models", s.handleCustomModels)
	// LLM gateway: serves custom models to Antigravity surfaces that accept a
	// bring-your-own-model gateway URL (the CLI's AGY_LLM_GATEWAY_URL).
	if gateway := s.getCustomModelsGateway(); gateway != nil {
		// Gemini-shaped surface for the CLI, OpenAI-shaped surface for the app's
		// agent cascade (it posts to /v1/chat/completions).
		mux.Handle("/v1beta/", gateway.Handler())
		mux.Handle("/v1/", gateway.Handler())
	}
	// The Cloud Code proxy is dispatched ahead of the mux (see cloudCodeDispatch):
	// ServeMux patterns cannot express "/v1internal:fetchAvailableModels".
	mux.HandleFunc("/api/custom_models/active", s.handleCustomModelActive)
	mux.HandleFunc("/api/custom_models/app_proxy", s.handleCustomModelAppProxy)
	mux.HandleFunc("/api/custom_models/restore_native", s.handleCustomModelRestoreNative)
	mux.HandleFunc("/api/custom_models/integration", s.handleCustomModelIntegration)
	mux.HandleFunc("/api/custom_models/launch", s.handleCustomModelLaunch)
	mux.HandleFunc("/api/custom_models/presets", s.handleCustomModelPresets)
	mux.HandleFunc("/api/custom_models/test", s.handleCustomModelTest)
	mux.HandleFunc("/api/custom_models/fetch_models", s.handleCustomModelFetchModels)
	mux.HandleFunc("/api/custom_models/thinking_level", s.handleCustomModelThinkingLevel)
	mux.HandleFunc("/api/custom_models/fetch_quota", s.handleCustomModelFetchQuota)
	mux.HandleFunc("/api/custom_models/refresh_quotas", s.handleCustomModelRefreshQuotas)
	mux.HandleFunc("/api/custom_models/cli_status", s.handleCustomModelCLIStatus)
	mux.HandleFunc("/api/custom_models/cli_setup", s.handleCustomModelCLISetup)
	mux.HandleFunc("/api/custom_models/security-audit", s.handleCustomModelSecurityAudit)
	mux.HandleFunc("/api/custom-models/security-audit", s.handleCustomModelSecurityAudit)

	// App Enhancements (Prompt Jump Bar, Tool Density, Breaker Line)
	mux.HandleFunc("/api/enhancements", s.handleEnhancements)
	mux.HandleFunc("/api/enhancements/update", s.handleEnhancementsUpdate)
	mux.HandleFunc("/api/enhancements/apply", s.handleEnhancementsApply)
	mux.HandleFunc("/api/extensions/community", s.handleCommunityExtensions)

	// Google OAuth Extraction
	mux.HandleFunc("/api/oauth/google/start", s.handleGoogleOAuthStart)
	mux.HandleFunc("/api/oauth/google/cancel", s.handleGoogleOAuthCancel)
	mux.HandleFunc("/api/oauth/google/url", s.handleGoogleOAuthURL)
	mux.HandleFunc("/api/oauth/google/exchange", s.handleGoogleOAuthExchange)

	// Multi-Surface Antigravity Session Inspector
	mux.HandleFunc("/api/surfaces", s.handleSurfaces)

	// App Access Password & Authentication
	mux.HandleFunc("/api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/auth/unlock", s.handleAuthUnlock)
	mux.HandleFunc("/api/settings/password", s.handleSettingsPassword)
	mux.HandleFunc("/api/settings/storage", s.handleSettingsStorage)
	mux.HandleFunc("/api/settings/app_path", s.handleSettingsAppPath)
	mux.HandleFunc("/api/settings/account_override", s.handleSettingsAccountOverride)
	mux.HandleFunc("/api/settings/cache_clear", s.handleSettingsCacheClear)
	mux.HandleFunc("/api/settings/privacy", s.handleSettingsPrivacy)
	mux.HandleFunc("/api/settings/runtime-mode", s.handleSettingsRuntimeMode)
	mux.HandleFunc("/api/settings/theme", s.handleSettingsTheme)
	mux.HandleFunc("/api/settings/diagnose-issue", s.handleSettingsDiagnoseIssue)
	mux.HandleFunc("/api/daemon/start", s.handleDaemonStart)
	mux.HandleFunc("/api/system/factory_reset", s.handleSystemFactoryReset)

	// Token & Cost Monitor
	mux.HandleFunc("/api/tokens/summary", s.handleTokensSummary)
	mux.HandleFunc("/api/tokens/pricing", s.handleTokensPricing)
	mux.HandleFunc("/api/tokens/chat-metrics", s.handleTokensChatMetrics)

	// Utilities & Interoperability (Chat Import & ACP Mesh)
	mux.HandleFunc("/api/utilities/acp", s.handleUtilitiesACP)
	mux.HandleFunc("/api/utilities/acp/agents", s.handleUtilitiesACPAgents)
	mux.HandleFunc("/api/utilities/acp/detect", s.handleUtilitiesACPAgentDetect)
	mux.HandleFunc("/api/utilities/acp/ping", s.handleUtilitiesACPPing)
	mux.HandleFunc("/api/utilities/acp/prompt", s.handleUtilitiesACPPrompt)
	mux.HandleFunc("/api/utilities/acp/sync-registry", s.handleUtilitiesACPSyncRegistry)
	mux.HandleFunc("/api/utilities/import", s.handleUtilitiesImport)
	mux.HandleFunc("/api/utilities/import/scan", s.handleUtilitiesImportScan)

	// Gemini Bridge (F41 / M15) Two-Way Agent Orchestration & Streamable HTTP MCP
	mux.HandleFunc("/api/bridge/status", s.handleBridgeStatus)
	mux.HandleFunc("/api/bridge/config", s.handleBridgeConfig)
	mux.HandleFunc("/api/bridge/dispatch", s.handleBridgeDispatch)
	mux.HandleFunc("/api/bridge/test", s.handleBridgeTest)
	mux.HandleFunc("/api/bridge/mcp", s.handleBridgeMCP)
	mux.HandleFunc("/mcp", s.handleBridgeMCP)
	// A2A (Agent-to-Agent Protocol) Discovery & High-Level Task Delegation
	mux.HandleFunc("/.well-known/agent-card.json", s.handleA2AAgentCardSpec)
	mux.HandleFunc("/.well-known/agent.json", s.handleA2AAgentCard)
	mux.HandleFunc("/agent.json", s.handleA2AAgentCard)
	mux.HandleFunc("/a2a/agent.json", s.handleA2AAgentCard)
	mux.HandleFunc("/api/bridge/a2a/agent.json", s.handleA2AAgentCard)
	// A2A JSON-RPC 2.0 transport (message/send, message/stream, tasks/get,
	// tasks/cancel, tasks/resubscribe).
	mux.HandleFunc("/a2a", s.handleA2AJSONRPC)
	mux.HandleFunc("/a2a/", s.handleA2AJSONRPC)
	mux.HandleFunc("/api/bridge/a2a", s.handleA2AJSONRPC)
	mux.HandleFunc("/api/bridge/a2a/rpc", s.handleA2AJSONRPC)
	mux.HandleFunc("/a2a/tasks", s.handleA2ATasks)
	mux.HandleFunc("/a2a/tasks/", s.handleA2ATasks)
	mux.HandleFunc("/api/bridge/a2a/tasks", s.handleA2ATasks)
	mux.HandleFunc("/api/bridge/a2a/tasks/", s.handleA2ATasks)
	mux.HandleFunc("/.well-known/oauth-protected-resource", s.handleOAuthProtectedResource)
	mux.HandleFunc("/.well-known/oauth-protected-resource/", s.handleOAuthProtectedResource)
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleOAuthMetadata)
	mux.HandleFunc("/.well-known/oauth-authorization-server/", s.handleOAuthMetadata)
	mux.HandleFunc("/.well-known/openid-configuration", s.handleOAuthMetadata)
	mux.HandleFunc("/oauth/register", s.handleOAuthRegister)
	mux.HandleFunc("/oauth/authorize", s.handleOAuthAuthorize)
	mux.HandleFunc("/oauth/token", s.handleOAuthToken)
	mux.HandleFunc("/api/bridge/oauth/register", s.handleOAuthRegister)
	mux.HandleFunc("/api/bridge/oauth/authorize", s.handleOAuthAuthorize)
	mux.HandleFunc("/api/bridge/oauth/token", s.handleOAuthToken)
	mux.HandleFunc("/api/bridge/tunnel/start", s.handleBridgeTunnelStart)
	mux.HandleFunc("/api/bridge/tunnel/stop", s.handleBridgeTunnelStop)
	mux.HandleFunc("/api/bridge/token/rotate", s.handleBridgeTokenRotate)
	mux.HandleFunc("/api/bridge/mode", s.handleBridgeMode)
	mux.HandleFunc("/api/bridge/agent", s.handleBridgeAgent)
	mux.HandleFunc("/api/bridge/http-allowlist", s.handleBridgeHTTPAllowlist)
	mux.HandleFunc("/api/bridge/remote/status", s.handleBridgeRemoteStatus)
	mux.HandleFunc("/api/bridge/remote/config", s.handleBridgeRemoteConfig)
	mux.HandleFunc("/api/bridge/remote/tunnel/start", s.handleBridgeRemoteTunnelStart)
	mux.HandleFunc("/api/bridge/remote/tunnel/stop", s.handleBridgeRemoteTunnelStop)
	mux.HandleFunc("/api/bridge/remote/pin/rotate", s.handleBridgeRemotePINRotate)
	mux.HandleFunc("/api/bridge/channels/status", s.handleBridgeChannelsStatus)
	mux.HandleFunc("/api/bridge/channels/config", s.handleBridgeChannelsConfig)
	mux.HandleFunc("/api/bridge/channels/test", s.handleBridgeChannelsTest)
	mux.HandleFunc("/api/bridge/channels/webhook", s.handleBridgeChannelsWebhook)
	mux.HandleFunc("/api/bridge/channels/telegram/webhook", s.handleBridgeTelegramWebhook)
	mux.HandleFunc("/api/bridge/channels/discord/webhook", s.handleBridgeDiscordWebhook)
	mux.HandleFunc("/remote", s.handleBridgeRemoteConsole)
	mux.HandleFunc("/remote/", s.handleBridgeRemoteConsole)

	// Quick Memos API
	mux.HandleFunc("/api/memos", s.handleMemos)
	mux.HandleFunc("/api/memos/save", s.handleMemosSave)
	mux.HandleFunc("/api/memos/delete", s.handleMemosDelete)
	mux.HandleFunc("/api/memos/config", s.handleMemosConfig)
	mux.HandleFunc("/api/memos/extract", s.handleMemosExtract)
	mux.HandleFunc("/api/memos/export", s.handleMemosExport)

	// Real Filesystem Explorer API
	mux.HandleFunc("/api/files/list", s.handleFilesList)
	mux.HandleFunc("/api/files/read", s.handleFilesRead)
	mux.HandleFunc("/api/files/write", s.handleFilesWrite)
	mux.HandleFunc("/api/files/rename", s.handleFilesRename)
	mux.HandleFunc("/api/files/delete", s.handleFilesDelete)
	mux.HandleFunc("/api/files/create", s.handleFilesCreate)
	mux.HandleFunc("/api/files/copy", s.handleFilesCopy)
	mux.HandleFunc("/api/files/move", s.handleFilesMove)
	mux.HandleFunc("/api/files/reveal", s.handleFilesReveal)
	mux.HandleFunc("/api/files/terminal", s.handleFilesTerminal)
	mux.HandleFunc("/api/files/open_ide", s.handleFilesOpenIDE)
	mux.HandleFunc("/api/files/ide", s.handleFilesOpenIDE)
	mux.HandleFunc("/api/files/ide/config", s.handleFilesIDEConfig)

	// GitHub Workspace & Agent Task Tracking API
	mux.HandleFunc("/api/github/repo", s.handleGitHubRepo)
	mux.HandleFunc("/api/github/issues", s.handleGitHubIssues)
	mux.HandleFunc("/api/github/issues/detail", s.handleGitHubIssueDetail)
	mux.HandleFunc("/api/github/issues/update", s.handleGitHubIssueUpdate)
	mux.HandleFunc("/api/github/issues/comment", s.handleGitHubIssueComment)
	mux.HandleFunc("/api/github/issues/create", s.handleGitHubIssueCreate)
	mux.HandleFunc("/api/github/prs", s.handleGitHubPRs)
	mux.HandleFunc("/api/github/prs/detail", s.handleGitHubPRDetail)
	mux.HandleFunc("/api/github/projects", s.handleGitHubProjects)
	mux.HandleFunc("/api/github/kanban", s.handleGitHubKanbanBoard)
	mux.HandleFunc("/api/github/kanban/move", s.handleGitHubKanbanMove)
	mux.HandleFunc("/api/github/projects/board", s.handleGitHubKanbanBoard)
	mux.HandleFunc("/api/github/agent-tasks", s.handleGitHubAgentTasks)
	mux.HandleFunc("/api/github/agent-tasks/bind", s.handleGitHubAgentTaskBind)
	mux.HandleFunc("/api/github/agent-tasks/label", s.handleGitHubAgentTaskLabel)
	mux.HandleFunc("/api/github/agent-tasks/report", s.handleGitHubAgentTaskReport)
	mux.HandleFunc("/api/github/context", s.handleGitHubContext)
	return mux
}

// Start starts listening and serving HTTP requests.
func (s *Server) Start() error {
	mux := s.setupRoutes()

	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.addr = l.Addr().String()
	if onServerStartHook != nil {
		onServerStartHook(s.addr, s.GetSessionToken())
	}
	s.startLanguageServerURLSync()

	s.httpServer = &http.Server{
		Handler:     s.cloudCodeDispatch(s.corsMiddleware(s.authMiddleware(maxBytesMiddleware(mux, 10*1024*1024)))),
		ReadTimeout: 30 * time.Second,
		// Model turns stream for minutes; WriteTimeout is an absolute deadline in
		// net/http, so any non-zero value severs a long SSE response mid-flight.
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	go s.httpServer.Serve(l)
	return nil
}

// Stop gracefully shuts down the web server.
func (s *Server) Stop() error {
	if s.syncLSURLStop != nil {
		select {
		case <-s.syncLSURLStop:
		default:
			close(s.syncLSURLStop)
		}
	}
	if onServerStopHook != nil && s.addr != "" {
		onServerStopHook(s.addr)
	}
	s.testIDEsMu.Lock()
	pids := s.testIDEs
	s.testIDEs = nil
	s.testIDEsMu.Unlock()
	for _, pid := range pids {
		_ = core.KillProcessGroup(pid)
		if proc, errProc := os.FindProcess(pid); errProc == nil {
			_ = proc.Kill()
		}
	}
	if core.IsRunningTests() || gui.IsTestExecution() {
		if out, err := exec.Command("pgrep", "-af", "transient").Output(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "transient") && strings.Contains(line, "code") {
					fields := strings.Fields(line)
					if len(fields) > 0 {
						if p, err := strconv.Atoi(fields[0]); err == nil && p > 0 {
							_ = core.KillProcessGroup(p)
							if proc, errProc := os.FindProcess(p); errProc == nil {
								_ = proc.Kill()
							}
						}
					}
				}
			}
		}
	}
	if s.syncLSURLStop != nil {
		close(s.syncLSURLStop)
		s.syncLSURLStop = nil
	}
	var stopErrs []error
	if s.isProxyModeActive() {
		if err := custommodels.CleanShellRCHooks(); err != nil {
			log.Printf("[webgui] shell RC cleanup failed during stop: %v", err)
			stopErrs = append(stopErrs, fmt.Errorf("clean shell RC hooks during stop: %w", err))
		}
		if err := s.restoreNative().err(); err != nil {
			stopErrs = append(stopErrs, err)
		}
	}
	if s.bridgeService != nil {
		s.bridgeService.Stop()
	}
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			stopErrs = append(stopErrs, err)
		}
	}
	return errors.Join(stopErrs...)
}

func (s *Server) syncLanguageServerCloudCodeURL() {
	if s == nil || s.addr == "" || core.IsRunningTests() || gui.IsTestExecution() {
		return
	}
	endpoint := "http://" + loopbackAddr(s.addr)
	_ = languageServerURLSyncer(endpoint)
}

func (s *Server) isProxyModeActive() bool {
	if s.customModelsStore != nil && s.customModelsStore.IsProxyModeActive() {
		return true
	}
	if s.syncLSURLStop != nil {
		return true
	}
	return false
}

func (s *Server) startLanguageServerURLSync() {
	if core.IsRunningTests() || gui.IsTestExecution() {
		return
	}
	s.syncLanguageServerCloudCodeURL()
	s.syncLSURLStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.syncLSURLStop:
				return
			case <-ticker.C:
				s.syncLanguageServerCloudCodeURL()
			}
		}
	}()
}

// Addr returns the network address the server is listening on.
func (s *Server) Addr() string {
	return s.addr
}

func maxBytesMiddleware(next http.Handler, maxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

var loopbackOriginRegex = regexp.MustCompile(`^https?://(127\.0\.0\.1|localhost|\[::1\])(:[0-9]+)?$`)

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			addVaryHeader(w.Header(), "Origin")
			if !isAllowedLoopbackOrigin(origin) && !isExternalOriginAllowedPath(r.URL.Path) {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				http.Error(w, "Forbidden origin", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if isAllowedLoopbackOrigin(origin) {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Origin, Accept, Access-Control-Request-Private-Network, X-Bridge-Token, X-Pairing-PIN, X-App-Password, X-Session-Token, Mcp-Session-Id")
			w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
			w.Header().Set("Access-Control-Allow-Private-Network", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isExternalOriginAllowedPath(p string) bool {
	clean := path.Clean(p)
	if clean == "/oauth" || strings.HasPrefix(clean, "/oauth/") {
		return true
	}
	if strings.HasPrefix(clean, "/.well-known/") {
		return true
	}
	if strings.HasPrefix(clean, "/api/bridge/oauth/") {
		return true
	}
	if clean == "/api/bridge/mcp" || strings.HasPrefix(clean, "/api/bridge/mcp/") {
		return true
	}
	if clean == "/mcp" || strings.HasPrefix(clean, "/mcp/") {
		return true
	}
	if clean == "/agent.json" || clean == "/a2a/agent.json" || clean == "/api/bridge/a2a/agent.json" {
		return true
	}
	if clean == "/a2a" || strings.HasPrefix(clean, "/a2a/") {
		return true
	}
	if clean == "/api/bridge/a2a" || clean == "/api/bridge/a2a/rpc" || strings.HasPrefix(clean, "/api/bridge/a2a/tasks") {
		return true
	}
	if clean == "/a2a/tasks" || strings.HasPrefix(clean, "/a2a/tasks/") || clean == "/api/bridge/a2a/tasks" || strings.HasPrefix(clean, "/api/bridge/a2a/tasks/") {
		return true
	}
	if clean == "/remote" || strings.HasPrefix(clean, "/remote/") {
		return true
	}
	if isPublicIconPath(clean) {
		return true
	}
	if clean == "/api/bridge/channels/webhook" || strings.HasPrefix(clean, "/api/bridge/channels/webhook/") || clean == "/api/bridge/channels/telegram/webhook" || clean == "/api/bridge/channels/discord/webhook" {
		return true
	}
	return false
}

func isAllowedLoopbackOrigin(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" || origin == "null" || origin == "*" {
		return false
	}
	lower := strings.ToLower(origin)
	if lower == "file://" || (strings.HasPrefix(lower, "file:///") && !strings.HasPrefix(lower, "file:////")) {
		return true
	}
	for _, scheme := range []string{
		"vscode-file://",
		"electron://",
		"vscode-app://",
		"vscode-webview://",
		"antigravity://",
		"app://",
		"devtools://",
		"chrome-extension://",
		"plugin://",
	} {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}
	return loopbackOriginRegex.MatchString(lower)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.loadStatus()
	if err != nil {
		writeAccountBackendError(w, err)
		return
	}
	if accObj, ok := status["running_antigravity_account"].(map[string]interface{}); ok {
		delete(accObj, "access_token")
		delete(accObj, "refresh_token")
		delete(accObj, "id_token")
	}
	writeJSON(w, status)
}

func (s *Server) loadStatus() (map[string]interface{}, error) {
	if s.backend != nil {
		if active := s.backend.ActiveState(); active != nil {
			return activeStateStatusPayload(active.Snapshot(false)), nil
		}
	}
	if service := s.accountService(); service != nil {
		accounts := service.ListAccounts()
		return statusPayload(activeAccountEmail(accounts), len(accounts), true), nil
	}
	var status map[string]interface{}
	if err := s.callAccountOwnerRPC("get status", "swiss.getStatus", nil, &status); err != nil {
		return nil, err
	}
	return status, nil
}

func statusPayload(active string, total int, daemonRunning bool) map[string]interface{} {
	shield := process.NewShield(0)
	procs, _ := shield.FindAntigravityUIProcesses()
	hostRunning := len(procs) > 0
	var hostPID int
	if hostRunning {
		hostPID = procs[0].PID
	}
	daemonPID := 0
	if daemonRunning {
		daemonPID = os.Getpid()
	}
	return map[string]interface{}{
		"daemon_running":              daemonRunning,
		"daemon_pid":                  daemonPID,
		"version":                     core.AppVersion,
		"active_account":              active,
		"total_accounts":              total,
		"antigravity_running":         hostRunning,
		"antigravity_pid":             hostPID,
		"running_antigravity_account": keyring.ResolveRunningAntigravityAccountRedacted("", ""),
	}
}

func redactAccount(acc keyring.Account) keyring.Account {
	r := acc
	if r.Password != "" {
		r.Password = "[REDACTED]"
	}
	if r.TOTPSecret != "" {
		r.TOTPSecret = "[REDACTED]"
	}
	if r.AccessToken != "" {
		r.AccessToken = "[REDACTED]"
	}
	if r.RefreshToken != "" {
		r.RefreshToken = "[REDACTED]"
	}
	if r.IDToken != "" {
		r.IDToken = "[REDACTED]"
	}
	if r.Credential != nil {
		c := *r.Credential
		if c.AccessToken != "" {
			c.AccessToken = "[REDACTED]"
		}
		if c.RefreshToken != "" {
			c.RefreshToken = "[REDACTED]"
		}
		if c.IDToken != "" {
			c.IDToken = "[REDACTED]"
		}
		r.Credential = &c
	}
	return r
}

func (s *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut || (r.Method == http.MethodPost && r.URL.Query().Get("action") == "update") {
		s.handleAccountUpdate(w, r)
		return
	}
	accounts, err := s.accountsSnapshot()
	if err != nil {
		http.Error(w, "failed to list accounts: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	redacted := make([]keyring.Account, len(accounts))
	for i, account := range accounts {
		if account != nil {
			redacted[i] = redactAccount(*account)
		}
	}
	writeJSON(w, redacted)
}

func (s *Server) handleAccountsScan(w http.ResponseWriter, r *http.Request) {
	if service := s.accountService(); service != nil {
		accounts, err := service.ScanAccounts()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, accounts)
		return
	}
	var accounts []keyring.DiscoveredAccount
	if s.client == nil {
		http.Error(w, "account backend is not initialized", http.StatusServiceUnavailable)
		return
	}
	if err := s.client.Call("swiss.scanLocalAccounts", nil, &accounts); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, accounts)
}

func (s *Server) handleAccountsImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req backend.AccountImport
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "invalid request body, missing email", http.StatusBadRequest)
		return
	}
	if service := s.accountService(); service != nil {
		account, err := service.ImportAccount(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, account)
		return
	}
	if s.client == nil {
		http.Error(w, "account backend is not initialized", http.StatusServiceUnavailable)
		return
	}
	var result interface{}
	if err := s.client.Call("swiss.importAccount", req, &result); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleAccountUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req backend.AccountUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "invalid request body, missing email", http.StatusBadRequest)
		return
	}
	if service := s.accountService(); service != nil {
		result, err := service.UpdateAccount(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, result)
		return
	}
	if s.client == nil {
		http.Error(w, "account backend is not initialized", http.StatusServiceUnavailable)
		return
	}
	var result map[string]interface{}
	if err := s.client.Call("swiss.updateAccount", req, &result); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleAccountDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "invalid request body, missing email", http.StatusBadRequest)
		return
	}
	if service := s.accountService(); service != nil {
		result, err := service.RemoveAccount(req.Email)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, result)
		return
	}
	if s.client == nil {
		http.Error(w, "account backend is not initialized", http.StatusServiceUnavailable)
		return
	}
	var result map[string]interface{}
	if err := s.client.Call("swiss.removeAccount", req, &result); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleAccountsExport(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var exported []keyring.AccountExport
	if service := s.accountService(); service != nil {
		exported = service.ExportAccounts()
	} else {
		if s.client == nil {
			http.Error(w, "account backend is not initialized", http.StatusServiceUnavailable)
			return
		}
		if err := s.client.Call("swiss.exportAccounts", nil, &exported); err != nil {
			http.Error(w, "failed to export accounts: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	if email := strings.TrimSpace(r.URL.Query().Get("email")); email != "" {
		filtered := make([]keyring.AccountExport, 0, len(exported))
		for _, item := range exported {
			if strings.EqualFold(item.Email, email) {
				filtered = append(filtered, item)
			}
		}
		exported = filtered
	}
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", "attachment; filename=\"antigravity_accounts.json\"")
	}
	writeJSON(w, exported)
}

func (s *Server) handleAccountsBatchImport(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "empty request body", http.StatusBadRequest)
		return
	}
	var items []keyring.BatchImportItem
	if err := json.Unmarshal(body, &items); err != nil {
		var wrapper struct {
			Accounts json.RawMessage `json:"accounts"`
		}
		if err2 := json.Unmarshal(body, &wrapper); err2 == nil && len(wrapper.Accounts) > 0 {
			if err3 := json.Unmarshal(wrapper.Accounts, &items); err3 != nil {
				var accountMap map[string]keyring.BatchImportItem
				if err4 := json.Unmarshal(wrapper.Accounts, &accountMap); err4 == nil {
					for email, item := range accountMap {
						if item.Email == "" && item.ID == "" {
							item.Email = email
						}
						items = append(items, item)
					}
				}
			}
		} else {
			var single keyring.BatchImportItem
			if err5 := json.Unmarshal(body, &single); err5 == nil && (single.Email != "" || single.ID != "") {
				items = []keyring.BatchImportItem{single}
			}
		}
	}
	if len(items) == 0 {
		http.Error(w, "no valid account entries found in JSON", http.StatusBadRequest)
		return
	}
	if service := s.accountService(); service != nil {
		count, err := service.BatchImportAccounts(items)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "imported": count, "message": fmt.Sprintf("Successfully imported %d accounts", count)})
		return
	}
	if s.client == nil {
		http.Error(w, "account backend is not initialized", http.StatusServiceUnavailable)
		return
	}
	var result map[string]interface{}
	if err := s.client.Call("swiss.batchImportAccounts", items, &result); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, result)
}

// preSwitchAccountStatus resolves the target account's quota status
// ("STANDBY", "COOLING", "ERROR", "BANNED", "ACTIVE", ...) using the same
// computation as the fleet view. It must be called before a switch is
// executed, because the switch flips the account's active flag and therefore
// its status. The second return value reports whether the account was found.
func (s *Server) preSwitchAccountStatus(email string) (string, bool) {
	accounts, err := s.accountsSnapshot()
	if err != nil {
		return "", false
	}
	cfg, _ := core.LoadConfig()
	threshold5h := core.DefaultAutoSwitchThresholdFraction
	thresholdWeekly := core.DefaultAutoSwitchWeeklyThresholdFraction
	allowNonGemini := false
	if cfg != nil {
		if cfg.AutoSwitchThreshold > 0 {
			threshold5h = cfg.AutoSwitchThreshold
		}
		if cfg.AutoSwitchWeeklyThreshold > 0 {
			thresholdWeekly = cfg.AutoSwitchWeeklyThreshold
		}
		allowNonGemini = cfg.AllowNonGeminiNativeModels
	}
	states := quota.BuildAccountQuotaStatesFromMapWithThresholds(accounts, nil, threshold5h, thresholdWeekly, allowNonGemini)
	for _, state := range states {
		if strings.EqualFold(strings.TrimSpace(state.Email), strings.TrimSpace(email)) {
			return state.Status, true
		}
	}
	return "", false
}

func (s *Server) accountService() backend.AccountService {
	if s.backend == nil {
		return nil
	}
	return s.backend.Accounts()
}

func (s *Server) quotaService() backend.QuotaService {
	if s.backend == nil {
		return nil
	}
	return s.backend.Quota()
}

func (s *Server) guiService() backend.GUIService {
	if s.backend == nil {
		return nil
	}
	return s.backend.GUI()
}

type accountBackendError struct {
	operation string
	err       error
}

func (e *accountBackendError) Error() string {
	return fmt.Sprintf("account backend %s failed: %v", e.operation, e.err)
}

func (e *accountBackendError) Unwrap() error {
	return e.err
}

func (s *Server) callAccountOwnerRPC(operation, method string, params, out interface{}) error {
	if s.client == nil {
		return &accountBackendError{operation: operation, err: errors.New("daemon RPC client is not initialized")}
	}
	if err := s.client.Call(method, params, out); err != nil {
		return &accountBackendError{operation: operation, err: err}
	}
	return nil
}

func writeAccountBackendError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadGateway)
}

func activeAccountEmail(accounts []*keyring.Account) string {
	for _, account := range accounts {
		if account.IsActive {
			return account.Email
		}
	}
	return ""
}

func (s *Server) accountsSnapshot() ([]*keyring.Account, error) {
	if service := s.accountService(); service != nil {
		return service.ListAccounts(), nil
	}
	if s.client == nil {
		return nil, errors.New("account backend is not initialized")
	}
	var accounts []*keyring.Account
	if err := s.client.Call("swiss.listAccounts", nil, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

// disableAutoSwitchForManualPin implements the manual account switching
// invariant: pinning an account that is NOT in STANDBY status turns
// auto-switch off so the rotation daemon does not immediately switch away
// from the manually selected account. Targets that ARE in STANDBY leave
// auto-switch untouched. It returns true when auto-switch was disabled.
func (s *Server) disableAutoSwitchForManualPin(email string) bool {
	status, found := s.preSwitchAccountStatus(email)
	if !found || strings.EqualFold(status, core.AccountStatusStandby) {
		return false
	}
	cfg, err := core.LoadConfig()
	if err != nil || cfg == nil || !cfg.AutoSwitchEnabled {
		return false
	}
	var res map[string]interface{}
	if s.client != nil && s.client.Call("swiss.setRuleConfig", map[string]interface{}{"auto_switch_enabled": false}, &res) == nil {
		return true
	}
	c, errCfg := core.LoadConfig()
	if errCfg != nil {
		return false
	}
	c.AutoSwitchEnabled = false
	_ = keyring.SyncCloudAccountsAutoSwitch("", false)
	if err := c.Save(); err != nil {
		return false
	}
	return true
}

func (s *Server) handleSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req backend.AccountSwitch
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}
	if !s.isSwitching.CompareAndSwap(false, true) {
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   "Account switch already in progress. Please wait for current operation to complete.",
		})
		return
	}
	defer s.isSwitching.Store(false)

	autoSwitchDisabled := s.disableAutoSwitchForManualPin(req.Email)
	var result backend.AccountSwitchResult
	var err error
	if service := s.accountService(); service != nil {
		result, err = service.SwitchAccount(req)
	} else if s.client != nil {
		err = s.client.Call("swiss.switchAccount", req, &result)
	} else {
		err = errors.New("account backend is not initialized")
	}
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, core.ErrAccountNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	if autoSwitchDisabled {
		result.AutoSwitchDisabled = true
	}
	writeJSON(w, result)
}

func (s *Server) handleTOTP(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "unauthorized",
			"message": "Authentication or app password required to access TOTP verification codes",
		})
		return
	}
	email := r.URL.Query().Get("email")
	if r.Method == http.MethodPost {
		var p struct {
			Email  string `json:"email"`
			Secret string `json:"secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		var res map[string]interface{}
		if service := s.accountService(); service != nil {
			if err := service.SetTOTPSecret(p.Email, p.Secret); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			res = map[string]interface{}{"success": true}
		} else if err := s.callAccountOwnerRPC("set TOTP secret", "swiss.setTOTPSecret", p, &res); err != nil {
			writeAccountBackendError(w, err)
			return
		}
		writeJSON(w, res)
		return
	}

	if service := s.accountService(); service != nil {
		target := email
		if target == "" {
			target = activeAccountEmail(service.ListAccounts())
		}
		acc, err := service.GetAccount(target)
		if err != nil && !errors.Is(err, core.ErrAccountNotFound) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if acc == nil || acc.TOTPSecret == "" {
			writeJSON(w, map[string]interface{}{
				"code":              "------",
				"remaining_seconds": 0,
				"has_totp":          false,
			})
			return
		}
		eng := totp.NewEngine()
		now := time.Now()
		code, _ := eng.GenerateCode(acc.TOTPSecret, now)
		cd := eng.GetCountdown(now)
		writeJSON(w, map[string]interface{}{
			"code":              code,
			"remaining_seconds": cd.RemainingSeconds,
			"progress_fraction": cd.ProgressFraction,
			"has_totp":          true,
		})
		return
	}

	var res map[string]interface{}
	if err := s.callAccountOwnerRPC("get TOTP code", "swiss.getTOTPCode", map[string]string{"email": email}, &res); err != nil {
		writeAccountBackendError(w, err)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleFingerprint(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if r.Method == http.MethodPost {
		var raw struct {
			Email            string                     `json:"email"`
			AccountEmail     string                     `json:"account_email"`
			Profile          *fingerprint.DeviceProfile `json:"profile"`
			MachineID        string                     `json:"machine_id"`
			UpdaterID        string                     `json:"updater_id"`
			InstallationID   string                     `json:"installation_id"`
			InstallationUUID string                     `json:"installation_uuid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		targetEmail := strings.TrimSpace(raw.Email)
		if targetEmail == "" {
			targetEmail = strings.TrimSpace(raw.AccountEmail)
		}
		var prof fingerprint.DeviceProfile
		if raw.Profile != nil {
			prof = *raw.Profile
			if targetEmail == "" {
				targetEmail = strings.TrimSpace(prof.AccountEmail)
			}
		} else {
			prof = fingerprint.DeviceProfile{
				AccountEmail:     targetEmail,
				MachineID:        strings.TrimSpace(raw.MachineID),
				UpdaterID:        strings.TrimSpace(raw.UpdaterID),
				InstallationID:   strings.TrimSpace(raw.InstallationID),
				InstallationUUID: strings.TrimSpace(raw.InstallationUUID),
			}
		}
		if targetEmail == "" {
			http.Error(w, "missing account email", http.StatusBadRequest)
			return
		}
		prof.AccountEmail = targetEmail

		reqPayload := map[string]interface{}{
			"email":   targetEmail,
			"profile": prof,
		}
		var res map[string]interface{}
		if err := s.client.Call("swiss.setFingerprofile", reqPayload, &res); err != nil {
			fpStore, errStore := fingerprint.NewStore("")
			if errStore != nil {
				http.Error(w, errStore.Error(), http.StatusInternalServerError)
				return
			}
			if err := fpStore.SetProfile(targetEmail, &prof); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			kStore, _ := keyring.NewStore("")
			if kStore != nil && strings.EqualFold(targetEmail, kStore.ActiveAccount()) {
				_ = keyring.SyncHardwareProfile(targetEmail, fpStore)
			}
			res = map[string]interface{}{"success": true}
		}
		writeJSON(w, res)
		return
	}

	if r.URL.Query().Get("action") == "generate" {
		fresh, err := fingerprint.GenerateRandom()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, fresh)
		return
	}

	if r.URL.Query().Get("list") == "true" {
		var list []fingerprint.DeviceProfile
		if err := s.client.Call("swiss.listFingerprofiles", map[string]interface{}{}, &list); err != nil {
			fpStore, errStore := fingerprint.NewStore("")
			if errStore != nil {
				http.Error(w, errStore.Error(), http.StatusInternalServerError)
				return
			}
			if kStore, errK := keyring.NewStore(""); errK == nil && kStore != nil {
				for _, acc := range kStore.ListAccounts() {
					if acc.Email != "" {
						_, _ = fpStore.GetOrCreateProfile(acc.Email)
					}
				}
			}
			writeJSON(w, fpStore.ListProfilesSlice())
			return
		}
		writeJSON(w, list)
		return
	}

	var prof fingerprint.DeviceProfile
	if err := s.client.Call("swiss.getFingerprofile", map[string]string{"email": email}, &prof); err != nil {
		fpStore, errStore := fingerprint.NewStore("")
		if errStore == nil {
			target := email
			if target == "" {
				kStore, _ := keyring.NewStore("")
				if kStore != nil {
					target = kStore.ActiveAccount()
				}
			}
			p, _ := fpStore.GetOrCreateProfile(target)
			if p != nil {
				writeJSON(w, p)
				return
			}
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, prof)
}

func (s *Server) handleCacheScan(w http.ResponseWriter, r *http.Request) {
	ageStr := r.URL.Query().Get("min_age_days")
	if ageStr == "" {
		ageStr = r.URL.Query().Get("days")
	}
	age := 0.0
	if ageStr != "" {
		if parsed, err := strconv.ParseFloat(ageStr, 64); err == nil && parsed >= 0 {
			age = parsed
		}
	}

	maxSizeGB := 0.0
	if sizeStr := r.URL.Query().Get("max_size_gb"); sizeStr != "" {
		if parsed, err := strconv.ParseFloat(sizeStr, 64); err == nil && parsed >= 0 {
			maxSizeGB = parsed
		}
	}

	var bd cache.Breakdown
	if err := s.client.Call("swiss.scanCache", map[string]float64{"min_age_days": age, "max_size_gb": maxSizeGB}, &bd); err != nil {
		ins := cache.NewInspector("", "")
		maxBytes := int64(maxSizeGB * 1024 * 1024 * 1024)
		scan, errScan := ins.ScanBreakdownWithLimit(age, maxBytes)
		if errScan != nil {
			http.Error(w, errScan.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, scan)
		return
	}
	writeJSON(w, bd)
}

func (s *Server) handleCachePrune(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var raw struct {
		MinAgeDays      *float64 `json:"min_age_days"`
		OlderThanDays   *float64 `json:"older_than_days"`
		MaxSizeGB       *float64 `json:"max_size_gb"`
		PruneScratch    bool     `json:"prune_scratch"`
		PruneSteps      bool     `json:"prune_steps"`
		PruneTasks      bool     `json:"prune_tasks"`
		DryRun          bool     `json:"dry_run"`
		ActiveCascadeID string   `json:"active_cascade_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&raw)

	age := 0.0
	if raw.MinAgeDays != nil {
		age = *raw.MinAgeDays
	} else if raw.OlderThanDays != nil {
		age = *raw.OlderThanDays
	}

	maxSizeGB := 0.0
	if raw.MaxSizeGB != nil && *raw.MaxSizeGB >= 0 {
		maxSizeGB = *raw.MaxSizeGB
	}

	opts := cache.PruneOptions{
		MinAgeDays:      age,
		MaxSizeGB:       maxSizeGB,
		PruneScratch:    raw.PruneScratch,
		PruneSteps:      raw.PruneSteps,
		PruneTasks:      raw.PruneTasks,
		DryRun:          raw.DryRun,
		ActiveCascadeID: raw.ActiveCascadeID,
	}

	var res cache.PruneResult
	if err := s.client.Call("swiss.pruneCache", opts, &res); err != nil {
		pruner := cache.NewPruner("")
		pruneRes, errPrune := pruner.Prune(opts)
		if errPrune != nil {
			http.Error(w, errPrune.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, pruneRes)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleCacheConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		var res map[string]interface{}
		if err := s.client.Call("swiss.getCacheConfig", nil, &res); err == nil && res != nil {
			writeJSON(w, res)
			return
		}
		cfg, _ := core.LoadConfig()
		if cfg == nil {
			cfg = core.DefaultConfig()
		}
		maxSize := cfg.AutoPruneMaxSizeGB
		if maxSize < 0 {
			maxSize = 0.0
		}
		writeJSON(w, map[string]interface{}{
			"auto_prune_enabled": cfg.AutoPruneEnabled,
			"prune_days":         cfg.AutoPruneMaxAgeDays,
			"max_size_gb":        maxSize,
		})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var p struct {
		AutoPruneEnabled *bool    `json:"auto_prune_enabled"`
		PruneDays        *float64 `json:"prune_days"`
		MaxSizeGB        *float64 `json:"max_size_gb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var res map[string]interface{}
	if err := s.client.Call("swiss.setCacheConfig", p, &res); err == nil && res != nil {
		writeJSON(w, res)
		return
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.DefaultConfig()
	}
	if p.AutoPruneEnabled != nil {
		cfg.AutoPruneEnabled = *p.AutoPruneEnabled
	}
	if p.PruneDays != nil && *p.PruneDays >= 0 {
		cfg.AutoPruneMaxAgeDays = *p.PruneDays
	}
	if p.MaxSizeGB != nil && *p.MaxSizeGB >= 0 {
		cfg.AutoPruneMaxSizeGB = *p.MaxSizeGB
	}
	if cfg.AutoPruneMaxSizeGB < 0 {
		cfg.AutoPruneMaxSizeGB = 0.0
	}
	_ = cfg.Save()
	writeJSON(w, map[string]interface{}{
		"success":            true,
		"auto_prune_enabled": cfg.AutoPruneEnabled,
		"prune_days":         cfg.AutoPruneMaxAgeDays,
		"max_size_gb":        cfg.AutoPruneMaxSizeGB,
	})
}

func (s *Server) handleQuota(w http.ResponseWriter, r *http.Request) {
	reqEmail := strings.TrimSpace(r.URL.Query().Get("email"))
	refreshParam := r.URL.Query().Get("refresh") == "true" || r.URL.Query().Get("refresh") == "1"
	if service := s.quotaService(); service != nil {
		summary, err := service.GetSummary(reqEmail, refreshParam)
		if err != nil {
			http.Error(w, fmt.Sprintf("quota backend summary failed: %v", err), http.StatusBadGateway)
			return
		}
		writeJSON(w, *summary)
		return
	}

	var q quota.QuotaSummary
	params := map[string]interface{}{}
	if reqEmail != "" {
		params["email"] = reqEmail
	}
	if refreshParam {
		params["refresh"] = true
	}
	if err := s.client.Call("swiss.getQuotaSummary", params, &q); err != nil {
		http.Error(w, fmt.Sprintf("quota RPC summary failed: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, q)
}

func (s *Server) handleFleetQuota(w http.ResponseWriter, r *http.Request) {
	var summary quota.FleetQuotaSummary
	if service := s.quotaService(); service != nil {
		summary = *service.GetFleet()
	} else if err := s.client.Call("swiss.getFleetQuota", nil, &summary); err != nil {
		http.Error(w, fmt.Sprintf("quota RPC fleet failed: %v", err), http.StatusBadGateway)
		return
	}
	for i := range summary.Accounts {
		summary.Accounts[i].Password = ""
		summary.Accounts[i].TOTPSecret = ""
		summary.Accounts[i].AccessToken = ""
		if strings.TrimSpace(summary.Accounts[i].RefreshToken) != "" {
			summary.Accounts[i].RefreshToken = "[REDACTED]"
		}
	}
	writeJSON(w, summary)
}

func (s *Server) handleQuotaRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if service := s.quotaService(); service != nil {
		if err := service.RefreshFleet(); err != nil {
			http.Error(w, fmt.Sprintf("quota backend refresh failed: %v", err), http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "refresh_started"})
		return
	}

	var res map[string]interface{}
	if err := s.client.Call("swiss.refreshFleetQuota", nil, &res); err != nil {
		http.Error(w, fmt.Sprintf("quota RPC refresh failed: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, res)
}

// isQuotaPolling reports whether the standalone (daemonless) fleet poll is running.
func (s *Server) isQuotaPolling() bool {
	s.quotaPollMu.Lock()
	defer s.quotaPollMu.Unlock()
	return s.quotaPolling
}

// pollFleetFallback runs a fleet quota poll when the daemon IPC client is unavailable,
// tracking progress so /api/quota/fleet can report refreshing=true meanwhile.
func (s *Server) pollFleetFallback(accounts []*keyring.Account, store *keyring.Store) {
	s.quotaPollMu.Lock()
	if s.quotaPolling {
		s.quotaPollMu.Unlock()
		return
	}
	s.quotaPolling = true
	s.quotaPollMu.Unlock()
	go func() {
		defer func() {
			s.quotaPollMu.Lock()
			s.quotaPolling = false
			s.quotaPollMu.Unlock()
		}()
		_ = quota.PollFleetAccounts(accounts, store)
	}()
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var p map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		var res map[string]interface{}
		if err := s.client.Call("swiss.setRuleConfig", p, &res); err != nil {
			c, errCfg := core.LoadConfig()
			if errCfg != nil {
				http.Error(w, errCfg.Error(), http.StatusInternalServerError)
				return
			}
			if val, ok := p["auto_switch_enabled"].(bool); ok {
				c.AutoSwitchEnabled = val
				_ = keyring.SyncCloudAccountsAutoSwitch("", val)
			}
			if val, ok := p["auto_switch_threshold"].(float64); ok {
				c.AutoSwitchThreshold = val
			}
			if val, ok := p["auto_switch_weekly_threshold"].(float64); ok {
				c.AutoSwitchWeeklyThreshold = val
			}
			if val, ok := p["switch_mode"].(string); ok {
				c.SwitchMode = quota.NormalizeSwitchMode(val)
			}
			if val, ok := p["quota_refresh_mode"].(string); ok {
				c.QuotaRefreshMode = core.NormalizeQuotaRefreshMode(val)
				c.DynamicQuotaRefreshEnabled = (c.QuotaRefreshMode == core.QuotaRefreshModeDynamic)
			}
			if val, ok := p["dynamic_quota_refresh_enabled"].(bool); ok {
				c.DynamicQuotaRefreshEnabled = val
				if val {
					c.QuotaRefreshMode = core.QuotaRefreshModeDynamic
				} else {
					c.QuotaRefreshMode = core.QuotaRefreshModeManual
				}
			}
			if val, ok := p["polling_interval_seconds"].(float64); ok {
				c.PollingIntervalSec = int(val)
			}
			if val, ok := p["active_polling_interval_seconds"].(float64); ok {
				c.ActivePollingIntervalSec = int(val)
			}
			if val, ok := p["standby_polling_interval_seconds"].(float64); ok {
				c.StandbyPollingIntervalSec = int(val)
			}
			if val, ok := p["standby_random_jitter_seconds"].(float64); ok {
				c.StandbyRandomJitterSec = int(val)
			}
			if val, ok := p["warmup_enabled"].(bool); ok {
				c.WarmupEnabled = val
			}
			if val, ok := p["post_reset_delay_seconds"].(float64); ok {
				c.PostResetDelaySec = val
				c.WarmupLeadTimeSec = val
			} else if val, ok := p["warmup_lead_time_seconds"].(float64); ok {
				c.WarmupLeadTimeSec = val
				c.PostResetDelaySec = val
			}
			if val, ok := p["preferred_native_model"].(string); ok {
				c.PreferredNativeModel = val
			}
			if val, ok := p["allow_ai_credits_usage"].(bool); ok {
				c.AllowAICreditsUsage = val
			}
			if val, ok := p["allow_non_gemini_native_models"].(bool); ok {
				c.AllowNonGeminiNativeModels = val
			}
			if val, ok := p["model_source_hierarchy"].([]interface{}); ok {
				var list []string
				for _, item := range val {
					if s, ok := item.(string); ok {
						list = append(list, s)
					}
				}
				c.ModelSourceHierarchy = list
			}
			if val, ok := p["default_gemini_model"].(string); ok {
				c.DefaultGeminiModel = val
				if _, hasExplicitLvl := p["default_gemini_reasoning_level"]; !hasExplicitLvl {
					lowVal := strings.ToLower(strings.TrimSpace(val))
					switch {
					case strings.HasSuffix(lowVal, "-low"):
						c.DefaultGeminiReasoningLevel = "low"
					case strings.HasSuffix(lowVal, "-medium"):
						c.DefaultGeminiReasoningLevel = "medium"
					case strings.HasSuffix(lowVal, "-off"):
						c.DefaultGeminiReasoningLevel = "off"
					case strings.HasSuffix(lowVal, "-high") || lowVal == "gemini-pro-agent":
						c.DefaultGeminiReasoningLevel = "high"
					}
				}
			}
			if val, ok := p["default_custom_model"].(string); ok {
				// Refuse to promote a custom model whose latest connectivity test failed.
				if val != "" && s.getCustomModelsStore() != nil {
					if target, findErr := s.customModelsStore.FindModelByIdOrName(val); findErr == nil && target != nil {
						if target.LastTestSuccess != nil && !*target.LastTestSuccess {
							http.Error(w, fmt.Sprintf("cannot set %q as the default custom model: its last connectivity test failed", val), http.StatusBadRequest)
							return
						}
					}
				}
				c.DefaultCustomModel = val
				if val == "" && s.getCustomModelsStore() != nil {
					_ = s.getCustomModelsStore().ClearDefaultFlags()
				}
			}
			if val, ok := p["default_non_gemini_model"].(string); ok {
				c.DefaultNonGeminiModel = val
			}
			if val, ok := p["default_gemini_reasoning_level"].(string); ok {
				c.DefaultGeminiReasoningLevel = val
			}
			if val, ok := p["auto_import_active_account"].(bool); ok {
				c.AutoImportActiveAccount = val
			}
			if val, ok := p["multi_app_sync_mode"].(string); ok {
				c.MultiAppSyncMode = val
				if c.MultiAppSyncMode != core.MultiAppSyncModeIndividual {
					c.MultiAppSyncMode = core.MultiAppSyncModeShared
				}
			}
			if val, ok := p["active_app_accounts"].(map[string]interface{}); ok {
				if c.ActiveAppAccounts == nil {
					c.ActiveAppAccounts = make(map[string]string)
				}
				for k, v := range val {
					if s, ok := v.(string); ok {
						c.ActiveAppAccounts[k] = s
					}
				}
			}
			if val, ok := p["subagent_custom_models_enabled"].(bool); ok {
				c.SubagentCustomModelsEnabled = val
			}
			if val, ok := p["subagent_model_strategy"].(string); ok {
				c.SubagentModelStrategy = core.NormalizeSubagentModelStrategy(val)
			}
			if val, ok := p["session_resume_prompt"].(string); ok {
				c.SessionResumePrompt = strings.TrimSpace(val)
			}
			if err := c.Save(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			res = map[string]interface{}{"success": true}
		}
		writeJSON(w, res)
		return
	}

	inst := s.getSystemDetector().DetectAll()
	installedMap := map[string]bool{
		"desktop": inst != nil && inst.DesktopApp.Installed,
		"agy":     inst != nil && inst.AgyCLI.Installed,
		"vscode":  inst != nil && inst.VSCodeExtension.Installed,
	}

	var cfg map[string]interface{}
	if err := s.client.Call("swiss.getRuleConfig", nil, &cfg); err != nil {
		c, _ := core.LoadConfig()
		geminiReasoning := "high"
		defaultGemini := "gemini-3.8-flash-high"
		defaultNonGemini := "claude-opus-4-6-thinking"
		if c != nil {
			if c.DefaultGeminiReasoningLevel != "" {
				geminiReasoning = c.DefaultGeminiReasoningLevel
			}
			if c.DefaultGeminiModel != "" {
				defaultGemini = c.DefaultGeminiModel
			}
			if c.DefaultNonGeminiModel != "" {
				defaultNonGemini = c.DefaultNonGeminiModel
			}
		}
		switchMode := core.DefaultSwitchMode
		if c != nil && c.SwitchMode != "" {
			switchMode = quota.NormalizeSwitchMode(c.SwitchMode)
		}
		threshWeekly := core.DefaultAutoSwitchWeeklyThresholdFraction
		if c != nil && c.AutoSwitchWeeklyThreshold > 0 {
			threshWeekly = c.AutoSwitchWeeklyThreshold
		}
		syncMode := core.MultiAppSyncModeShared
		activeApps := make(map[string]string)
		subagentStrategy := core.DefaultSubagentModelStrategy
		if c != nil {
			syncMode = c.GetMultiAppSyncMode()
			activeApps = c.GetActiveAppAccounts()
			subagentStrategy = c.GetSubagentModelStrategy()
		}
		cfg = map[string]interface{}{
			"auto_switch_enabled":              c.AutoSwitchEnabled,
			"auto_switch_threshold":            c.AutoSwitchThreshold,
			"auto_switch_weekly_threshold":     threshWeekly,
			"switch_mode":                      switchMode,
			"quota_refresh_mode":               c.GetQuotaRefreshMode(),
			"dynamic_quota_refresh_enabled":    c.IsDynamicQuotaRefreshEnabled(),
			"polling_interval_seconds":         c.PollingIntervalSec,
			"active_polling_interval_seconds":  c.ActivePollingIntervalSec,
			"standby_polling_interval_seconds": c.StandbyPollingIntervalSec,
			"standby_random_jitter_seconds":    c.StandbyRandomJitterSec,
			"warmup_enabled":                   c.WarmupEnabled,
			"warmup_lead_time_seconds":         c.GetPostResetDelaySec(),
			"post_reset_delay_seconds":         c.GetPostResetDelaySec(),
			"preferred_native_model":           c.PreferredNativeModel,
			"allow_ai_credits_usage":           c.AllowAICreditsUsage,
			"allow_non_gemini_native_models":   c.AllowNonGeminiNativeModels,
			"model_source_hierarchy":           c.ModelSourceHierarchy,
			"default_gemini_model":             defaultGemini,
			"default_custom_model":             c.DefaultCustomModel,
			"default_non_gemini_model":         defaultNonGemini,
			"default_gemini_reasoning_level":   geminiReasoning,
			"auto_import_active_account":       c.AutoImportActiveAccount,
			"multi_app_sync_mode":              syncMode,
			"active_app_accounts":              activeApps,
			"subagent_custom_models_enabled":   c.GetSubagentCustomModelsEnabled(),
			"subagent_model_strategy":          subagentStrategy,
			"session_resume_prompt":            c.GetSessionResumePrompt(),
			"installed_apps":                   installedMap,
		}
	} else if cfg != nil {
		if sm, ok := cfg["switch_mode"].(string); !ok || sm == "" {
			cfg["switch_mode"] = core.DefaultSwitchMode
		} else {
			cfg["switch_mode"] = quota.NormalizeSwitchMode(sm)
		}
		if _, ok := cfg["auto_switch_weekly_threshold"]; !ok {
			c, _ := core.LoadConfig()
			if c != nil && c.AutoSwitchWeeklyThreshold > 0 {
				cfg["auto_switch_weekly_threshold"] = c.AutoSwitchWeeklyThreshold
			} else {
				cfg["auto_switch_weekly_threshold"] = core.DefaultAutoSwitchWeeklyThresholdFraction
			}
		}
		if _, ok := cfg["default_gemini_reasoning_level"]; !ok {
			c, _ := core.LoadConfig()
			if c != nil && c.DefaultGeminiReasoningLevel != "" {
				cfg["default_gemini_reasoning_level"] = c.DefaultGeminiReasoningLevel
			} else {
				cfg["default_gemini_reasoning_level"] = "high"
			}
		}
		if gm, ok := cfg["default_gemini_model"].(string); !ok || gm == "" {
			cfg["default_gemini_model"] = "gemini-3.8-flash-high"
		}
		if ngm, ok := cfg["default_non_gemini_model"].(string); !ok || ngm == "" {
			cfg["default_non_gemini_model"] = "claude-opus-4-6-thinking"
		}
		if _, ok := cfg["installed_apps"]; !ok {
			cfg["installed_apps"] = installedMap
		}
		if _, ok := cfg["multi_app_sync_mode"]; !ok {
			c, _ := core.LoadConfig()
			if c != nil {
				cfg["multi_app_sync_mode"] = c.GetMultiAppSyncMode()
				cfg["active_app_accounts"] = c.GetActiveAppAccounts()
			} else {
				cfg["multi_app_sync_mode"] = core.MultiAppSyncModeShared
				cfg["active_app_accounts"] = make(map[string]string)
			}
		}
		if _, ok := cfg["subagent_model_strategy"]; !ok {
			c, _ := core.LoadConfig()
			if c != nil {
				cfg["subagent_model_strategy"] = c.GetSubagentModelStrategy()
			} else {
				cfg["subagent_model_strategy"] = core.DefaultSubagentModelStrategy
			}
		}
		if _, ok := cfg["subagent_custom_models_enabled"]; !ok {
			c, _ := core.LoadConfig()
			if c != nil {
				cfg["subagent_custom_models_enabled"] = c.GetSubagentCustomModelsEnabled()
			} else {
				cfg["subagent_custom_models_enabled"] = false
			}
		}
		if _, ok := cfg["session_resume_prompt"]; !ok {
			c, _ := core.LoadConfig()
			if c != nil {
				cfg["session_resume_prompt"] = c.GetSessionResumePrompt()
			} else {
				cfg["session_resume_prompt"] = core.DefaultSessionResumePrompt
			}
		}
	}
	writeJSON(w, cfg)
}

func (s *Server) handleAvailableModels(w http.ResponseWriter, r *http.Request) {
	store, _ := keyring.NewStore("")
	var accounts []*keyring.Account
	var acc *keyring.Account
	if store != nil {
		accounts = store.ListAccounts()
		active := store.ActiveAccount()
		if active != "" {
			acc, _ = store.GetAccount(active)
		}
	}
	force := r.URL.Query().Get("force") == "true" || r.URL.Query().Get("refresh") == "true"
	catalog := quota.GetAvailableModelCatalogFleet(accounts, acc, force)
	writeJSON(w, catalog)
}

func (s *Server) handleSurfaces(w http.ResponseWriter, r *http.Request) {
	var res map[string]interface{}
	if err := s.client.Call("swiss.getSurfaces", nil, &res); err != nil {
		home, _ := os.UserHomeDir()
		configDir := filepath.Join(home, ".config", "Antigravity")
		if custom := os.Getenv("ANTIGRAVITY_CONFIG_DIR"); custom != "" {
			configDir = custom
		}
		res = map[string]interface{}{
			"surfaces":               keyring.DetectAllSurfacesRedacted(home, configDir),
			"active_surface_account": keyring.ResolveRunningAntigravityAccountRedacted(home, configDir),
			"priority_sequence":      []string{"Antigravity 2.0 Desktop", "Antigravity VS Code Extension", "Antigravity CLI"},
		}
	}
	if surfaces, ok := res["surfaces"].(map[string]interface{}); ok {
		for _, sVal := range surfaces {
			if sObj, ok := sVal.(map[string]interface{}); ok {
				delete(sObj, "access_token")
				delete(sObj, "refresh_token")
				delete(sObj, "id_token")
			}
		}
	}
	if activeAcc, ok := res["active_surface_account"].(map[string]interface{}); ok {
		delete(activeAcc, "access_token")
		delete(activeAcc, "refresh_token")
		delete(activeAcc, "id_token")
	}
	writeJSON(w, res)
}

func (s *Server) handleAutoSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		var cfg map[string]interface{}
		if err := s.client.Call("swiss.getRuleConfig", nil, &cfg); err != nil {
			c, _ := core.LoadConfig()
			writeJSON(w, map[string]interface{}{
				"auto_switch_enabled": c.AutoSwitchEnabled,
			})
			return
		}
		writeJSON(w, cfg)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	var res map[string]interface{}
	callPayload := map[string]interface{}{"auto_switch_enabled": p.Enabled}
	if err := s.client.Call("swiss.setRuleConfig", callPayload, &res); err != nil {
		c, errCfg := core.LoadConfig()
		if errCfg != nil {
			http.Error(w, errCfg.Error(), http.StatusInternalServerError)
			return
		}
		c.AutoSwitchEnabled = p.Enabled
		_ = keyring.SyncCloudAccountsAutoSwitch("", p.Enabled)
		if err := c.Save(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		res = map[string]interface{}{"success": true, "auto_switch_enabled": p.Enabled}
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIConfig(w http.ResponseWriter, r *http.Request) {
	if service := s.guiService(); service != nil {
		if r.Method == http.MethodPost {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			updated, err := service.UpdateConfigPartial(bodyBytes)
			if err != nil {
				http.Error(w, fmt.Sprintf("GUI backend config update failed: %v", err), http.StatusInternalServerError)
				return
			}
			writeJSON(w, updated)
			return
		}
		writeJSON(w, service.GetConfig())
		return
	}

	if r.Method == http.MethodPost {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var res gui.Config
		if err := s.client.Call("swiss.setGUIConfig", json.RawMessage(bodyBytes), &res); err != nil {
			http.Error(w, fmt.Sprintf("GUI RPC config update failed: %v", err), http.StatusBadGateway)
			return
		}
		writeJSON(w, res)
		return
	}

	var cfg gui.Config
	if err := s.client.Call("swiss.getGUIConfig", nil, &cfg); err != nil {
		http.Error(w, fmt.Sprintf("GUI RPC config failed: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, cfg)
}

func (s *Server) handleGUIProjects(w http.ResponseWriter, r *http.Request) {
	if service := s.guiService(); service != nil {
		projects, err := service.DetectProjects()
		if err != nil {
			http.Error(w, fmt.Sprintf("GUI backend projects failed: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, projects)
		return
	}

	var projects []gui.ProjectItem
	if err := s.client.Call("swiss.getGUIProjects", nil, &projects); err != nil {
		http.Error(w, fmt.Sprintf("GUI RPC projects failed: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, projects)
}

func (s *Server) handleGUIProjectColor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Name == "" || p.Color == "" {
		http.Error(w, "invalid parameters, name and color are required", http.StatusBadRequest)
		return
	}

	if service := s.guiService(); service != nil {
		if err := service.SetProjectColor(p.Name, p.Color); err != nil {
			http.Error(w, fmt.Sprintf("GUI backend project color failed: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "name": p.Name, "color": p.Color})
		return
	}
	var res interface{}
	if err := s.client.Call("swiss.setGUIProjectColor", p, &res); err != nil {
		http.Error(w, fmt.Sprintf("GUI RPC project color failed: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIProjectDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Name == "" {
		http.Error(w, "invalid parameters, name is required", http.StatusBadRequest)
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	if err := gui.ValidateProjectName(p.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if service := s.guiService(); service != nil {
		if err := service.DeleteProject(p.Name); err != nil {
			http.Error(w, fmt.Sprintf("GUI backend project delete failed: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "name": p.Name})
		return
	}

	var res interface{}
	if err := s.client.Call("swiss.deleteGUIProject", p, &res); err != nil {
		http.Error(w, fmt.Sprintf("GUI RPC project delete failed: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIProjectColorDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Name == "" {
		http.Error(w, "invalid parameters, name is required", http.StatusBadRequest)
		return
	}

	if service := s.guiService(); service != nil {
		if err := service.RemoveProjectColor(p.Name); err != nil {
			http.Error(w, fmt.Sprintf("GUI backend project color delete failed: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"success":          true,
			"name":             p.Name,
			"reset_to_factory": false,
			"color":            "",
		})
		return
	}

	var res interface{}
	if err := s.client.Call("swiss.removeGUIProjectColor", p, &res); err != nil {
		http.Error(w, fmt.Sprintf("GUI RPC project color delete failed: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]interface{}{
		"success":          true,
		"name":             p.Name,
		"reset_to_factory": false,
		"color":            "",
	})
}

func (s *Server) handleGUIProjectsArchived(w http.ResponseWriter, r *http.Request) {
	if service := s.guiService(); service != nil {
		archived, err := service.GetArchivedProjects()
		if err != nil {
			http.Error(w, fmt.Sprintf("GUI backend archived projects failed: %v", err), http.StatusInternalServerError)
			return
		}
		if archived == nil {
			archived = []gui.ArchivedProjectItem{}
		}
		writeJSON(w, archived)
		return
	}
	guiStore := s.getGUIStore()
	if guiStore == nil {
		writeJSON(w, []gui.ArchivedProjectItem{})
		return
	}
	archived, err := guiStore.GetArchivedProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if archived == nil {
		archived = []gui.ArchivedProjectItem{}
	}
	writeJSON(w, archived)
}

func (s *Server) handleGUIProjectsArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	target := p.Name
	if target == "" {
		target = p.ID
	}
	if target == "" {
		http.Error(w, "project name or id required", http.StatusBadRequest)
		return
	}
	if service := s.guiService(); service != nil {
		if err := service.ArchiveProject(target); err != nil {
			http.Error(w, fmt.Sprintf("GUI backend project archive failed: %v", err), http.StatusInternalServerError)
			return
		}
	} else if s.guiStore != nil {
		if err := s.guiStore.ArchiveProject(target); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, map[string]interface{}{"success": true, "archived": target})
}

func (s *Server) handleGUIProjectsRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	target := p.Name
	if target == "" {
		target = p.ID
	}
	if target == "" {
		http.Error(w, "project name or id required", http.StatusBadRequest)
		return
	}
	if service := s.guiService(); service != nil {
		if err := service.RestoreProject(target); err != nil {
			http.Error(w, fmt.Sprintf("GUI backend project restore failed: %v", err), http.StatusInternalServerError)
			return
		}
	} else if s.guiStore != nil {
		if err := s.guiStore.RestoreProject(target); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, map[string]interface{}{"success": true, "restored": target})
}

func (s *Server) handleGUIProjectsOpenSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	target := p.Name
	if target == "" {
		target = p.ID
	}
	if target == "" {
		http.Error(w, "project name or id required", http.StatusBadRequest)
		return
	}
	if service := s.guiService(); service != nil {
		if err := service.OpenProjectSettings(target); err != nil {
			http.Error(w, fmt.Sprintf("GUI backend project settings failed: %v", err), http.StatusInternalServerError)
			return
		}
	} else if s.guiStore != nil {
		if err := s.guiStore.OpenProjectSettings(target); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, map[string]interface{}{"success": true, "project": target})
}

func (s *Server) handleGUIConversationsAutoArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Horizon string `json:"horizon"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)

	if service := s.guiService(); service != nil {
		result, err := service.ArchiveStaleConversations(p.Horizon)
		if err != nil {
			http.Error(w, fmt.Sprintf("GUI backend auto-archive failed: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, result)
		return
	}

	guiStore := s.getGUIStore()
	if guiStore == nil {
		http.Error(w, "gui store not initialized", http.StatusInternalServerError)
		return
	}

	result, err := guiStore.ArchiveStaleConversations(p.Horizon)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleGUIConversationsPruned(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	vm := s.getVaultManager()
	if vm != nil {
		cfg, _ := core.LoadConfig()
		if cfg == nil || cfg.ConversationVaultEnabled {
			_, _ = vm.Sync()
		}
	}
	pruned := gui.GetPrunedConversationIDs()
	if pruned == nil {
		pruned = []string{}
	}
	writeJSON(w, pruned)
}

func (s *Server) handleVaultStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, _ := core.LoadConfig()
	enabled := true
	if cfg != nil {
		enabled = cfg.ConversationVaultEnabled
	}
	status, err := s.getVaultManager().GetStatus(enabled)
	if err != nil {
		writeJSON(w, map[string]interface{}{"error": err.Error()})
		return
	}
	writeJSON(w, status)
}

func (s *Server) handleVaultSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	res, err := s.getVaultManager().Sync()
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleVaultToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.DefaultConfig()
	}
	cfg.ConversationVaultEnabled = p.Enabled
	if err := cfg.Save(); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	if p.Enabled {
		_, _ = s.getVaultManager().Sync()
	}
	writeJSON(w, map[string]interface{}{"success": true, "enabled": p.Enabled})
}

func (s *Server) handleGUIProjectOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Order []string `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid parameters, order array required", http.StatusBadRequest)
		return
	}

	if service := s.guiService(); service != nil {
		if err := service.UpdateProjectOrder(p.Order); err != nil {
			http.Error(w, fmt.Sprintf("GUI backend project order failed: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "order": p.Order})
		return
	}
	var res interface{}
	if err := s.client.Call("swiss.updateGUIProjectOrder", p, &res); err != nil {
		http.Error(w, fmt.Sprintf("GUI RPC project order failed: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIApply(w http.ResponseWriter, r *http.Request) {
	if service := s.guiService(); service != nil {
		applyRes, err := service.Apply()
		if err != nil && applyRes == nil {
			http.Error(w, fmt.Sprintf("GUI backend apply failed: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, applyRes)
		return
	}
	var res gui.ApplyResult
	if err := s.client.Call("swiss.applyGUIStyles", nil, &res); err != nil {
		http.Error(w, fmt.Sprintf("GUI RPC apply failed: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIDesktopInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var res gui.ApplyResult
	if err := s.client.Call("swiss.installDesktopLoader", nil, &res); err != nil {
		if s.guiStore != nil {
			instRes, errInst := s.guiStore.InstallDesktopLoader()
			if errInst != nil && instRes == nil {
				http.Error(w, errInst.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, instRes)
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIDesktopRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var res gui.ApplyResult
	if err := s.client.Call("swiss.restoreFactoryDefaults", nil, &res); err != nil {
		if s.guiStore != nil {
			restRes, errRest := s.guiStore.RestoreFactoryDefaults()
			if errRest != nil && restRes == nil {
				http.Error(w, errRest.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, restRes)
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIDesktopStatus(w http.ResponseWriter, r *http.Request) {
	var status gui.DesktopStatus
	if err := s.client.Call("swiss.getDesktopStatus", nil, &status); err != nil {
		if s.guiStore != nil {
			writeJSON(w, s.guiStore.GetDesktopStatus())
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, status)
}

func (s *Server) handleGUIDesktopPersistence(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.handleGUIDesktopStatus(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var res map[string]interface{}
	if err := s.client.Call("swiss.setDesktopPersistence", p, &res); err == nil && res != nil {
		writeJSON(w, res)
		return
	}

	cfg, err := core.LoadConfig()
	if err != nil || cfg == nil {
		cfg = core.DefaultConfig()
	}
	cfg.PersistentVisualEffects = p.Enabled
	_ = cfg.Save()

	if s.guiStore != nil {
		var applyRes *gui.ApplyResult
		var applyErr error
		if p.Enabled {
			applyRes, applyErr = s.guiStore.InstallDesktopLoader()
		} else {
			applyRes, applyErr = s.guiStore.UninstallDesktopLoader()
		}
		msg := ""
		if applyRes != nil {
			msg = applyRes.Message
		} else if applyErr != nil {
			msg = applyErr.Error()
		}
		writeJSON(w, map[string]interface{}{
			"success":                   true,
			"persistent_visual_effects": p.Enabled,
			"installed":                 p.Enabled && applyRes != nil && applyRes.Success,
			"message":                   msg,
		})
		return
	}
	writeJSON(w, map[string]interface{}{
		"success":                   true,
		"persistent_visual_effects": p.Enabled,
	})
}

func (s *Server) handleDesktopRelaunch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" {
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": "Antigravity host IDE relaunch skipped (dry run)",
		})
		return
	}
	var res map[string]interface{}
	if err := s.client.Call("swiss.relaunchIDE", map[string]interface{}{}, &res); err != nil {
		go func() {
			time.Sleep(200 * time.Millisecond)
			_ = process.NewShield(0).RelaunchHostIDE()
		}()
		res = map[string]interface{}{
			"success": true,
			"message": "Antigravity host IDE relaunch initiated",
		}
	}
	writeJSON(w, res)
}

func (s *Server) handleDesktopLaunch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" {
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": "Antigravity host IDE launch skipped (dry run)",
		})
		return
	}
	var res map[string]interface{}
	if err := s.client.Call("swiss.launchIDE", map[string]interface{}{}, &res); err != nil {
		go func() {
			time.Sleep(200 * time.Millisecond)
			_ = process.NewShield(0).LaunchHostIDE()
		}()
		res = map[string]interface{}{
			"success": true,
			"message": "Antigravity host IDE launch initiated",
		}
	}
	writeJSON(w, res)
}

// handleVSCodeLaunch starts VS Code with --remote-debugging-port so the daemon can
// attach and inject the Swiss Knife enhancements into the Antigravity extension.
func (s *Server) handleVSCodeLaunch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" {
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": "VS Code launch skipped (dry run)",
		})
		return
	}

	var payload struct {
		Folder string `json:"folder"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}

	var res map[string]interface{}
	if err := s.client.Call("swiss.launchVSCode", payload, &res); err != nil {
		go func() {
			time.Sleep(200 * time.Millisecond)
			_, _ = gui.LaunchVSCode(payload.Folder)
		}()
		res = map[string]interface{}{
			"success": true,
			"message": "VS Code launch with CDP debugging initiated",
		}
	}
	writeJSON(w, res)
}

func (s *Server) handleSystemInstallations(w http.ResponseWriter, r *http.Request) {
	res := s.getSystemDetector().DetectAll()
	writeJSON(w, res)
}

func (s *Server) handleSystemCheckUpdates(w http.ResponseWriter, r *http.Request) {
	res, err := s.getSystemDetector().CheckUpdates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleAppRelease(w http.ResponseWriter, r *http.Request) {
	cfg, _ := core.LoadConfig()
	info := s.getAppReleaseManager().GetCachedReleaseInfo(cfg)
	writeJSON(w, info)
}

func (s *Server) handleCheckAppRelease(w http.ResponseWriter, r *http.Request) {
	cfg, _ := core.LoadConfig()
	info, err := s.getAppReleaseManager().CheckForUpdates(r.Context(), cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, info)
}

func (s *Server) handleAppReleaseSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		AutoCheck   bool `json:"auto_check"`
		AutoUpgrade bool `json:"auto_upgrade"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.DefaultConfig()
	}
	cfg.AutoCheckUpdates = payload.AutoCheck
	cfg.AutoUpgrade = payload.AutoUpgrade
	if err := cfg.Save(); err != nil {
		http.Error(w, "failed to save configuration: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{
		"success":      true,
		"auto_check":   cfg.AutoCheckUpdates,
		"auto_upgrade": cfg.AutoUpgrade,
	})
}

func (s *Server) handleAppReleaseUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, _ := core.LoadConfig()
	relMgr := s.getAppReleaseManager()
	info, err := relMgr.CheckForUpdates(r.Context(), cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res, err := relMgr.ExecuteUpgrade(r.Context(), info)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, res)
}

func (s *Server) isGeminiQuotaExhausted() bool {
	store, err := keyring.NewStore("")
	if err != nil || store == nil {
		return false
	}
	accounts := store.ListAccounts()
	if len(accounts) == 0 {
		return false
	}
	active := store.ActiveAccount()
	states := quota.BuildAccountQuotaStatesFromMap(accounts, nil)
	summary := quota.ComputeFleetSummary(states, active)
	return len(accounts) > 0 && summary.Fleet5hGeminiAvailable <= 0 && summary.FleetWeeklyGeminiAvailable <= 0
}

func customModelEndpointMatches(m custommodels.CustomModel, saved *custommodels.CustomModel) bool {
	if saved == nil || m.ProviderType != saved.ProviderType {
		return false
	}
	normalize := func(value string) string {
		return strings.TrimRight(strings.TrimSpace(value), "/")
	}
	baseURL := normalize(m.BaseURL)
	return baseURL != "" && strings.EqualFold(baseURL, normalize(saved.BaseURL))
}

func (s *Server) hydrateCustomModelSecrets(m *custommodels.CustomModel) error {
	if m == nil || strings.TrimSpace(m.ID) == "" {
		return nil
	}
	store := s.getCustomModelsStore()
	if store == nil {
		return nil
	}
	saved, err := store.GetModel(m.ID)
	if err != nil || saved == nil {
		return nil
	}
	needsAPIKey := (m.APIKey == "" || m.APIKey == "[REDACTED]") && saved.APIKey != ""
	needsGCPKey := (m.GCPServiceAccountKey == "" || m.GCPServiceAccountKey == "[REDACTED]") && saved.GCPServiceAccountKey != ""
	if !customModelEndpointMatches(*m, saved) {
		if needsAPIKey || needsGCPKey {
			return fmt.Errorf("re-enter credentials when changing the provider or endpoint")
		}
		return nil
	}
	if m.APIKey == "" || m.APIKey == "[REDACTED]" {
		m.APIKey = saved.APIKey
	}
	if m.GCPServiceAccountKey == "" || m.GCPServiceAccountKey == "[REDACTED]" {
		m.GCPServiceAccountKey = saved.GCPServiceAccountKey
	}
	if m.GCPProjectID == "" {
		m.GCPProjectID = saved.GCPProjectID
	}
	if m.GCPRegion == "" {
		m.GCPRegion = saved.GCPRegion
	}
	if m.BalanceValue == "" {
		m.BalanceValue = saved.BalanceValue
	}
	if len(m.QuotaBuckets) == 0 && len(saved.QuotaBuckets) > 0 {
		m.QuotaBuckets = saved.QuotaBuckets
	}
	return nil
}

func (s *Server) hydrateFetchModelsRequest(req *custommodels.FetchModelsRequest) error {
	if req == nil {
		return nil
	}
	model := custommodels.CustomModel{
		ID:           req.ID,
		ProviderType: req.ProviderType,
		BaseURL:      req.BaseURL,
		APIKey:       req.APIKey,
	}
	if err := s.hydrateCustomModelSecrets(&model); err != nil {
		return err
	}
	req.APIKey = model.APIKey
	return nil
}

func (s *Server) handleCustomModels(w http.ResponseWriter, r *http.Request) {
	store := s.getCustomModelsStore()
	if store == nil {
		http.Error(w, "custom models store not initialized", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.reconcileCustomModelBalances(r.Context())
		cfg := store.GetConfigRedacted()
		cfg.InferenceSupported = true
		cfg.GeminiQuotaExhausted = s.isGeminiQuotaExhausted()
		if coreCfg, err := core.LoadConfig(); err == nil {
			cfg.DefaultCustomModel = coreCfg.DefaultCustomModel
			if coreCfg.DefaultCustomModel != "" {
				for i := range cfg.Models {
					cfg.Models[i].IsDefault = (cfg.Models[i].ID == coreCfg.DefaultCustomModel || cfg.Models[i].Name == coreCfg.DefaultCustomModel)
				}
			}
		}
		writeJSON(w, cfg)
	case http.MethodPost:
		var m custommodels.CustomModel
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(m.ID) == "" {
			slug := strings.ToLower(strings.TrimSpace(m.Name))
			slug = strings.ReplaceAll(slug, " ", "-")
			slug = strings.ReplaceAll(slug, "/", "-")
			slug = strings.ReplaceAll(slug, ":", "-")
			if slug == "" {
				slug = "model"
			}
			prefix := string(m.ProviderType)
			if prefix == "" {
				prefix = "custom"
			}
			m.ID = fmt.Sprintf("%s-%s-%d", prefix, slug, time.Now().Unix())
		}
		if err := s.hydrateCustomModelSecrets(&m); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Auto-derive balance or quota unless manually overridden by user
		if !m.QuotaManualOverride {
			quotaRes := custommodels.DetectAndFetchQuota(m)
			if quotaRes.QuotaType != custommodels.QuotaTypeNA || (m.QuotaType == "" || m.QuotaType == custommodels.QuotaTypeNA) {
				custommodels.ApplyFetchedQuota(&m, quotaRes)
			}
		}

		// Auto-derive token pricing (Provider -> 3rd-Party Backup -> null) unless manually overridden
		nowShort := time.Now().Format("2006-01-02 15:04")
		if m.PriceSource == custommodels.PriceSourceManual {
			if m.InputPricePerM == nil && m.CachedInputPricePerM == nil && m.OutputPricePerM == nil {
				m.PriceSource = custommodels.PriceSourceUnconfigured
			}
			m.PriceUpdatedAt = nowShort
		} else if m.InputPricePerM != nil || m.CachedInputPricePerM != nil || m.OutputPricePerM != nil {
			if m.PriceSource == "" {
				m.PriceSource = custommodels.PriceSourceManual
			}
			m.PriceUpdatedAt = nowShort
		} else {
			inP, cacheP, outP, src := custommodels.FetchModelPricing(r.Context(), m.Name, string(m.ProviderType), m.BaseURL, m.APIKey)
			m.InputPricePerM = inP
			m.CachedInputPricePerM = cacheP
			m.OutputPricePerM = outP
			m.PriceSource = src
			m.PriceUpdatedAt = nowShort
		}

		if m.TotalBudget <= 0 {
			if m.PrepaidBalance > 0 {
				m.TotalBudget = m.PrepaidBalance
			} else if m.BalanceValue != "" {
				clean := strings.TrimLeft(strings.TrimSpace(m.BalanceValue), "$¥€£")
				if val, err := strconv.ParseFloat(clean, 64); err == nil && val > 0 {
					m.TotalBudget = val
					m.PrepaidBalance = val
				}
			}
		}
		if m.TotalBudget > 0 && m.QuotaFraction == nil {
			frac := m.PrepaidBalance / m.TotalBudget
			if frac < 0 {
				frac = 0
			}
			if frac > 1 {
				frac = 1
			}
			m.QuotaFraction = &frac
		}
		if err := store.SaveModel(m); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if m.TotalBudget > 0 {
			s.reconcileCustomModelBalances(r.Context())
		}
		if m.IsDefault {
			if c, err := core.LoadConfig(); err == nil {
				c.DefaultCustomModel = m.ID
				_ = c.Save()
			}
		} else {
			if c, err := core.LoadConfig(); err == nil && (c.DefaultCustomModel == m.ID || c.DefaultCustomModel == m.Name) {
				c.DefaultCustomModel = ""
				_ = c.Save()
				_ = store.ClearDefaultFlags()
			}
		}
		if saved, err := store.GetModel(m.ID); err == nil && saved != nil {
			m = *saved
		}
		s.syncLanguageServerCloudCodeURL()
		writeJSON(w, map[string]interface{}{"success": true, "model": custommodels.RedactCustomModel(m)})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			var p struct {
				ID string `json:"id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&p)
			id = p.ID
		}
		if id == "" {
			http.Error(w, "missing model id parameter", http.StatusBadRequest)
			return
		}
		deletedName := ""
		if existing, err := store.GetModel(id); err == nil && existing != nil {
			deletedName = existing.Name
		}
		if err := store.DeleteModel(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if c, err := core.LoadConfig(); err == nil && c.DefaultCustomModel != "" && (c.DefaultCustomModel == id || (deletedName != "" && c.DefaultCustomModel == deletedName)) {
			c.DefaultCustomModel = ""
			_ = c.Save()
			_ = store.ClearDefaultFlags()
		}
		s.syncLanguageServerCloudCodeURL()
		writeJSON(w, map[string]interface{}{"success": true, "deleted": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func addVaryHeader(header http.Header, value string) {
	for _, line := range header.Values("Vary") {
		for _, existing := range strings.Split(line, ",") {
			if strings.EqualFold(strings.TrimSpace(existing), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

func (s *Server) handleCustomModelPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, custommodels.GetPresets())
}

func (s *Server) handleCustomModelTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var m custommodels.CustomModel
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.hydrateCustomModelSecrets(&m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tester := s.getCustomModelsTester()
	res, err := tester.TestEndpoint(m)
	if err != nil {
		writeJSON(w, res)
		return
	}
	// Persist the latest test outcome so the default-model gate can refuse
	// models whose most recent connectivity test failed. Draft models that are
	// not stored yet simply have nothing to record.
	if m.ID != "" && s.getCustomModelsStore() != nil {
		_ = s.customModelsStore.RecordModelTestResult(m.ID, res.Success)
	}
	writeJSON(w, res)
}

// cloudCodeDispatch sends every /v1internal* call to the Cloud Code proxy before
// the mux sees it, because Go's ServeMux cannot express the "/v1internal:<method>"
// form the language server uses.
func (s *Server) cloudCodeDispatch(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1internal") {
			if proxy := s.getCloudCodeProxy(); proxy != nil {
				proxy.Handler().ServeHTTP(w, r)
				return
			}
		}
		// Surface trace: what the Antigravity language server actually calls when it
		// runs a turn. Opt-in, because it logs every non-GUI request.
		if os.Getenv("SWISS_SURFACE_TRACE") == "1" && !strings.HasPrefix(r.URL.Path, "/api") && !strings.HasPrefix(r.URL.Path, "/assets") {
			log.Printf("[surface-request] %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			if strings.HasPrefix(r.URL.Path, "/v1/chat/completions") && r.Body != nil {
				logSurfacePayload(r)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// logSurfacePayload records the shape of an app model request: header names, the
// top-level body keys, and enough about the message history to tell whether the
// app trims it between turns. Values and content are never logged.
func logSurfacePayload(r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	names := make([]string, 0, len(r.Header))
	for name := range r.Header {
		names = append(names, strings.ToLower(name))
	}
	sort.Strings(names)

	var payload map[string]interface{}
	_ = json.Unmarshal(body, &payload)
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	summary := ""
	if messages, ok := payload["messages"].([]interface{}); ok {
		first := ""
		if len(messages) > 0 {
			if entry, ok := messages[0].(map[string]interface{}); ok {
				if content, ok := entry["content"].(string); ok && len(content) > 24 {
					first = content[:24]
				} else if content, ok := entry["content"].(string); ok {
					first = content
				}
			}
		}
		summary = fmt.Sprintf("messages=%d first_role=%v first_len=%d first_prefix=%q", len(messages), roleOf(messages), firstLen(messages), first)
	}
	log.Printf("[surface-payload] headers=%v keys=%v %s", names, keys, summary)
}

func roleOf(messages []interface{}) string {
	if len(messages) == 0 {
		return ""
	}
	if entry, ok := messages[0].(map[string]interface{}); ok {
		if role, ok := entry["role"].(string); ok {
			return role
		}
	}
	return ""
}

func firstLen(messages []interface{}) int {
	if len(messages) == 0 {
		return 0
	}
	if entry, ok := messages[0].(map[string]interface{}); ok {
		if content, ok := entry["content"].(string); ok {
			return len(content)
		}
	}
	return 0
}

// loopbackAddr turns the listen address into a dialable loopback host:port.
func loopbackAddr(addr string) string {
	host, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return "127.0.0.1:8765"
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// gatewayModelIDs lists the enabled custom model ids the app should surface.
func gatewayModelIDs(store *custommodels.Store) []string {
	models := store.ListModels()
	ids := make([]string, 0, len(models))
	for _, model := range models {
		if model.Enabled {
			ids = append(ids, model.ID)
		}
	}
	return ids
}

// handleCustomModelLaunch starts a surface with the model source its settings
// request: native launches plainly, custom launches an isolated profile pointed at
// the model gateway.
func (s *Server) handleCustomModelLaunch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		App string `json:"app"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	store := s.getCustomModelsStore()
	if store == nil {
		http.Error(w, "custom models store not initialized", http.StatusInternalServerError)
		return
	}
	settings := store.IntegrationSettings()
	profile, ok := settings.Apps[payload.App]
	if !ok {
		http.Error(w, "unknown app: use antigravity or vscode", http.StatusBadRequest)
		return
	}
	endpoint := "http://" + loopbackAddr(s.addr)

	switch payload.App {
	case "antigravity":
		pid, err := process.LaunchAntigravityProfile(profile.Source, endpoint, store.GatewayToken(), gatewayModelIDs(store), process.NewShield(0))
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "app": "antigravity", "source": profile.Source, "pid": pid})
	case "vscode":
		settingsPath := ""
		if profile.Source == "custom" {
			// The isolated instance reads its own settings tree.
			settingsPath = process.VSCodeSettingsPathFor(process.IsolatedProfileDir("vscode"))
		}
		path, err := process.ApplyVSCodeModelSource(profile.Source, endpoint, settingsPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		launchErr := ""
		if err := process.LaunchVSCode(profile.Source, endpoint, store.GatewayToken(), gatewayModelIDs(store)); err != nil {
			launchErr = err.Error()
		}
		writeJSON(w, map[string]interface{}{
			"success":     true,
			"app":         "vscode",
			"source":      profile.Source,
			"settings":    path,
			"launch_note": launchErr,
		})
	default:
		http.Error(w, "unknown app", http.StatusBadRequest)
	}
}

// handleCustomModelIntegration reads and writes the Model Source settings.
func (s *Server) handleCustomModelIntegration(w http.ResponseWriter, r *http.Request) {
	store := s.getCustomModelsStore()
	if store == nil {
		http.Error(w, "custom models store not initialized", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		integration, pinSet := redactIntegrationSettings(store.IntegrationSettings())
		writeJSON(w, map[string]interface{}{
			"success":     true,
			"integration": integration,
			"endpoint":    "http://" + loopbackAddr(s.addr) + "/v1",
			"gemini_base": "http://" + loopbackAddr(s.addr) + "/v1beta",
			"pin_set":     pinSet,
		})
	case http.MethodPost:
		var payload custommodels.IntegrationSettings
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		existing := store.IntegrationSettings()
		if payload.Proxy.PIN == "" || payload.Proxy.PIN == "[REDACTED]" {
			payload.Proxy.PIN = existing.Proxy.PIN
		}
		if err := store.SetIntegrationSettings(payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		integration, pinSet := redactIntegrationSettings(store.IntegrationSettings())
		writeJSON(w, map[string]interface{}{
			"success":     true,
			"integration": integration,
			"pin_set":     pinSet,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func redactIntegrationSettings(settings custommodels.IntegrationSettings) (custommodels.IntegrationSettings, bool) {
	pinSet := settings.Proxy.PIN != ""
	settings.Proxy.PIN = ""
	return settings, pinSet
}

// handleCustomModelAppProxy launches (or restarts) the Antigravity app with
// CLOUD_CODE_URL pointed at this daemon, which is what makes the app's own
// generation calls reach the proxy.
func (s *Server) handleCustomModelAppProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	endpoint := "http://" + loopbackAddr(s.addr)
	restart := strings.EqualFold(payload.Action, "restart")
	pid, err := process.LaunchAntigravityWithCloudCodeProxy(endpoint, restart, process.NewShield(0))
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	s.syncLanguageServerCloudCodeURL()
	writeJSON(w, map[string]interface{}{
		"success":        true,
		"pid":            pid,
		"cloud_code_url": endpoint,
		"restarted":      restart,
	})
}

// handleCustomModelRestoreNative terminates proxy usage and restarts running
// Antigravity instances so they connect directly to native Google Cloud Code.
func (s *Server) handleCustomModelRestoreNative(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := custommodels.CleanShellRCHooks(); err != nil {
		log.Printf("[webgui] shell RC cleanup failed while restoring native mode: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, map[string]interface{}{
			"success":         false,
			"restored_native": false,
			"error":           fmt.Sprintf("failed to clean shell RC hooks: %v", err),
		})
		return
	}
	if err := s.restoreNative().err(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, map[string]interface{}{
			"success":         false,
			"restored_native": false,
			"error":           err.Error(),
		})
		return
	}
	writeJSON(w, map[string]interface{}{
		"success":         true,
		"restored_native": true,
	})
}

func (s *Server) restoreNative() restoreNativeOutcome {
	deps := s.restoreNativeDeps
	if deps.syncURL == nil {
		deps.syncURL = languageServerURLSyncer
	}
	if deps.restartApps == nil {
		deps.restartApps = func() error {
			return process.RestartAntigravityAppsToNative(process.NewShield(0))
		}
	}

	outcome := restoreNativeOutcome{SyncErr: deps.syncURL("")}
	outcome.RestartErr = deps.restartApps()
	return outcome
}

// handleCustomModelActive records which custom model the app should use. Empty
// model_id hands the session back to the native model.
func (s *Server) handleCustomModelActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		ModelID string `json:"model_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	store := s.getCustomModelsStore()
	if store == nil {
		http.Error(w, "custom models store not initialized", http.StatusInternalServerError)
		return
	}
	if err := store.SetActiveModel(payload.ModelID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.syncLanguageServerCloudCodeURL()
	writeJSON(w, map[string]interface{}{"success": true, "active_model_id": store.GetConfig().ActiveModelID})
}

func (s *Server) handleCustomModelFetchModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req custommodels.FetchModelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.hydrateFetchModelsRequest(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res := custommodels.FetchModels(req)
	writeJSON(w, res)
}

func (s *Server) handleCustomModelThinkingLevel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		ModelID string `json:"model_id"`
		Level   string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.ModelID == "" {
		http.Error(w, "invalid request body, missing model_id", http.StatusBadRequest)
		return
	}
	store := s.getCustomModelsStore()
	if store == nil {
		http.Error(w, "custom models store not initialized", http.StatusInternalServerError)
		return
	}
	if err := store.SetThinkingLevel(p.ModelID, p.Level); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true, "model_id": p.ModelID, "level": p.Level})
}

// handleCustomModelCLIStatus returns the installation and gateway status of agy CLI.
func (s *Server) handleCustomModelCLIStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store := s.getCustomModelsStore()
	status := custommodels.CheckCLIGatewayStatus(store)
	status.GatewayToken = ""
	writeJSON(w, map[string]interface{}{"success": true, "status": status})
}

// handleCustomModelCLISetup installs or repairs the CLI gateway integration.
func (s *Server) handleCustomModelCLISetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store := s.getCustomModelsStore()
	if err := custommodels.SetupCLIGatewayIntegration(store); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	status := custommodels.CheckCLIGatewayStatus(store)
	status.GatewayToken = ""
	writeJSON(w, map[string]interface{}{"success": true, "status": status})
}

func (s *Server) handleCustomModelFetchQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var m custommodels.CustomModel
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.hydrateCustomModelSecrets(&m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res := custommodels.DetectAndFetchQuota(m)
	inP, cacheP, outP, src := custommodels.FetchModelPricing(r.Context(), m.Name, string(m.ProviderType), m.BaseURL, m.APIKey)
	res.InputPricePerM = inP
	res.CachedInputPricePerM = cacheP
	res.OutputPricePerM = outP
	res.PriceSource = src
	writeJSON(w, res)
}

func (s *Server) handleCustomModelRefreshQuotas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store := s.getCustomModelsStore()
	if store == nil {
		http.Error(w, "custom models store not initialized", http.StatusInternalServerError)
		return
	}
	cfg := store.GetConfig()
	nowShort := time.Now().Format("2006-01-02 15:04")
	for i := range cfg.Models {
		quotaRes := custommodels.DetectAndFetchQuota(cfg.Models[i])
		if !cfg.Models[i].QuotaManualOverride {
			// Persist every reported quota window (5H + 7D + 30D) so the
			// model card can render one indicator per window.
			custommodels.ApplyFetchedQuota(&cfg.Models[i], quotaRes)
		}
		if cfg.Models[i].PriceSource != custommodels.PriceSourceManual {
			inP, cacheP, outP, src := custommodels.FetchModelPricing(r.Context(), cfg.Models[i].Name, string(cfg.Models[i].ProviderType), cfg.Models[i].BaseURL, cfg.Models[i].APIKey)
			cfg.Models[i].InputPricePerM = inP
			cfg.Models[i].CachedInputPricePerM = cacheP
			cfg.Models[i].OutputPricePerM = outP
			cfg.Models[i].PriceSource = src
			cfg.Models[i].PriceUpdatedAt = nowShort
		}
		_ = store.SaveModel(cfg.Models[i])
	}
	s.reconcileCustomModelBalances(r.Context())
	writeJSON(w, store.GetConfigRedacted())
}

// Enhancements handlers

func (s *Server) handleEnhancements(w http.ResponseWriter, r *http.Request) {
	store := s.getEnhancementsStore()
	if store == nil {
		http.Error(w, "enhancements store not initialized", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		writeJSON(w, store.GetConfig())
		return
	}

	if r.Method == http.MethodPost {
		var cfg enhancements.EnhancementsConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := store.UpdateConfig(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, store.GetConfig())
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleEnhancementsUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleEnhancements(w, r)
}

func (s *Server) handleEnhancementsApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Re-apply styles through the owning GUI store, which includes enhancement injection.
	if s.backend != nil {
		res, err := s.backend.GUI().Apply()
		if err != nil && res == nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, res)
		return
	}
	if s.guiStore != nil {
		res, err := s.guiStore.Apply()
		if err != nil && res == nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, res)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true, "message": "Enhancements configuration updated"})
}

func (s *Server) handleCommunityExtensions(w http.ResponseWriter, r *http.Request) {
	store := s.getEnhancementsStore()
	if store == nil {
		http.Error(w, "enhancements store not initialized", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]interface{}{
			"success":    true,
			"extensions": store.GetCommunityExtensions(),
		})
		return

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]interface{}{"success": false, "error": "failed to read request body"})
			return
		}

		trimmed := bytes.TrimSpace(body)
		if len(trimmed) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]interface{}{"success": false, "error": "empty request body"})
			return
		}

		if trimmed[0] == '[' {
			var list []enhancements.CommunityExtension
			if err := json.Unmarshal(trimmed, &list); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				writeJSON(w, map[string]interface{}{"success": false, "error": "invalid json array: " + err.Error()})
				return
			}
			for _, ext := range list {
				if _, err := store.SaveCommunityExtension(ext); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
					return
				}
			}
			if s.guiStore != nil {
				_, _ = s.guiStore.Apply()
			}
			writeJSON(w, map[string]interface{}{
				"success":    true,
				"extensions": store.GetCommunityExtensions(),
			})
			return
		}

		var envelope struct {
			Extensions *[]enhancements.CommunityExtension `json:"extensions"`
			Extension  *enhancements.CommunityExtension   `json:"extension"`
		}
		if err := json.Unmarshal(trimmed, &envelope); err == nil && (envelope.Extensions != nil || envelope.Extension != nil) {
			if envelope.Extensions != nil {
				for _, ext := range *envelope.Extensions {
					if _, err := store.SaveCommunityExtension(ext); err != nil {
						w.WriteHeader(http.StatusBadRequest)
						writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
						return
					}
				}
				if s.guiStore != nil {
					_, _ = s.guiStore.Apply()
				}
				writeJSON(w, map[string]interface{}{
					"success":    true,
					"extensions": store.GetCommunityExtensions(),
				})
				return
			}
			if envelope.Extension != nil {
				saved, err := store.SaveCommunityExtension(*envelope.Extension)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
					return
				}
				if s.guiStore != nil {
					_, _ = s.guiStore.Apply()
				}
				writeJSON(w, map[string]interface{}{
					"success":    true,
					"extension":  saved,
					"extensions": store.GetCommunityExtensions(),
				})
				return
			}
		}

		var direct enhancements.CommunityExtension
		if err := json.Unmarshal(trimmed, &direct); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]interface{}{"success": false, "error": "invalid request body: " + err.Error()})
			return
		}

		saved, err := store.SaveCommunityExtension(direct)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		if s.guiStore != nil {
			_, _ = s.guiStore.Apply()
		}
		writeJSON(w, map[string]interface{}{
			"success":    true,
			"extension":  saved,
			"extensions": store.GetCommunityExtensions(),
		})
		return

	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if err := store.DeleteCommunityExtension(id); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		if s.guiStore != nil {
			_, _ = s.guiStore.Apply()
		}
		writeJSON(w, map[string]interface{}{
			"success":    true,
			"deleted":    id,
			"extensions": store.GetCommunityExtensions(),
		})
		return

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGoogleOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed: POST required", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	res, err := s.oauthMgr.StartFlow(ctx, true)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	writeJSON(w, map[string]interface{}{
		"success":       true,
		"email":         res.Email,
		"refresh_token": res.RefreshToken,
		"access_token":  res.AccessToken,
	})
}

func (s *Server) handleGoogleOAuthCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed: POST required", http.StatusMethodNotAllowed)
		return
	}
	s.oauthMgr.CancelFlow()
	writeJSON(w, map[string]interface{}{
		"success":   true,
		"cancelled": true,
	})
}

func (s *Server) handleGoogleOAuthURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed: GET required", http.StatusMethodNotAllowed)
		return
	}
	authURL := s.oauthMgr.GetActiveAuthURL()
	state := s.oauthMgr.GetActiveState()
	writeJSON(w, map[string]interface{}{
		"success":  true,
		"active":   authURL != "",
		"auth_url": authURL,
		"state":    state,
	})
}

func (s *Server) handleGoogleOAuthExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed: POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		CallbackURL string `json:"callback_url"`
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri"`
		State       string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}
	res, err := s.oauthMgr.Exchange(req.CallbackURL, req.Code, req.RedirectURI, req.State)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	writeJSON(w, map[string]interface{}{
		"success":       true,
		"email":         res.Email,
		"refresh_token": res.RefreshToken,
		"access_token":  res.AccessToken,
	})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	cfg, err := core.LoadConfig()
	if err != nil {
		writeJSON(w, map[string]interface{}{"password_required": false})
		return
	}
	writeJSON(w, map[string]interface{}{
		"password_required": cfg.AppPasswordEnabled,
	})
}

func (s *Server) handleAuthUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	cfg, err := core.LoadConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !cfg.AppPasswordEnabled {
		tok, tokenErr := s.createSessionToken()
		if tokenErr != nil {
			http.Error(w, "failed to create session token", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "token": tok, "session_token": tok})
		return
	}
	if !s.allowLegacyAppPasswordAttempt(r) {
		http.Error(w, "too many password attempts", http.StatusTooManyRequests)
		return
	}
	if verifyAppPasswordForAuth(cfg, p.Password) {
		tok, tokenErr := s.createSessionToken()
		if tokenErr != nil {
			http.Error(w, "failed to create session token", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "token": tok, "session_token": tok})
	} else {
		writeJSON(w, map[string]interface{}{"success": false, "error": "Incorrect password"})
	}
}

func (s *Server) handleSettingsPassword(w http.ResponseWriter, r *http.Request) {
	cfg, err := core.LoadConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		writeJSON(w, map[string]interface{}{
			"enabled": cfg.AppPasswordEnabled,
		})
		return
	}

	if r.Method == http.MethodPost {
		var p struct {
			Password        string `json:"password"`
			CurrentPassword string `json:"current_password"`
			Remove          bool   `json:"remove"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if cfg.AppPasswordEnabled && cfg.AppPasswordHash != "" {
			if !cfg.VerifyAppPassword(p.CurrentPassword) {
				writeJSON(w, map[string]interface{}{"success": false, "error": "Current password incorrect"})
				return
			}
		}

		if p.Remove || p.Password == "" {
			if err := cfg.SetAppPassword(""); err != nil {
				writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
				return
			}
			writeJSON(w, map[string]interface{}{"success": true, "enabled": false})
			return
		}

		if err := cfg.SetAppPassword(p.Password); err != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "enabled": true})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleSettingsStorage(w http.ResponseWriter, r *http.Request) {
	cfg, err := core.LoadConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		info := system.GetStorageInfo(cfg)
		writeJSON(w, info)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			StorageMode string `json:"storage_mode"`
			MigrateData bool   `json:"migrate_data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		info, err := system.SwitchStorageMode(req.StorageMode, req.MigrateData, cfg)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		writeJSON(w, map[string]interface{}{
			"success": true,
			"storage": info,
		})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleSettingsRuntimeMode(w http.ResponseWriter, r *http.Request) {
	s.desktopSettingsMu.Lock()
	defer s.desktopSettingsMu.Unlock()
	settingsPath := filepath.Join(core.GetConfigDir(), "desktop_settings.json")

	if r.Method == http.MethodGet {
		mode := "app"
		if data, err := os.ReadFile(settingsPath); err == nil {
			var parsed struct {
				RuntimeMode string `json:"runtime_mode"`
			}
			if json.Unmarshal(data, &parsed) == nil && (parsed.RuntimeMode == "daemon" || parsed.RuntimeMode == "app") {
				mode = parsed.RuntimeMode
			}
		}
		writeJSON(w, map[string]interface{}{"runtime_mode": mode})
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			RuntimeMode string `json:"runtime_mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		mode := "app"
		if req.RuntimeMode == "daemon" {
			mode = "daemon"
		}
		var data map[string]interface{} = make(map[string]interface{})
		if raw, err := os.ReadFile(settingsPath); err == nil {
			_ = json.Unmarshal(raw, &data)
		}
		data["runtime_mode"] = mode
		if b, err := json.MarshalIndent(data, "", "  "); err == nil {
			tmp := settingsPath + ".tmp"
			if err := os.WriteFile(tmp, b, 0600); err == nil {
				_ = os.Rename(tmp, settingsPath)
			}
		}
		writeJSON(w, map[string]interface{}{"success": true, "runtime_mode": mode})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleDaemonStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var status map[string]interface{}
	if err := s.client.Call("swiss.getStatus", nil, &status); err == nil {
		writeJSON(w, map[string]interface{}{"success": true, "daemon_running": true, "message": "daemon already running"})
		return
	}

	exe, err := os.Executable()
	if err != nil {
		exe = "swiss"
	}
	cmd := exec.Command(exe, "daemon")
	core.SetHiddenConsole(cmd)
	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("failed to spawn daemon: %v", err), http.StatusInternalServerError)
		return
	}
	_ = cmd.Process.Release()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(150 * time.Millisecond)
		if err := s.client.Call("swiss.getStatus", nil, &status); err == nil {
			writeJSON(w, map[string]interface{}{"success": true, "daemon_running": true})
			return
		}
	}
	writeJSON(w, map[string]interface{}{"success": true, "daemon_running": false, "message": "daemon spawned, awaiting socket"})
}

// handleSettingsTheme serves GET (current theme mode) and POST (update theme mode)
// for the UI appearance setting ("auto" | "light" | "dark").
func (s *Server) handleSettingsTheme(w http.ResponseWriter, r *http.Request) {
	cfg, err := core.LoadConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		writeJSON(w, map[string]interface{}{
			"theme": cfg.GetTheme(),
		})
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Theme string `json:"theme"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		theme := core.NormalizeThemeMode(req.Theme)
		if req.Theme != "" && theme != req.Theme {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]interface{}{"success": false, "error": "invalid theme: must be 'auto', 'light', or 'dark'"})
			return
		}
		if err := cfg.SetTheme(theme); err != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "theme": theme})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleSettingsAppPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, err := core.LoadConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var req struct {
		AppType string `json:"app_type"`
		Path    string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	info, err := system.SaveAppPath(req.AppType, req.Path, cfg)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"storage": info,
	})
}

func (s *Server) handleSettingsAccountOverride(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, err := core.LoadConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var req struct {
		AppType string `json:"app_type"`
		Email   string `json:"email"`
		Path    string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	info, err := system.SaveAccountOverride(req.AppType, req.Email, req.Path, cfg)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"storage": info,
	})
}

func (s *Server) handleSettingsCacheClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AppType string `json:"app_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	res, err := system.ClearAppCache(req.AppType)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleSystemFactoryReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var guiMsg string
	if s.guiStore != nil {
		res, err := s.guiStore.RestoreFactoryDefaults()
		if err == nil && res != nil {
			guiMsg = res.Message
		}
	} else if s.client != nil {
		var res gui.ApplyResult
		if err := s.client.Call("swiss.restoreFactoryDefaults", nil, &res); err == nil {
			guiMsg = res.Message
		}
	}

	cfg, _ := core.LoadConfig()
	if cfg != nil {
		cfg.DesktopAppPath = ""
		cfg.AgyCLIPath = ""
		cfg.VSCodeExtensionPath = ""
		cfg.AppAccountOverrides = make(map[string]map[string]string)
		_ = cfg.Save()
	}

	msg := "All Antigravity apps restored to an unmodified status by turning off all features and restoring backed-up files/code."
	if guiMsg != "" {
		msg += " (" + guiMsg + ")"
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": msg,
	})
}

func (s *Server) handleSettingsPrivacy(w http.ResponseWriter, r *http.Request) {
	cfg, err := core.LoadConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		writeJSON(w, map[string]interface{}{
			"anonymous_error_reports": cfg.AnonymousErrorReports,
			"anonymous_telemetry":     cfg.AnonymousTelemetry,
			"github_repo":             system.PublicGitHubRepo,
		})
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			AnonymousErrorReports bool `json:"anonymous_error_reports"`
			AnonymousTelemetry    bool `json:"anonymous_telemetry"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		cfg.AnonymousErrorReports = req.AnonymousErrorReports
		cfg.AnonymousTelemetry = req.AnonymousTelemetry
		if err := cfg.Save(); err != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		writeJSON(w, map[string]interface{}{
			"success":                 true,
			"anonymous_error_reports": cfg.AnonymousErrorReports,
			"anonymous_telemetry":     cfg.AnonymousTelemetry,
		})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleSettingsDiagnoseIssue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req system.DiagnosticRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	cfg, _ := core.LoadConfig()
	keyringStore, _ := keyring.NewStore("")

	result, err := system.RunIssueDiagnosis(req, cfg, keyringStore, s.getCustomModelsStore())
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, result)
}

func getAgentImporterScriptPath() string {
	candidates := []string{
		"scripts/agent_importer.py",
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "scripts", "agent_importer.py"))
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "scripts", "agent_importer.py"))
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "scripts/agent_importer.py"
}

func (s *Server) handleUtilitiesImportScan(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "opencode"
	}

	antigravityBase, appStoragePath := importer.DefaultPaths()

	// Pure Go native scanner for Claude Code
	if source == "claude-code" {
		candidates, err := importer.ScanClaudeCode("", antigravityBase, appStoragePath)
		if err == nil {
			writeJSON(w, map[string]interface{}{
				"success":    true,
				"source":     source,
				"count":      len(candidates),
				"candidates": candidates,
			})
			return
		}
	}

	// Python script fallback for legacy formats
	scriptPath := getAgentImporterScriptPath()
	cmd := exec.Command("python3", scriptPath, "scan", "--source", source)
	core.SetHiddenConsole(cmd)
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		var resp map[string]interface{}
		if err := json.Unmarshal(out, &resp); err == nil {
			writeJSON(w, resp)
			return
		}
	}

	// Fallback response if script failed
	writeJSON(w, map[string]interface{}{
		"success":    false,
		"source":     source,
		"count":      0,
		"candidates": []interface{}{},
		"error":      fmt.Sprintf("Failed to run scanner: %v (%s)", err, string(out)),
	})
}

func (s *Server) handleUtilitiesImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		CandidateIDs []string `json:"candidate_ids"`
		Source       string   `json:"source"`
		Mode         string   `json:"mode"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Source == "" {
		req.Source = "opencode"
	}
	// Whitelist source and mode
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_\-]+$`, req.Source); !matched {
		req.Source = "opencode"
	}
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_\-]+$`, req.Mode); !matched {
		req.Mode = "auto"
	}

	validIDRegex := regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)
	var sanitizedIDs []string
	for _, cid := range req.CandidateIDs {
		cid = strings.TrimSpace(cid)
		if cid == "" || strings.Contains(cid, "\x00") || strings.Contains(cid, "..") {
			continue
		}
		if filepath.Base(cid) != cid || strings.ContainsAny(cid, `/\`) {
			continue
		}
		if validIDRegex.MatchString(cid) {
			sanitizedIDs = append(sanitizedIDs, cid)
		}
	}
	req.CandidateIDs = sanitizedIDs

	antigravityBase, appStoragePath := importer.DefaultPaths()

	// Pure Go native importer for Claude Code
	if req.Source == "claude-code" {
		home, _ := os.UserHomeDir()
		claudeDir := filepath.Join(home, ".claude", "transcripts")
		var results []importer.ImportResult
		for _, cid := range req.CandidateIDs {
			filename := cid
			if !strings.HasSuffix(filename, ".jsonl") {
				filename += ".jsonl"
			}
			p := filepath.Join(claudeDir, filename)
			rel, err := filepath.Rel(claudeDir, p)
			if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
				continue
			}
			res, err := importer.ImportClaudeCodeSession(p, antigravityBase, appStoragePath, "")
			if err == nil && res != nil {
				results = append(results, *res)
			}
		}
		writeJSON(w, map[string]interface{}{
			"success":        len(results) > 0,
			"imported_count": len(results),
			"results":        results,
		})
		return
	}

	// Python script fallback
	scriptPath := getAgentImporterScriptPath()
	idsArg := strings.Join(req.CandidateIDs, ",")
	cmd := exec.Command("python3", scriptPath, "import", "--source", req.Source, "--ids", idsArg, "--mode", req.Mode)
	core.SetHiddenConsole(cmd)
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		var resp map[string]interface{}
		if err := json.Unmarshal(out, &resp); err == nil {
			writeJSON(w, resp)
			return
		}
	}

	writeJSON(w, map[string]interface{}{
		"success":        false,
		"imported_count": 0,
		"error":          fmt.Sprintf("Failed to execute import: %v (%s)", err, string(out)),
	})
}

func (s *Server) handleCustomModelSecurityAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var m custommodels.CustomModel
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.hydrateCustomModelSecrets(&m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	auditor := custommodels.NewAuditor()
	report := auditor.RunAudit(m)
	if strings.TrimSpace(m.ID) != "" {
		cms := s.getCustomModelsStore()
		if cms != nil {
			if existing, err := cms.GetModel(m.ID); err == nil && existing != nil {
				existing.SecurityRiskLevel = report.RiskLevel
				existing.SecurityAuditScore = report.RiskScore
				existing.SecurityGrade = report.SecurityGrade
				existing.LastSecurityAudit = report.AuditedAt
				_ = cms.SaveModel(*existing)
			}
		}
	}
	writeJSON(w, report)
}

// MemoItem represents a quick text, voice, or image memo.
type MemoItem struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	Type          string   `json:"type"` // "text" | "audio" | "image"
	AudioData     string   `json:"audio_data,omitempty"`
	ImageData     string   `json:"image_data,omitempty"`
	CreatedAt     string   `json:"created_at"`
	Tags          []string `json:"tags"`
	Transcript    string   `json:"transcript,omitempty"`
	WorkspacePath string   `json:"workspace_path,omitempty"`
	Project       string   `json:"project,omitempty"`
	Duration      string   `json:"duration,omitempty"`
}

func getMemosPath() string {
	dir := core.GetConfigDir()
	target := filepath.Join(dir, "memos.json")
	if _, err := os.Stat(target); err == nil {
		return target
	}
	if !core.IsRunningTests() {
		home, _ := os.UserHomeDir()
		if home != "" {
			legacy := filepath.Join(home, ".config", "antigravity-swiss", "memos.json")
			if legacy != target {
				if _, err := os.Stat(legacy); err == nil {
					return legacy
				}
			}
		}
	}
	return target
}

func (s *Server) getGlobalMemosPath() string {
	if s != nil && s.globalMemosPath != "" {
		return s.globalMemosPath
	}
	return getMemosPath()
}

func (s *Server) resolveWorkspacePath(r *http.Request) string {
	if r == nil {
		return ""
	}
	ws := r.URL.Query().Get("workspace_path")
	if ws == "" {
		ws = r.Header.Get("X-Workspace-Path")
	}
	return cleanUserPath(ws)
}

func (s *Server) resolveStorageDestination(r *http.Request, storageMode, wsPath string) (string, bool) {
	if storageMode == "" {
		cfg, err := core.LoadConfig()
		if err == nil {
			storageMode = cfg.GetMemoConfig().StorageLocation
		}
	}
	if wsPath == "" && r != nil {
		wsPath = s.resolveWorkspacePath(r)
	}

	if storageMode == "project" {
		if wsPath == "" {
			return s.getGlobalMemosPath(), true
		}
		fi, err := os.Stat(wsPath)
		if err != nil || !fi.IsDir() {
			return s.getGlobalMemosPath(), true
		}
		dotDir := filepath.Join(wsPath, ".antigravity")
		if err := os.MkdirAll(dotDir, 0700); err != nil {
			return s.getGlobalMemosPath(), true
		}
		testFile := filepath.Join(dotDir, ".write_probe")
		if err := os.WriteFile(testFile, []byte(""), 0600); err != nil {
			return s.getGlobalMemosPath(), true
		}
		_ = os.Remove(testFile)

		return filepath.Join(dotDir, "memos.json"), false
	}

	return s.getGlobalMemosPath(), false
}

func readMemosFile(p string) []MemoItem {
	data, err := os.ReadFile(p)
	if err != nil {
		return []MemoItem{}
	}
	str := strings.TrimSpace(string(data))
	if str == "" || str == "null" {
		return []MemoItem{}
	}
	var memos []MemoItem
	if err := json.Unmarshal(data, &memos); err != nil {
		return []MemoItem{}
	}
	if memos == nil {
		return []MemoItem{}
	}
	return memos
}

func writeMemosFile(p string, memos []MemoItem) error {
	if memos == nil {
		memos = []MemoItem{}
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(memos, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

func (s *Server) loadAllProjectMemos(currentWs string) []MemoItem {
	memosMap := make(map[string]MemoItem)
	var order []string

	addMemos := func(list []MemoItem) {
		for _, m := range list {
			if m.ID == "" {
				continue
			}
			if _, exists := memosMap[m.ID]; !exists {
				order = append(order, m.ID)
			}
			memosMap[m.ID] = m
		}
	}

	// 1. Current workspace if valid
	if currentWs != "" {
		projFile := filepath.Join(currentWs, ".antigravity", "memos.json")
		addMemos(readMemosFile(projFile))
	}

	// 2. Known workspaces recorded in server
	if s != nil {
		for ws := range s.knownWorkspaces {
			if ws != "" && ws != currentWs {
				projFile := filepath.Join(ws, ".antigravity", "memos.json")
				addMemos(readMemosFile(projFile))
			}
		}
	}

	// 3. Global memos
	addMemos(readMemosFile(s.getGlobalMemosPath()))

	// 4. Scan known projects from Antigravity databases
	home, _ := os.UserHomeDir()
	if home != "" {
		antigravityBase := filepath.Join(home, ".gemini", "antigravity")
		appStorage := filepath.Join(home, ".config", "Antigravity", "app_storage.json")
		if projects, err := importer.GetKnownProjects(antigravityBase, appStorage); err == nil {
			for _, p := range projects {
				for _, pPath := range p.Paths {
					if pPath != "" && pPath != currentWs {
						addMemos(readMemosFile(filepath.Join(pPath, ".antigravity", "memos.json")))
					}
				}
			}
		}
		// 5. Scan ~/.gemini/config/projects/*.json
		projConfigDir := filepath.Join(home, ".gemini", "config", "projects")
		if entries, err := os.ReadDir(projConfigDir); err == nil {
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".json") && !strings.HasSuffix(e.Name(), ".deleted") {
					data, err := os.ReadFile(filepath.Join(projConfigDir, e.Name()))
					if err == nil {
						var meta struct {
							FolderURI string `json:"folderUri"`
							Path      string `json:"path"`
						}
						if err := json.Unmarshal(data, &meta); err == nil {
							targetP := meta.Path
							if targetP == "" && meta.FolderURI != "" {
								targetP = cleanUserPath(meta.FolderURI)
							}
							if targetP != "" && targetP != currentWs {
								addMemos(readMemosFile(filepath.Join(targetP, ".antigravity", "memos.json")))
							}
						}
					}
				}
			}
		}
	}

	result := make([]MemoItem, 0, len(order))
	for _, id := range order {
		result = append(result, memosMap[id])
	}
	return result
}

// resolveProjectAlias maps a bare project name to its configured absolute
// folder. Inputs containing path separators, "." or ".." are never treated as
// aliases: they are paths and must pass the absolute-path rule.
func resolveProjectAlias(raw string) string {
	name := strings.TrimSpace(raw)
	name = strings.Trim(name, "\"'`")
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return ""
	}
	resolved := github.ResolveProjectPath(name)
	if resolved == "" || !filepath.IsAbs(resolved) {
		return ""
	}
	return resolved
}

// defaultUserPath returns the working directory for requests that omit a path.
// The file API accepts absolute paths only, so "." can no longer be passed to
// validateJailedPath directly.
func defaultUserPath() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// normaliseUserPath decodes a user-supplied path without ever resolving it
// against the working directory: quotes, percent escapes, file:// prefixes, ~
// and '+' aliases are handled, but relative input stays relative. File API
// endpoints use this so that a relative path cannot silently become an
// absolute one through cwd or project lookup.
func normaliseUserPath(raw string) string {
	p := stripExtendedPrefix(strings.TrimSpace(raw))
	p = strings.Trim(p, "\"'`")
	for strings.HasPrefix(p, "file://") {
		p = strings.TrimPrefix(p, "file://")
		p = stripExtendedPrefix(p)
		if strings.HasPrefix(p, "localhost/") {
			p = strings.TrimPrefix(p, "localhost")
		}
	}
	originalBeforeUnescape := p
	// Decode percent-encoded spaces and characters (e.g. %20, %2F)
	for strings.Contains(p, "%") {
		if unescaped, err := url.QueryUnescape(p); err == nil && unescaped != p {
			p = unescaped
		} else if unescaped, err := url.PathUnescape(p); err == nil && unescaped != p {
			p = unescaped
		} else {
			break
		}
	}
	// If unescaped path does not exist on disk but original literal with % did, restore it
	if p != originalBeforeUnescape {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if _, errOrig := os.Stat(originalBeforeUnescape); errOrig == nil {
				p = originalBeforeUnescape
			}
		}
	}
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "\"'`")
	if p == "" {
		return ""
	}
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			p = home
		}
	} else if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			p = filepath.Join(home, p[2:])
		}
	}
	// If path does not exist as-is and contains '+', check if replacing '+' with ' ' exists
	if strings.Contains(p, "+") {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			withSpaces := strings.ReplaceAll(p, "+", " ")
			if _, err2 := os.Stat(withSpaces); err2 == nil {
				p = withSpaces
			}
		}
	}
	return p
}

// cleanUserPath resolves relative inputs against the working directory or a
// known project. It is used for workspace/memo bookkeeping only; file API
// requests go through normaliseUserPath and require an absolute path.
func cleanUserPath(raw string) string {
	p := normaliseUserPath(raw)
	if p == "" {
		return ""
	}
	// If relative path, check if it exists relative to working directory or resolves to a known project
	if !filepath.IsAbs(p) {
		if p == "." || strings.EqualFold(p, "global") {
			if resolved := github.ResolveProjectPath(p); filepath.IsAbs(resolved) {
				p = resolved
			}
		} else {
			if wd, err := os.Getwd(); err == nil && wd != "" {
				candidate := filepath.Join(wd, p)
				if _, err := os.Stat(candidate); err == nil {
					p = candidate
				}
			}
			if !filepath.IsAbs(p) {
				if resolved := github.ResolveProjectPath(p); filepath.IsAbs(resolved) {
					p = resolved
				}
			}
		}
	}
	return stripExtendedPrefix(filepath.Clean(p))
}

// stripExtendedPrefix removes Windows extended-length path prefixes (\\?\, \??\, //?/, \\?\UNC\, //?/UNC/).
func stripExtendedPrefix(p string) string {
	if strings.HasPrefix(p, `\\?\UNC\`) || strings.HasPrefix(p, `//?/UNC/`) {
		return `\\` + p[8:]
	}
	if strings.HasPrefix(p, `\\?\`) || strings.HasPrefix(p, `\??\`) || strings.HasPrefix(p, `//?/`) {
		return p[4:]
	}
	return p
}

// isUNCPath reports whether p begins with a UNC network path prefix (\\ or //).
func isUNCPath(p string) bool {
	p = stripExtendedPrefix(strings.TrimSpace(p))
	return strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, `//`)
}

// hasWindowsDrive reports whether p begins with a Windows drive letter (e.g. C:, D:).
func hasWindowsDrive(p string) bool {
	p = stripExtendedPrefix(strings.TrimSpace(p))
	if len(p) >= 2 {
		ch := p[0]
		if ((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) && p[1] == ':' {
			return true
		}
	}
	return false
}

// getWindowsVolume extracts the volume or drive letter from a path, regardless of the host OS.
func getWindowsVolume(p string) string {
	p = stripExtendedPrefix(strings.TrimSpace(p))
	if len(p) >= 2 {
		ch := p[0]
		if ((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) && p[1] == ':' {
			return strings.ToUpper(p[:2])
		}
	}
	if runtime.GOOS == "windows" {
		return strings.ToUpper(filepath.VolumeName(p))
	}
	return ""
}

// find8dot3Aliases discovers 8.3 short name or long canonical equivalents for a target directory
// by inspecting surrounding components and cross-referencing with environment/temp probes.
func find8dot3Aliases(target string, probes ...string) []string {
	var aliases []string
	target = stripExtendedPrefix(strings.TrimSpace(target))
	if target == "" {
		return nil
	}

	normTarget := strings.ReplaceAll(target, "\\", "/")
	normTarget = path.Clean(normTarget)
	parent := path.Dir(normTarget)
	base := path.Base(normTarget)
	if parent == normTarget || base == "" || base == "." {
		return nil
	}

	isWin := hasWindowsDrive(target) || isUNCPath(target) || strings.Contains(target, "\\") || runtime.GOOS == "windows"

	seen := make(map[string]bool)
	addAlias := func(a string) {
		a = stripExtendedPrefix(a)
		if isWin {
			a = strings.ReplaceAll(a, "/", "\\")
		} else {
			a = path.Clean(a)
		}
		key := strings.ToLower(a)
		if !seen[key] && !strings.EqualFold(a, target) {
			seen[key] = true
			aliases = append(aliases, a)
		}
	}

	for _, probe := range probes {
		probeNorm := strings.ReplaceAll(stripExtendedPrefix(probe), "\\", "/")
		probeNorm = path.Clean(probeNorm)
		parentPrefix := parent
		if !strings.HasSuffix(parentPrefix, "/") {
			parentPrefix += "/"
		}
		if strings.HasPrefix(strings.ToLower(probeNorm), strings.ToLower(parentPrefix)) {
			rel := probeNorm[len(parentPrefix):]
			parts := strings.Split(rel, "/")
			if len(parts) > 0 {
				child := parts[0]
				if !strings.EqualFold(child, base) {
					if strings.Contains(child, "~") {
						prefix := strings.Split(child, "~")[0]
						cleanBase := strings.ToLower(strings.ReplaceAll(base, " ", ""))
						if strings.HasPrefix(cleanBase, strings.ToLower(prefix)) {
							addAlias(path.Join(parent, child))
						}
					} else if strings.Contains(base, "~") {
						prefix := strings.Split(base, "~")[0]
						cleanChild := strings.ToLower(strings.ReplaceAll(child, " ", ""))
						if strings.HasPrefix(cleanChild, strings.ToLower(prefix)) {
							addAlias(path.Join(parent, child))
						}
					}
				}
			}
		}
	}

	cleanName := strings.ToUpper(strings.ReplaceAll(base, " ", ""))
	if len(cleanName) > 6 && !strings.Contains(base, "~") {
		shortCandidate := path.Join(parent, cleanName[:6]+"~1")
		if runtime.GOOS == "windows" {
			if eval, err := filepath.EvalSymlinks(shortCandidate); err == nil {
				if strings.EqualFold(stripExtendedPrefix(eval), target) {
					addAlias(shortCandidate)
				}
			} else if st, errS := os.Stat(shortCandidate); errS == nil && st.IsDir() {
				addAlias(shortCandidate)
			}
		} else if hasWindowsDrive(target) {
			addAlias(shortCandidate)
		}
	}

	return aliases
}

func normalizeMacOSPathAlias(p string) string {
	normalized := path.Clean(strings.ReplaceAll(stripExtendedPrefix(p), "\\", "/"))
	if normalized == "/var/folders" || strings.HasPrefix(normalized, "/var/folders/") ||
		normalized == "/private/var/folders" || strings.HasPrefix(normalized, "/private/var/folders/") {
		return strings.TrimPrefix(normalized, "/private")
	}
	if runtime.GOOS == "darwin" && (normalized == "/var" || strings.HasPrefix(normalized, "/var/")) {
		return "/private" + normalized
	}
	return p
}

// pathMatchesOrIsSubpath checks if candidate matches root or is inside root, with case-insensitivity on Windows
// or when Windows drive letters are present. It also strips extended-length prefixes (\\?\) and prevents double-slash boundary bugs.
func pathMatchesOrIsSubpath(candidate, root string) bool {
	candidate = stripExtendedPrefix(strings.TrimSpace(candidate))
	root = stripExtendedPrefix(strings.TrimSpace(root))
	candidate = normalizeMacOSPathAlias(candidate)
	root = normalizeMacOSPathAlias(root)

	if candidate == "" || root == "" {
		return false
	}

	isUNCCandidate := isUNCPath(candidate)
	isUNCRoot := isUNCPath(root)
	hasDriveCandidate := hasWindowsDrive(candidate)
	hasDriveRoot := hasWindowsDrive(root)

	isWindows := runtime.GOOS == "windows" || hasDriveCandidate || hasDriveRoot || isUNCCandidate || isUNCRoot

	// Cross-volume / UNC mismatch prevention
	if isUNCCandidate != isUNCRoot {
		return false
	}
	if hasDriveCandidate != hasDriveRoot {
		return false
	}
	if hasDriveCandidate && hasDriveRoot {
		if !strings.EqualFold(candidate[:2], root[:2]) {
			return false
		}
	}

	if isWindows {
		candidate = strings.ToLower(candidate)
		root = strings.ToLower(root)
	}

	// Normalize all separators to forward slash
	candidate = strings.ReplaceAll(candidate, "\\", "/")
	root = strings.ReplaceAll(root, "\\", "/")

	if isUNCCandidate {
		candidate = "//" + path.Clean(strings.TrimPrefix(candidate, "//"))
		root = "//" + path.Clean(strings.TrimPrefix(root, "//"))
	} else {
		candidate = path.Clean(candidate)
		root = path.Clean(root)
	}

	// Clean strips trailing slash; handle drive roots like C:/ which clean to C:
	if candidate == root {
		return true
	}

	prefix := root
	if len(prefix) == 2 && prefix[1] == ':' {
		prefix += "/"
	} else if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	return strings.HasPrefix(candidate, prefix)
}

// isSameOrAncestor checks if ancestorCandidate is equal to child or an ancestor of child.
func isSameOrAncestor(ancestorCandidate, child string) bool {
	ancestorCandidate = stripExtendedPrefix(strings.TrimSpace(ancestorCandidate))
	child = stripExtendedPrefix(strings.TrimSpace(child))

	ancestorCandidate = filepath.Clean(ancestorCandidate)
	child = filepath.Clean(child)

	if evalA, err := filepath.EvalSymlinks(ancestorCandidate); err == nil {
		ancestorCandidate = stripExtendedPrefix(evalA)
	}
	if evalC, err := filepath.EvalSymlinks(child); err == nil {
		child = stripExtendedPrefix(evalC)
	}

	return pathMatchesOrIsSubpath(child, ancestorCandidate)
}

// isStrictSubpath reports whether candidate is beneath root, excluding equality.
func isStrictSubpath(candidate, root string) bool {
	return pathMatchesOrIsSubpath(candidate, root) && !pathMatchesOrIsSubpath(root, candidate)
}

func tempPathRoots() []string {
	roots := []string{os.TempDir()}
	if resolved, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		roots = append(roots, resolved)
	}
	return roots
}

func isAllowedTempPath(candidate string) bool {
	for _, root := range tempPathRoots() {
		if isStrictSubpath(candidate, root) {
			return true
		}
	}
	return false
}

func isTempPathRoot(candidate string) bool {
	for _, root := range tempPathRoots() {
		if pathMatchesOrIsSubpath(candidate, root) && pathMatchesOrIsSubpath(root, candidate) {
			return true
		}
	}
	return false
}

// validateJailedPath resolves a user-supplied path for read-only operations
// (list, read, reveal, terminal/IDE launch).
//
// There is intentionally no directory whitelist: this daemon listens on the
// loopback interface and requires X-Session-Token, and the operator explicitly
// asked for unrestricted browsing (issue #157). The remaining checks are not
// browsing restrictions — they only enforce the input contract:
//   - the path must be absolute (a relative path is never resolved against the
//     working directory, so "..\..\Windows\..." cannot turn into a real path);
//   - null bytes and percent-encoded traversal/null forms are rejected;
//   - dangling or unresolvable symlinks are rejected.
//
// Reading sensitive locations (system directories, ~/.ssh, ...) is allowed.
func validateJailedPath(raw string) (string, error) {
	return resolveUserPath(raw, false)
}

// validateDestructivePath resolves a path for operations that modify or remove
// data (write, create, rename, move, delete). In addition to the read-path
// checks it blocks credential stores and OS directories. Reading those
// locations stays allowed; only mutation is refused.
func validateDestructivePath(raw string) (string, error) {
	return resolveUserPath(raw, true)
}

func resolveUserPath(raw string, destructive bool) (string, error) {
	if strings.ContainsRune(raw, 0) || strings.Contains(raw, "\x00") {
		return "", fmt.Errorf("access denied: null byte prohibited in path")
	}
	lowerRaw := strings.ToLower(raw)
	if strings.Contains(lowerRaw, "%00") || strings.Contains(lowerRaw, "%2500") {
		return "", fmt.Errorf("access denied: null byte prohibited in path")
	}
	if strings.Contains(lowerRaw, "..%2f") || strings.Contains(lowerRaw, "%2f..") ||
		strings.Contains(lowerRaw, "..%5c") || strings.Contains(lowerRaw, "%5c..") ||
		strings.Contains(lowerRaw, "%2e%2e") || strings.Contains(lowerRaw, "%2e.") ||
		strings.Contains(lowerRaw, ".%2e") || strings.Contains(lowerRaw, "%252e") ||
		strings.Contains(lowerRaw, "%252f") || strings.Contains(lowerRaw, "%255c") {
		return "", fmt.Errorf("access denied: encoded path traversal prohibited in path")
	}

	normalised := normaliseUserPath(raw)
	if normalised == "" {
		return "", fmt.Errorf("empty path")
	}
	cleaned := stripExtendedPrefix(filepath.Clean(normalised))
	if strings.ContainsRune(cleaned, 0) || strings.Contains(cleaned, "\x00") {
		return "", fmt.Errorf("access denied: null byte prohibited in path")
	}

	// Only absolute paths are accepted. Relative input is rejected rather than
	// resolved against the working directory: otherwise "../../etc/passwd"
	// turns into a valid absolute path and the traversal check becomes a no-op.
	// '~' and file:// inputs are already expanded by normaliseUserPath, so a
	// still-relative path here cannot be trusted.
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("access denied: absolute path required: %s", cleaned)
	}

	fi, errLstat := os.Lstat(cleaned)
	isSymlink := errLstat == nil && (fi.Mode()&os.ModeSymlink != 0)

	evalPath, errEval := filepath.EvalSymlinks(cleaned)
	if errEval != nil {
		// Check if cleaned itself or any ancestor directory is an unresolvable or dangling symlink
		curr := cleaned
		for curr != "" && curr != "." && curr != "/" && getWindowsVolume(curr) != curr {
			if st, errL := os.Lstat(curr); errL == nil && (st.Mode()&os.ModeSymlink != 0) {
				if _, errEvalCurr := filepath.EvalSymlinks(curr); errEvalCurr != nil {
					return "", fmt.Errorf("access denied: unresolvable or dangling symlink: %s", curr)
				}
			}
			parent := filepath.Dir(curr)
			if parent == curr {
				break
			}
			curr = parent
		}
		// File does not exist yet; find nearest existing ancestor directory to resolve 8.3 short names and symlinks
		existing := cleaned
		for existing != "" && existing != "." && existing != "/" && getWindowsVolume(existing) != existing {
			if st, errS := os.Stat(existing); errS == nil && st.IsDir() {
				break
			}
			parent := filepath.Dir(existing)
			if parent == existing {
				break
			}
			existing = parent
		}
		if evalExisting, errEE := filepath.EvalSymlinks(existing); errEE == nil {
			evalExisting = stripExtendedPrefix(evalExisting)
			if rel, errR := filepath.Rel(existing, cleaned); errR == nil {
				evalPath = stripExtendedPrefix(filepath.Clean(filepath.Join(evalExisting, rel)))
			} else {
				evalPath = cleaned
			}
		} else {
			evalPath = cleaned
		}
	} else {
		evalPath = stripExtendedPrefix(filepath.Clean(evalPath))
	}

	if destructive {
		if err := ensureNotSensitiveLocation(cleaned, evalPath); err != nil {
			return "", err
		}
	}

	if isSymlink {
		return evalPath, nil
	}
	return cleaned, nil
}

// ensureNotSensitiveLocation is the destructive-operation floor. The paths below
// hold credentials or operating-system binaries: reading them is allowed (the
// operator asked for unrestricted browsing), but writing, renaming, moving,
// creating inside them, or deleting them is refused.
func ensureNotSensitiveLocation(cleaned, evalPath string) error {
	candidates := []string{cleaned}
	if evalPath != "" && !strings.EqualFold(stripExtendedPrefix(evalPath), stripExtendedPrefix(cleaned)) {
		candidates = append(candidates, evalPath)
	}

	systemPrefixes := []string{
		"/etc", "/proc", "/sys", "/dev", "/boot", "/root", "/var", "/private/var", "/bin", "/sbin", "/usr",
		"c:/windows", "c:/program files", "c:/program files (x86)", "c:/programdata",
	}
	for _, candidate := range candidates {
		norm := strings.ToLower(strings.ReplaceAll(stripExtendedPrefix(candidate), "\\", "/"))
		for _, sysPrefix := range systemPrefixes {
			if (sysPrefix == "/var" || sysPrefix == "/private/var") && isAllowedTempPath(candidate) {
				continue
			}
			if norm == sysPrefix || strings.HasPrefix(norm, sysPrefix+"/") {
				return fmt.Errorf("access denied: destructive operation on system directory %q", sysPrefix)
			}
		}
		if strings.Contains(norm, "windows/system32") {
			return fmt.Errorf("access denied: destructive operation on system directory Windows/System32")
		}
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	homeRoots := []string{home}
	if eh, errH := filepath.EvalSymlinks(home); errH == nil {
		homeRoots = append(homeRoots, stripExtendedPrefix(eh))
	}
	homeRoots = append(homeRoots, find8dot3Aliases(home, os.TempDir())...)
	for _, h := range homeRoots {
		sensitivePaths := []string{
			filepath.Join(h, ".ssh"),
			filepath.Join(h, ".aws"),
			filepath.Join(h, ".gnupg"),
			filepath.Join(h, ".bashrc"),
			filepath.Join(h, ".bash_profile"),
			filepath.Join(h, ".profile"),
			filepath.Join(h, ".zshrc"),
			filepath.Join(h, ".config", "gcloud"),
			// Browser credential stores.
			filepath.Join(h, ".mozilla"),
			filepath.Join(h, ".config", "google-chrome"),
			filepath.Join(h, ".config", "chromium"),
			filepath.Join(h, "AppData", "Local", "Google", "Chrome", "User Data"),
			filepath.Join(h, "AppData", "Local", "Microsoft", "Edge", "User Data"),
			filepath.Join(h, "AppData", "Local", "BraveSoftware"),
			filepath.Join(h, "AppData", "Roaming", "Mozilla", "Firefox", "Profiles"),
		}
		for _, sp := range sensitivePaths {
			for _, candidate := range candidates {
				if pathMatchesOrIsSubpath(candidate, sp) {
					return fmt.Errorf("access denied: destructive operation on sensitive location %q", sp)
				}
			}
		}
	}
	return nil
}

// validateDeletablePath strictly validates paths requested for deletion.
// There is no directory whitelist (issue #157); instead it prevents destructive
// wipes by:
//  1. Explicitly rejecting "", "~", ".", "/", "\", and tilde root shortcuts.
//  2. Explicitly rejecting any candidate containing ".." parent directory traversal.
//  3. Rejecting Windows drive roots (e.g. C:\, D:\, C:) and filesystem roots.
//  4. Rejecting system and credential-store directories (via validateDestructivePath).
//  5. Prohibiting deletion of the current working directory, config directory,
//     user home directory, or any of their parent/ancestor directories.
func validateDeletablePath(target string) (string, error) {
	trimmed := strings.TrimSpace(target)
	trimmed = strings.Trim(trimmed, "\"'`")
	if trimmed == "" || trimmed == "~" || trimmed == "." || trimmed == "/" || trimmed == "\\" {
		return "", fmt.Errorf("refusing to delete root or top-level alias: %q", target)
	}
	if trimmed == "~/" || trimmed == "~\\" || strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, "~\\") {
		sub := strings.TrimPrefix(strings.TrimPrefix(trimmed, "~/"), "~\\")
		if strings.TrimSpace(sub) == "" {
			return "", fmt.Errorf("refusing to delete home directory root: %q", target)
		}
	}

	// Explicitly reject any candidate containing '..' parent directory traversal
	lowerTrimmed := strings.ToLower(trimmed)
	if strings.Contains(target, "..") || strings.Contains(trimmed, "..") ||
		strings.Contains(lowerTrimmed, "..") || strings.Contains(lowerTrimmed, "%2e") ||
		strings.Contains(lowerTrimmed, "%252e") || strings.Contains(lowerTrimmed, "..%2f") ||
		strings.Contains(lowerTrimmed, "..%5c") {
		return "", fmt.Errorf("prohibited: cannot delete path containing parent traversal '..': %q", target)
	}

	normalized := filepath.ToSlash(trimmed)
	for _, sys := range []string{"/etc", "/usr", "/var", "/private/var", "/bin", "/sbin", "/boot", "/proc", "/sys", "/dev", "/root"} {
		if (sys == "/var" || sys == "/private/var") && isAllowedTempPath(normalized) {
			continue
		}
		if normalized == sys || strings.HasPrefix(normalized, sys+"/") {
			return "", fmt.Errorf("prohibited: cannot delete system directory: %s", target)
		}
	}

	if runtime.GOOS == "windows" || hasWindowsDrive(trimmed) {
		if (strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "\\")) && getWindowsVolume(trimmed) == "" {
			return "", fmt.Errorf("prohibited: drive-relative or root-relative path not permitted for deletion: %q", target)
		}
	}

	cPath, err := validateDestructivePath(target)
	if err != nil {
		return "", err
	}

	cleanPath := stripExtendedPrefix(filepath.Clean(cPath))
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("prohibited: cannot delete path containing parent traversal '..': %s", cleanPath)
	}

	// 1. Explicitly block Windows drive roots (e.g. C:\, D:\, C:, D:)
	vol := getWindowsVolume(cleanPath)
	if vol != "" {
		cleanNorm := strings.ReplaceAll(cleanPath, "/", "\\")
		if strings.EqualFold(cleanNorm, vol) || strings.EqualFold(cleanNorm, vol+"\\") {
			return "", fmt.Errorf("prohibited: cannot delete filesystem volume root: %s", cleanPath)
		}
	}

	// 2. Explicitly block Unix root / and top-level aliases
	if cleanPath == "/" || cleanPath == "\\" || cleanPath == "." {
		return "", fmt.Errorf("prohibited: cannot delete filesystem root")
	}

	// Reject the temporary root even when it is outside the listed system directories.
	if isTempPathRoot(cleanPath) {
		return "", fmt.Errorf("prohibited: cannot delete temporary directory root: %s", cleanPath)
	}

	// 3. Explicitly block system directories
	systemDirs := []string{
		"/etc", "/usr", "/var", "/private/var", "/bin", "/sbin", "/boot", "/proc", "/sys", "/dev", "/root",
		"C:\\Windows", "C:\\Program Files", "C:\\Program Files (x86)", "C:\\ProgramData",
	}
	for _, sysDir := range systemDirs {
		if (sysDir == "/var" || sysDir == "/private/var") && isAllowedTempPath(cleanPath) {
			continue
		}
		if strings.EqualFold(cleanPath, filepath.Clean(sysDir)) || pathMatchesOrIsSubpath(cleanPath, sysDir) {
			return "", fmt.Errorf("prohibited: cannot delete system directory: %s", cleanPath)
		}
	}

	// 4. Prohibit deleting current working directory or any of its parent/ancestor directories
	if wd, errW := os.Getwd(); errW == nil && wd != "" {
		if isSameOrAncestor(cleanPath, wd) {
			return "", fmt.Errorf("prohibited: cannot delete working directory or any parent/ancestor directory: %s", cleanPath)
		}
	}

	// Prohibit deleting config directory or any of its parent/ancestor directories
	if cfgDir := core.GetConfigDir(); cfgDir != "" {
		if isSameOrAncestor(cleanPath, cfgDir) {
			return "", fmt.Errorf("prohibited: cannot delete configuration directory or any parent/ancestor directory: %s", cleanPath)
		}
	}

	// Prohibit deleting user home directory or any of its parent/ancestor directories
	if home, errH := os.UserHomeDir(); errH == nil && home != "" {
		if isSameOrAncestor(cleanPath, home) {
			return "", fmt.Errorf("prohibited: cannot delete user home directory or any parent/ancestor directory: %s", cleanPath)
		}
		for _, alias := range find8dot3Aliases(home, os.TempDir()) {
			if isSameOrAncestor(cleanPath, alias) {
				return "", fmt.Errorf("prohibited: cannot delete user home directory or any parent/ancestor directory: %s", cleanPath)
			}
		}
	}

	return cleanPath, nil
}

func (s *Server) handleMemos(w http.ResponseWriter, r *http.Request) {
	s.memoMu.RLock()
	defer s.memoMu.RUnlock()

	cfg, _ := core.LoadConfig()
	memoCfg := cfg.GetMemoConfig()

	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = memoCfg.ViewScope
	}
	storageQuery := r.URL.Query().Get("storage")
	ws := s.resolveWorkspacePath(r)

	var memos []MemoItem
	fallback := false

	if scope == "current" || (scope == "" && storageQuery == "project") {
		if ws != "" {
			fi, err := os.Stat(ws)
			if err == nil && fi.IsDir() {
				projFile := filepath.Join(ws, ".antigravity", "memos.json")
				projMemos := readMemosFile(projFile)
				globalMemos := readMemosFile(s.getGlobalMemosPath())
				seen := make(map[string]bool)
				for _, m := range projMemos {
					memos = append(memos, m)
					seen[m.ID] = true
				}
				for _, m := range globalMemos {
					if !seen[m.ID] {
						cleanMPath := cleanUserPath(m.WorkspacePath)
						if cleanMPath == ws || (m.Project != "" && m.Project == filepath.Base(ws)) {
							memos = append(memos, m)
							seen[m.ID] = true
						}
					}
				}
			} else {
				memos = readMemosFile(s.getGlobalMemosPath())
				fallback = true
			}
		} else {
			memos = readMemosFile(s.getGlobalMemosPath())
			fallback = true
		}
	} else {
		// scope == "all"
		memos = s.loadAllProjectMemos(ws)
	}

	if memos == nil {
		memos = []MemoItem{}
	}
	res := map[string]interface{}{"success": true, "memos": memos}
	if fallback {
		res["fallback"] = true
	}
	writeJSON(w, res)
}

func (s *Server) handleMemosSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var memo MemoItem
	if err := json.NewDecoder(r.Body).Decode(&memo); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if memo.ID == "" {
		memo.ID = fmt.Sprintf("memo-%d", time.Now().UnixNano())
	}
	if memo.CreatedAt == "" {
		memo.CreatedAt = time.Now().Format("2006-01-02 15:04")
	}

	ws := s.resolveWorkspacePath(r)
	if memo.WorkspacePath == "" && ws != "" {
		memo.WorkspacePath = ws
	}
	if memo.WorkspacePath != "" {
		memo.WorkspacePath = cleanUserPath(memo.WorkspacePath)
	}
	if memo.Project == "" && memo.WorkspacePath != "" {
		memo.Project = filepath.Base(memo.WorkspacePath)
	}

	storageQuery := r.URL.Query().Get("storage")
	storagePath, isFallback := s.resolveStorageDestination(r, storageQuery, memo.WorkspacePath)
	memoCfg := loadMemoConfig()

	s.memoMu.Lock()

	if memo.WorkspacePath != "" {
		if fi, err := os.Stat(memo.WorkspacePath); err == nil && fi.IsDir() {
			if s.knownWorkspaces == nil {
				s.knownWorkspaces = make(map[string]struct{})
			}
			s.knownWorkspaces[memo.WorkspacePath] = struct{}{}
		}
	}

	memos := readMemosFile(storagePath)
	found := false
	for i, m := range memos {
		if m.ID == memo.ID {
			memos[i] = memo
			found = true
			break
		}
	}
	if !found {
		memos = append([]MemoItem{memo}, memos...)
	}
	writeErr := writeMemosFile(storagePath, memos)
	s.memoMu.Unlock()

	if writeErr != nil {
		http.Error(w, "failed to persist memo: "+writeErr.Error(), http.StatusInternalServerError)
		return
	}

	res := map[string]interface{}{
		"success": true,
		"memo":    memo,
	}
	if isFallback {
		res["fallback"] = true
		res["storage_location_effective"] = "global"
	}
	writeJSON(w, res)

	// Extraction runs after the response so it can never slow down or fail the
	// save; the memo itself is already persisted at this point.
	s.scheduleMemoAutoExtract(memo, storagePath, memoCfg)
}

func (s *Server) handleMemosDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		id = req.ID
	}
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	ws := s.resolveWorkspacePath(r)
	storageQuery := r.URL.Query().Get("storage")
	if storageQuery == "" && ws != "" {
		if fi, err := os.Stat(filepath.Join(ws, ".antigravity", "memos.json")); err == nil && !fi.IsDir() {
			storageQuery = "project"
		}
	}
	storagePath, _ := s.resolveStorageDestination(r, storageQuery, ws)

	s.memoMu.Lock()
	defer s.memoMu.Unlock()

	memos := readMemosFile(storagePath)
	filtered := make([]MemoItem, 0, len(memos))
	for _, m := range memos {
		if m.ID != id {
			filtered = append(filtered, m)
		}
	}
	_ = writeMemosFile(storagePath, filtered)

	globalPath := s.getGlobalMemosPath()
	if storagePath != globalPath {
		gMemos := readMemosFile(globalPath)
		gFiltered := make([]MemoItem, 0, len(gMemos))
		gChanged := false
		for _, m := range gMemos {
			if m.ID == id {
				gChanged = true
			} else {
				gFiltered = append(gFiltered, m)
			}
		}
		if gChanged {
			_ = writeMemosFile(globalPath, gFiltered)
		}
	}

	writeJSON(w, map[string]interface{}{"success": true, "deleted": id})
}

// memoConfigPayload is the wire shape accepted by POST /api/memos/config.
// The pointer fields let an absent value mean "keep the current setting",
// which matters for auto_extract: a plain bool cannot distinguish false from
// absent.
type memoConfigPayload struct {
	StorageLocation string  `json:"storage_location"`
	ViewScope       string  `json:"view_scope"`
	SearchScope     string  `json:"search_scope"`
	AutoExtract     *bool   `json:"auto_extract"`
	VoiceLanguage   *string `json:"voice_language"`
	OCRLanguage     *string `json:"ocr_language"`
}

// apply overlays the fields that were present on m.
func (p memoConfigPayload) apply(m *core.MemoConfig) {
	if p.StorageLocation != "" {
		m.StorageLocation = p.StorageLocation
	}
	if p.ViewScope != "" {
		m.ViewScope = p.ViewScope
	}
	if p.SearchScope != "" {
		m.SearchScope = p.SearchScope
	}
	if p.AutoExtract != nil {
		m.AutoExtract = *p.AutoExtract
	}
	if p.VoiceLanguage != nil {
		m.VoiceLanguage = *p.VoiceLanguage
	}
	if p.OCRLanguage != nil {
		m.OCRLanguage = *p.OCRLanguage
	}
}

func memoConfigResponse(m core.MemoConfig) map[string]interface{} {
	return map[string]interface{}{
		"success":          true,
		"config":           m,
		"storage_location": m.StorageLocation,
		"view_scope":       m.ViewScope,
		"search_scope":     m.SearchScope,
		"auto_extract":     m.AutoExtract,
		"voice_language":   m.VoiceLanguage,
		"ocr_language":     m.OCRLanguage,
	}
}

func (s *Server) handleMemosConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.DefaultConfig()
	}

	if r.Method == http.MethodGet {
		writeJSON(w, memoConfigResponse(cfg.GetMemoConfig()))
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			memoConfigPayload
			Config *memoConfigPayload `json:"config,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		current := cfg.GetMemoConfig()
		if req.Config != nil {
			req.Config.apply(&current)
		}
		req.memoConfigPayload.apply(&current)
		if current.StorageLocation != "project" && current.StorageLocation != "global" {
			current.StorageLocation = "global"
		}
		if current.ViewScope != "current" && current.ViewScope != "all" {
			current.ViewScope = "all"
		}
		if current.SearchScope != "all" && current.SearchScope != "text" {
			current.SearchScope = "text"
		}

		if err := cfg.SetMemoConfig(current); err != nil {
			http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, memoConfigResponse(cfg.GetMemoConfig()))
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// FileItem represents a filesystem entry returned by /api/files/list
type FileItem struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Type    string `json:"type"`
	Size    string `json:"size"`
	Path    string `json:"path"`
	ModTime string `json:"modTime"`
}

func (s *Server) handleFilesList(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" && strings.Contains(r.URL.RawQuery, "path=") {
		for _, part := range strings.Split(r.URL.RawQuery, "&") {
			if strings.HasPrefix(part, "path=") {
				rawPath = strings.TrimPrefix(part, "path=")
				break
			}
		}
	}
	if rawPath == "" {
		rawPath = defaultUserPath()
	}
	// A bare project name (no path separators) is a lookup key configured in
	// ~/.gemini/config/projects, not a relative filesystem path. Map it to its
	// absolute folder before validation; everything that looks like a path is
	// left to the absolute-path rule and stays rejected when relative.
	if !filepath.IsAbs(rawPath) {
		if resolved := resolveProjectAlias(rawPath); resolved != "" {
			rawPath = resolved
		}
	}
	dirPath, errJail := validateJailedPath(rawPath)
	if errJail != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": errJail.Error(), "path": rawPath, "files": []FileItem{}})
		return
	}

	fi, errStat := os.Stat(dirPath)
	// If dirPath does not exist or is not a directory, attempt resolving rawPath via project configuration
	if errStat != nil || !fi.IsDir() {
		if resolved := github.ResolveProjectPath(rawPath); resolved != "" && resolved != dirPath {
			if fiRes, errRes := os.Stat(resolved); errRes == nil && fiRes.IsDir() {
				if jailedRes, errJ := validateJailedPath(resolved); errJ == nil {
					dirPath = jailedRes
					fi = fiRes
					errStat = nil
				}
			}
		}
	}

	if errStat != nil {
		writeJSONStatus(w, filesListErrorStatus(errStat), map[string]interface{}{"success": false, "error": errStat.Error(), "path": dirPath, "files": []FileItem{}})
		return
	}
	if !fi.IsDir() {
		writeJSONStatus(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": fmt.Sprintf("not a directory: %s", dirPath), "path": dirPath, "files": []FileItem{}})
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		writeJSONStatus(w, filesListErrorStatus(err), map[string]interface{}{"success": false, "error": err.Error(), "path": dirPath, "files": []FileItem{}})
		return
	}

	files := []FileItem{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		itemPath := filepath.Join(dirPath, entry.Name())
		fType := "other"
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".md", ".markdown":
			fType = "markdown"
		case ".pdf":
			fType = "pdf"
		case ".xlsx", ".xls", ".csv", ".tsv":
			fType = "office"
		case ".ts", ".tsx", ".js", ".jsx", ".go", ".json", ".py", ".sh", ".html", ".css", ".rs", ".java", ".c", ".cpp":
			fType = "code"
		}
		sizeStr := ""
		if !entry.IsDir() {
			s := info.Size()
			if s >= 1024*1024 {
				sizeStr = fmt.Sprintf("%.1f MB", float64(s)/(1024*1024))
			} else if s >= 1024 {
				sizeStr = fmt.Sprintf("%.1f KB", float64(s)/1024)
			} else {
				sizeStr = fmt.Sprintf("%d B", s)
			}
		}
		files = append(files, FileItem{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Type:    fType,
			Size:    sizeStr,
			Path:    itemPath,
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
		})
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"path":    dirPath,
		"files":   files,
	})
}

func filesListErrorStatus(err error) int {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, fs.ErrPermission):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func (s *Server) handleFilesRead(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" && strings.Contains(r.URL.RawQuery, "path=") {
		for _, part := range strings.Split(r.URL.RawQuery, "&") {
			if strings.HasPrefix(part, "path=") {
				rawPath = strings.TrimPrefix(part, "path=")
				break
			}
		}
	}
	filePath, errJail := validateJailedPath(rawPath)
	if errJail != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": errJail.Error(), "path": rawPath})
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error(), "path": filePath})
		return
	}

	// Limit response string length to 1MB to avoid browser freeze
	maxLen := 1024 * 1024
	content := string(data)
	if len(content) > maxLen {
		content = content[:maxLen] + "\n\n...[Truncated: file exceeds 1MB preview limit]..."
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"path":    filePath,
		"content": content,
		"size":    len(data),
	})
}

func (s *Server) handleFilesWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Path == "" {
		http.Error(w, "missing path parameter", http.StatusBadRequest)
		return
	}
	targetPath, errJail := validateDestructivePath(p.Path)
	if errJail != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": errJail.Error(), "path": p.Path})
		return
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	if err := os.WriteFile(targetPath, []byte(p.Content), 0600); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"success": true, "path": targetPath, "size": len(p.Content)})
}

func (s *Server) handleFilesRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.OldPath == "" || p.NewPath == "" {
		http.Error(w, "missing old_path or new_path", http.StatusBadRequest)
		return
	}
	oldPath, errOld := validateDestructivePath(p.OldPath)
	if errOld != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": "old_path: " + errOld.Error()})
		return
	}
	newPath, errNew := validateDestructivePath(p.NewPath)
	if errNew != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": "new_path: " + errNew.Error()})
		return
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"success": true, "old_path": oldPath, "new_path": newPath})
}

func (s *Server) handleFilesDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Path  string   `json:"path"`
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	targets := p.Paths
	if len(targets) == 0 && p.Path != "" {
		targets = []string{p.Path}
	}
	if len(targets) == 0 {
		http.Error(w, "missing path or paths", http.StatusBadRequest)
		return
	}

	var deleted []string
	for _, target := range targets {
		cPath, errJail := validateDeletablePath(target)
		if errJail != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": errJail.Error(), "partial_deleted": deleted})
			return
		}
		if err := os.RemoveAll(cPath); err != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": err.Error(), "partial_deleted": deleted})
			return
		}
		deleted = append(deleted, cPath)
	}
	writeJSON(w, map[string]interface{}{"success": true, "deleted": p.Path, "paths": deleted})
}

func (s *Server) handleFilesCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	cPath, errJail := validateDestructivePath(p.Path)
	if errJail != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": errJail.Error()})
		return
	}
	var err error
	if p.IsDir {
		err = os.MkdirAll(cPath, 0700)
	} else {
		_ = os.MkdirAll(filepath.Dir(cPath), 0700)
		err = os.WriteFile(cPath, []byte(""), 0600)
	}
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"success": true, "path": cPath, "is_dir": p.IsDir})
}

func copyPathRecursive(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		if err := os.MkdirAll(dst, 0700); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			srcChild := filepath.Join(src, entry.Name())
			dstChild := filepath.Join(dst, entry.Name())
			if err := copyPathRecursive(srcChild, dstChild); err != nil {
				return err
			}
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}

	// Handle symlink
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := os.Readlink(src)
		if err == nil {
			_ = os.Remove(dst)
			return os.Symlink(linkTarget, dst)
		}
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return dstFile.Sync()
}

func resolveCopyMoveDst(src, dst string, isCopy bool) string {
	targetDst := dst
	if fi, err := os.Stat(targetDst); err == nil && fi.IsDir() {
		if filepath.Clean(src) != filepath.Clean(targetDst) && filepath.Base(src) != filepath.Base(targetDst) {
			targetDst = filepath.Join(targetDst, filepath.Base(src))
		}
	}
	if isCopy {
		if _, err := os.Stat(targetDst); err == nil || filepath.Clean(src) == filepath.Clean(targetDst) {
			targetDst = generateUniquePath(targetDst)
		}
	}
	return targetDst
}

func generateUniquePath(p string) string {
	dir := filepath.Dir(p)
	base := filepath.Base(p)
	fi, err := os.Stat(p)
	isDir := err == nil && fi.IsDir()

	ext := ""
	stem := base
	if !isDir {
		ext = filepath.Ext(base)
		stem = strings.TrimSuffix(base, ext)
	}

	candidate := filepath.Join(dir, fmt.Sprintf("%s (copy)%s", stem, ext))
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}

	for i := 2; i < 1000; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (copy %d)%s", stem, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return candidate
}

func (s *Server) handleFilesCopy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Src   string `json:"src"`
		Dst   string `json:"dst"`
		Items []struct {
			Src string `json:"src"`
			Dst string `json:"dst"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	items := p.Items
	if len(items) == 0 && p.Src != "" && p.Dst != "" {
		items = []struct {
			Src string `json:"src"`
			Dst string `json:"dst"`
		}{
			{Src: p.Src, Dst: p.Dst},
		}
	}
	if len(items) == 0 {
		http.Error(w, "missing src or dst", http.StatusBadRequest)
		return
	}

	type copyResult struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	}
	var results []copyResult

	for _, item := range items {
		if strings.TrimSpace(item.Src) == "" || strings.TrimSpace(item.Dst) == "" {
			continue
		}
		src, errSrc := validateJailedPath(item.Src)
		if errSrc != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": "src: " + errSrc.Error()})
			return
		}
		dst, errDst := validateDestructivePath(item.Dst)
		if errDst != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": "dst: " + errDst.Error()})
			return
		}
		targetDst := resolveCopyMoveDst(src, dst, true)
		if err := copyPathRecursive(src, targetDst); err != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		results = append(results, copyResult{Src: src, Dst: targetDst})
	}

	resp := map[string]interface{}{
		"success": true,
		"results": results,
		"count":   len(results),
	}
	if len(results) == 1 {
		resp["src"] = results[0].Src
		resp["dst"] = results[0].Dst
	}
	writeJSON(w, resp)
}

func (s *Server) handleFilesMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Src   string `json:"src"`
		Dst   string `json:"dst"`
		Items []struct {
			Src string `json:"src"`
			Dst string `json:"dst"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	items := p.Items
	if len(items) == 0 && p.Src != "" && p.Dst != "" {
		items = []struct {
			Src string `json:"src"`
			Dst string `json:"dst"`
		}{
			{Src: p.Src, Dst: p.Dst},
		}
	}
	if len(items) == 0 {
		http.Error(w, "missing src or dst", http.StatusBadRequest)
		return
	}

	type moveResult struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	}
	var results []moveResult

	for _, item := range items {
		if strings.TrimSpace(item.Src) == "" || strings.TrimSpace(item.Dst) == "" {
			continue
		}
		src, errSrc := validateDestructivePath(item.Src)
		if errSrc != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": "src: " + errSrc.Error()})
			return
		}
		dst, errDst := validateDestructivePath(item.Dst)
		if errDst != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": "dst: " + errDst.Error()})
			return
		}
		targetDst := resolveCopyMoveDst(src, dst, false)
		_ = os.MkdirAll(filepath.Dir(targetDst), 0700)
		if err := os.Rename(src, targetDst); err != nil {
			// Cross-device link fallback or directory move fallback
			if copyErr := copyPathRecursive(src, targetDst); copyErr != nil {
				writeJSON(w, map[string]interface{}{"success": false, "error": fmt.Sprintf("move failed: %v", copyErr)})
				return
			}
			_ = os.RemoveAll(src)
		}
		results = append(results, moveResult{Src: src, Dst: targetDst})
	}

	resp := map[string]interface{}{
		"success": true,
		"results": results,
		"count":   len(results),
	}
	if len(results) == 1 {
		resp["src"] = results[0].Src
		resp["dst"] = results[0].Dst
	}
	writeJSON(w, resp)
}

func (s *Server) handleFilesReveal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Path string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	raw := p.Path
	if raw == "" {
		raw = defaultUserPath()
	}
	target, errJail := validateJailedPath(raw)
	if errJail != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": errJail.Error()})
		return
	}
	isFile := false
	if fi, err := os.Stat(target); err == nil && !fi.IsDir() {
		isFile = true
	}
	if !core.IsRunningTests() && !gui.IsTestExecution() {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			cleanPath := filepath.Clean(target)
			if isFile {
				cmd = exec.Command("explorer.exe", "/select,"+cleanPath)
			} else {
				cmd = exec.Command("explorer.exe", cleanPath)
			}
		case "darwin":
			if isFile {
				cmd = exec.Command("open", "-R", target)
			} else {
				cmd = exec.Command("open", target)
			}
		default:
			dir := target
			if isFile {
				dir = filepath.Dir(target)
			}
			cmd = exec.Command("xdg-open", dir)
		}
		if cmd != nil {
			_ = core.LaunchDetachedProcess(cmd)
		}
	}
	revealed := target
	if isFile && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		revealed = filepath.Dir(target)
	}
	writeJSON(w, map[string]interface{}{"success": true, "revealed": revealed})
}

func (s *Server) handleFilesTerminal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Path string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	raw := p.Path
	if raw == "" {
		raw = defaultUserPath()
	}
	dir, errJail := validateJailedPath(raw)
	if errJail != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": errJail.Error()})
		return
	}
	if fi, err := os.Stat(dir); err == nil && !fi.IsDir() {
		dir = filepath.Dir(dir)
	}
	if core.IsRunningTests() || gui.IsTestExecution() {
		writeJSON(w, map[string]interface{}{"success": true, "dir": dir})
		return
	}
	var terms [][]string
	if runtime.GOOS == "windows" {
		terms = [][]string{
			{"wt.exe", "-d", dir},
			{"powershell.exe"},
			{"cmd.exe", "/c", "start", "cmd.exe"},
		}
	} else if runtime.GOOS == "darwin" {
		terms = [][]string{
			{"open", "-a", "Terminal", dir},
		}
	} else {
		terms = [][]string{
			{"ptyxis", "--working-directory=" + dir},
			{"gnome-terminal", "--working-directory=" + dir},
			{"konsole", "--workdir", dir},
			{"alacritty", "--working-directory", dir},
			{"kitty", "--directory", dir},
			{"xfce4-terminal", "--working-directory=" + dir},
			{"x-terminal-emulator"},
			{"xterm"},
		}
	}
	spawned := false
	for _, term := range terms {
		cmd := exec.Command(term[0], term[1:]...)
		cmd.Dir = dir
		if err := core.LaunchDetachedProcess(cmd); err == nil {
			spawned = true
			break
		}
	}
	writeJSON(w, map[string]interface{}{"success": spawned, "dir": dir})
}

func (s *Server) handleFilesOpenIDE(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		pref := "code"
		if cfg, err := core.LoadConfig(); err == nil {
			pref = cfg.GetPreferredIDE()
		}
		writeJSON(w, map[string]interface{}{
			"success":       true,
			"preferred_ide": pref,
		})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Path string `json:"path"`
		IDE  string `json:"ide"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	rawDir := p.Path
	if rawDir == "" {
		rawDir = defaultUserPath()
	}
	dir, errJail := validateJailedPath(rawDir)
	if errJail != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": "invalid path: " + errJail.Error()})
		return
	}
	if fi, err := os.Stat(dir); err == nil && !fi.IsDir() {
		dir = filepath.Dir(dir)
	}

	targetIDE := strings.TrimSpace(p.IDE)
	if targetIDE == "" {
		if cfg, err := core.LoadConfig(); err == nil {
			targetIDE = cfg.GetPreferredIDE()
		}
	}
	if targetIDE == "" {
		targetIDE = "code"
	}

	canonicalIDE, errIDE := ipc.ValidateIDE(targetIDE)
	if errIDE != nil {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   "disallowed or invalid IDE: " + errIDE.Error(),
		})
		return
	}

	candidates := []string{canonicalIDE}
	if canonicalIDE == "code" {
		candidates = append(candidates, "vscode")
	} else if canonicalIDE == "vscodium" {
		candidates = append(candidates, "codium")
	}

	if canonicalIDE == "code" || p.IDE == "" {
		for _, fb := range []string{"cursor", "windsurf", "codium", "vscodium", "zed"} {
			found := false
			for _, c := range candidates {
				if c == fb {
					found = true
					break
				}
			}
			if !found {
				candidates = append(candidates, fb)
			}
		}
	}

	_ = core.EnsureVSCodeWindowCloseGuard()

	if core.IsRunningTests() || gui.IsTestExecution() {
		spawned := false
		launchedCandidate := ""
		if os.Getenv("ANTIGRAVITY_ALLOW_TEST_OPEN_IDE") == "1" {
			spawned, launchedCandidate = s.launchSandboxedIDE(dir, candidates)
		} else if len(candidates) > 0 {
			launchedCandidate = candidates[0]
		}
		writeJSON(w, map[string]interface{}{
			"success":  true,
			"dir":      dir,
			"ide":      canonicalIDE,
			"launched": launchedCandidate,
			"spawned":  spawned,
		})
		return
	}

	spawned, launched := launchIDETarget(dir, candidates)

	writeJSON(w, map[string]interface{}{
		"success":  spawned,
		"dir":      dir,
		"ide":      canonicalIDE,
		"launched": launched,
	})
}

func launchIDETarget(dir string, candidates []string) (bool, string) {
	for _, cand := range candidates {
		canonical, err := ipc.ValidateIDE(cand)
		if err != nil {
			continue
		}
		binPath := canonical
		if resolved, lookErr := exec.LookPath(canonical); lookErr == nil {
			binPath = resolved
		}
		cmd := core.CommandForExecutable(binPath, dir)
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			cmd.Dir = dir
		}
		if err := core.LaunchDetachedProcess(cmd); err == nil {
			return true, cand
		}
	}
	return false, ""
}

// launchSandboxedIDE opens the target in a throwaway, isolated editor instance
// so test runs never touch the user's real IDE windows. VS Code-family binaries
// get --transient, which runs with temporary user-data and extension dirs. The
// spawned process group is recorded and killed by Server.Stop.
func (s *Server) launchSandboxedIDE(dir string, candidates []string) (bool, string) {
	for _, cand := range candidates {
		canonical, err := ipc.ValidateIDE(cand)
		if err != nil {
			continue
		}
		var args []string
		if isVSCodeFamilyBin(canonical) {
			args = append(args, "--transient", "--new-window", "--disable-extensions")
		}
		args = append(args, dir)
		binPath := canonical
		if resolved, lookErr := exec.LookPath(canonical); lookErr == nil {
			binPath = resolved
		}
		cmd := core.CommandForExecutable(binPath, args...)
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			cmd.Dir = dir
		}
		if err := core.LaunchDetachedProcess(cmd); err != nil {
			continue
		}
		s.testIDEsMu.Lock()
		s.testIDEs = append(s.testIDEs, cmd.Process.Pid)
		s.testIDEsMu.Unlock()
		return true, cand
	}
	return false, ""
}

func isVSCodeFamilyBin(bin string) bool {
	base := strings.ToLower(filepath.Base(bin))
	for _, name := range []string{"code", "code-insiders", "codium", "vscodium", "cursor", "windsurf"} {
		if base == name {
			return true
		}
	}
	return false
}

func (s *Server) handleFilesIDEConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		pref := "code"
		if cfg, err := core.LoadConfig(); err == nil {
			pref = cfg.GetPreferredIDE()
		}
		writeJSON(w, map[string]interface{}{
			"success":       true,
			"preferred_ide": pref,
		})
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			PreferredIDE string `json:"preferred_ide"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		ide := strings.TrimSpace(req.PreferredIDE)
		if ide == "" {
			ide = "code"
		}
		canonicalIDE, errIDE := ipc.ValidateIDE(ide)
		if errIDE != nil {
			writeJSON(w, map[string]interface{}{
				"success": false,
				"error":   "disallowed IDE: " + errIDE.Error(),
			})
			return
		}
		cfg, err := core.LoadConfig()
		if err != nil {
			cfg = core.DefaultConfig()
		}
		_ = cfg.SetPreferredIDE(canonicalIDE)
		writeJSON(w, map[string]interface{}{
			"success":       true,
			"preferred_ide": canonicalIDE,
		})
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleConversationsActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	targetApp := r.URL.Query().Get("target_app")
	if targetApp == "" {
		targetApp = r.URL.Query().Get("app")
	}

	var info *revival.ActiveSessionInfo
	if s.backend == nil && s.client != nil {
		_ = s.client.Call("swiss.getActiveConversations", map[string]interface{}{"target_app": targetApp}, &info)
	}
	engine := s.getRevivalEngine()
	if info == nil && engine != nil {
		detected, err := engine.Detector.Detect(targetApp)
		if err == nil {
			info = detected
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if info != nil {
		_ = json.NewEncoder(w).Encode(info)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{})
}

func (s *Server) handleConversationsRevive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		TargetApp      string `json:"target_app"`
		ConversationID string `json:"conversation_id"`
		Prompt         string `json:"prompt"`
		ForcePrompt    bool   `json:"force_prompt"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)

	if p.ConversationID != "" && !gui.IsValidConversationID(p.ConversationID) {
		http.Error(w, "invalid conversation_id", http.StatusBadRequest)
		return
	}

	var res map[string]interface{}
	if s.backend != nil {
		engine := s.getRevivalEngine()
		if engine == nil {
			http.Error(w, "revival engine not available", http.StatusInternalServerError)
			return
		}
		if err := engine.ReviveConversation(p.TargetApp, p.ConversationID, p.Prompt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		res = map[string]interface{}{
			"success": true,
			"status":  "revived",
			"method":  "cdp_nudge",
		}
	} else {
		var err error
		if s.client != nil {
			err = s.client.Call("swiss.reviveConversation", p, &res)
		}
		if err != nil || s.client == nil {
			engine := s.getRevivalEngine()
			if engine == nil {
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else {
					http.Error(w, "revival engine not available", http.StatusInternalServerError)
				}
				return
			}
			if reviveErr := engine.ReviveConversation(p.TargetApp, p.ConversationID, p.Prompt); reviveErr != nil {
				http.Error(w, reviveErr.Error(), http.StatusInternalServerError)
				return
			}
			res = map[string]interface{}{
				"success": true,
				"status":  "revived",
				"method":  "cdp_nudge",
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleConversationsStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetID := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	if targetID == "" {
		targetID = strings.TrimSpace(r.URL.Query().Get("cascade_id"))
	}
	targetID = strings.TrimPrefix(targetID, "/c/")

	var status *revival.RevivalStatus
	if s.backend == nil && s.client != nil {
		var params map[string]interface{}
		if targetID != "" {
			params = map[string]interface{}{"conversation_id": targetID}
		}
		_ = s.client.Call("swiss.getRevivalStatus", params, &status)
	}
	engine := s.getRevivalEngine()
	if status == nil && engine != nil {
		sStatus, err := engine.GetRevivalStatus(targetID)
		if err == nil {
			status = sStatus
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if status == nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
		})
		return
	}

	resp := map[string]interface{}{
		"success":         status.Success,
		"active_session":  status.ActiveSession,
		"pending_intents": status.PendingIntents,
		"pending_intent":  status.PendingIntent,
		"last_revived_at": status.LastRevivedAt,
	}
	if status.PendingIntent != nil {
		resp["cascade_id"] = status.PendingIntent.RootConversationID
		resp["prompt"] = status.PendingIntent.TriggerPrompt
		resp["timestamp"] = status.PendingIntent.Timestamp
		resp["resumed"] = status.PendingIntent.Resumed
		resp["status"] = status.PendingIntent.Status
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleConversationsAck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		CascadeID string `json:"cascade_id"`
		IntentID  string `json:"intent_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)

	if s.backend == nil && s.client != nil {
		var res map[string]interface{}
		_ = s.client.Call("swiss.ackContinuation", p, &res)
	}
	if engine := s.getRevivalEngine(); engine != nil {
		_ = engine.AcknowledgeContinuation(p.CascadeID)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func (s *Server) handleBridgeStatus(w http.ResponseWriter, r *http.Request) {
	if s.bridgeService == nil {
		s.bridgeService, _ = bridge.NewService("")
	}
	writeJSON(w, s.bridgeService.GetStatus())
}

func (s *Server) handleBridgeConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.bridgeService == nil {
		s.bridgeService, _ = bridge.NewService("")
	}
	var cfg bridge.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := s.bridgeService.UpdateConfig(cfg); err != nil {
		http.Error(w, "failed to update config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s.bridgeService.GetStatus())
}

// handleBridgeHTTPAllowlist lets the desktop UI inspect and edit the outbound
// HTTP allowlist and resolve the approvals agents requested.
// handleBridgeAgent selects which Antigravity surface the bridge drives.
func (s *Server) handleBridgeAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	bridgeSvc := s.getBridgeService()
	var payload struct {
		Agent string `json:"agent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	resolved, err := bridgeSvc.SetAgentBackend(payload.Agent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true, "agent_backend": resolved, "status": bridgeSvc.GetStatus()})
}

func (s *Server) handleBridgeHTTPAllowlist(w http.ResponseWriter, r *http.Request) {
	bridgeSvc := s.getBridgeService()
	if bridgeSvc == nil {
		http.Error(w, "bridge service not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]interface{}{
			"success":   true,
			"mode":      bridgeSvc.HTTPAllowlistMode(),
			"allowlist": bridgeSvc.AllowlistHosts(),
			"pending":   bridgeSvc.PendingHTTPApprovals(),
		})
	case http.MethodPost:
		var payload struct {
			Action string `json:"action"`
			Host   string `json:"host"`
			Mode   string `json:"mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		action := strings.ToLower(strings.TrimSpace(payload.Action))
		switch action {
		case "set_mode", "mode":
			if err := bridgeSvc.SetHTTPAllowlistMode(payload.Mode); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "allow":
			if _, err := bridgeSvc.AllowHTTPHost(payload.Host); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "revoke":
			if err := bridgeSvc.RevokeHTTPHost(payload.Host); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "deny":
			if err := bridgeSvc.DenyHTTPHost(payload.Host); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		default:
			if payload.Mode != "" {
				if err := bridgeSvc.SetHTTPAllowlistMode(payload.Mode); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				http.Error(w, "unsupported action: use allow, revoke, deny or set_mode", http.StatusBadRequest)
				return
			}
		}
		writeJSON(w, map[string]interface{}{
			"success":   true,
			"mode":      bridgeSvc.HTTPAllowlistMode(),
			"allowlist": bridgeSvc.AllowlistHosts(),
			"pending":   bridgeSvc.PendingHTTPApprovals(),
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleBridgeDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	bridgeSvc := s.getBridgeService()
	var p struct {
		Prompt string `json:"prompt"`
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	toolName := "antigravity_dispatch_task"
	toolArgs := map[string]interface{}{
		"prompt": p.Prompt,
		"target": p.Target,
	}
	switch strings.TrimSpace(p.Target) {
	case "local_run_command":
		toolName = "local_run_command"
		toolArgs = map[string]interface{}{"command": p.Prompt}
	case "antigravity_lead_send_message":
		toolName = "antigravity_lead_send_message"
		toolArgs = map[string]interface{}{"message": p.Prompt}
	case "antigravity_lead_create_conversation":
		toolName = "antigravity_lead_create_conversation"
		toolArgs = map[string]interface{}{"prompt": p.Prompt}
	}
	res, err := bridgeSvc.CallTool(toolName, toolArgs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"result":  res,
	})
}

func (s *Server) handleBridgeTest(w http.ResponseWriter, r *http.Request) {
	if s.bridgeService == nil {
		s.bridgeService, _ = bridge.NewService("")
	}
	st := s.bridgeService.GetStatus()
	if !st.Enabled {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"status":  st,
			"message": "Remote access is stopped. Start Remote Access before verifying bridge surfaces.",
		})
		return
	}

	activeSurfaces := 0
	if st.Surfaces.DesktopAppConnected {
		activeSurfaces++
	}
	if st.Surfaces.AgyCLIInstalled {
		activeSurfaces++
	}
	if st.Surfaces.VSCodeConnected {
		activeSurfaces++
	}

	msg := fmt.Sprintf("Gemini Bridge verified: %d of 3 local surfaces active (Desktop CDP: %t, CLI: %t, VS Code: %t).",
		activeSurfaces, st.Surfaces.DesktopAppConnected, st.Surfaces.AgyCLIInstalled, st.Surfaces.VSCodeConnected)

	writeJSON(w, map[string]interface{}{
		"success": true,
		"status":  st,
		"message": msg,
	})
}

func (s *Server) handleBridgeMCP(w http.ResponseWriter, r *http.Request) {
	bridgeSvc := s.getBridgeService()

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Header.Get("X-Swiss-KeepAlive") == "1" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"alive"}`))
		return
	}

	// Validate Authentication: Bearer token, Pairing PIN, or query parameters
	granted, reason := bridgeSvc.ValidateRequest(r)
	if !granted {
		w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer realm=\"Antigravity Swiss Knife MCP\", resource_metadata=\"%s/.well-known/oauth-protected-resource\"", bridge.ResolveExternalBaseURL(r, bridgeSvc.GetStatus().GatewayURL)))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":       "unauthorized",
			"message":     "Valid Bearer token or pairing PIN required to access Gemini Bridge MCP endpoint.",
			"reason":      reason,
			"setup_guide": "Open Antigravity Swiss Knife Desktop > Gemini Bridge tab to view your credentials.",
		})
		return
	}

	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if ok {
			q := url.Values{}
			q.Set("session", fmt.Sprintf("%d", time.Now().UnixNano()))
			for _, k := range []string{"pin", "token", "auth"} {
				if v := strings.TrimSpace(r.URL.Query().Get(k)); v != "" {
					q.Set(k, v)
				}
			}
			endpointURL := "/api/bridge/mcp?" + q.Encode()
			fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
			flusher.Flush()
		}
		return
	}

	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		resp, mErr := bridgeSvc.HandleMCPRequest(body)
		if mErr != nil {
			http.Error(w, mErr.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if resp != nil {
			_, _ = w.Write(resp)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleBridgeTunnelStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.bridgeService == nil {
		s.bridgeService, _ = bridge.NewService("")
	}
	var req struct {
		Type      string `json:"type"`
		CustomURL string `json:"custom_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Type = "quick"
	}
	st, err := s.bridgeService.StartTunnel(req.Type, req.CustomURL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"tunnel":  st,
		})
		return
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"tunnel":  st,
		"status":  s.bridgeService.GetStatus(),
	})
}

func (s *Server) handleBridgeTunnelStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.bridgeService == nil {
		s.bridgeService, _ = bridge.NewService("")
	}
	s.bridgeService.StopTunnel()
	writeJSON(w, map[string]interface{}{
		"success": true,
		"status":  s.bridgeService.GetStatus(),
	})
}

func (s *Server) handleBridgeTokenRotate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.isAuthorized(r) {
		http.Error(w, "unauthorized: authentication required to rotate bridge credentials", http.StatusUnauthorized)
		return
	}
	bridgeSvc := s.getBridgeService()
	token, pin, err := bridgeSvc.RotateCredentials()
	if err != nil {
		http.Error(w, "failed to rotate credentials", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{
		"success":     true,
		"auth_token":  token,
		"pairing_pin": pin,
		"status":      bridgeSvc.GetStatus(),
	})
}

func (s *Server) handleBridgeMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	bridgeSvc := s.getBridgeService()
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := bridgeSvc.SetOperatingMode(req.Mode); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"status":  bridgeSvc.GetStatus(),
	})
}

func (s *Server) handleBridgeRemoteStatus(w http.ResponseWriter, r *http.Request) {
	bridgeSvc := s.getBridgeService()
	activeEmail := ""
	store, _ := keyring.NewStore("")
	if store != nil {
		activeEmail = store.ActiveAccount()
	}
	shield := process.NewShield(0)
	procs, _ := shield.FindAntigravityProcesses()
	pid := 0
	if len(procs) > 0 {
		pid = procs[0].PID
	}
	st := bridgeSvc.GetRemoteStatus(activeEmail, pid)
	writeJSON(w, map[string]interface{}{
		"success": true,
		"status":  st,
		"config":  bridgeSvc.GetRemoteConfig(),
	})
}

func (s *Server) handleBridgeRemoteConfig(w http.ResponseWriter, r *http.Request) {
	bridgeSvc := s.getBridgeService()
	if r.Method == http.MethodGet {
		writeJSON(w, map[string]interface{}{
			"success": true,
			"config":  bridgeSvc.GetRemoteConfig(),
		})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var cfg bridge.RemoteControlConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := bridgeSvc.UpdateRemoteConfig(cfg); err != nil {
		http.Error(w, "failed to update remote config", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"config":  bridgeSvc.GetRemoteConfig(),
	})
}

func (s *Server) handleBridgeRemotePINRotate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.bridgeService == nil {
		s.bridgeService, _ = bridge.NewService("")
	}
	pin, err := s.bridgeService.RotateRemotePIN()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{
		"success":      true,
		"security_pin": pin,
		"config":       s.bridgeService.GetRemoteConfig(),
	})
}

func (s *Server) handleBridgeRemoteTunnelStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	bridgeSvc := s.getBridgeService()
	var req struct {
		Type      string `json:"type"`
		CustomURL string `json:"custom_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Type = "quick"
	}
	st, err := bridgeSvc.StartRemoteTunnel(req.Type, req.CustomURL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"tunnel":  st,
		})
		return
	}
	activeEmail := ""
	store, _ := keyring.NewStore("")
	if store != nil {
		activeEmail = store.ActiveAccount()
	}
	shield := process.NewShield(0)
	procs, _ := shield.FindAntigravityProcesses()
	pid := 0
	if len(procs) > 0 {
		pid = procs[0].PID
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"tunnel":  st,
		"status":  bridgeSvc.GetRemoteStatus(activeEmail, pid),
		"config":  bridgeSvc.GetRemoteConfig(),
	})
}

func (s *Server) handleBridgeRemoteTunnelStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	bridgeSvc := s.getBridgeService()
	bridgeSvc.StopRemoteTunnel()
	activeEmail := ""
	store, _ := keyring.NewStore("")
	if store != nil {
		activeEmail = store.ActiveAccount()
	}
	shield := process.NewShield(0)
	procs, _ := shield.FindAntigravityProcesses()
	pid := 0
	if len(procs) > 0 {
		pid = procs[0].PID
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"tunnel":  bridgeSvc.GetRemoteTunnelStatus(),
		"status":  bridgeSvc.GetRemoteStatus(activeEmail, pid),
		"config":  bridgeSvc.GetRemoteConfig(),
	})
}

func (s *Server) handleBridgeChannelsStatus(w http.ResponseWriter, r *http.Request) {
	bridgeSvc := s.getBridgeService()
	st := bridgeSvc.GetChannelsStatus()
	writeJSON(w, map[string]interface{}{
		"success": true,
		"status":  st,
	})
}

func (s *Server) handleBridgeChannelsConfig(w http.ResponseWriter, r *http.Request) {
	bridgeSvc := s.getBridgeService()
	if r.Method == http.MethodGet {
		cfg := bridgeSvc.GetChannelsConfigRedacted()
		if r.URL.Query().Get("unredacted") == "true" {
			cfg = bridgeSvc.GetChannelsConfig()
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"config":  cfg,
		})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var cfg bridge.ChannelsConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := bridgeSvc.UpdateChannelsConfig(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"config":  bridgeSvc.GetChannelsConfigRedacted(),
		"status":  bridgeSvc.GetChannelsStatus(),
	})
}

func (s *Server) handleBridgeChannelsTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	bridgeSvc := s.getBridgeService()
	var req struct {
		Channel string `json:"channel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	msg, err := bridgeSvc.TestChannel(req.Channel)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": msg,
		"status":  bridgeSvc.GetChannelsStatus(),
	})
}

func (s *Server) handleBridgeRemoteConsole(w http.ResponseWriter, r *http.Request) {
	bridgeSvc := s.getBridgeService()
	if rh, ok := any(bridgeSvc).(interface {
		ServeRemoteHTTP(http.ResponseWriter, *http.Request)
	}); ok {
		rh.ServeRemoteHTTP(w, r)
		return
	}
	http.Error(w, "remote console not initialized", http.StatusServiceUnavailable)
}

func (s *Server) handleOAuthProtectedResource(w http.ResponseWriter, r *http.Request) {
	s.getBridgeService().ServeOAuthProtectedResource(w, r)
}

func (s *Server) handleOAuthMetadata(w http.ResponseWriter, r *http.Request) {
	s.getBridgeService().ServeOAuthMetadata(w, r)
}

func (s *Server) handleOAuthRegister(w http.ResponseWriter, r *http.Request) {
	s.getBridgeService().ServeOAuthRegister(w, r)
}

func (s *Server) handleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	s.getBridgeService().ServeOAuthAuthorize(w, r)
}

func (s *Server) handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	s.getBridgeService().ServeOAuthToken(w, r)
}

func (s *Server) handleBridgeChannelsWebhook(w http.ResponseWriter, r *http.Request) {
	bridgeSvc := s.getBridgeService()
	cfg := bridgeSvc.GetChannelsConfig()
	expectedToken := strings.TrimSpace(cfg.WhatsApp.VerifyToken)

	switch r.Method {
	case http.MethodGet:
		mode := r.URL.Query().Get("hub.mode")
		token := r.URL.Query().Get("hub.verify_token")
		challenge := r.URL.Query().Get("hub.challenge")

		if mode == "subscribe" && expectedToken != "" && token == expectedToken {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(challenge))
			return
		}
		http.Error(w, "verification failed", http.StatusForbidden)
		return

	case http.MethodPost:
		if expectedToken != "" {
			token := strings.TrimSpace(r.Header.Get("X-Verify-Token"))
			if token == "" {
				token = strings.TrimSpace(r.URL.Query().Get("verify_token"))
			}
			if token == "" {
				token = strings.TrimSpace(r.URL.Query().Get("hub.verify_token"))
			}
			if token != expectedToken {
				http.Error(w, "invalid verify token", http.StatusForbidden)
				return
			}
		}

		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if len(bytes.TrimSpace(body)) == 0 {
			http.Error(w, "empty webhook payload", http.StatusBadRequest)
			return
		}
		inbound, err := bridge.ParseWhatsAppInbound(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Status callbacks and non-text messages carry nothing to relay; acknowledging
		// them is correct, not a fake success.
		project := cfg.DefaultProject
		for _, message := range inbound {
			if !whatsAppNumberAllowed(cfg.WhatsApp.AllowedNumbers, message.From) {
				log.Printf("[channels] whatsapp message from %s dropped: not in the allowed numbers", message.From)
				continue
			}
			if !bridge.MarkMessageSeen(message.ID) {
				continue
			}
			go s.relayWhatsApp(cfg, message, project)
		}
		writeJSON(w, map[string]interface{}{"accepted": len(inbound)})
		return

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// relayWhatsApp answers one inbound message on the configured backend and posts the
// reply back through the Cloud API. It runs outside the request so WhatsApp gets its
// 200 immediately; a long turn must not hold the webhook open.
func (s *Server) relayWhatsApp(cfg bridge.ChannelsConfig, message bridge.InboundMessage, project string) {
	service := s.getBridgeService()
	reply, err := service.AnswerChannelMessage(message.Text, project)
	if err != nil {
		log.Printf("[channels] whatsapp relay failed for %s: %v", message.ID, err)
		s.replyWhatsApp(cfg, message.From, "The agent could not answer that message. Check the bridge logs.")
		return
	}
	if strings.TrimSpace(cfg.WhatsApp.PhoneNumberID) == "" || strings.TrimSpace(cfg.WhatsApp.AccessToken) == "" {
		log.Printf("[channels] whatsapp reply dropped: phone number id or access token missing")
		return
	}
	s.replyWhatsApp(cfg, message.From, reply)
}

// replyWhatsApp posts one text message to a WhatsApp number, logging rather than
// swallowing a rejection.
func (s *Server) replyWhatsApp(cfg bridge.ChannelsConfig, to, reply string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text":              map[string]string{"body": reply},
	})
	url := "https://graph.facebook.com/v21.0/" + cfg.WhatsApp.PhoneNumberID + "/messages"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		log.Printf("[channels] whatsapp reply request: %v", err)
		return
	}
	request.Header.Set("Authorization", "Bearer "+cfg.WhatsApp.AccessToken)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		log.Printf("[channels] whatsapp reply failed: %v", err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		log.Printf("[channels] whatsapp reply rejected: HTTP %d %s", response.StatusCode, strings.TrimSpace(string(snippet)))
		return
	}
	s.getBridgeService().RecordChannelActivity("whatsapp")
}

// handleBridgeTelegramWebhook receives one Telegram update, relays the text to the
// configured agent and replies in the same chat.
func (s *Server) handleBridgeTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	service := s.getBridgeService()
	cfg := service.GetChannelsConfig()
	if !cfg.Telegram.Enabled {
		http.Error(w, "telegram bridge is disabled", http.StatusForbidden)
		return
	}
	// Telegram sends the secret in a header. A token in the URL would be recorded by
	// every proxy and access log between here and Telegram.
	provided := strings.TrimSpace(r.Header.Get("X-Telegram-Bot-Api-Secret-Token"))
	if provided == "" || provided != strings.TrimSpace(cfg.Telegram.BotToken) {
		http.Error(w, "invalid token", http.StatusForbidden)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	inbound, err := bridge.ParseTelegramInbound(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, message := range inbound {
		if !telegramChatAllowed(cfg.Telegram.AllowedChatIDs, message.From) {
			log.Printf("[channels] telegram message from %s dropped: not in the allowed chats", message.From)
			continue
		}
		if !bridge.MarkMessageSeen(message.ID) {
			continue
		}
		go s.relayTelegram(cfg, message)
	}
	writeJSON(w, map[string]interface{}{"accepted": len(inbound)})
}

// relayTelegram answers one message and posts the reply back to its chat.
func (s *Server) relayTelegram(cfg bridge.ChannelsConfig, message bridge.InboundMessage) {
	service := s.getBridgeService()
	reply, err := service.AnswerChannelMessage(message.Text, cfg.DefaultProject)
	if err != nil {
		log.Printf("[channels] telegram relay failed for %s: %v", message.ID, err)
		if sendErr := bridge.SendTelegramReply(cfg.Telegram.BotToken, message.From, "The agent could not answer that message. Check the bridge logs."); sendErr != nil {
			log.Printf("[channels] telegram failure notice could not be sent: %v", sendErr)
		}
		return
	}
	if err := bridge.SendTelegramReply(cfg.Telegram.BotToken, message.From, reply); err != nil {
		log.Printf("[channels] telegram reply failed for %s: %v", message.ID, err)
		return
	}
	service.RecordChannelActivity("telegram")
}

// handleBridgeDiscordWebhook receives a Discord webhook event, relays it to the configured agent,
// and posts the response back to Discord.
func (s *Server) handleBridgeDiscordWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	service := s.getBridgeService()
	cfg := service.GetChannelsConfig()
	if !cfg.Discord.Enabled {
		http.Error(w, "discord bridge is disabled", http.StatusForbidden)
		return
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	cleanToken := strings.TrimSpace(cfg.Discord.BotToken)
	expectedAuth := cleanToken
	if !strings.HasPrefix(expectedAuth, "Bot ") {
		expectedAuth = "Bot " + expectedAuth
	}
	if authHeader == "" || (authHeader != expectedAuth && authHeader != cleanToken) {
		http.Error(w, "invalid authorization token", http.StatusForbidden)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	inbound, err := bridge.ParseDiscordInbound(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, message := range inbound {
		msg := bridge.DiscordMessage{
			ID:        strings.TrimPrefix(message.ID, "dc-"),
			ChannelID: message.ChannelID,
			Author: bridge.DiscordMessageAuthor{
				ID: message.From,
			},
			Content: message.Text,
		}
		go service.RelayDiscordMessage(cfg, msg)
	}
	writeJSON(w, map[string]interface{}{"accepted": len(inbound)})
}

// whatsAppNumberAllowed applies the configured whitelist. An empty list means "no
// restriction", which is what the settings page promises.
func whatsAppNumberAllowed(allowed []string, from string) bool {
	if len(allowed) == 0 {
		return true
	}
	sender := strings.TrimPrefix(strings.TrimSpace(from), "+")
	for _, entry := range allowed {
		if strings.TrimPrefix(strings.TrimSpace(entry), "+") == sender {
			return true
		}
	}
	return false
}

// telegramChatAllowed applies the configured chat whitelist.
func telegramChatAllowed(allowed []string, chatID string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, entry := range allowed {
		if strings.TrimSpace(entry) == strings.TrimSpace(chatID) {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

//go:embed index.html
var indexHTML string
