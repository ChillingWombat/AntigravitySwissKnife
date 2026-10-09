package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/webgui"
)

// TestAdversarial_SubagentModelStrategy_EdgeCasesAndJunk exercises 50+ adversarial inputs against NormalizeSubagentModelStrategy.
func TestAdversarial_SubagentModelStrategy_EdgeCasesAndJunk(t *testing.T) {
	validAutoInputs := []string{
		"auto_decide",
		"auto",
		"autodecide",
		"AUTO_DECIDE",
		"AUTO",
		"AUTODECIDE",
		"  auto_decide  ",
		"  auto  ",
		"  autodecide  ",
		"\tauto_decide\n",
		"AuTo_DeCiDe",
	}

	for _, in := range validAutoInputs {
		got := core.NormalizeSubagentModelStrategy(in)
		if got != core.SubagentModelStrategyAutoDecide {
			t.Errorf("NormalizeSubagentModelStrategy(%q) = %q, expected %q", in, got, core.SubagentModelStrategyAutoDecide)
		}
	}

	fallbackJunkInputs := []string{
		"",
		" ",
		"    ",
		"\t\n\r",
		"default_custom_only",
		"DEFAULT_CUSTOM_ONLY",
		"  default_custom_only  ",
		"default",
		"custom",
		"only",
		"auto-decide",
		"auto decide",
		"autodecided",
		"auto_decision",
		"gemini",
		"claude",
		"null",
		"undefined",
		"NaN",
		"[object Object]",
		"true",
		"false",
		"0",
		"1",
		"auto_decide; DROP TABLE accounts;--",
		"<script>alert(1)</script>",
		"🚀 auto_decide",
		"\x00\x01\x02",
		strings.Repeat("x", 4096),
		strings.Repeat("auto_decide", 100),
	}

	for _, in := range fallbackJunkInputs {
		got := core.NormalizeSubagentModelStrategy(in)
		if got != core.SubagentModelStrategyDefaultCustomOnly {
			t.Errorf("NormalizeSubagentModelStrategy(%q) = %q, expected fallback to %q", in, got, core.SubagentModelStrategyDefaultCustomOnly)
		}
	}

	// Verify LoadConfig handling of missing or junk values on disk
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	cfgPath := filepath.Join(tempDir, "config.json")

	// 1. Completely missing key in JSON
	_ = os.WriteFile(cfgPath, []byte(`{"storage_mode":"system_default"}`), 0600)
	cfg, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error on missing key: %v", err)
	}
	if cfg.GetSubagentModelStrategy() != core.SubagentModelStrategyDefaultCustomOnly {
		t.Errorf("expected default strategy on missing key, got %q", cfg.GetSubagentModelStrategy())
	}

	// 2. Junk string in JSON
	_ = os.WriteFile(cfgPath, []byte(`{"subagent_model_strategy":"some_corrupt_junk"}`), 0600)
	cfgJunk, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error on junk string: %v", err)
	}
	if cfgJunk.GetSubagentModelStrategy() != core.SubagentModelStrategyDefaultCustomOnly {
		t.Errorf("expected fallback on junk string, got %q", cfgJunk.GetSubagentModelStrategy())
	}

	// 3. Empty string in JSON
	_ = os.WriteFile(cfgPath, []byte(`{"subagent_model_strategy":""}`), 0600)
	cfgEmpty, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error on empty string: %v", err)
	}
	if cfgEmpty.GetSubagentModelStrategy() != core.SubagentModelStrategyDefaultCustomOnly {
		t.Errorf("expected fallback on empty string, got %q", cfgEmpty.GetSubagentModelStrategy())
	}
}

// TestAdversarial_SubagentModelStrategy_ConcurrentStress runs 50 readers and 50 writers concurrently.
func TestAdversarial_SubagentModelStrategy_ConcurrentStress(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	cfg := core.DefaultConfig()
	if err := cfg.Save(); err != nil {
		t.Fatalf("cfg.Save failed: %v", err)
	}

	var wg sync.WaitGroup
	numWorkers := 50
	iterations := 200

	// 25 workers setting auto_decide
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = cfg.SetSubagentModelStrategy("auto_decide")
			}
		}(i)
	}

	// 25 workers setting default_custom_only
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = cfg.SetSubagentModelStrategy("default_custom_only")
			}
		}(i)
	}

	// 50 workers reading GetSubagentModelStrategy()
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				val := cfg.GetSubagentModelStrategy()
				if val != core.SubagentModelStrategyDefaultCustomOnly && val != core.SubagentModelStrategyAutoDecide {
					t.Errorf("concurrent reader got invalid strategy: %q", val)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify disk integrity after heavy concurrent writes
	reloaded, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("failed to reload config after stress test: %v", err)
	}
	finalVal := reloaded.GetSubagentModelStrategy()
	if finalVal != core.SubagentModelStrategyDefaultCustomOnly && finalVal != core.SubagentModelStrategyAutoDecide {
		t.Fatalf("final strategy on disk invalid: %q", finalVal)
	}
}

// TestAdversarial_SubagentModelStrategy_WebGUIRestPersistence tests REST POST and disk persistence.
func TestAdversarial_SubagentModelStrategy_WebGUIRestPersistence(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	dummySock := filepath.Join(tempDir, "dummy.sock")

	srv := webgui.NewServer("127.0.0.1:0", dummySock)
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	// Step 1: POST /api/rules with "auto_decide"
	payloadAuto := map[string]interface{}{
		"subagent_model_strategy": "auto_decide",
	}
	b, _ := json.Marshal(payloadAuto)
	resp, err := http.Post(baseURL+"/api/rules", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST auto_decide failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	// Direct disk inspection
	cfgPath := filepath.Join(tempDir, "config.json")
	diskData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read config.json directly: %v", err)
	}
	var diskMap map[string]interface{}
	if err := json.Unmarshal(diskData, &diskMap); err != nil {
		t.Fatalf("disk config corrupted: %v", err)
	}
	if strat, ok := diskMap["subagent_model_strategy"].(string); !ok || strat != "auto_decide" {
		t.Fatalf("disk config subagent_model_strategy expected auto_decide, got %v", diskMap["subagent_model_strategy"])
	}

	// Step 2: POST /api/rules with invalid junk
	payloadJunk := map[string]interface{}{
		"subagent_model_strategy": "invalid_junk_12345",
	}
	bJunk, _ := json.Marshal(payloadJunk)
	respJunk, err := http.Post(baseURL+"/api/rules", "application/json", bytes.NewReader(bJunk))
	if err != nil || respJunk.StatusCode != http.StatusOK {
		t.Fatalf("POST junk failed: err=%v, code=%d", err, respJunk.StatusCode)
	}
	respJunk.Body.Close()

	diskData2, _ := os.ReadFile(cfgPath)
	_ = json.Unmarshal(diskData2, &diskMap)
	if strat, ok := diskMap["subagent_model_strategy"].(string); !ok || strat != "default_custom_only" {
		t.Fatalf("disk config subagent_model_strategy expected fallback to default_custom_only, got %v", diskMap["subagent_model_strategy"])
	}

	// Step 3: POST /api/rules with empty string ""
	payloadEmpty := map[string]interface{}{
		"subagent_model_strategy": "",
	}
	bEmpty, _ := json.Marshal(payloadEmpty)
	respEmpty, err := http.Post(baseURL+"/api/rules", "application/json", bytes.NewReader(bEmpty))
	if err != nil || respEmpty.StatusCode != http.StatusOK {
		t.Fatalf("POST empty failed: err=%v, code=%d", err, respEmpty.StatusCode)
	}
	respEmpty.Body.Close()

	diskData3, _ := os.ReadFile(cfgPath)
	_ = json.Unmarshal(diskData3, &diskMap)
	if strat, ok := diskMap["subagent_model_strategy"].(string); !ok || strat != "default_custom_only" {
		t.Fatalf("disk config subagent_model_strategy expected fallback to default_custom_only on empty, got %v", diskMap["subagent_model_strategy"])
	}

	// Step 4: Re-set to auto_decide, then POST unrelated fields
	_, _ = http.Post(baseURL+"/api/rules", "application/json", bytes.NewReader(b))
	payloadUnrelated := map[string]interface{}{
		"auto_switch_enabled": false,
	}
	bUnrelated, _ := json.Marshal(payloadUnrelated)
	respUnrelated, err := http.Post(baseURL+"/api/rules", "application/json", bytes.NewReader(bUnrelated))
	if err != nil || respUnrelated.StatusCode != http.StatusOK {
		t.Fatalf("POST unrelated failed: err=%v, code=%d", err, respUnrelated.StatusCode)
	}
	respUnrelated.Body.Close()

	diskData4, _ := os.ReadFile(cfgPath)
	_ = json.Unmarshal(diskData4, &diskMap)
	if strat, ok := diskMap["subagent_model_strategy"].(string); !ok || strat != "auto_decide" {
		t.Fatalf("unrelated POST wiped out subagent_model_strategy: got %v", diskMap["subagent_model_strategy"])
	}
}

// TestAdversarial_SubagentModelStrategy_DaemonRPC tests rules.set_config and swiss.setRuleConfig with nil vs string vs junk.
func TestAdversarial_SubagentModelStrategy_DaemonRPC(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	sockPath := filepath.Join(tmpDir, "daemon_subagent_test.sock")
	cfg := core.DefaultConfig()

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}
	if err := d.Start(); err != nil {
		t.Fatalf("d.Start error: %v", err)
	}
	defer d.Stop()

	client := ipc.NewClient(sockPath)

	// 1. Initial check: rules.get_config should return default_custom_only
	var getConf map[string]interface{}
	if err := client.Call("rules.get_config", nil, &getConf); err != nil {
		t.Fatalf("rules.get_config failed: %v", err)
	}
	if strat, ok := getConf["subagent_model_strategy"].(string); !ok || strat != core.SubagentModelStrategyDefaultCustomOnly {
		t.Errorf("initial strategy expected default_custom_only, got %v", getConf["subagent_model_strategy"])
	}

	// 2. Set to "auto_decide" via rules.set_config
	autoVal := "auto_decide"
	setReq := map[string]interface{}{
		"subagent_model_strategy": &autoVal,
	}
	var setResp map[string]interface{}
	if err := client.Call("rules.set_config", setReq, &setResp); err != nil {
		t.Fatalf("rules.set_config with auto_decide failed: %v", err)
	}
	if d.Config.GetSubagentModelStrategy() != core.SubagentModelStrategyAutoDecide {
		t.Errorf("daemon in-memory config expected auto_decide, got %q", d.Config.GetSubagentModelStrategy())
	}

	// 3. Call rules.set_config with nil (omitted subagent_model_strategy)
	unrelatedReq := map[string]interface{}{
		"switch_mode": "max_tokens",
	}
	if err := client.Call("rules.set_config", unrelatedReq, &setResp); err != nil {
		t.Fatalf("rules.set_config with nil subagent_model_strategy failed: %v", err)
	}
	if d.Config.GetSubagentModelStrategy() != core.SubagentModelStrategyAutoDecide {
		t.Errorf("nil strategy unexpectedly altered existing value to %q", d.Config.GetSubagentModelStrategy())
	}

	// 4. Call swiss.setRuleConfig with explicit null
	nullReq := map[string]interface{}{
		"subagent_model_strategy": nil,
	}
	if err := client.Call("swiss.setRuleConfig", nullReq, &setResp); err != nil {
		t.Fatalf("swiss.setRuleConfig with null failed: %v", err)
	}
	if d.Config.GetSubagentModelStrategy() != core.SubagentModelStrategyAutoDecide {
		t.Errorf("explicit null unexpectedly altered existing value to %q", d.Config.GetSubagentModelStrategy())
	}

	// 5. Call rules.set_config with junk string
	junkVal := "garbage_junk_model_option"
	junkReq := map[string]interface{}{
		"subagent_model_strategy": &junkVal,
	}
	if err := client.Call("rules.set_config", junkReq, &setResp); err != nil {
		t.Fatalf("rules.set_config with junk failed: %v", err)
	}
	if d.Config.GetSubagentModelStrategy() != core.SubagentModelStrategyDefaultCustomOnly {
		t.Errorf("junk strategy expected fallback to default_custom_only, got %q", d.Config.GetSubagentModelStrategy())
	}

	// 6. Verify swiss.getRuleConfig returns the normalized fallback
	var swissGetConf map[string]interface{}
	if err := client.Call("swiss.getRuleConfig", nil, &swissGetConf); err != nil {
		t.Fatalf("swiss.getRuleConfig failed: %v", err)
	}
	if strat, ok := swissGetConf["subagent_model_strategy"].(string); !ok || strat != core.SubagentModelStrategyDefaultCustomOnly {
		t.Errorf("swiss.getRuleConfig expected default_custom_only, got %v", swissGetConf["subagent_model_strategy"])
	}
}
