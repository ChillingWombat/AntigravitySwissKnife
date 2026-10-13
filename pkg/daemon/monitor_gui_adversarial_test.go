package daemon

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// Helper to set private injector field on gui.Store for empirical test isolation
func setStoreInjector(s *gui.Store, inj *gui.Injector) bool {
	if s == nil {
		return false
	}
	f, ok := reflect.TypeOf(s).Elem().FieldByName("injector")
	if !ok {
		return false
	}
	ptr := unsafe.Pointer(uintptr(unsafe.Pointer(s)) + f.Offset)
	*(*unsafe.Pointer)(ptr) = unsafe.Pointer(inj)
	return true
}

func readWSTestFrame(r *bufio.Reader) ([]byte, error) {
	_, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	b1, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	isMasked := (b1 & 0x80) != 0
	payloadLen := int(b1 & 0x7F)

	if payloadLen == 126 {
		var l uint16
		if err := binary.Read(r, binary.BigEndian, &l); err != nil {
			return nil, err
		}
		payloadLen = int(l)
	} else if payloadLen == 127 {
		var l uint64
		if err := binary.Read(r, binary.BigEndian, &l); err != nil {
			return nil, err
		}
		payloadLen = int(l)
	}

	var maskKey [4]byte
	if isMasked {
		if _, err := io.ReadFull(r, maskKey[:]); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	if isMasked {
		for i := 0; i < payloadLen; i++ {
			payload[i] ^= maskKey[i%4]
		}
	}

	return payload, nil
}

func writeUnmaskedWSTestFrame(w *bufio.Writer, payload []byte) error {
	_ = w.WriteByte(0x81) // FIN + text opcode
	l := len(payload)
	if l <= 125 {
		_ = w.WriteByte(byte(l))
	} else if l <= 65535 {
		_ = w.WriteByte(126)
		_ = binary.Write(w, binary.BigEndian, uint16(l))
	} else {
		_ = w.WriteByte(127)
		_ = binary.Write(w, binary.BigEndian, uint64(l))
	}
	_, err := w.Write(payload)
	if err != nil {
		return err
	}
	return w.Flush()
}

// TestAdversarial_MonitorGUI_SimulatedCDP_TriggersApplyWhenUninjected tests that when CDP returns
// an un-injected window (checkScript returns false), monitorAndInjectGUI triggers Apply().
func TestAdversarial_MonitorGUI_SimulatedCDP_TriggersApplyWhenUninjected(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	var mu sync.Mutex
	var evaluatedScripts []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Browser":"Chrome/132.0.0.0"}`))
		case "/json":
			targets := []gui.DevToolsTarget{
				{
					ID:                   "page-uninjected-1",
					Title:                "Antigravity",
					Type:                 "page",
					URL:                  "http://" + r.Host + "/",
					WebSocketDebuggerURL: "ws://" + r.Host + "/ws/page-1",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(targets)
		case "/ws/page-1":
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijack not supported", http.StatusInternalServerError)
				return
			}
			conn, rw, err := hj.Hijack()
			if err != nil {
				return
			}
			defer conn.Close()

			_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
			_ = rw.Flush()

			for {
				payload, err := readWSTestFrame(rw.Reader)
				if err != nil {
					return
				}
				var rpcReq struct {
					ID     int    `json:"id"`
					Method string `json:"method"`
					Params struct {
						Expression string `json:"expression"`
					} `json:"params"`
				}
				if err := json.Unmarshal(payload, &rpcReq); err != nil {
					return
				}

				mu.Lock()
				evaluatedScripts = append(evaluatedScripts, rpcReq.Params.Expression)
				mu.Unlock()

				var val interface{} = false
				// If this is the check script, return false to simulate un-injected window
				if strings.Contains(rpcReq.Params.Expression, "antigravity-swiss-styles") {
					val = false
				} else {
					val = true
				}

				respObj := map[string]interface{}{
					"id": rpcReq.ID,
					"result": map[string]interface{}{
						"value": val,
					},
				}
				respBytes, _ := json.Marshal(respObj)
				if err := writeUnmaskedWSTestFrame(rw.Writer, respBytes); err != nil {
					return
				}
			}
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	sockPath := filepath.Join(tmpDir, "daemon.sock")
	cfg := core.DefaultConfig()

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	mockInj := gui.NewInjector(port)
	if !setStoreInjector(d.GUIStore, mockInj) {
		t.Fatalf("failed to set store injector")
	}

	// Run monitorAndInjectGUI
	d.monitorAndInjectGUI()

	mu.Lock()
	scripts := evaluatedScripts
	mu.Unlock()

	if len(scripts) < 2 {
		t.Fatalf("expected at least 2 evaluated scripts (checkScript + Apply script), got %d: %v", len(scripts), scripts)
	}

	foundCheck := false
	foundApply := false
	for _, s := range scripts {
		if strings.Contains(s, "antigravity-swiss-styles") {
			foundCheck = true
		}
		if strings.Contains(s, "window.antigravitySwissImprovements") || strings.Contains(s, "Swiss Knife") {
			foundApply = true
		}
	}

	if !foundCheck {
		t.Errorf("expected check script to be evaluated")
	}
	if !foundApply {
		t.Errorf("expected Apply injection script to be evaluated after un-injected window detected")
	}
}

// TestAdversarial_MonitorGUI_SimulatedCDP_SkipsApplyWhenAlreadyInjected tests that when CDP returns
// an already-injected window (checkScript returns true), monitorAndInjectGUI does NOT trigger Apply().
func TestAdversarial_MonitorGUI_SimulatedCDP_SkipsApplyWhenAlreadyInjected(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	var mu sync.Mutex
	var evaluatedScripts []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Browser":"Chrome/132.0.0.0"}`))
		case "/json":
			targets := []gui.DevToolsTarget{
				{
					ID:                   "page-injected-1",
					Title:                "Antigravity",
					Type:                 "page",
					URL:                  "http://" + r.Host + "/",
					WebSocketDebuggerURL: "ws://" + r.Host + "/ws/page-injected",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(targets)
		case "/ws/page-injected":
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijack not supported", http.StatusInternalServerError)
				return
			}
			conn, rw, err := hj.Hijack()
			if err != nil {
				return
			}
			defer conn.Close()

			_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
			_ = rw.Flush()

			for {
				payload, err := readWSTestFrame(rw.Reader)
				if err != nil {
					return
				}
				var rpcReq struct {
					ID     int    `json:"id"`
					Method string `json:"method"`
					Params struct {
						Expression string `json:"expression"`
					} `json:"params"`
				}
				if err := json.Unmarshal(payload, &rpcReq); err != nil {
					return
				}

				mu.Lock()
				evaluatedScripts = append(evaluatedScripts, rpcReq.Params.Expression)
				mu.Unlock()

				respObj := map[string]interface{}{
					"id": rpcReq.ID,
					"result": map[string]interface{}{
						"value": true, // Window is ALREADY injected
					},
				}
				respBytes, _ := json.Marshal(respObj)
				if err := writeUnmaskedWSTestFrame(rw.Writer, respBytes); err != nil {
					return
				}
			}
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	sockPath := filepath.Join(tmpDir, "daemon.sock")
	cfg := core.DefaultConfig()

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	mockInj := gui.NewInjector(port)
	if !setStoreInjector(d.GUIStore, mockInj) {
		t.Fatalf("failed to set store injector")
	}

	// Run monitorAndInjectGUI
	d.monitorAndInjectGUI()

	mu.Lock()
	scripts := evaluatedScripts
	mu.Unlock()

	// Exactly 1 check script should have been executed, and NO Apply injection script
	if len(scripts) != 1 {
		t.Fatalf("expected exactly 1 evaluated script (checkScript only), got %d: %v", len(scripts), scripts)
	}
	if !strings.Contains(scripts[0], "antigravity-swiss-styles") {
		t.Errorf("expected script to be checkScript, got: %s", scripts[0])
	}
}

// TestAdversarial_MonitorGUI_SimulatedCDP_NonPageTargets tests that non-page targets (service worker, etc.)
// do not trigger injection.
func TestAdversarial_MonitorGUI_SimulatedCDP_NonPageTargets(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Browser":"Chrome/132.0.0.0"}`))
		case "/json":
			targets := []gui.DevToolsTarget{
				{
					ID:                   "sw-1",
					Title:                "Service Worker",
					Type:                 "service_worker",
					URL:                  "http://" + r.Host + "/sw.js",
					WebSocketDebuggerURL: "ws://" + r.Host + "/ws/sw-1",
				},
				{
					ID:                   "bg-1",
					Title:                "Background Page",
					Type:                 "background_page",
					URL:                  "http://" + r.Host + "/bg.html",
					WebSocketDebuggerURL: "ws://" + r.Host + "/ws/bg-1",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(targets)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	sockPath := filepath.Join(tmpDir, "daemon.sock")
	cfg := core.DefaultConfig()

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	mockInj := gui.NewInjector(port)
	if !setStoreInjector(d.GUIStore, mockInj) {
		t.Fatalf("failed to set store injector")
	}

	// Should safely return without calling Apply or panicking
	d.monitorAndInjectGUI()
}

// TestAdversarial_MonitorGUI_SimulatedCDP_FaultTolerance tests that CDP server errors (500, invalid JSON, socket drops)
// are handled cleanly without crashing the daemon.
func TestAdversarial_MonitorGUI_SimulatedCDP_FaultTolerance(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	// Case 1: Server returns HTTP 500
	srv500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv500.Close()

	u1, _ := url.Parse(srv500.URL)
	port1, _ := strconv.Atoi(u1.Port())

	d, err := NewDaemon(core.DefaultConfig(), filepath.Join(tmpDir, "d1.sock"))
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}
	setStoreInjector(d.GUIStore, gui.NewInjector(port1))
	d.monitorAndInjectGUI() // Must not panic

	// Case 2: Server returns invalid JSON syntax
	srvCorrupt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json syntax[`))
	}))
	defer srvCorrupt.Close()

	u2, _ := url.Parse(srvCorrupt.URL)
	port2, _ := strconv.Atoi(u2.Port())
	setStoreInjector(d.GUIStore, gui.NewInjector(port2))
	d.monitorAndInjectGUI() // Must not panic
}

// TestAdversarial_MonitorGUI_ConcurrentStress tests concurrent calls to monitorAndInjectGUI.
func TestAdversarial_MonitorGUI_ConcurrentStress(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	sockPath := filepath.Join(tmpDir, "daemon.sock")
	cfg := core.DefaultConfig()

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				d.mu.Lock()
				d.Config.PersistentVisualEffects = (idx%2 == 0)
				d.mu.Unlock()
				d.monitorAndInjectGUI()
				time.Sleep(2 * time.Millisecond)
			}
		}(i)
	}
	wg.Wait()
}

// TestAdversarial_MonitorGUI_NilStore_Safety tests that nil GUIStore does not panic.
func TestAdversarial_MonitorGUI_NilStore_Safety(t *testing.T) {
	d := &Daemon{GUIStore: nil}
	d.monitorAndInjectGUI() // Must not panic
}
