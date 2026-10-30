package process

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestCleanNativeEnvironment(t *testing.T) {
	dirtyEnv := []string{
		"PATH=/usr/bin:/bin",
		"CLOUD_CODE_URL=http://127.0.0.1:8765",
		"USER=david",
		"AGY_LLM_GATEWAY_URL=http://127.0.0.1:8765",
		"AGY_LLM_GATEWAY_API_KEY=secret_test_token",
		"AGY_LLM_GATEWAY_MODELS=deepseek-v4.1-flash",
		"HOME=/home/david",
	}

	cleaned := CleanNativeEnvironment(dirtyEnv)
	for _, v := range cleaned {
		if strings.HasPrefix(v, "CLOUD_CODE_URL=") {
			t.Errorf("CleanNativeEnvironment retained CLOUD_CODE_URL: %s", v)
		}
		if strings.HasPrefix(v, "AGY_LLM_GATEWAY_") {
			t.Errorf("CleanNativeEnvironment retained gateway variable: %s", v)
		}
	}

	hasPath := false
	hasHome := false
	for _, v := range cleaned {
		if strings.HasPrefix(v, "PATH=") {
			hasPath = true
		}
		if strings.HasPrefix(v, "HOME=") {
			hasHome = true
		}
	}
	if !hasPath || !hasHome {
		t.Errorf("CleanNativeEnvironment lost standard environment variables: %v", cleaned)
	}
}

func TestRestartAntigravityAppsToNative_NoRunningApp(t *testing.T) {
	shield := NewShield(0)
	if err := RestartAntigravityAppsToNative(shield); err != nil {
		t.Errorf("RestartAntigravityAppsToNative returned error when no apps running: %v", err)
	}
}

func TestIsProxyEndpointAvailable(t *testing.T) {
	// 1. Live listener should return true
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, portStr, err := net.SplitHostPort(ts.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, _ := strconv.Atoi(portStr)

	if !isProxyEndpointAvailable(port) {
		t.Errorf("expected isProxyEndpointAvailable(%d) = true for active listener", port)
	}

	// 2. Unbound port should return false
	unboundPort := 59876
	if isProxyEndpointAvailable(unboundPort) {
		t.Errorf("expected isProxyEndpointAvailable(%d) = false for unbound port", unboundPort)
	}
}

func TestRestartAntigravityAppsToNativePropagatesFailures(t *testing.T) {
	detectionErr := errors.New("detection failed")
	relaunchErr := errors.New("relaunch failed")
	terminationErr := errors.New("termination failed")
	tests := []struct {
		name       string
		deps       restartNativeDeps
		wantErrors []error
	}{
		{
			name: "detection",
			deps: restartNativeDeps{
				desktopRunning: func(*Shield) (bool, error) { return false, detectionErr },
				relaunch:       func(*Shield) error { return nil },
				terminate:      func(*Shield) error { return nil },
			},
			wantErrors: []error{detectionErr},
		},
		{
			name: "relaunch",
			deps: restartNativeDeps{
				desktopRunning: func(*Shield) (bool, error) { return true, nil },
				relaunch:       func(*Shield) error { return relaunchErr },
				terminate:      func(*Shield) error { return nil },
			},
			wantErrors: []error{relaunchErr},
		},
		{
			name: "termination",
			deps: restartNativeDeps{
				desktopRunning: func(*Shield) (bool, error) { return false, nil },
				relaunch:       func(*Shield) error { return nil },
				terminate:      func(*Shield) error { return terminationErr },
			},
			wantErrors: []error{terminationErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := restartAntigravityAppsToNative(NewShield(0), tt.deps)
			for _, want := range tt.wantErrors {
				if !errors.Is(err, want) {
					t.Fatalf("expected error %v, got %v", want, err)
				}
			}
		})
	}
}
