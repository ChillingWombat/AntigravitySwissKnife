package main

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProbeMockEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	acpBin := filepath.Join(tmpDir, "antigravity-acp"+ext)
	probeBin := filepath.Join(tmpDir, "acp-probe"+ext)

	// Build binaries into temp directory
	buildCmd := exec.Command("go", "build", "-o", acpBin, "../antigravity-acp")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build antigravity-acp: %v\noutput: %s", err, string(out))
	}

	buildProbe := exec.Command("go", "build", "-o", probeBin, ".")
	if out, err := buildProbe.CombinedOutput(); err != nil {
		t.Fatalf("failed to build acp-probe: %v\noutput: %s", err, string(out))
	}

	t.Run("NewSessionAndPrompt", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, probeBin,
			"--mock",
			"--server", acpBin+" --surface ls",
			"--prompt", "ping-test",
		)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			t.Fatalf("acp-probe failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
		}

		out := stdout.String()
		if !strings.Contains(out, "agent: antigravity-acp") {
			t.Errorf("expected output to contain 'agent: antigravity-acp', got:\n%s", out)
		}
		if !strings.Contains(out, "session: mock-cascade-") {
			t.Errorf("expected output to contain 'session: mock-cascade-', got:\n%s", out)
		}
		if !strings.Contains(out, "mock: ping-test") {
			t.Errorf("expected output to contain 'mock: ping-test', got:\n%s", out)
		}
		if !strings.Contains(out, "stop: end_turn") {
			t.Errorf("expected output to contain 'stop: end_turn', got:\n%s", out)
		}
	})

	t.Run("LoadSessionHistoryReplay", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, probeBin,
			"--mock",
			"--server", acpBin+" --surface ls",
			"--session", "mock-session-test",
			"--prompt", "follow-up-turn",
		)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			t.Fatalf("acp-probe failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
		}

		out := stdout.String()
		if !strings.Contains(out, "[history user] past user prompt") {
			t.Errorf("expected replayed user history '[history user] past user prompt', got:\n%s", out)
		}
		if !strings.Contains(out, "[history agent] past agent response") {
			t.Errorf("expected replayed agent history '[history agent] past agent response', got:\n%s", out)
		}
		if !strings.Contains(out, "loaded session: mock-session-test") {
			t.Errorf("expected output to contain 'loaded session: mock-session-test', got:\n%s", out)
		}
		if !strings.Contains(out, "mock: follow-up-turn") {
			t.Errorf("expected streamed output 'mock: follow-up-turn', got:\n%s", out)
		}
		if !strings.Contains(out, "stop: end_turn") {
			t.Errorf("expected output to contain 'stop: end_turn', got:\n%s", out)
		}
	})
}
