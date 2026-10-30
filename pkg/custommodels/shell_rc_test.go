package custommodels

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPlanShellRCRewrite(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		owned   bool
		changed bool
		legacy  bool
		wantErr bool
	}{
		{
			name: "unrelated export is byte-preserved",
			data: "alias ls='ls --color=auto'\nexport CLOUD_CODE_URL=\"https://example.test\"\n",
			want: "alias ls='ls --color=auto'\nexport CLOUD_CODE_URL=\"https://example.test\"\n",
		},
		{
			name:  "managed block removed",
			data:  "before\n" + shellRCBeginMarker + "\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n" + shellRCEndMarker + "\nafter\n",
			want:  "before\nafter\n",
			owned: true, changed: true,
		},
		{
			name:  "crlf and missing trailing newline preserved",
			data:  "before\r\n" + shellRCBeginMarker + "\r\nexport AGY_LLM_GATEWAY_URL=\"http://127.0.0.1:8765\"\r\n" + shellRCEndMarker + "\r\nafter",
			want:  "before\r\nafter",
			owned: true, changed: true,
		},
		{
			name:  "legacy block removed once",
			data:  "alias ls='ls --color=auto'\n# Antigravity Swiss Knife - CLI Integration Environment\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\nexport AGY_LLM_GATEWAY_URL=\"http://127.0.0.1:8765\"\n",
			want:  "alias ls='ls --color=auto'\n",
			owned: true, changed: true, legacy: true,
		},
		{
			name:  "unrelated export after legacy block survives",
			data:  "# Antigravity Swiss Knife - CLI Integration Environment\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\nalias keep=1\nexport CLOUD_CODE_URL=\"https://example.test\"\n",
			want:  "alias keep=1\nexport CLOUD_CODE_URL=\"https://example.test\"\n",
			owned: true, changed: true, legacy: true,
		},
		{
			name:    "unterminated managed block fails closed",
			data:    "before\n" + shellRCBeginMarker + "\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n",
			wantErr: true,
		},
		{
			name:    "stray end marker fails closed",
			data:    "before\n" + shellRCEndMarker + "\n",
			wantErr: true,
		},
		{
			name:    "mismatched markers fail closed",
			data:    shellRCBeginMarker + "\n# END Antigravity Swiss Knife CLI Integration\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := scanShellRC([]byte(tt.data))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected scan error")
				}
				return
			}
			if err != nil {
				t.Fatalf("scanShellRC: %v", err)
			}
			if string(plan.Rewritten) != tt.want {
				t.Fatalf("rewritten mismatch\nwant: %q\n got: %q", tt.want, string(plan.Rewritten))
			}
			if plan.Owned != tt.owned || plan.Changed != tt.changed || plan.Legacy != tt.legacy {
				t.Fatalf("plan flags = Owned:%v Changed:%v Legacy:%v", plan.Owned, plan.Changed, plan.Legacy)
			}
		})
	}
}

func TestCleanSingleShellRCPreservesMetadataAndAtomicReplace(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "zshrc-target")
	link := filepath.Join(dir, ".zshrc")
	content := "alias ls='ls --color=auto'\n" + shellRCBeginMarker + "\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n" + shellRCEndMarker + "\nexport CLOUD_CODE_URL=\"https://example.test\"\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := cleanSingleShellRC(link); err != nil {
		t.Fatalf("cleanSingleShellRC: %v", err)
	}
	linkInfo, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatal("shell RC symlink was replaced")
	}
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != target {
		t.Fatalf("symlink target = %q, want %q", resolved, target)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	want := "alias ls='ls --color=auto'\nexport CLOUD_CODE_URL=\"https://example.test\"\n"
	if string(data) != want {
		t.Fatalf("content = %q, want %q", string(data), want)
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, ".zshrc-target.swiss-tmp-*")); len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}

	if err := cleanSingleShellRC(link); err != nil {
		t.Fatalf("second clean: %v", err)
	}
	again, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !again.ModTime().Equal(info.ModTime()) {
		t.Fatal("unchanged plan rewrote the shell RC")
	}
}

func TestCleanSingleShellRCLegacyBackupOnce(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	original := "alias ls='ls --color=auto'\n# Antigravity Swiss Knife - CLI Integration Environment\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\nexport AGY_LLM_GATEWAY_URL=\"http://127.0.0.1:8765\"\n"
	if err := os.WriteFile(rc, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	plan, err := scanShellRC([]byte(original))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 30, 12, 34, 56, 123456789, time.UTC)
	if err := commitShellRC(rc, plan, now); err != nil {
		t.Fatalf("commitShellRC: %v", err)
	}
	backup := rc + ".swiss-backup-" + now.Format("20060102T150405.000000000Z")
	backupData, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backupData) != original {
		t.Fatalf("backup = %q, want original bytes", string(backupData))
	}
	if err := commitShellRC(rc, plan, now.Add(time.Second)); err != nil {
		t.Fatalf("second commitShellRC: %v", err)
	}
	backups, err := filepath.Glob(rc + ".swiss-backup-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("backups = %v, want exactly one", backups)
	}
}

func TestCleanSingleShellRCFailsClosedOnMalformedMarker(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	original := "before\n" + shellRCBeginMarker + "\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n"
	if err := os.WriteFile(rc, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cleanSingleShellRC(rc); err == nil {
		t.Fatal("expected malformed marker error")
	}
	data, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatal("malformed marker block was rewritten")
	}
	backups, _ := filepath.Glob(rc + ".swiss-backup-*")
	if len(backups) != 0 {
		t.Fatalf("malformed marker created backups: %v", backups)
	}
}

func TestCleanShellRCHooksRepeatedAndConcurrent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	zshrc := filepath.Join(home, ".zshrc")
	bashrc := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(zshrc, []byte("# Antigravity Swiss Knife - CLI Integration Environment\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\nexport AGY_LLM_GATEWAY_URL=\"http://127.0.0.1:8765\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bashrc, []byte(shellRCBeginMarker+"\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n"+shellRCEndMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- CleanShellRCHooks()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("CleanShellRCHooks: %v", err)
		}
	}
	if err := CleanShellRCHooks(); err != nil {
		t.Fatalf("repeated CleanShellRCHooks: %v", err)
	}
	for _, rc := range []string{zshrc, bashrc} {
		data, err := os.ReadFile(rc)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "CLOUD_CODE_URL") {
			t.Fatalf("%s still contains managed export: %q", rc, string(data))
		}
	}
	backups, _ := filepath.Glob(zshrc + ".swiss-backup-*")
	if len(backups) != 1 {
		t.Fatalf("legacy backups = %v, want one", backups)
	}
}

func TestCleanShellRCHooksMissingFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := CleanShellRCHooks(); err != nil {
		t.Fatalf("missing shell RC files: %v", err)
	}
}

func TestCleanShellRCHooksFailsClosedAcrossAllRCs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	zshrc := filepath.Join(home, ".zshrc")
	bashrc := filepath.Join(home, ".bashrc")
	zshOriginal := "before\n" + shellRCBeginMarker + "\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n"
	bashOriginal := "alias keep=1\n" + shellRCBeginMarker + "\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n" + shellRCEndMarker + "\n"
	if err := os.WriteFile(zshrc, []byte(zshOriginal), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bashrc, []byte(bashOriginal), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := CleanShellRCHooks(); err == nil {
		t.Fatal("expected malformed .zshrc to fail cleanup")
	}
	for path, want := range map[string]string{zshrc: zshOriginal, bashrc: bashOriginal} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("%s changed after plan failure: %q", path, string(data))
		}
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".swiss") {
			t.Fatalf("plan failure left residue: %s", entry.Name())
		}
	}
}

func TestCleanShellRCHooksCommitsBothValidRCs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	zshrc := filepath.Join(home, ".zshrc")
	bashLink := filepath.Join(home, ".bashrc")
	dotfiles := filepath.Join(home, "dotfiles")
	bashTarget := filepath.Join(dotfiles, "bashrc")
	if err := os.MkdirAll(dotfiles, 0o700); err != nil {
		t.Fatal(err)
	}
	zshContent := "before\n" + shellRCBeginMarker + "\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n" + shellRCEndMarker + "\nafter\n"
	bashContent := "alias keep=1\n" + shellRCBeginMarker + "\nexport AGY_LLM_GATEWAY_URL=\"http://127.0.0.1:8765\"\n" + shellRCEndMarker + "\n"
	if err := os.WriteFile(zshrc, []byte(zshContent), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bashTarget, []byte(bashContent), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(bashTarget, bashLink); err != nil {
		t.Fatal(err)
	}

	if err := CleanShellRCHooks(); err != nil {
		t.Fatalf("CleanShellRCHooks: %v", err)
	}
	for _, path := range []string{zshrc, bashTarget} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), shellRCBeginMarker) || strings.Contains(string(data), shellRCEndMarker) {
			t.Fatalf("%s still contains managed markers: %q", path, string(data))
		}
	}
	info, err := os.Lstat(bashLink)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("shell RC symlink was replaced")
	}
}

func TestCheckCLIGatewayStatusIgnoresUnrelatedExports(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	agy := filepath.Join(bin, "agy")
	if err := os.WriteFile(agy, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin)

	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("export CLOUD_CODE_URL=\"https://example.test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	status := CheckCLIGatewayStatus(nil)
	if status.ShellRCHooked {
		t.Fatal("unrelated CLOUD_CODE_URL export marked shell RC as hooked")
	}

	if err := os.WriteFile(zshrc, []byte(shellRCBeginMarker+"\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n"+shellRCEndMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	status = CheckCLIGatewayStatus(nil)
	if !status.ShellRCHooked {
		t.Fatal("managed marker block was not detected")
	}
}
