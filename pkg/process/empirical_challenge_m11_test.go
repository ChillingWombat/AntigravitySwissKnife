package process

import (
	"testing"
)

// ============================================================================
// Milestone M11: Adversarial Process Shield & Path Normalization Challenge
// ============================================================================

// 1. Windows Backslashes & Separator Variations
func TestAdversarialM11_WindowsPathVariants(t *testing.T) {
	validPaths := []string{
		// Standard drive letter paths
		`C:\Program Files\Google\Antigravity\Antigravity.exe`,
		`c:\program files\google\antigravity\antigravity.exe`,
		`D:\Tools\Antigravity\Antigravity.exe`,
		`E:\antigravity\Antigravity.exe`,
		`Z:\Antigravity.exe`,
		`c:\Antigravity.exe`,

		// Root and relative paths
		`C:\antigravity`,
		`D:\Antigravity`,
		`.\Antigravity.exe`,
		`..\Antigravity.exe`,
		`..\..\Google\Antigravity\Antigravity.exe`,
		`.\antigravity`,
		`..\antigravity`,

		// Mixed forward and backward slashes
		`C:\Program Files/Google\Antigravity/Antigravity.exe`,
		`C:/Program Files\Google/Antigravity\antigravity.exe`,
		`C:\foo/bar\baz/Antigravity.exe`,
		`/opt\google/antigravity`,
		`/usr\bin/antigravity.exe`,
		`/var\opt/tools\antigravity`,
		`C:/foo\bar/Antigravity`,

		// Consecutive and redundant separators
		`C:\\Program Files\\\\Google\\Antigravity\\\\Antigravity.exe`,
		`C://Program Files////Google//Antigravity////Antigravity.exe`,
		`C:\/\Program Files\//Antigravity\/Antigravity.exe`,
		`/usr//bin///antigravity`,
		`\\network-share\\tools\\\\Antigravity.exe`,
	}

	for _, p := range validPaths {
		if !IsExactAntigravityExecutable(p) {
			t.Errorf("Adversarial Failure: expected IsExactAntigravityExecutable(%q) = true, got false", p)
		}
	}
}

// 2. UNC Network Paths and Device/Namespace Formats
func TestAdversarialM11_UNCNetworkPaths(t *testing.T) {
	validUNC := []string{
		// Standard UNC paths
		`\\server\share\Antigravity.exe`,
		`\\server\share\tools\bin\Antigravity.exe`,
		`\\nas-storage\apps\Google\Antigravity\antigravity.exe`,
		`\\192.168.1.100\deploy\Antigravity.exe`,
		`\\127.0.0.1\c$\Antigravity\Antigravity.exe`,

		// Mixed-separator UNC paths
		`\\server/share\tools/Antigravity.exe`,
		`//server\share/tools\antigravity.exe`,
		`//server/share/Antigravity.exe`,
		`//nas-storage/apps/antigravity.exe`,

		// Win32 Device & Extended Namespaces
		`\\?\C:\Program Files\Antigravity\Antigravity.exe`,
		`\\.\C:\Program Files\Antigravity\Antigravity.exe`,
		`\\?\UNC\server\share\Antigravity.exe`,
		`\\?\UNC\192.168.1.50\tools\Antigravity.exe`,
		`//?/C:/Program Files/Antigravity/Antigravity.exe`,
		`//./C:/Program Files/Antigravity/Antigravity.exe`,
	}

	for _, unc := range validUNC {
		if !IsExactAntigravityExecutable(unc) {
			t.Errorf("Adversarial Failure: expected UNC path IsExactAntigravityExecutable(%q) = true, got false", unc)
		}
	}
}

// 3. Quotes, Tabs, Carriage Returns, and Outer Whitespace Handling
func TestAdversarialM11_QuotesAndWhitespace(t *testing.T) {
	validQuoted := []string{
		// Double quotes
		`"C:\Program Files\Antigravity\Antigravity.exe"`,
		`"Antigravity.exe"`,
		`"/usr/bin/antigravity"`,

		// Single quotes
		`'C:\Program Files\Antigravity\Antigravity.exe'`,
		`'Antigravity.exe'`,
		`'/usr/bin/antigravity'`,

		// Outer whitespace around quotes
		`   "C:\Program Files\Antigravity\Antigravity.exe"   `,
		"\t'C:\\Program Files\\Antigravity\\Antigravity.exe'\r\n",
		" \r\n \"Antigravity.exe\" \t ",
		" \r\n 'antigravity' \t ",

		// Whitespace inside quotes (spaces around valid executable)
		`" C:\Program Files\Antigravity\Antigravity.exe "`,
		`' C:\Program Files\Antigravity\Antigravity.exe '`,
		"\" \t C:\\Program Files\\Antigravity\\Antigravity.exe \r\n \"",

		// Unmatched quotes gracefully trimmed
		`"C:\Program Files\Antigravity\Antigravity.exe`,
		`C:\Program Files\Antigravity\Antigravity.exe"`,
		`'C:\Program Files\Antigravity\Antigravity.exe`,
		`C:\Program Files\Antigravity\Antigravity.exe'`,
	}

	for _, q := range validQuoted {
		if !IsExactAntigravityExecutable(q) {
			t.Errorf("Adversarial Failure: expected quoted path IsExactAntigravityExecutable(%q) = true, got false", q)
		}
	}

	// Degenerate quotes & whitespace-only inputs that must evaluate to false
	invalidDegenerate := []string{
		"",
		" ",
		"    ",
		"\t",
		"\r\n",
		"\t\r\n   ",
		`""`,
		`''`,
		`" "`,
		`' '`,
		`"" ""`,
		`"'"`,
		`'""'`,
		`""""`,
	}

	for _, d := range invalidDegenerate {
		if IsExactAntigravityExecutable(d) {
			t.Errorf("Adversarial Failure: expected degenerate input IsExactAntigravityExecutable(%q) = false, got true", d)
		}
	}
}

// 4. Trailing Slashes & Directory Rejection
func TestAdversarialM11_TrailingSlashesAndDirectories(t *testing.T) {
	directories := []string{
		// Directories named antigravity
		`C:\Program Files\Google\Antigravity\`,
		`C:\Program Files\Google\Antigravity/`,
		`/opt/google/antigravity/`,
		`/opt/google/antigravity//`,
		`antigravity/`,
		`antigravity\`,
		`Antigravity/`,
		`Antigravity\`,

		// Directories deceptively ending with .exe/ or .exe\
		`C:\Program Files\Antigravity.exe\`,
		`C:\Program Files\Antigravity.exe/`,
		`C:\Tools\Antigravity.EXE\`,
		`antigravity.exe/`,
		`antigravity.exe\`,
		`Antigravity.exe/`,
		`Antigravity.exe\`,

		// Quoted directory paths with trailing slashes
		`"C:\Program Files\Google\Antigravity\"`,
		`"C:\Program Files\Google\Antigravity/"`,
		`'C:\Program Files\Google\Antigravity/'`,
		`'C:\Program Files\Google\Antigravity\'`,
		`"antigravity/"`,
		`"antigravity\"`,
		`"Antigravity.exe\"`,
		`"Antigravity.exe/"`,

		// UNC directories
		`\\server\share\Antigravity\`,
		`\\server\share\Antigravity/`,
		`\\?\C:\Program Files\Antigravity\`,
		`\\?\UNC\server\share\Antigravity/`,

		// Pure root directories
		`/`,
		`\`,
		`//`,
		`\\`,
		`C:\`,
		`C:/`,
		`D:\`,
	}

	for _, dir := range directories {
		if IsExactAntigravityExecutable(dir) {
			t.Errorf("Adversarial Failure: directory path %q was incorrectly accepted as an executable", dir)
		}
	}
}

// 5. Case Permutations
func TestAdversarialM11_CasePermutations(t *testing.T) {
	casePermutations := []string{
		"antigravity",
		"Antigravity",
		"ANTIGRAVITY",
		"aNtIgRaViTy",
		"AnTiGrAvItY",
		"antiGRAVITY",
		"ANTIgravity",
		"antigravity.exe",
		"Antigravity.exe",
		"ANTIGRAVITY.EXE",
		"Antigravity.Exe",
		"antigravity.EXE",
		"ANTIGRAVITY.exe",
		"aNtIgRaViTy.ExE",
		"AnTiGrAvItY.eXe",
		`C:\PROGRAM FILES\GOOGLE\ANTIGRAVITY\ANTIGRAVITY.EXE`,
		`c:\program files\google\antigravity\antigravity.exe`,
		`C:\Program Files\Google\Antigravity\Antigravity.Exe`,
		`\\SERVER\SHARE\TOOLS\ANTIGRAVITY.EXE`,
		`\\server\share\tools\antigravity.exe`,
	}

	for _, cp := range casePermutations {
		if !IsExactAntigravityExecutable(cp) {
			t.Errorf("Adversarial Failure: case permutation %q failed to match", cp)
		}
	}
}

// 6. Suffix, Prefix, Extension, and Helper Spoofing Attacks
func TestAdversarialM11_SuffixAndPrefixSpoofing(t *testing.T) {
	spoofedBinaries := []string{
		// Suffix spoofing
		"fake_antigravity.exe",
		"fake-antigravity.exe",
		"not-antigravity.exe",
		"not_antigravity.exe",
		"antigravity_fake.exe",
		"antigravity-fake.exe",
		"antigravity_helper.exe",
		"antigravity-helper.exe",
		"antigravity_daemon.exe",
		"antigravity-service.exe",
		"antigravity_installer.exe",
		"antigravity_updater.exe",
		"antigravity_crashpad_handler.exe",
		"antigravity_agent.exe",
		"antigravity_bridge.exe",
		"antigravity2.exe",
		"antigravity_v2.exe",
		"antigravity32.exe",
		"antigravity64.exe",
		"myantigravity.exe",
		"superantigravity.exe",

		// Backup and secondary extensions
		"antigravity.exe.bak",
		"antigravity.exe.old",
		"antigravity.exe.tmp",
		"antigravity.exe.1",
		"antigravity.exe.backup",
		"antigravity.exe.swp",
		"antigravity.exe.orig",
		"antigravity.exe.dist",
		"antigravity.exe.disabled",

		// Script and shortcut extensions
		"antigravity.exe.lnk",
		"antigravity.exe.bat",
		"antigravity.exe.cmd",
		"antigravity.exe.ps1",
		"antigravity.exe.vbs",
		"antigravity.lnk",
		"antigravity.bat",
		"antigravity.cmd",
		"antigravity.ps1",
		"antigravity.vbs",
		"antigravity.sh",
		"antigravity.bash",
		"antigravity.zsh",
		"antigravity.py",
		"antigravity.js",
		"antigravity.ts",

		// Libraries and drivers
		"antigravity.dll",
		"antigravity.so",
		"antigravity.dylib",
		"libantigravity.so",
		"libantigravity.dylib",
		"antigravity.sys",
		"antigravity.drv",

		// Alternate data streams (NTFS ADS)
		"antigravity.exe:ads",
		"antigravity.exe:stream",
		"antigravity.exe:$DATA",

		// Prefix spoofing without separator
		"fakeantigravity",
		"anti-antigravity",
		"notantigravity",
		"myantigravity",
		"real_antigravity",

		// Unrelated common binaries
		"cmd.exe",
		"powershell.exe",
		"bash",
		"sh",
		"zsh",
		"python.exe",
		"python3",
		"node.exe",
		"code.exe",
		"notepad.exe",
	}

	for _, sb := range spoofedBinaries {
		if IsExactAntigravityExecutable(sb) {
			t.Errorf("Adversarial Security Failure: spoofed binary %q was erroneously accepted as Antigravity host IDE", sb)
		}
	}
}

// 7. Swiss Knife Companion Self-Exclusion
func TestAdversarialM11_SwissCompanionSelfExclusion(t *testing.T) {
	swissCompanions := []string{
		"swiss",
		"swiss.exe",
		"SWISS.EXE",
		"Swiss.exe",
		"swiss_daemon",
		"swiss_daemon.exe",
		"swiss-daemon.exe",
		"antigravity-swiss",
		"antigravity-swiss.exe",
		"ANTIGRAVITY-SWISS.EXE",
		"swiss-antigravity.exe",
		"antigravity_swiss.exe",
		"antigravity.swiss.exe",
		"antigravity-swi",
		"swiss.linux",
		"swiss.darwin",
		`C:\Program Files\Antigravity Swiss Knife\swiss.exe`,
		`C:\swiss\antigravity-swiss.exe`,
		`/usr/local/bin/swiss`,
		`/opt/antigravity-swiss/swiss`,
		`/usr/bin/antigravity-swiss`,
		`"C:\Program Files\Antigravity Swiss Knife\swiss.exe"`,
		`'C:\Program Files\Antigravity Swiss Knife\antigravity-swiss.exe'`,
	}

	for _, sc := range swissCompanions {
		if IsExactAntigravityExecutable(sc) {
			t.Errorf("Adversarial Security Failure: Swiss Knife companion %q must NEVER be identified as the host IDE", sc)
		}
	}
}

// 8. Unicode Directories and Homoglyph Spoofing Defense
func TestAdversarialM11_UnicodeAndHomoglyphs(t *testing.T) {
	// Legitimate international directory paths containing valid ASCII binary
	validUnicodeDirs := []string{
		`C:\用户\Google\Antigravity\Antigravity.exe`,
		`C:\ユーザー\Antigravity\Antigravity.exe`,
		`C:\Пользователи\Antigravity\Antigravity.exe`,
		`C:\Bénéficiaire\Antigravity\Antigravity.exe`,
		`D:\Проекты\Antigravity\Antigravity.exe`,
		`D:\项目\Antigravity\Antigravity.exe`,
		`C:\José\Tools\Antigravity\Antigravity.exe`,
		`D:\München\Antigravity\Antigravity.exe`,
		`/home/ユーザー/antigravity`,
		`/opt/développement/antigravity`,
	}

	for _, ud := range validUnicodeDirs {
		if !IsExactAntigravityExecutable(ud) {
			t.Errorf("Adversarial Failure: unicode directory path %q failed to match valid binary", ud)
		}
	}

	// Malicious homoglyph and non-ASCII binary names that attempt homograph spoofing
	maliciousHomoglyphs := []string{
		"ａｎｔｉｇｒａｖｉｔｙ.exe",           // Full-width ASCII
		"ａｎｔｉｇｒａｖｉｔｙ",               // Full-width without .exe
		"antigravity\u200B.exe",          // Zero-width space
		"аntigravity.exe",                // Cyrillic small letter 'а' (U+0430)
		"antigrаvity.exe",                // Cyrillic small letter 'а' in middle
		"antigravіty.exe",                // Cyrillic small letter 'і' (U+0456)
		"antigravity\u00A0.exe",          // Non-breaking space
	}

	for _, hg := range maliciousHomoglyphs {
		if IsExactAntigravityExecutable(hg) {
			t.Errorf("Adversarial Security Failure: unicode homoglyph spoofing %q was falsely accepted as valid binary", hg)
		}
	}
}

// 9. extractArgv0 Command-Line Token Extraction & Parsing Stress Harness
func TestAdversarialM11_ExtractArgv0_StressHarness(t *testing.T) {
	type testCase struct {
		cmdline      string
		expectedArg0 string
		isHostIDE    bool
	}

	cases := []testCase{
		// Quoted Windows paths with spaces and various argument styles
		{
			cmdline:      `"C:\Program Files\Google\Antigravity\Antigravity.exe" --flag1 --flag2="with spaces"`,
			expectedArg0: `C:\Program Files\Google\Antigravity\Antigravity.exe`,
			isHostIDE:    true,
		},
		{
			cmdline:      `'C:\Program Files\Google\Antigravity\Antigravity.exe' -arg`,
			expectedArg0: `C:\Program Files\Google\Antigravity\Antigravity.exe`,
			isHostIDE:    true,
		},
		{
			cmdline:      `"C:\Program Files\Google\Antigravity\Antigravity.exe"`,
			expectedArg0: `C:\Program Files\Google\Antigravity\Antigravity.exe`,
			isHostIDE:    true,
		},
		{
			cmdline:      `"\\server\share\Google\Antigravity\Antigravity.exe" --no-sandbox`,
			expectedArg0: `\\server\share\Google\Antigravity\Antigravity.exe`,
			isHostIDE:    true,
		},

		// Unquoted Windows paths with spaces followed by flags (.exe boundary or dash)
		{
			cmdline:      `C:\Program Files\Google\Antigravity\Antigravity.exe --type=renderer --no-sandbox`,
			expectedArg0: `C:\Program Files\Google\Antigravity\Antigravity.exe`,
			isHostIDE:    true,
		},
		{
			cmdline:      `C:\Program Files\Google\Antigravity\Antigravity.exe /high /prefetch:1`,
			expectedArg0: `C:\Program Files\Google\Antigravity\Antigravity.exe`,
			isHostIDE:    true,
		},
		{
			cmdline:      `C:\Program Files\Google\Antigravity\Antigravity.exe -v`,
			expectedArg0: `C:\Program Files\Google\Antigravity\Antigravity.exe`,
			isHostIDE:    true,
		},
		{
			cmdline:      `C:\Program Files\Google\Antigravity\Antigravity.exe`,
			expectedArg0: `C:\Program Files\Google\Antigravity\Antigravity.exe`,
			isHostIDE:    true,
		},

		// Linux / macOS paths
		{
			cmdline:      `/usr/bin/antigravity --no-sandbox`,
			expectedArg0: `/usr/bin/antigravity`,
			isHostIDE:    true,
		},
		{
			cmdline:      `/opt/Antigravity/antigravity`,
			expectedArg0: `/opt/Antigravity/antigravity`,
			isHostIDE:    true,
		},
		{
			cmdline:      `antigravity --remote-debugging-port=9222`,
			expectedArg0: `antigravity`,
			isHostIDE:    true,
		},

		// Unrelated commands mentioning antigravity in arguments or flags
		{
			cmdline:      `python3 -m antigravity`,
			expectedArg0: `python3`,
			isHostIDE:    false,
		},
		{
			cmdline:      `python -c "import antigravity"`,
			expectedArg0: `python`,
			isHostIDE:    false,
		},
		{
			cmdline:      `node "C:\scripts\antigravity.js"`,
			expectedArg0: `node`,
			isHostIDE:    false,
		},
		{
			cmdline:      `cmd.exe /c Antigravity.exe`,
			expectedArg0: `cmd.exe`,
			isHostIDE:    false,
		},
		{
			cmdline:      `powershell.exe -File antigravity.ps1`,
			expectedArg0: `powershell.exe`,
			isHostIDE:    false,
		},
		{
			cmdline:      `grep -rn "antigravity" /var/log`,
			expectedArg0: `grep`,
			isHostIDE:    false,
		},

		// Swiss companion command lines
		{
			cmdline:      `"C:\Program Files\Antigravity Swiss Knife\swiss.exe" daemon --web`,
			expectedArg0: `C:\Program Files\Antigravity Swiss Knife\swiss.exe`,
			isHostIDE:    false,
		},
		{
			cmdline:      `antigravity-swiss.exe daemon`,
			expectedArg0: `antigravity-swiss.exe`,
			isHostIDE:    false,
		},
		{
			cmdline:      `bin/swiss daemon --web`,
			expectedArg0: `bin/swiss`,
			isHostIDE:    false,
		},

		// Deceptive / spoofed executables
		{
			cmdline:      `"C:\Program Files\Fake\fake_antigravity.exe" --arg`,
			expectedArg0: `C:\Program Files\Fake\fake_antigravity.exe`,
			isHostIDE:    false,
		},
		{
			cmdline:      `fake_antigravity.exe --type=renderer`,
			expectedArg0: `fake_antigravity.exe`,
			isHostIDE:    false,
		},
		{
			cmdline:      `antigravity_helper.exe --parent=1234`,
			expectedArg0: `antigravity_helper.exe`,
			isHostIDE:    false,
		},
	}

	for _, tc := range cases {
		argv0 := extractArgv0(tc.cmdline)
		if argv0 != tc.expectedArg0 {
			t.Errorf("extractArgv0(%q) = %q, expected %q", tc.cmdline, argv0, tc.expectedArg0)
		}
		isIDE := IsExactAntigravityExecutable(argv0)
		if isIDE != tc.isHostIDE {
			t.Errorf("IsExactAntigravityExecutable(extractArgv0(%q)) = %v, expected %v", tc.cmdline, isIDE, tc.isHostIDE)
		}
	}
}

// 10. Process Shield End-to-End Evaluation & Liveness Integration
func TestAdversarialM11_ShieldProcessLifecycle_Stress(t *testing.T) {
	shield := NewShield(0)

	// Scenario A: Main IDE running with unquoted Windows path containing spaces
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{
			{
				PID:     4001,
				Name:    "Antigravity.exe",
				Cmdline: `C:\Program Files\Google\Antigravity\Antigravity.exe`,
			},
		}, nil
	})
	if !shield.IsAntigravityRunning() {
		t.Fatalf("Adversarial Failure: IsAntigravityRunning() should be true for unquoted Windows path with spaces")
	}

	// Scenario B: Main IDE running with UNC path and uppercase name
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{
			{
				PID:     4002,
				Name:    "ANTIGRAVITY.EXE",
				Cmdline: `\\network-share\deploy\ANTIGRAVITY.EXE`,
			},
		}, nil
	})
	if !shield.IsAntigravityRunning() {
		t.Fatalf("Adversarial Failure: IsAntigravityRunning() should be true for UNC path with uppercase name")
	}

	// Scenario C: Only Electron sub-processes running (--type=renderer, --type=gpu-process, etc.)
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{
			{
				PID:     5001,
				Name:    "Antigravity.exe",
				Cmdline: `"C:\Program Files\Google\Antigravity\Antigravity.exe" --type=renderer --no-sandbox`,
			},
			{
				PID:     5002,
				Name:    "Antigravity.exe",
				Cmdline: `"C:\Program Files\Google\Antigravity\Antigravity.exe" --type=gpu-process`,
			},
			{
				PID:     5003,
				Name:    "Antigravity.exe",
				Cmdline: `"C:\Program Files\Google\Antigravity\Antigravity.exe" --type=utility`,
			},
		}, nil
	})
	if shield.IsAntigravityRunning() {
		t.Fatalf("Adversarial Failure: IsAntigravityRunning() must be FALSE when only Electron sub-processes exist")
	}

	// Scenario D: Spoofed processes and Swiss companions co-existing
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{
			{
				PID:     6001,
				Name:    "fake_antigravity.exe",
				Cmdline: `C:\fake\fake_antigravity.exe`,
			},
			{
				PID:     6002,
				Name:    "antigravity-swiss.exe",
				Cmdline: `C:\swiss\antigravity-swiss.exe daemon`,
			},
			{
				PID:     6003,
				Name:    "python3",
				Cmdline: `python3 -m antigravity`,
			},
			{
				PID:     6004,
				Name:    "Antigravity.exe",
				Cmdline: `"C:\Program Files\Google\Antigravity\Antigravity.exe" --type=crashpad-handler`,
			},
		}, nil
	})
	if shield.IsAntigravityRunning() {
		t.Fatalf("Adversarial Failure: IsAntigravityRunning() must be FALSE when no authentic main IDE process is active")
	}

	// Scenario E: Main process along with Swiss and renderers -> must find main IDE
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{
			{
				PID:     7001,
				Name:    "antigravity-swiss.exe",
				Cmdline: `C:\swiss\antigravity-swiss.exe daemon`,
			},
			{
				PID:     7002,
				Name:    "Antigravity.exe",
				Cmdline: `"C:\Program Files\Google\Antigravity\Antigravity.exe" --type=renderer`,
			},
			{
				PID:     7003,
				Name:    "Antigravity.exe",
				Cmdline: `"C:\Program Files\Google\Antigravity\Antigravity.exe"`,
			},
		}, nil
	})
	if !shield.IsAntigravityRunning() {
		t.Fatalf("Adversarial Failure: IsAntigravityRunning() should be TRUE when authentic main IDE process is present")
	}
}
