package gui

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestMainLoaderScript_IncludesBrokerAndPassesNodeCheck(t *testing.T) {
	script := generateMainLoaderScript()
	for _, want := range []string{
		"__SWISS_MAIN_LOADER__",
		"buildDaemonBrokerScript",
		`path.join(configDir, "session.token")`,
		"const xhr = new XMLHttpRequest();",
		"installDaemonAuthBridge",
		"onBeforeSendHeaders",
		`headers['X-Session-Token'] = token`,
		"window.swiss = { request }",
		"__swissMainScriptInjected",
		"__swissDesktopScriptInjected",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("generateMainLoaderScript missing %s", want)
		}
	}
	if regexp.MustCompile(`[0-9a-fA-F]{64}`).MatchString(script) {
		t.Fatal("generateMainLoaderScript must not embed a daemon session token")
	}

	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is required for the main loader syntax check")
	}
	cmd := exec.Command("node", "--check")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("node syntax error in generateMainLoaderScript: %v\n%s", err, string(out))
	}
}

func TestInjectedDaemonRoutes_CoverPersistentUI(t *testing.T) {
	required := map[string][]string{
		"GET": {
			"/api/custom_models",
			"/api/gui/config",
			"/api/enhancements",
			"/api/conversations/status",
			"/api/tokens/chat-metrics",
		},
		"POST": {
			"/api/custom_models/active",
			"/api/custom_models/thinking_level",
			"/api/gui/color",
			"/api/enhancements",
			"/api/files/write",
			"/api/memos/save",
			"/api/github/issues/comment",
		},
		"DELETE": {
			"/api/extensions/community",
		},
	}

	for method, paths := range required {
		var allowed []string
		switch method {
		case "GET":
			allowed = injectedDaemonRouteAllowlist.GET
		case "POST":
			allowed = injectedDaemonRouteAllowlist.POST
		case "DELETE":
			allowed = injectedDaemonRouteAllowlist.DELETE
		}
		for _, path := range paths {
			if !containsString(allowed, path) {
				t.Errorf("%s route %s is missing from the injected daemon allowlist", method, path)
			}
		}
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
