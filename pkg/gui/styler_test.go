package gui

import (
	"strings"
	"testing"
)

func TestGenerateScriptWithCustomModels_InstallsAuthInterceptor(t *testing.T) {
	script := GenerateScript(nil)
	if !strings.Contains(script, "__swissAuthFetchPatched") {
		t.Fatalf("expected the fetch auth interceptor to be installed")
	}
	if !strings.Contains(script, `headers.set("X-Session-Token", token)`) {
		t.Fatalf("expected the interceptor to attach X-Session-Token for daemon-origin requests")
	}
	if !strings.Contains(script, "http://127.0.0.1:8765") {
		t.Fatalf("expected the interceptor to scope itself to the daemon origin")
	}
}

// CI has no session.token: the interceptor must still be installed so a fresh
// install authenticates as soon as the daemon writes one.
func TestGenerateScript_InterceptorWithoutSessionToken(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	script := GenerateScript(nil)
	if !strings.Contains(script, "__swissAuthFetchPatched") {
		t.Fatalf("interceptor must be present even when no session token exists")
	}
	if strings.Contains(script, "__SWISS_SESSION_TOKEN__ = \"\"") {
		t.Fatalf("must not assign an empty token")
	}
}
