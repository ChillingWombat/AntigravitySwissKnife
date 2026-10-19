package webgui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
)

// TestEmpirical_CORSValidation stress-tests isAllowedLoopbackOrigin and corsMiddleware
// against hostile origins and Antigravity desktop origins.
func TestEmpirical_CORSValidation(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	handler := srv.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	hostileOrigins := []string{
		"http://127.0.0.1.attacker.com",
		"http://127.0.0.1.attacker.com:8765",
		"http://evil.com",
		"http://attacker.com:8765",
		"http://localhost.evil.com",
		"http://127.0.0.1evil.com",
		"http://[::1].evil.com",
		"null",
		"*",
		"file://attacker",
		"file://attacker/share/evil.html",
		"file:////attacker",
		"file:////attacker/share",
		"file://///attacker",
		"file://10.0.0.1/share",
		"file://192.168.1.1/share/evil.html",
		"javascript:alert(1)",
		"data:text/html,evil",
		"https://evil-127.0.0.1.com",
		"http://localhost:evil",
	}

	for _, origin := range hostileOrigins {
		// 1. Direct function test
		allowed := isAllowedLoopbackOrigin(origin)

		// 2. HTTP OPTIONS preflight test
		reqOptions := httptest.NewRequest(http.MethodOptions, "/api/gui/config", nil)
		reqOptions.Header.Set("Origin", origin)
		recOptions := httptest.NewRecorder()
		handler.ServeHTTP(recOptions, reqOptions)

		// 3. HTTP GET request test
		reqGet := httptest.NewRequest(http.MethodGet, "/api/gui/config", nil)
		reqGet.Header.Set("Origin", origin)
		recGet := httptest.NewRecorder()
		handler.ServeHTTP(recGet, reqGet)

		t.Logf("[CORS Hostile] Origin: %-35s -> isAllowed: %v, OPTIONS code: %d, GET code: %d",
			origin, allowed, recOptions.Code, recGet.Code)

		if allowed {
			t.Errorf("[VULNERABILITY] Hostile origin %q was ACCEPTED by isAllowedLoopbackOrigin!", origin)
		}
		if recOptions.Code != http.StatusForbidden {
			t.Errorf("[VULNERABILITY] Hostile origin %q preflight received status %d, want 403 Forbidden", origin, recOptions.Code)
		}
		if recGet.Code != http.StatusForbidden {
			t.Errorf("[VULNERABILITY] Hostile origin %q GET received status %d, want 403 Forbidden", origin, recGet.Code)
		}
	}

	validDesktopOrigins := []string{
		"http://127.0.0.1:8765",
		"http://127.0.0.1:5173",
		"http://localhost:5173",
		"http://localhost:8765",
		"http://127.0.0.1:3000",
		"http://[::1]:8765",
		"file://",
		"file:///var/projects/project/index.html",
		"file:///tmp/app.html",
		"vscode-file://vscode-app",
		"electron://",
		"electron://antigravity",
		"vscode-app://antigravity-window",
		"vscode-webview://antigravity-panel",
		"antigravity://",
		"antigravity://workbench",
		"chrome-extension://swiss-knife",
		"plugin://swiss-tools",
	}

	for _, origin := range validDesktopOrigins {
		allowed := isAllowedLoopbackOrigin(origin)
		if !allowed {
			t.Errorf("[REGRESSION] Legitimate desktop origin %q was REJECTED by isAllowedLoopbackOrigin", origin)
		}

		reqOptions := httptest.NewRequest(http.MethodOptions, "/api/gui/config", nil)
		reqOptions.Header.Set("Origin", origin)
		recOptions := httptest.NewRecorder()
		handler.ServeHTTP(recOptions, reqOptions)

		t.Logf("[CORS Legitimate] Origin: %-38s -> isAllowed: %v, OPTIONS code: %d",
			origin, allowed, recOptions.Code)

		if recOptions.Code != http.StatusNoContent {
			t.Errorf("[REGRESSION] Legitimate desktop origin %q preflight got %d, want 204", origin, recOptions.Code)
		}
		if gotAllow := recOptions.Header().Get("Access-Control-Allow-Origin"); gotAllow != origin {
			t.Errorf("[REGRESSION] Expected Access-Control-Allow-Origin %q, got %q", origin, gotAllow)
		}
	}
}

// TestEmpirical_AbsolutePathContract verifies the contract that replaced the
// directory whitelist in issue #157:
//  1. malformed input stays rejected — relative paths (including ".." and
//     encoded variants), null bytes and encoded traversal;
//  2. any absolute path is readable, including directories outside the user
//     home directory;
//  3. reads of sensitive locations (system directories, ~/.ssh, ~/.aws,
//     ~/.config/gcloud) succeed, while destructive operations on the same
//     locations are refused — read allowed, write denied.
//
// Renamed from TestEmpirical_PathJailing: with the whitelist gone there is no
// jail left, so the old name described behaviour the code no longer has.
func TestEmpirical_AbsolutePathContract(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Vectors that must stay invalid under the absolute-path contract.
	//    A relative path is never resolved against the working directory, so
	//    "../../etc/passwd" cannot silently become a real absolute path.
	rejectedVectors := []struct {
		name string
		path string
	}{
		{"Deep relative traversal", "../../../../../../../../../../etc/passwd"},
		{"Relative traversal to Windows", `..\..\Windows\System32\cmd.exe`},
		{"Relative sibling", "some/relative/file.txt"},
		{"Current directory alias", "."},
		{"Home-relative without tilde", "home/user/.ssh/config"},
		{"Encoded dot dot slash ..%2f", "..%2f..%2fetc/passwd"},
		{"Encoded dot dot slash %2e%2e", "%2e%2e%2f%2e%2e%2fetc%2fpasswd"},
		{"Encoded dot dot slash %252e", "%252e%252e%252fetc/passwd"},
		{"Encoded dot dot slash %252f", "..%252f..%252fetc/passwd"},
		{"Encoded dot dot slash deep escape", filepath.Join(tempDir, "../../../../../../../../..%2f..%2fetc/passwd")},
		{"Null byte injection 1 (null in filename)", filepath.Join(tempDir, "safe.txt\x00")},
		{"Null byte injection 2 (null in directory)", "/tmp/\x00/etc/passwd"},
		{"URL-encoded null byte %00 in path", filepath.Join(tempDir, "safe%00.txt")},
		{"Double URL-encoded null byte %2500 in path", filepath.Join(tempDir, "safe%2500.txt")},
		{"Triple URL-encoded null byte %252500 in path", filepath.Join(tempDir, "safe%252500.txt")},
		{"Triple URL-encoded traversal %25252e", "%25252e%25252e/%25252e%25252e/etc/passwd"},
		{"Encoded Windows backslash ..%5c", "..%5c..%5cetc/passwd"},
		{"Double encoded Windows backslash %255c", "..%255c..%255cetc/passwd"},
		{"Direct file://attacker/share/etc/passwd", "file://attacker/share/etc/passwd"},
		{"Path with space and null byte", filepath.Join(tempDir, "safe test\x00.txt")},
		{"Path with plus and null byte", filepath.Join(tempDir, "safe+test\x00.txt")},
		{"Raw %00 vector", "%00"},
		{"Raw %2500 vector", "%2500"},
		{"Raw ..%2f vector", "..%2f"},
		{"Raw %2e%2e vector", "%2e%2e"},
		{"Raw %252e vector", "%252e"},
		{"Raw %252f vector", "%252f"},
	}

	for _, tc := range rejectedVectors {
		resolved, err := validateJailedPath(tc.path)
		t.Logf("[Absolute path contract] Rejected vector: %-38s (%q) -> err: %v, resolved: %q", tc.name, tc.path, err, resolved)
		if err == nil {
			t.Errorf("[VULNERABILITY] validateJailedPath accepted malicious vector %s (%q) -> resolved to %q",
				tc.name, tc.path, resolved)
		}
	}

	// 2. An absolute directory outside the user home is readable. This is the
	//    behaviour the whitelist removal was for: a project on another drive.
	outsideDir := createDirOutsideHome(t)
	marker := filepath.Join(outsideDir, "outside-home.txt")
	if err := os.WriteFile(marker, []byte("reachable"), 0600); err != nil {
		t.Fatalf("failed to write marker file: %v", err)
	}
	readPath, err := validateJailedPath(marker)
	if err != nil {
		t.Errorf("[REGRESSION] absolute path outside home rejected: %q: %v", marker, err)
	} else {
		if content, errRead := os.ReadFile(readPath); errRead != nil || string(content) != "reachable" {
			t.Errorf("[REGRESSION] resolved outside-home path is not readable: %q: %v", readPath, errRead)
		}
	}
	if entries, errList := os.ReadDir(outsideDir); errList != nil || len(entries) == 0 {
		t.Errorf("[REGRESSION] outside-home directory is not listable: %q: %v", outsideDir, errList)
	}

	// 3. Reading sensitive locations is allowed; mutating them is not.
	var sensitiveReadPaths, sensitiveWritePaths []string
	if runtime.GOOS == "windows" {
		sensitiveReadPaths = []string{`C:\Windows\win.ini`, `C:\Windows\System32\drivers\etc\hosts`, `C:\ProgramData`}
	} else {
		sensitiveReadPaths = []string{"/etc/passwd", "/proc/cpuinfo", "/sys/kernel", "/bin/sh"}
	}
	for _, vec := range sensitiveReadPaths {
		res, err := validateJailedPath(vec)
		if err != nil {
			t.Errorf("[REGRESSION] validateJailedPath rejected readable path %q: %v", vec, err)
		} else if res == "" {
			t.Errorf("[REGRESSION] validateJailedPath(%q) returned empty path", vec)
		}
	}

	// Destructive operations must stay refused. Windows system paths are
	// expressed with backslashes and checked on every platform: the floor
	// normalises separators before matching.
	// Destructive operations must stay refused. System paths are OS-specific;
	// credential stores below the home directory are checked on every platform.
	if runtime.GOOS == "windows" {
		sensitiveWritePaths = []string{
			`C:\Windows\System32\evil.dll`,
			`C:\Program Files\evil.exe`,
			`C:\Program Files (x86)\evil.exe`,
		}
	} else {
		sensitiveWritePaths = []string{"/etc/passwd", "/usr/local/bin/evil", "/var/log/evil"}
	}
	home, errHome := os.UserHomeDir()
	if errHome == nil && home != "" {
		sensitiveWritePaths = append(sensitiveWritePaths,
			filepath.Join(home, ".ssh", "config"),
			filepath.Join(home, ".aws", "credentials"),
			filepath.Join(home, ".config", "gcloud", "application_default_credentials.json"),
		)
	}
	for _, vec := range sensitiveWritePaths {
		if res, err := validateDestructivePath(vec); err == nil {
			t.Errorf("[VULNERABILITY] destructive operation on sensitive path %q was allowed -> %q", vec, res)
		}
	}

	// Symlinks resolving outside the working directory are readable.
	if runtime.GOOS != "windows" {
		symlinkDir := filepath.Join(tempDir, "symlink_to_etc_dir")
		if err := os.Symlink("/etc", symlinkDir); err != nil {
			t.Skipf("cannot create directory symlink: %v", err)
		}
		if res, err := validateJailedPath(filepath.Join(symlinkDir, "passwd")); err != nil {
			t.Errorf("[REGRESSION] readable symlink to /etc/passwd rejected: %v", err)
		} else if res == "" {
			t.Errorf("expected non-empty resolved path for symlink")
		}
	}

	// 4. Verify legitimate paths are accepted
	legitTempFile := filepath.Join(tempDir, "legit_file.txt")
	if err := os.WriteFile(legitTempFile, []byte("ok"), 0600); err != nil {
		t.Fatalf("failed to write legit file: %v", err)
	}

	res, err := validateJailedPath(legitTempFile)
	if err != nil {
		t.Errorf("[REGRESSION] validateJailedPath rejected legitimate temp file %q: %v", legitTempFile, err)
	}
	if res != legitTempFile {
		t.Logf("Legitimate path normalized from %q to %q", legitTempFile, res)
	}
}

// TestEmpirical_ValidateIDE stress-tests ValidateIDE against command injection payloads.
func TestEmpirical_ValidateIDE(t *testing.T) {
	injectionPayloads := []string{
		"antigravity; rm -rf /",
		"cursor & calc",
		"$(whoami)",
		"`id`",
		"cursor | bash",
		"antigravity\nreboot",
		"antigravity\rreboot",
		"cursor>pwned",
		"cursor<input",
		"code /etc/passwd",
		"code --no-sandbox",
		"code \"injected\"",
		"code 'injected'",
		"code\t--flag",
		"code\\sub",
		"cursor && calc",
		"antigravity || id",
		"evil_ide",
		"/bin/bash",
		"/bin/sh",
		"cmd.exe",
		"powershell.exe",
		"python3",
		"curl http://attacker.com",
	}

	for _, payload := range injectionPayloads {
		res, err := ipc.ValidateIDE(payload)
		isAllowed := ipc.IsAllowedIDE(payload)
		t.Logf("[IDE Injection] Payload: %-30q -> err: %v, res: %q, isAllowed: %v", payload, err, res, isAllowed)

		if err == nil {
			t.Errorf("[VULNERABILITY] ValidateIDE accepted injection payload %q -> returned %q", payload, res)
		}
		if isAllowed {
			t.Errorf("[VULNERABILITY] IsAllowedIDE reported true for injection payload %q", payload)
		}
	}

	validIDEs := []struct {
		input    string
		expected string
	}{
		{"antigravity", "antigravity"},
		{"code", "code"},
		{"vscode", "code"},
		{"cursor", "cursor"},
		{"windsurf", "windsurf"},
		{"trae", "trae"},
		{"vscodium", "vscodium"},
		{"codium", "codium"},
		{"zed", "zed"},
		{"antigravity.exe", "antigravity"},
		{"cursor.exe", "cursor"},
		{"", "code"},
	}

	for _, tc := range validIDEs {
		res, err := ipc.ValidateIDE(tc.input)
		if err != nil {
			t.Errorf("[REGRESSION] ValidateIDE rejected valid IDE %q: %v", tc.input, err)
		}
		if res != tc.expected {
			t.Errorf("[REGRESSION] ValidateIDE(%q) = %q, want %q", tc.input, res, tc.expected)
		}
		if !ipc.IsAllowedIDE(tc.input) {
			t.Errorf("[REGRESSION] IsAllowedIDE(%q) returned false, want true", tc.input)
		}
	}
}
