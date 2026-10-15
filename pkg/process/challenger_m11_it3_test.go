package process

import (
	"strings"
	"testing"
)

// TestChallenger_Iteration3_AllInvariants executes an extensive empirical stress test
// on IsExactAntigravityExecutable and extractArgv0 across all boundary, platform, and adversarial conditions.
func TestChallenger_Iteration3_AllInvariants(t *testing.T) {
	// =========================================================================
	// Tier 1: Valid Antigravity Paths (must return true for IsExactAntigravityExecutable)
	// =========================================================================
	validPaths := []struct {
		desc string
		path string
	}{
		// Windows Standard Paths
		{"win_standard", `C:\Program Files\Google\Antigravity\Antigravity.exe`},
		{"win_x86", `C:\Program Files (x86)\Google\Antigravity\Antigravity.exe`},
		{"win_localappdata", `C:\Users\David\AppData\Local\Programs\Antigravity\Antigravity.exe`},
		{"win_drive_d", `D:\Antigravity\Antigravity.exe`},
		{"win_lower_drive", `c:\program files\google\antigravity\antigravity.exe`},
		{"win_uppercase", `C:\PROGRAM FILES\GOOGLE\ANTIGRAVITY\ANTIGRAVITY.EXE`},

		// Windows Separator Variations
		{"win_mixed_separators_1", `C:/tools\Antigravity.exe`},
		{"win_mixed_separators_2", `C:\tools/Antigravity.exe`},
		{"win_mixed_separators_spaces", `C:/Program Files/Google/Antigravity\Antigravity.exe`},
		{"win_forward_slashes", `C:/Program Files/Google/Antigravity/Antigravity.exe`},
		{"win_double_forward_root", `C://Program Files//Google//Antigravity//Antigravity.exe`},

		// Windows UNC & Device Paths
		{"win_unc_standard", `\\server\share\Antigravity.exe`},
		{"win_unc_forward", `//server/share/Antigravity.exe`},
		{"win_unc_with_spaces", `\\network-storage\my dev tools\Antigravity.exe`},
		{"win_unc_ip", `\\192.168.1.100\share\Antigravity.exe`},
		{"win_unc_extended", `\\?\UNC\server\share\Antigravity.exe`},
		{"win_unc_extended_forward", `//?/UNC/server/share/Antigravity.exe`},
		{"win_device_unc", `\\.\UNC\server\share\Antigravity.exe`},
		{"win_device_unc_forward", `//./UNC/server/share/Antigravity.exe`},
		{"win_extended_drive", `\\?\C:\Program Files\Google\Antigravity\Antigravity.exe`},
		{"win_device_drive", `\\.\C:\tools\Antigravity.exe`},
		{"win_nt_drive", `\??\C:\tools\Antigravity.exe`},

		// Linux & macOS Paths
		{"linux_usr_bin", "/usr/bin/antigravity"},
		{"linux_usr_local_bin", "/usr/local/bin/antigravity"},
		{"linux_opt", "/opt/antigravity/antigravity"},
		{"linux_opt_with_spaces", "/opt/Google Antigravity/antigravity"},
		{"linux_home_local", "/home/david/.local/share/antigravity/antigravity"},
		{"macos_bundle", "/Applications/Google Antigravity.app/Contents/MacOS/antigravity"},
		{"macos_bundle_with_spaces", "/Applications/My Apps/Google Antigravity.app/Contents/MacOS/antigravity"},

		// Bare binary names
		{"bare_antigravity_exe", "Antigravity.exe"},
		{"bare_antigravity_lower", "antigravity.exe"},
		{"bare_antigravity_upper", "ANTIGRAVITY.EXE"},
		{"bare_linux_antigravity", "antigravity"},
		{"bare_linux_antigravity_upper", "ANTIGRAVITY"},

		// Quoted Paths
		{"quoted_win_double", `"C:\Program Files\Google\Antigravity\Antigravity.exe"`},
		{"quoted_win_single", `'C:\Program Files\Google\Antigravity\Antigravity.exe'`},
		{"quoted_linux_double", `"/opt/Google Antigravity/antigravity"`},
		{"quoted_linux_single", `'/opt/Google Antigravity/antigravity'`},
	}

	for _, tc := range validPaths {
		t.Run("Valid_"+tc.desc, func(t *testing.T) {
			if !IsExactAntigravityExecutable(tc.path) {
				t.Fatalf("[ASSERTION FAILURE] Valid Antigravity path was rejected: %q", tc.path)
			}
			argv0 := extractArgv0(tc.path)
			if !IsExactAntigravityExecutable(argv0) {
				t.Fatalf("[ASSERTION FAILURE] extractArgv0(%q) produced %q which was rejected by IsExactAntigravityExecutable", tc.path, argv0)
			}
		})
	}

	// =========================================================================
	// Tier 2: Invalid / Hostile Commands (must return false for IsExactAntigravityExecutable)
	// =========================================================================
	invalidCommands := []struct {
		desc    string
		cmdline string
	}{
		// Dispatch prompt cases
		{"dispatch_python_script", "python scripts/antigravity.py"},
		{"dispatch_git_log_grep", "git log --grep=antigravity"},
		{"dispatch_tab_cmd_arg", "cmd\targ"},
		{"dispatch_tab_with_antigravity", "cat\t/usr/bin/antigravity"},
		{"dispatch_tab_windows", `C:\Windows\System32\cmd.exe` + "\t" + `C:\tools\Antigravity.exe`},

		// Arguments on valid Antigravity paths
		{"args_win_dash_type", `C:\Program Files\Google\Antigravity\Antigravity.exe --type=renderer`},
		{"args_win_slash_high", `C:\Program Files\Google\Antigravity\Antigravity.exe /high`},
		{"args_win_file_target", `C:\Program Files\Google\Antigravity\Antigravity.exe C:\file.txt`},
		{"args_linux_dash_no_sandbox", "/usr/bin/antigravity --no-sandbox"},
		{"args_linux_spaces_dash_flag", "/opt/Google Antigravity/antigravity --flag"},
		{"args_unc_dash_arg", `\\server\share\Antigravity.exe --arg`},
		{"args_unc_extended_dash_arg", `\\?\UNC\server\share\Antigravity.exe --arg`},
		{"args_bare_antigravity_dash", "antigravity --version"},
		{"args_bare_antigravity_exe_slash", "Antigravity.exe /?"},

		// Relative commands & script interpreters
		{"rel_python_module", "python -m antigravity"},
		{"rel_python_script_arg", "python script.py mypkg/antigravity"},
		{"rel_node_script", "node antigravity.js"},
		{"rel_node_dist_arg", "node app.js dist/antigravity"},
		{"rel_bash_script", "bash scripts/antigravity"},
		{"rel_sh_script", "sh ./antigravity"},
		{"rel_cat_dir", "cat dir/antigravity"},
		{"rel_vim_src", "vim src/antigravity"},
		{"rel_notepad_tools", `notepad.exe tools\Antigravity.exe`},
		{"rel_docker_run", "docker run local/antigravity"},
		{"rel_docker_pull", "docker pull registry.internal/antigravity"},
		{"rel_kubectl_logs", "kubectl logs pod/antigravity"},
		{"rel_npm_install", "npm install @scope/antigravity"},

		// Tab-separated commands
		{"tab_cat_unix", "cat\t/usr/bin/antigravity"},
		{"tab_notepad_unc", "notepad.exe\t\\\\server\\share\\Antigravity.exe"},
		{"tab_relative_path", "cat\t./antigravity"},
		{"tab_absolute_unix", "/usr/bin/cat\t/usr/bin/antigravity"},
		{"tab_absolute_windows", `C:\Windows\System32\notepad.exe` + "\t" + `C:\tools\Antigravity.exe`},

		// URLs & schemes
		{"url_http", "http://example.com/antigravity"},
		{"url_https", "https://example.com/Antigravity.exe"},
		{"url_curl_domain", "curl example.com/antigravity"},
		{"url_wget_domain", "wget dl.google.com/antigravity"},
		{"url_git_clone_no_scheme", "git clone github.com/foo/antigravity"},
		{"url_custom_scheme", "antigravity://open/workspace"},

		// Git operations
		{"git_checkout_branch", "git checkout feature/antigravity"},
		{"git_merge_branch", "git merge feature/antigravity"},

		// Directory paths (trailing separators)
		{"dir_unix_trailing_slash", "/opt/antigravity/"},
		{"dir_win_trailing_backslash", `C:\Program Files\Google\Antigravity\`},
		{"dir_win_trailing_exe_backslash", `C:\Program Files\Google\Antigravity\Antigravity.exe\`},
		{"dir_linux_trailing_exe_slash", "/usr/bin/antigravity/"},

		// Mid-string colons
		{"mid_string_drive", `notepad.exe C:\tools\Antigravity.exe`},
		{"mid_string_arbitrary_colon", `app:antigravity.exe`},

		// Companion exclusions
		{"companion_bare_swiss", "antigravity-swiss"},
		{"companion_bare_swiss_exe", "antigravity-swiss.exe"},
		{"companion_unix_path", "/usr/bin/antigravity-swiss"},
		{"companion_win_path", `C:\Program Files\Google\Antigravity\antigravity-swiss.exe`},
		{"companion_swiss_knife", "antigravity-swiss-knife"},

		// Other non-antigravity extensions and names
		{"ext_shell_script", "/usr/bin/antigravity.sh"},
		{"ext_python_script", "/opt/antigravity.py"},
		{"ext_javascript", "/var/www/antigravity.js"},
		{"ext_backup", "/usr/bin/antigravity.bak"},
		{"similar_notantigravity", "/usr/bin/notantigravity"},
		{"similar_antigravity_helper", "/usr/bin/antigravity_helper"},

		// Degenerate inputs
		{"empty_string", ""},
		{"spaces_only", "   "},
		{"tabs_only", "\t\t"},
		{"newlines_only", "\n\r\n"},
	}

	for _, tc := range invalidCommands {
		t.Run("Invalid_"+tc.desc, func(t *testing.T) {
			if IsExactAntigravityExecutable(tc.cmdline) {
				t.Fatalf("[VULNERABILITY] Command accepted as exact Antigravity executable: %q", tc.cmdline)
			}
		})
	}

	// =========================================================================
	// Tier 3: extractArgv0 Behavior on Valid Invocations with Arguments
	// =========================================================================
	commandsWithArgs := []struct {
		desc         string
		fullCmd      string
		expectedArg0 string
	}{
		{
			desc:         "unquoted_win_path_spaces_dash_arg",
			fullCmd:      `C:\Program Files\Google\Antigravity\Antigravity.exe --type=renderer --no-sandbox`,
			expectedArg0: `C:\Program Files\Google\Antigravity\Antigravity.exe`,
		},
		{
			desc:         "unquoted_win_path_spaces_slash_arg",
			fullCmd:      `C:\Program Files\Google\Antigravity\Antigravity.exe /high`,
			expectedArg0: `C:\Program Files\Google\Antigravity\Antigravity.exe`,
		},
		{
			desc:         "quoted_win_path_spaces_dash_arg",
			fullCmd:      `"C:\Program Files\Google\Antigravity\Antigravity.exe" --inspect=9229`,
			expectedArg0: `C:\Program Files\Google\Antigravity\Antigravity.exe`,
		},
		{
			desc:         "single_quoted_linux_path_spaces_arg",
			fullCmd:      `'/opt/Google Antigravity/antigravity' --flag=value`,
			expectedArg0: `/opt/Google Antigravity/antigravity`,
		},
		{
			desc:         "unquoted_linux_path_no_spaces_arg",
			fullCmd:      `/usr/bin/antigravity --no-sandbox`,
			expectedArg0: `/usr/bin/antigravity`,
		},
		{
			desc:         "unquoted_unc_path_arg",
			fullCmd:      `\\server\share\Antigravity.exe --arg`,
			expectedArg0: `\\server\share\Antigravity.exe`,
		},
		{
			desc:         "bare_antigravity_exe_arg",
			fullCmd:      `Antigravity.exe --type=gpu`,
			expectedArg0: `Antigravity.exe`,
		},
	}

	for _, tc := range commandsWithArgs {
		t.Run("Argv0_"+tc.desc, func(t *testing.T) {
			// 1. The full command line MUST NOT be accepted as exact executable
			if IsExactAntigravityExecutable(tc.fullCmd) {
				t.Fatalf("[VULNERABILITY] Full command with args accepted as exact executable: %q", tc.fullCmd)
			}

			// 2. extractArgv0 MUST isolate the executable binary
			extracted := extractArgv0(tc.fullCmd)
			if !strings.EqualFold(extracted, tc.expectedArg0) {
				t.Fatalf("[FAILURE] extractArgv0(%q) = %q, expected %q", tc.fullCmd, extracted, tc.expectedArg0)
			}

			// 3. The extracted argv0 MUST be recognized as authentic Antigravity executable
			if !IsExactAntigravityExecutable(extracted) {
				t.Fatalf("[FAILURE] Extracted argv0 %q was rejected by IsExactAntigravityExecutable", extracted)
			}
		})
	}
}
