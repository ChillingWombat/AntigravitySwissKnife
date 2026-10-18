package webgui

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// TestChallenger_M11_It3_SpecificRequirements explicitly asserts all items
// listed in the Challenger 2 dispatch mission for Milestone M11 Iteration 3.
func TestChallenger_M11_It3_SpecificRequirements(t *testing.T) {
	// Item 1: Root containment: C:\Users\runner allows C:\Users\runner\foo but blocks C:\Users\runneradmin and C:\Windows
	t.Run("Root containment runner vs runneradmin and Windows", func(t *testing.T) {
		root := `C:\Users\runner`

		// Allowed subpaths
		allowed := []string{
			`C:\Users\runner\foo`,
			`C:\Users\runner\foo\bar.txt`,
			`C:\Users\runner\Documents`,
			`C:\Users\runner/foo`,
			`c:\users\runner\foo`,
		}
		for _, p := range allowed {
			if !pathMatchesOrIsSubpath(p, root) {
				t.Errorf("[VIOLATION] pathMatchesOrIsSubpath(%q, %q) = false; expected true", p, root)
			}
		}

		// Blocked paths: runneradmin, runner_fake, Windows, etc.
		blocked := []string{
			`C:\Users\runneradmin`,
			`C:\Users\runneradmin\foo`,
			`C:\Users\runneradmin\sub\secret.txt`,
			`C:\Users\runner_fake`,
			`C:\Users\runner2`,
			`C:\Users\runner.bak`,
			`C:\Windows`,
			`C:\Windows\System32`,
			`C:\Windows\System32\cmd.exe`,
			`C:\Program Files`,
			`D:\Users\runner\foo`,
		}
		for _, p := range blocked {
			if pathMatchesOrIsSubpath(p, root) {
				t.Errorf("[CONTAINMENT BREACH] pathMatchesOrIsSubpath(%q, %q) = true; expected false", p, root)
			}
		}
	})

	// Item 2: Trailing slash roots: C:\Users\runner\ allows C:\Users\runner and subpaths
	t.Run("Trailing slash roots", func(t *testing.T) {
		root := `C:\Users\runner\`

		// Must allow exact match and subpaths
		allowed := []string{
			`C:\Users\runner`,
			`C:\Users\runner\`,
			`C:\Users\runner\foo`,
			`C:\Users\runner\foo\bar`,
			`C:\Users\runner/foo`,
			`c:\users\runner`,
			`c:\users\runner\subpath`,
		}
		for _, p := range allowed {
			if !pathMatchesOrIsSubpath(p, root) {
				t.Errorf("[VIOLATION] pathMatchesOrIsSubpath(%q, %q) = false; expected true", p, root)
			}
		}

		// Must still block sibling prefix attacks
		blocked := []string{
			`C:\Users\runneradmin`,
			`C:\Users\runneradmin\`,
			`C:\Users\runneradmin\foo`,
			`C:\Users\runner_fake`,
			`C:\Windows`,
		}
		for _, p := range blocked {
			if pathMatchesOrIsSubpath(p, root) {
				t.Errorf("[CONTAINMENT BREACH] pathMatchesOrIsSubpath(%q, %q) = true; expected false", p, root)
			}
		}
	})

	// Item 3: 8.3 aliasing: C:\Users\RUNNER~1 correctly resolves and matches when runneradmin exists
	t.Run("8.3 aliasing RUNNER~1 and runneradmin", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("8.3 short-name aliases are an NTFS/Windows feature")
		}
		target := `C:\Users\runneradmin`
		probe := `C:\Users\RUNNER~1\AppData\Local\Temp`

		aliases := find8dot3Aliases(target, probe)
		found := false
		for _, a := range aliases {
			if strings.EqualFold(a, `C:\Users\RUNNER~1`) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("[8.3 RESOLUTION FAILURE] find8dot3Aliases(%q, %q) failed to produce C:\\Users\\RUNNER~1, got %v",
				target, probe, aliases)
		}

		// Also verify alias generation without probe (synthetic generator fallback).
		// On Windows the synthetic candidate is only reported when it actually
		// resolves on disk (8.3 names must exist to be real aliases), so the
		// presence assertion only applies when the target itself exists.
		aliasesNoProbe := find8dot3Aliases(target)
		if _, statErr := os.Stat(target); statErr == nil {
			foundNoProbe := false
			for _, a := range aliasesNoProbe {
				if strings.EqualFold(a, `C:\Users\RUNNER~1`) {
					foundNoProbe = true
					break
				}
			}
			if !foundNoProbe {
				t.Errorf("[8.3 SYNTHETIC FAILURE] find8dot3Aliases(%q) failed to produce C:\\Users\\RUNNER~1, got %v",
					target, aliasesNoProbe)
			}
		} else if len(aliasesNoProbe) > 0 {
			t.Errorf("[8.3 FABRICATION] find8dot3Aliases(%q) fabricated aliases for a non-existent target, got %v",
				target, aliasesNoProbe)
		}

		// Matching under 8.3 short name root
		aliasRoot := `C:\Users\RUNNER~1`
		if !pathMatchesOrIsSubpath(`C:\Users\RUNNER~1\foo`, aliasRoot) {
			t.Errorf("[8.3 MATCH FAILURE] pathMatchesOrIsSubpath for alias subpath failed")
		}
		if !pathMatchesOrIsSubpath(`c:\users\runner~1\foo\bar.txt`, aliasRoot) {
			t.Errorf("[8.3 MATCH FAILURE] pathMatchesOrIsSubpath case-insensitive failed")
		}

		// Sibling 8.3 attacks must be blocked
		if pathMatchesOrIsSubpath(`C:\Users\RUNNER~12`, aliasRoot) {
			t.Errorf("[CONTAINMENT BREACH] RUNNER~12 matched RUNNER~1 root")
		}
		if pathMatchesOrIsSubpath(`C:\Users\RUNNER~2`, aliasRoot) {
			t.Errorf("[CONTAINMENT BREACH] RUNNER~2 matched RUNNER~1 root")
		}
		if pathMatchesOrIsSubpath(`C:\Users\RUNNER~1_fake`, aliasRoot) {
			t.Errorf("[CONTAINMENT BREACH] RUNNER~1_fake matched RUNNER~1 root")
		}
	})

	// Item 4: UNC path handling: \\server\share\path does not get collapsed into Unix root /server/share/path
	t.Run("UNC path handling and non-collapse into Unix root", func(t *testing.T) {
		uncRoot := `\\server\share\path`
		uncSub := `\\server\share\path\file.txt`
		unixPath := `/server/share/path`
		unixSub := `/server/share/path/file.txt`

		// UNC subpath matches UNC root
		if !pathMatchesOrIsSubpath(uncSub, uncRoot) {
			t.Errorf("[UNC MATCH FAILURE] pathMatchesOrIsSubpath(%q, %q) = false; expected true", uncSub, uncRoot)
		}

		// UNC exact match
		if !pathMatchesOrIsSubpath(uncRoot, uncRoot) {
			t.Errorf("[UNC MATCH FAILURE] pathMatchesOrIsSubpath exact match failed")
		}

		// Trailing slash in UNC root
		if !pathMatchesOrIsSubpath(uncSub, uncRoot+`\`) {
			t.Errorf("[UNC MATCH FAILURE] pathMatchesOrIsSubpath with trailing backslash failed")
		}
		if !pathMatchesOrIsSubpath(uncRoot, uncRoot+`\`) {
			t.Errorf("[UNC MATCH FAILURE] pathMatchesOrIsSubpath root vs root with trailing slash failed")
		}

		// UNC path MUST NOT match Unix root or vice-versa
		if pathMatchesOrIsSubpath(uncSub, unixPath) {
			t.Errorf("[UNC SECURITY BREACH] UNC subpath %q matched Unix root %q", uncSub, unixPath)
		}
		if pathMatchesOrIsSubpath(unixSub, uncRoot) {
			t.Errorf("[UNC SECURITY BREACH] Unix subpath %q matched UNC root %q", unixSub, uncRoot)
		}
		if pathMatchesOrIsSubpath(uncRoot, unixPath) {
			t.Errorf("[UNC SECURITY BREACH] UNC path %q collapsed/matched Unix path %q", uncRoot, unixPath)
		}

		// Cross-share UNC rejection
		if pathMatchesOrIsSubpath(`\\server\share2\path`, uncRoot) {
			t.Errorf("[UNC SHARE CONFUSION] \\\\server\\share2\\path matched \\\\server\\share\\path")
		}
		if pathMatchesOrIsSubpath(`\\otherserver\share\path`, uncRoot) {
			t.Errorf("[UNC SERVER CONFUSION] \\\\otherserver\\share\\path matched \\\\server\\share\\path")
		}

		// Forward slash UNC equivalent: //server/share/path vs \\server\share\path
		if !pathMatchesOrIsSubpath(`//server/share/path/file.txt`, uncRoot) {
			t.Errorf("[UNC FORWARD SLASH FAILURE] forward slash UNC failed to match backslash UNC root")
		}
		if !pathMatchesOrIsSubpath(uncSub, `//server/share/path`) {
			t.Errorf("[UNC FORWARD SLASH FAILURE] backslash UNC failed to match forward slash UNC root")
		}
	})
}
