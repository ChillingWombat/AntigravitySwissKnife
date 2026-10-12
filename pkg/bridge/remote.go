package bridge

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// RemoteControlConfig holds settings for the account-resilient remote control wrapper.
type RemoteControlConfig struct {
	Enabled           bool   `json:"enabled"`
	BindAddr          string `json:"bind_addr"`
	Port              int    `json:"port"`
	SecurityPIN       string `json:"security_pin"`
	Quality           string `json:"quality"` // "high" | "medium" | "low"
	FrameRate         int    `json:"frame_rate"`
	AccountResilience bool   `json:"account_resilience"`
}

// RemoteControlStatus details the live state of the wrapper across account switches.
type RemoteControlStatus struct {
	Active                bool   `json:"active"`
	EndpointURL           string `json:"endpoint_url"`
	AccountResilience     bool   `json:"account_resilience"`
	CurrentAntigravityPID int    `json:"current_antigravity_pid"`
	DevToolsPort          int    `json:"devtools_port"`
	ActiveGoogleAccount   string `json:"active_google_account"`
	ClientsCount          int    `json:"clients_count"`
	LastRotatedAt         string `json:"last_rotated_at,omitempty"`
}

// RemoteManager wraps factory remote control to keep it consistent across account rotations.
type RemoteManager struct {
	mu         sync.RWMutex
	config     RemoteControlConfig
	configPath string
	injector   *gui.Injector
	lastRotated time.Time
}

// DefaultRemoteConfig returns standard default remote control settings.
func DefaultRemoteConfig() RemoteControlConfig {
	pin, _ := rand.Int(rand.Reader, big.NewInt(900000))
	pinStr := fmt.Sprintf("%06d", pin.Int64()+100000)
	return RemoteControlConfig{
		Enabled:           true,
		BindAddr:          "127.0.0.1",
		Port:              8766,
		SecurityPIN:       pinStr,
		Quality:           "high",
		FrameRate:         30,
		AccountResilience: true,
	}
}

// NewRemoteManager initializes a remote control session wrapper.
func NewRemoteManager(configPath string) *RemoteManager {
	if configPath == "" {
		configPath = filepath.Join(core.GetConfigDir(), "remote_control.json")
	}
	rm := &RemoteManager{
		config:     DefaultRemoteConfig(),
		configPath: configPath,
		injector:   gui.NewInjector(0),
	}
	_ = rm.load()
	return rm
}

func (rm *RemoteManager) load() error {
	data, err := os.ReadFile(rm.configPath)
	if err != nil {
		return err
	}
	var c RemoteControlConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	rm.config = c
	return nil
}

func (rm *RemoteManager) save() error {
	_ = os.MkdirAll(filepath.Dir(rm.configPath), 0755)
	data, err := json.MarshalIndent(rm.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rm.configPath, data, 0644)
}

// GetConfig returns current remote control configuration.
func (rm *RemoteManager) GetConfig() RemoteControlConfig {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.config
}

// UpdateConfig persists updated remote control configuration.
func (rm *RemoteManager) UpdateConfig(c RemoteControlConfig) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.config = c
	return rm.save()
}

// RotatePIN generates a fresh 6-digit PIN.
func (rm *RemoteManager) RotatePIN() (string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	pin, _ := rand.Int(rand.Reader, big.NewInt(900000))
	newPIN := fmt.Sprintf("%06d", pin.Int64()+100000)
	rm.config.SecurityPIN = newPIN
	err := rm.save()
	return newPIN, err
}

// GetStatus returns the live operational status of the wrapper.
func (rm *RemoteManager) GetStatus(activeEmail string, activePID int) RemoteControlStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	cdpPort := 0
	if port, err := rm.injector.FindDevToolsPort(); err == nil {
		cdpPort = port
	}

	endpoint := fmt.Sprintf("http://%s:%d/remote", rm.config.BindAddr, rm.config.Port)
	if rm.config.BindAddr == "0.0.0.0" {
		endpoint = fmt.Sprintf("http://127.0.0.1:%d/remote", rm.config.Port)
	}

	rotatedStr := ""
	if !rm.lastRotated.IsZero() {
		rotatedStr = rm.lastRotated.Format("2006-01-02 15:04:05")
	}

	return RemoteControlStatus{
		Active:                rm.config.Enabled && cdpPort > 0,
		EndpointURL:           endpoint,
		AccountResilience:     rm.config.AccountResilience,
		CurrentAntigravityPID: activePID,
		DevToolsPort:          cdpPort,
		ActiveGoogleAccount:   activeEmail,
		ClientsCount:          0,
		LastRotatedAt:         rotatedStr,
	}
}

// NotifyAccountSwitch is called by Daemon when account rotation finishes to seamlessly re-bind remote session.
func (rm *RemoteManager) NotifyAccountSwitch(newAccount string, newPID int) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.lastRotated = time.Now()
}
