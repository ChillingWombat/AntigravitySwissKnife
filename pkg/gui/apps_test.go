package gui

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNormalizeAppID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", AppAntigravity},
		{"antigravity", AppAntigravity},
		{"Antigravity", AppAntigravity},
		{"desktop", AppAntigravity},
		{"vscode", AppVSCode},
		{"  VSCODE  ", AppVSCode},
		{"vscode-insiders", AppVSCodeInsiders},
		{"vscode-insider", AppVSCodeInsiders},
		{"code-insiders", AppVSCodeInsiders},
		{"junk", "junk"},
	}
	for _, tt := range tests {
		if got := NormalizeAppID(tt.in); got != tt.want {
			t.Errorf("NormalizeAppID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDevToolsActivePortFiles_PerOSAndApp(t *testing.T) {
	const (
		appData = `C:\Users\dev\AppData\Roaming`
		home    = `C:\Users\dev`
		posix   = "/home/dev"
	)

	tests := []struct {
		name    string
		app     string
		goos    string
		appData string
		home    string
		want    []string
	}{
		{
			name: "vscode windows uses APPDATA Code",
			app:  AppVSCode, goos: "windows", appData: appData, home: home,
			want: []string{filepath.Join(appData, "Code", "DevToolsActivePort")},
		},
		{
			name: "vscode darwin uses Application Support",
			app:  AppVSCode, goos: "darwin", appData: "", home: posix,
			want: []string{filepath.Join(posix, "Library", "Application Support", "Code", "DevToolsActivePort")},
		},
		{
			name: "vscode linux uses .config",
			app:  AppVSCode, goos: "linux", appData: "", home: posix,
			want: []string{filepath.Join(posix, ".config", "Code", "DevToolsActivePort")},
		},
		{
			name: "vscode-insiders windows uses Code - Insiders",
			app:  AppVSCodeInsiders, goos: "windows", appData: appData, home: home,
			want: []string{filepath.Join(appData, "Code - Insiders", "DevToolsActivePort")},
		},
		{
			name: "vscode-insiders linux uses Code - Insiders",
			app:  AppVSCodeInsiders, goos: "linux", appData: "", home: posix,
			want: []string{filepath.Join(posix, ".config", "Code - Insiders", "DevToolsActivePort")},
		},
		{
			name: "antigravity windows keeps host dir then legacy fallback",
			app:  AppAntigravity, goos: "windows", appData: appData, home: home,
			want: []string{
				filepath.Join(appData, "Antigravity", "DevToolsActivePort"),
				filepath.Join(home, ".config", "antigravity", "DevToolsActivePort"),
			},
		},
		{
			name: "antigravity linux keeps host dir then legacy fallback",
			app:  AppAntigravity, goos: "linux", appData: "", home: posix,
			want: []string{
				filepath.Join(posix, ".config", "Antigravity", "DevToolsActivePort"),
				filepath.Join(posix, ".config", "antigravity", "DevToolsActivePort"),
			},
		},
		{
			name: "unknown app has no candidates",
			app:  "junk", goos: "windows", appData: appData, home: home,
			want: nil,
		},
		{
			name: "empty app defaults to antigravity",
			app:  "", goos: "linux", appData: "", home: posix,
			want: []string{
				filepath.Join(posix, ".config", "Antigravity", "DevToolsActivePort"),
				filepath.Join(posix, ".config", "antigravity", "DevToolsActivePort"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DevToolsActivePortFiles(tt.app, tt.goos, tt.appData, tt.home)
			if len(got) != len(tt.want) {
				t.Fatalf("DevToolsActivePortFiles(%q, %q) = %v, want %v", tt.app, tt.goos, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("candidate[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsInjectableTarget_AntigravityKeepsDesktopRules(t *testing.T) {
	ws := "ws://127.0.0.1:9222/devtools/page/1"
	tests := []struct {
		name       string
		targetType string
		targetURL  string
		wsURL      string
		want       bool
	}{
		{"page target with websocket", "page", "https://127.0.0.1:5000/", ws, true},
		{"webview target with websocket", "webview", "https://example.test/", ws, true},
		{"app target with websocket", "app", "", ws, true},
		{"worker target is not injectable", "worker", "https://example.test/", ws, false},
		{"iframe target is not an antigravity page", "iframe", "https://example.test/", ws, false},
		{"missing websocket url", "page", "https://example.test/", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInjectableTarget(AppAntigravity, tt.targetType, tt.targetURL, tt.wsURL); got != tt.want {
				t.Errorf("IsInjectableTarget(antigravity, %q, %q, %q) = %v, want %v",
					tt.targetType, tt.targetURL, tt.wsURL, got, tt.want)
			}
		})
	}
}

func TestIsInjectableTarget_VSCodeWorkbenchAndWebviews(t *testing.T) {
	ws := "ws://127.0.0.1:9333/devtools/page/1"
	tests := []struct {
		name       string
		app        string
		targetType string
		targetURL  string
		wsURL      string
		want       bool
	}{
		{"workbench page", AppVSCode, "page", "vscode-file://vscode-app/c:/Program%20Files/Microsoft%20VS%20Code/resources/app/out/vs/code/electron-sandbox/workbench/workbench.html", ws, true},
		{"webview iframe", AppVSCode, "iframe", "vscode-webview://0a1b2c3d/index.html?id=abc", ws, true},
		{"webview target", AppVSCode, "webview", "vscode-webview://0a1b2c3d/index.html", ws, true},
		{
			"antigravity extension content frame",
			AppVSCode,
			"iframe",
			"http://127.0.0.1:58544/?extensionView=true&extensionVariant=vs-code&hostVersion=1.8.0&ideVersion=1.141.2&useWebSocket=true",
			ws,
			true,
		},
		{
			"antigravity extension content frame on localhost",
			AppVSCode,
			"iframe",
			"http://localhost:58544/?extensionView=true",
			ws,
			true,
		},
		{
			"plain loopback dev server is skipped",
			AppVSCode,
			"iframe",
			"http://127.0.0.1:58544/index.html?foo=bar",
			ws,
			false,
		},
		{"insiders workbench page", AppVSCodeInsiders, "page", "vscode-file://vscode-app/x/workbench.html", ws, true},
		{"devtools page is skipped", AppVSCode, "page", "devtools://devtools/bundled/worker_app.html", ws, false},
		{"chrome extension page is skipped", AppVSCode, "page", "chrome-extension://abc/panel.html", ws, false},
		{"node target is skipped", AppVSCode, "node", "vscode-file://vscode-app/x/workbench.html", ws, false},
		{"missing websocket url", AppVSCode, "page", "vscode-file://vscode-app/x/workbench.html", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInjectableTarget(tt.app, tt.targetType, tt.targetURL, tt.wsURL); got != tt.want {
				t.Errorf("IsInjectableTarget(%q, %q, %q, %q) = %v, want %v",
					tt.app, tt.targetType, tt.targetURL, tt.wsURL, got, tt.want)
			}
		})
	}
}

func TestIsInjectableTarget_UnknownAppHasNoTargets(t *testing.T) {
	if IsInjectableTarget("junk", "page", "vscode-file://vscode-app/x", "ws://127.0.0.1:1/x") {
		t.Error("unknown apps must not have injectable targets")
	}
}

func TestFilterInjectableTargets(t *testing.T) {
	targets := []DevToolsTarget{
		{ID: "1", Type: "page", URL: "vscode-file://vscode-app/workbench.html", WebSocketDebuggerURL: "ws://127.0.0.1:9333/1"},
		{ID: "2", Type: "page", URL: "devtools://devtools/bundled/worker_app.html", WebSocketDebuggerURL: "ws://127.0.0.1:9333/2"},
		{ID: "3", Type: "iframe", URL: "vscode-webview://0a1b/index.html", WebSocketDebuggerURL: "ws://127.0.0.1:9333/3"},
		{ID: "4", Type: "page", URL: "vscode-file://vscode-app/workbench.html", WebSocketDebuggerURL: ""},
		{ID: "5", Type: "iframe", URL: "http://127.0.0.1:58544/?extensionView=true&extensionVariant=vs-code", WebSocketDebuggerURL: "ws://127.0.0.1:9333/5"},
		{ID: "6", Type: "service_worker", URL: "vscode-webview://0a1b/service-worker.js", WebSocketDebuggerURL: "ws://127.0.0.1:9333/6"},
	}

	got := FilterInjectableTargets(AppVSCode, targets)
	if len(got) != 3 {
		t.Fatalf("FilterInjectableTargets(vscode) returned %d targets, want 3 (%+v)", len(got), got)
	}
	if got[0].ID != "1" || got[1].ID != "3" || got[2].ID != "5" {
		t.Errorf("unexpected targets kept: %+v", got)
	}

	// Antigravity keeps page targets with a websocket URL; iframe, service worker
	// and websocket-less targets stay excluded.
	if all := FilterInjectableTargets(AppAntigravity, targets); len(all) != 2 {
		t.Errorf("antigravity should keep the 2 page targets with a websocket URL, got %d", len(all))
	}
}

func TestIsAntigravityExtensionViewURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"http://127.0.0.1:58544/?extensionView=true&extensionVariant=vs-code&hostVersion=1.8.0", true},
		{"http://localhost:1234/?extensionView=true", true},
		{"https://127.0.0.1:1234/?extensionView=true", true},
		{"http://127.0.0.1:58544/index.html", false},
		{"http://192.168.1.10:58544/?extensionView=true", false},
		{"https://example.com/?extensionView=true", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsAntigravityExtensionViewURL(tt.url); got != tt.want {
			t.Errorf("IsAntigravityExtensionViewURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestFindDevToolsPortForApp_VSCodeIsANoOpUnderUnitTests(t *testing.T) {
	inj := NewInjector(0)
	_, err := inj.FindDevToolsPortForApp(AppVSCode)
	if err == nil {
		t.Fatal("expected the live host CDP probe to be skipped under unit tests")
	}
	if !strings.Contains(err.Error(), "unit test mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFindDevToolsPortForApp_RejectsUnknownApp(t *testing.T) {
	inj := NewInjector(0)
	_, err := inj.FindDevToolsPortForApp("junk")
	if err == nil || !strings.Contains(err.Error(), "unsupported app") {
		t.Fatalf("expected unsupported app error, got %v", err)
	}
}

func TestIsLoopbackPortFreeAndPick(t *testing.T) {
	port, err := PickFreeLoopbackPort()
	if err != nil {
		t.Fatalf("PickFreeLoopbackPort failed: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("PickFreeLoopbackPort returned out-of-range port %d", port)
	}

	// The picked port must be free at pick time (it may be taken again later).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open probe listener: %v", err)
	}
	busyPort := ln.Addr().(*net.TCPAddr).Port
	if IsLoopbackPortFree(busyPort) {
		t.Errorf("IsLoopbackPortFree(%d) = true for a held port", busyPort)
	}
	_ = ln.Close()

	if IsLoopbackPortFree(0) || IsLoopbackPortFree(-1) || IsLoopbackPortFree(70000) {
		t.Error("out-of-range ports must not be reported free")
	}
}

func TestResolveVSCodeDebugPort(t *testing.T) {
	if p, err := ResolveVSCodeDebugPort(0); err != nil || p <= 0 {
		t.Fatalf("ResolveVSCodeDebugPort(0) = (%d, %v), want a free port", p, err)
	}

	// A busy configured port must fall back to a freshly picked free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open probe listener: %v", err)
	}
	defer ln.Close()
	busyPort := ln.Addr().(*net.TCPAddr).Port

	got, err := ResolveVSCodeDebugPort(busyPort)
	if err != nil {
		t.Fatalf("ResolveVSCodeDebugPort(%d) failed: %v", busyPort, err)
	}
	if got == busyPort {
		t.Errorf("busy configured port %d must not be reused", busyPort)
	}
	if got <= 0 || got > 65535 {
		t.Errorf("fallback port %d out of range", got)
	}
}

func TestResolveVSCodeDebugPort_ReusesLiveCDPEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" || r.URL.Path == "/json" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse test server url: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("failed to parse test server port: %v", err)
	}

	if !isCDPEndpointLive(port) {
		t.Fatal("test CDP server should be reported live")
	}
	// The port is busy (held by the test server) but already serves CDP, so the
	// configured port must be reused instead of picking a different one.
	got, err := ResolveVSCodeDebugPort(port)
	if err != nil {
		t.Fatalf("ResolveVSCodeDebugPort(%d) failed: %v", port, err)
	}
	if got != port {
		t.Errorf("ResolveVSCodeDebugPort(%d) = %d, want the live CDP port", port, got)
	}
}

func TestGenerateVSCodeScript_ContainsSharedStyleAndEnhancements(t *testing.T) {
	script := GenerateVSCodeScript(DefaultConfig())
	if !strings.Contains(script, "antigravity-swiss-styles") {
		t.Error("VS Code script must manage the shared #antigravity-swiss-styles element")
	}
	if !strings.Contains(script, "Antigravity Swiss Knife: App Enhancements") {
		t.Error("VS Code script must include the enhancements script")
	}
	if !strings.Contains(script, "__SWISS_ENH_CONFIG__") {
		t.Error("VS Code script must expose the enhancements config marker for the re-inject monitor")
	}

	if _, err := exec.LookPath("node"); err == nil {
		cmd := exec.Command("node", "--check")
		cmd.Stdin = strings.NewReader(script)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("node syntax error in GenerateVSCodeScript: %v\n%s", err, string(out))
		}
	}
}

func TestWithoutElectronRunAsNode(t *testing.T) {
	in := []string{
		"PATH=C:\\Windows",
		"ELECTRON_RUN_AS_NODE=1",
		"electron_run_as_node=",
		"NODE_OPTIONS=--max-old-space-size=4096",
	}
	got := withoutElectronRunAsNode(in)
	want := []string{
		"PATH=C:\\Windows",
		"NODE_OPTIONS=--max-old-space-size=4096",
	}
	if len(got) != len(want) {
		t.Fatalf("withoutElectronRunAsNode = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("env[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLaunchVSCode_MissingBinaryFailsCleanly(t *testing.T) {
	t.Setenv("VSCODE_BIN_PATH", filepath.Join(t.TempDir(), "does-not-exist", "Code.exe"))
	res, err := LaunchVSCode("")
	if err == nil {
		t.Fatal("expected an error when the VS Code binary does not exist")
	}
	if res == nil || res.Success {
		t.Fatalf("expected a failed ApplyResult, got %+v", res)
	}
}
