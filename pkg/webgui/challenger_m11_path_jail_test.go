package webgui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestChallenger_PathMatchesOrIsSubpath_Boundaries tests boundary conditions for pathMatchesOrIsSubpath:
// - Exact match on root
// - Subpaths with various separators and trailing slashes
// - Sibling prefix confusion (runner_fake vs runner)
// - Root drive boundaries (C:\)
func TestChallenger_PathMatchesOrIsSubpath_Boundaries(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		root      string
		expected  bool
	}{
		// 1. Exact matches and simple descendants
		{"Exact match clean", `C:\Users\runner`, `C:\Users\runner`, true},
		{"Exact match with trailing slash candidate", `C:\Users\runner\`, `C:\Users\runner`, true},
		{"Exact match with trailing slash root", `C:\Users\runner`, `C:\Users\runner\`, true},
		{"Exact match both trailing slashes", `C:\Users\runner\`, `C:\Users\runner\`, true},
		{"Subpath simple", `C:\Users\runner\Documents\file.txt`, `C:\Users\runner`, true},
		{"Subpath single file", `C:\Users\runner\a`, `C:\Users\runner`, true},

		// 2. Mixed forward and backward slashes
		{"Candidate forward slash", `C:/Users/runner/Documents`, `C:\Users\runner`, true},
		{"Root forward slash", `C:\Users\runner/Documents`, `C:/Users/runner`, true},
		{"Both forward slash", `C:/Users/runner/Documents`, `C:/Users/runner`, true},
		{"Candidate trailing forward slash", `C:/Users/runner/`, `C:\Users\runner`, true},
		{"Root trailing forward slash", `C:\Users\runner`, `C:/Users/runner/`, true},

		// 3. Sibling prefix collisions (CRITICAL boundary test)
		{"Sibling prefix collision (runner_fake)", `C:\Users\runner_fake`, `C:\Users\runner`, false},
		{"Sibling prefix collision with slash", `C:\Users\runner_fake\sub`, `C:\Users\runner`, false},
		{"Sibling prefix collision number", `C:\Users\runner2`, `C:\Users\runner`, false},
		{"Sibling prefix collision dot", `C:\Users\runner.bak`, `C:\Users\runner`, false},
		{"Parent is not subpath of child", `C:\Users`, `C:\Users\runner`, false},

		// 4. Drive root boundaries
		{"Drive root exact match", `C:\`, `C:\`, true},
		{"Drive root match forward slash", `C:/`, `C:\`, true},
		{"Drive root no trailing slash root", `C:\Users`, `C:\`, true},
		{"Drive root child", `C:\Users\runner`, `C:\`, true},
		{"Drive root root child forward", `C:/Users/runner`, `C:\`, true},

		// 5. Whitespace around paths
		{"Whitespace around candidate", `  C:\Users\runner\foo  `, `C:\Users\runner`, true},
		{"Whitespace around root", `C:\Users\runner\foo`, `  C:\Users\runner  `, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := pathMatchesOrIsSubpath(tc.candidate, tc.root)
			if actual != tc.expected {
				t.Errorf("[BOUNDARY DISCREPANCY] pathMatchesOrIsSubpath(%q, %q) = %v; expected %v (runtime.GOOS=%s)",
					tc.candidate, tc.root, actual, tc.expected, runtime.GOOS)
			}
		})
	}
}

// TestChallenger_PathMatchesOrIsSubpath_DriveCasing tests case-insensitivity:
// - Lowercase vs uppercase drive letters
// - Mixed case directory names
func TestChallenger_PathMatchesOrIsSubpath_DriveCasing(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		root      string
		expected  bool
	}{
		{"Lowercase candidate drive", `c:\users\runner`, `C:\Users\runner`, true},
		{"Lowercase root drive", `C:\Users\runner`, `c:\users\runner`, true},
		{"Both lowercase", `c:\users\runner`, `c:\users\runner`, true},
		{"Mixed case paths", `c:\UsErS\RuNnEr\SuBdIr`, `C:\uSeRs\rUnNeR`, true},
		{"Drive case with forward slash", `c:/users/runner/file.txt`, `C:\Users\runner`, true},
		{"Different drive letters must not match", `D:\Users\runner`, `C:\Users\runner`, false},
		{"Different drive letters candidate sub", `d:\users\runner\sub`, `c:\users\runner`, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := pathMatchesOrIsSubpath(tc.candidate, tc.root)
			if actual != tc.expected {
				t.Errorf("[DRIVE CASING FAILURE] pathMatchesOrIsSubpath(%q, %q) = %v; expected %v",
					tc.candidate, tc.root, actual, tc.expected)
			}
		})
	}
}

// TestChallenger_PathMatchesOrIsSubpath_ExtendedPrefixes tests Windows extended-length prefixes:
// - \\?\
// - \??\
// - //?/
// - \\?\UNC\
// - //?/UNC/
func TestChallenger_PathMatchesOrIsSubpath_ExtendedPrefixes(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		root      string
		expected  bool
	}{
		{"Candidate with \\\\?\\", `\\?\C:\Users\runner\foo`, `C:\Users\runner`, true},
		{"Root with \\\\?\\", `C:\Users\runner\foo`, `\\?\C:\Users\runner`, true},
		{"Both with \\\\?\\", `\\?\C:\Users\runner\foo`, `\\?\C:\Users\runner`, true},
		{"Candidate with \\??\\", `\??\C:\Users\runner\foo`, `C:\Users\runner`, true},
		{"Root with \\??\\", `C:\Users\runner\foo`, `\??\C:\Users\runner`, true},
		{"Candidate with //?/", `//?/C:/Users/runner/foo`, `C:\Users\runner`, true},
		{"UNC path candidate with \\\\?\\UNC\\", `\\?\UNC\server\share\folder\file`, `\\server\share\folder`, true},
		{"UNC path root with \\\\?\\UNC\\", `\\server\share\folder\file`, `\\?\UNC\server\share\folder`, true},
		{"UNC path candidate with //?/UNC/", `//?/UNC/server/share/folder/file`, `\\server\share\folder`, true},
		{"Extended prefix outside root", `\\?\D:\secret\file.txt`, `C:\Users\runner`, false},
		{"Extended prefix sibling attack", `\\?\C:\Users\runner_fake\evil.txt`, `C:\Users\runner`, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := pathMatchesOrIsSubpath(tc.candidate, tc.root)
			if actual != tc.expected {
				t.Errorf("[EXTENDED PREFIX FAILURE] pathMatchesOrIsSubpath(%q, %q) = %v; expected %v (runtime.GOOS=%s)",
					tc.candidate, tc.root, actual, tc.expected, runtime.GOOS)
			}
		})
	}
}

// TestChallenger_PathMatchesOrIsSubpath_CrossVolumeAndTraversal tests traversal and cross-volume rejection.
func TestChallenger_PathMatchesOrIsSubpath_CrossVolumeAndTraversal(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		root      string
		expected  bool
	}{
		{"Cross-volume candidate D vs root C", `D:\secret\file.txt`, `C:\Users\runner`, false},
		{"Cross-volume candidate C vs root D", `C:\secret\file.txt`, `D:\Users\runner`, false},
		{"Relative traversal escaping root", `C:\Users\runner\..\..\Windows\System32`, `C:\Users\runner`, false},
		{"Deep traversal to root", `C:\Users\runner\..\..\..\..\..\..\..\..\Windows`, `C:\Users\runner`, false},
		{"UNC cross-share candidate", `\\evil\share\file.txt`, `\\good\share`, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := pathMatchesOrIsSubpath(tc.candidate, tc.root)
			if actual != tc.expected {
				t.Errorf("[TRAVERSAL/CROSS-VOLUME FAILURE] pathMatchesOrIsSubpath(%q, %q) = %v; expected %v (runtime.GOOS=%s)",
					tc.candidate, tc.root, actual, tc.expected, runtime.GOOS)
			}
		})
	}
}

// TestChallenger_Windows8dot3Aliasing tests 8.3 short name alias generation and containment:
// - find8dot3Aliases generates short forms from long names
// - find8dot3Aliases discovers short forms from probes
// - sensitive dotfiles under 8.3 alias are blocked
func TestChallenger_Windows8dot3Aliasing(t *testing.T) {
	// Test alias generation for runneradmin -> RUNNER~1
	target := `C:\Users\runneradmin`
	probe := `C:\Users\RUNNER~1\AppData\Local\Temp`
	aliases := find8dot3Aliases(target, probe)

	foundRunner1 := false
	for _, a := range aliases {
		if strings.EqualFold(a, `C:\Users\RUNNER~1`) {
			foundRunner1 = true
			break
		}
	}
	if !foundRunner1 {
		t.Errorf("[8.3 ALIAS FAILURE] find8dot3Aliases(%q, %q) did not generate C:\\Users\\RUNNER~1 alias, got: %v (runtime.GOOS=%s)",
			target, probe, aliases, runtime.GOOS)
	}

	// Test alias containment: RUNNER~1 temp file considered subpath of alias root
	shortRoot := `C:\Users\RUNNER~1`
	shortFile := `C:\Users\RUNNER~1\AppData\Local\Temp\build.log`
	if !pathMatchesOrIsSubpath(shortFile, shortRoot) {
		t.Errorf("[8.3 CONTAINMENT FAILURE] pathMatchesOrIsSubpath(%q, %q) = false; expected true", shortFile, shortRoot)
	}

	// Long form subpath of long form root
	longRoot := `C:\Users\runneradmin`
	longFile := `C:\Users\runneradmin\AppData\Local\Temp\build.log`
	if !pathMatchesOrIsSubpath(longFile, longRoot) {
		t.Errorf("[LONG FORM FAILURE] pathMatchesOrIsSubpath(%q, %q) = false; expected true", longFile, longRoot)
	}
}

// TestChallenger_ValidateDeletablePath_Adversarial tests validateDeletablePath against hostile deletion attempts:
// - Volume roots (C:\, D:\, c:\)
// - Root shortcuts (~, ., /, \)
// - Traversal patterns (including quad dot ....)
// - System directories
func TestChallenger_ValidateDeletablePath_Adversarial(t *testing.T) {
	hostileDeletions := []struct {
		name string
		path string
	}{
		{"Unix root", "/"},
		{"Windows drive root C", `C:\`},
		{"Windows drive root D", `D:\`},
		{"Windows drive root lowercase", `c:\`},
		{"Windows drive without trailing slash", `C:`},
		{"Windows drive with forward slash", `C:/`},
		{"Extended syntax drive root", `\\?\C:\`},
		{"Dot directory", "."},
		{"Tilde root", "~"},
		{"Tilde slash", "~/"},
		{"Tilde backslash", `~\`},
		{"Backslash root", `\`},
		{"Parent traversal simple", `../something`},
		{"Parent traversal backslash", `..\something`},
		{"Quadruple dot traversal", `C:\Users\runner\....\foo`},
		{"Encoded traversal in delete", `%2e%2e/something`},
		{"Windows System directory", `C:\Windows`},
		{"Windows System32", `C:\Windows\System32`},
		{"Windows Program Files", `C:\Program Files`},
		{"Unix /etc directory", `/etc`},
		{"Unix /usr directory", `/usr`},
		{"Unix /root directory", `/root`},
	}

	for _, tc := range hostileDeletions {
		t.Run(tc.name, func(t *testing.T) {
			res, err := validateDeletablePath(tc.path)
			if err == nil {
				t.Errorf("[CRITICAL DELETION VULNERABILITY] validateDeletablePath allowed deletion of hostile target %q -> resolved to %q",
					tc.path, res)
			}
		})
	}
}

// TestChallenger_ValidateJailedPath_WindowsVectors tests validateJailedPath against Windows-specific vectors:
// - Windows extended length paths
// - Windows system directory traversal
// - Encoded traversal with backslashes
func TestChallenger_ValidateJailedPath_WindowsVectors(t *testing.T) {
	vectors := []struct {
		name string
		path string
	}{
		{"Extended path system directory", `\\?\C:\Windows\System32\cmd.exe`},
		{"Extended path traversal", `\\?\C:\Users\runner\..\..\Windows\System32`},
		{"Backslash traversal", `..\..\Windows\System32\cmd.exe`},
		{"Encoded backslash traversal", `..%5c..%5cWindows`},
		{"Double encoded backslash traversal", `..%255c..%255cWindows`},
		{"Null byte in Windows path", `C:\Users\runner\safe.txt` + "\x00" + `evil.bat`},
		{"URL encoded null byte in Windows path", `C:\Users\runner\safe.txt%00evil.bat`},
	}

	for _, tc := range vectors {
		t.Run(tc.name, func(t *testing.T) {
			res, err := validateJailedPath(tc.path)
			if err == nil {
				t.Errorf("[VULNERABILITY] validateJailedPath accepted hostile Windows vector %q -> %q", tc.path, res)
			}
		})
	}
}

// TestChallenger_SymlinkAndJailEscapes tests symlink escape boundaries
func TestChallenger_SymlinkAndJailEscapes(t *testing.T) {
	tempDir := t.TempDir()

	// Legitimate nested file must be allowed
	legit := filepath.Join(tempDir, "safe_dir", "nested.txt")
	resolved, err := validateJailedPath(legit)
	if err != nil {
		t.Errorf("[UNEXPECTED REJECTION] validateJailedPath rejected legitimate nested temp path: %v", err)
	}
	if resolved == "" {
		t.Errorf("expected non-empty resolved path")
	}

	// Symlink pointing outside tempDir must be rejected
	if runtime.GOOS != "windows" {
		evilLink := filepath.Join(tempDir, "evil_link")
		_ = os.Symlink("/etc", evilLink)
		_, err := validateJailedPath(evilLink)
		if err == nil {
			t.Errorf("[VULNERABILITY] symlink to /etc accepted by validateJailedPath")
		}
	}
}
