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
