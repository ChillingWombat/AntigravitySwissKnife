package process

import (
	"testing"
)

func TestChallenger_Iteration2_AdversarialCommands(t *testing.T) {
	// Commands that contain arguments ending in antigravity or Antigravity.exe,
	// but are NOT the Antigravity IDE executable.
	hostileCommands := []struct {
		desc    string
		cmdline string
	}{
		// 1. Relative path arguments without leading slash/backslash
		{"relative path arg to cat", "cat dir/antigravity"},
		{"relative path arg to vim", "vim src/antigravity"},
		{"relative path arg to notepad", `notepad.exe tools\Antigravity.exe`},
		{"relative path arg with subdirs", `notepad.exe sub\tools\Antigravity.exe`},
		{"relative path arg to python", "python script.py mypkg/antigravity"},
		{"relative path arg to node", "node app.js dist/antigravity"},
		{"relative path arg to bash", "bash scripts/antigravity"},
		{"relative path arg to docker", "docker run local/antigravity"},

		// 2. Absolute binary path with relative argument ending in antigravity
		{"absolute binary with relative arg unix", "/usr/bin/cat dir/antigravity"},
		{"absolute binary with relative arg windows", `C:\Windows\System32\notepad.exe tools\Antigravity.exe`},

		// 3. Tab-separated command arguments
		{"tab-separated cat unix path", "cat\t/usr/bin/antigravity"},
		{"tab-separated notepad UNC", "notepad.exe\t\\\\server\\share\\Antigravity.exe"},
		{"tab-separated relative path", "cat\t./antigravity"},
		{"tab-separated absolute unix binary", "/usr/bin/cat\t/usr/bin/antigravity"},
		{"tab-separated absolute windows binary", `C:\Windows\System32\notepad.exe` + "\t" + `C:\tools\Antigravity.exe`},

		// 4. URLs without http:// or https:// scheme
		{"curl domain path", "curl example.com/antigravity"},
		{"wget domain path", "wget dl.google.com/antigravity"},
		{"git clone without scheme", "git clone github.com/foo/antigravity"},

		// 5. Git branches, container images, and package names ending in antigravity
		{"git checkout feature branch", "git checkout feature/antigravity"},
		{"git merge feature branch", "git merge feature/antigravity"},
		{"docker pull registry", "docker pull registry.internal/antigravity"},
		{"kubectl logs pod", "kubectl logs pod/antigravity"},
		{"npm install package", "npm install @scope/antigravity"},
	}

	for _, tc := range hostileCommands {
		t.Run(tc.desc, func(t *testing.T) {
			// Direct check on IsExactAntigravityExecutable
			if IsExactAntigravityExecutable(tc.cmdline) {
				t.Errorf("[VULNERABILITY] IsExactAntigravityExecutable(%q) = true, expected false", tc.cmdline)
			}

			// End-to-end check via extractArgv0 + IsExactAntigravityExecutable
			argv0 := extractArgv0(tc.cmdline)
			if IsExactAntigravityExecutable(argv0) {
				t.Errorf("[VULNERABILITY] extractArgv0(%q) yielded %q which IsExactAntigravityExecutable accepted!", tc.cmdline, argv0)
			}
		})
	}
}

func TestChallenger_Iteration2_ShieldFalsePositive_Subprocess(t *testing.T) {
	// Verify that common developer commands do NOT fool Shield.IsAntigravityRunning()
	falsePositiveCommands := []string{
		"git checkout feature/antigravity",
		"docker pull internal-registry/antigravity",
		"kubectl logs pod/antigravity",
		"cat dir/antigravity",
		"vim src/antigravity",
		`notepad.exe tools\Antigravity.exe`,
		"/usr/bin/cat dir/antigravity",
		`C:\Windows\System32\notepad.exe tools\Antigravity.exe`,
		"cat\t/usr/bin/antigravity",
		`C:\Windows\System32\notepad.exe` + "\t" + `C:\tools\Antigravity.exe`,
	}

	for _, cmd := range falsePositiveCommands {
		t.Run(cmd, func(t *testing.T) {
			shield := NewShield(0)
			shield.SetProcessFinder(func() ([]ProcessInfo, error) {
				return []ProcessInfo{
					{
						PID:     9999,
						Name:    "bash",
						Cmdline: cmd,
					},
				}, nil
			})

			if shield.IsAntigravityRunning() {
				t.Errorf("[CRITICAL FALSE POSITIVE] Process shield believes Antigravity IDE is running for command: %q", cmd)
			}
		})
	}
}
