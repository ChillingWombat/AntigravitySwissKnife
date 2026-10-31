package gui

import (
	"os/exec"
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
		`'X-Session-Token': token`,
		"window.swiss = { request }",
		"__swissMainScriptInjected",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("generateMainLoaderScript missing %s", want)
		}
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
