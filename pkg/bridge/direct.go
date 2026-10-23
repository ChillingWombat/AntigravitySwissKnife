package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// DirectHandler implements local machine operations when Gemini acts as a direct actuator.
type DirectHandler struct{}

// NewDirectHandler creates a new direct machine handler.
func NewDirectHandler() *DirectHandler {
	return &DirectHandler{}
}

// RunCommand executes a shell command on the host with an execution timeout.
func (dh *DirectHandler) RunCommand(args map[string]interface{}) (string, error) {
	cmdStr, _ := args["command"].(string)
	if strings.TrimSpace(cmdStr) == "" {
		return "", fmt.Errorf("command is required")
	}

	cwd, _ := args["cwd"].(string)
	if cwd == "" {
		cwd = "."
	}

	timeoutSec := 30
	if ts, ok := args["timeout_seconds"].(float64); ok && int(ts) > 0 {
		timeoutSec = int(ts)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// Execute argv directly. A shell would hand caller-supplied text to bash or
	// cmd.exe, which turns any argument into a second command. Pipelines and
	// redirection therefore belong in the caller's own tooling, not here.
	tokens, splitErr := splitCommand(cmdStr)
	if splitErr != nil {
		return "", splitErr
	}
	if len(tokens) == 0 {
		return "", fmt.Errorf("command is required")
	}
	shellCmd := exec.CommandContext(ctx, tokens[0], tokens[1:]...)
	core.SetHiddenConsoleInherit(shellCmd)
	shellCmd.Dir = cwd

	out, err := shellCmd.CombinedOutput()
	result := string(out)
	if ctx.Err() == context.DeadlineExceeded {
		return result, fmt.Errorf("command timed out after %d seconds", timeoutSec)
	}
	if err != nil {
		return result, fmt.Errorf("command failed: %w\nOutput:\n%s", err, result)
	}

	if len(result) > 20000 {
		result = result[:20000] + "\n... [truncated]"
	}
	return result, nil
}

// ReadFile reads the content of a local file.
func (dh *DirectHandler) ReadFile(args map[string]interface{}) (string, error) {
	filePath, _ := args["path"].(string)
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("path is required")
	}

	maxBytes := int64(100 * 1024) // 100 KB default limit
	if mb, ok := args["max_bytes"].(float64); ok && int64(mb) > 0 {
		maxBytes = int64(mb)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, maxBytes)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return "", fmt.Errorf("read file: %w", err)
	}

	return string(buf[:n]), nil
}

// WriteFile writes content to a local file.
func (dh *DirectHandler) WriteFile(args map[string]interface{}) (string, error) {
	filePath, _ := args["path"].(string)
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("path is required")
	}
	content, _ := args["content"].(string)

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), filePath), nil
}

// ListDirectory lists contents of a local directory.
func (dh *DirectHandler) ListDirectory(args map[string]interface{}) (string, error) {
	dirPath, _ := args["path"].(string)
	if strings.TrimSpace(dirPath) == "" {
		dirPath = "."
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("read directory %s: %w", dirPath, err)
	}

	type EntryInfo struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size,omitempty"`
	}

	items := make([]EntryInfo, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		items = append(items, EntryInfo{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  size,
		})
	}

	data, _ := json.MarshalIndent(items, "", "  ")
	return string(data), nil
}

// SearchFiles searches a directory with ripgrep when available and falls back to a Go walk.
func (dh *DirectHandler) SearchFiles(args map[string]interface{}) (string, error) {
	pattern, _ := args["pattern"].(string)
	if strings.TrimSpace(pattern) == "" {
		return "", fmt.Errorf("pattern is required")
	}
	root, _ := args["path"].(string)
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	maxResults := intArgument(args["max_results"], 50)
	searchPattern, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid search pattern: %w", err)
	}

	type Match struct {
		Path    string `json:"path"`
		Line    int    `json:"line"`
		Content string `json:"content"`
	}
	matches := make([]Match, 0)
	if rgPath, lookErr := exec.LookPath("rg"); lookErr == nil {
		cmdArgs := []string{"-n", "--no-heading", "--color", "never", "--null", "--glob", "!.git/**", "--glob", "!node_modules/**", "--", pattern, root}
		cmd := exec.Command(rgPath, cmdArgs...)
		out, runErr := cmd.Output()
		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
				return "", fmt.Errorf("rg search failed: %w", runErr)
			}
		}
		for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
			if line == "" {
				continue
			}
			nul := strings.IndexByte(line, 0)
			if nul < 0 {
				continue
			}
			parts := strings.SplitN(line[nul+1:], ":", 2)
			if len(parts) < 2 {
				continue
			}
			lineNo, convErr := strconv.Atoi(parts[0])
			if convErr != nil {
				continue
			}
			matches = append(matches, Match{Path: line[:nul], Line: lineNo, Content: parts[1]})
			if len(matches) >= maxResults {
				break
			}
		}
	} else {
		absRoot, absErr := filepath.Abs(root)
		if absErr != nil {
			return "", fmt.Errorf("resolve search path: %w", absErr)
		}
		walkErr := filepath.Walk(absRoot, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			name := info.Name()
			if info.IsDir() && (name == ".git" || name == "node_modules") {
				return filepath.SkipDir
			}
			if info.IsDir() {
				return nil
			}
			data, readErr := os.ReadFile(filePath)
			if readErr != nil {
				return nil
			}
			for i, text := range strings.Split(string(data), "\n") {
				if searchPattern.MatchString(text) {
					relPath, relErr := filepath.Rel(root, filePath)
					if relErr != nil {
						relPath = filePath
					}
					matches = append(matches, Match{Path: relPath, Line: i + 1, Content: text})
					if len(matches) >= maxResults {
						return filepath.SkipAll
					}
				}
			}
			return nil
		})
		if walkErr != nil && walkErr != filepath.SkipAll {
			return "", fmt.Errorf("search files: %w", walkErr)
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Path == matches[j].Path {
			return matches[i].Line < matches[j].Line
		}
		return matches[i].Path < matches[j].Path
	})
	data, _ := json.MarshalIndent(matches, "", "  ")
	return string(data), nil
}

func intArgument(value interface{}, fallback int) int {
	switch v := value.(type) {
	case float64:
		if int(v) > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case json.Number:
		if parsed, err := strconv.Atoi(v.String()); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

// SystemInfo gathers system metrics and environment info.
func (dh *DirectHandler) SystemInfo() (string, error) {
	hostname, _ := os.Hostname()
	info := map[string]interface{}{
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"cpus":        runtime.NumCPU(),
		"hostname":    hostname,
		"time":        time.Now().Format(time.RFC3339),
		"go_version":  runtime.Version(),
		"working_dir": func() string { d, _ := os.Getwd(); return d }(),
	}

	data, _ := json.MarshalIndent(info, "", "  ")
	return string(data), nil
}

// ComputerUse executes UI automation primitives.
func (dh *DirectHandler) ComputerUse(args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return "", fmt.Errorf("action is required")
	}

	switch action {
	case "status":
		var availableTools []string
		for _, tool := range []string{"grim", "scrot", "import"} {
			if _, err := exec.LookPath(tool); err == nil {
				availableTools = append(availableTools, tool)
			}
		}
		if runtime.GOOS == "windows" {
			availableTools = append(availableTools, "windows-screen-capture")
		}
		if driver := inputDriver(); driver != "" {
			availableTools = append(availableTools, driver)
		}
		if len(availableTools) == 0 {
			return "Computer use actuator not configured: no screen capture tool installed", nil
		}
		return fmt.Sprintf("Computer use actuator ready (available tools: %s)", strings.Join(availableTools, ", ")), nil

	case "screenshot":
		// Genuine capture implementation:
		// Check for available Linux CLI screenshot utilities
		for _, tool := range []string{"grim", "scrot", "import"} {
			if bin, err := exec.LookPath(tool); err == nil {
				outPath := filepath.Join(os.TempDir(), fmt.Sprintf("bridge_screenshot_%d.png", time.Now().UnixNano()))
				var cmd *exec.Cmd
				switch tool {
				case "grim":
					cmd = exec.Command(bin, outPath)
				case "scrot":
					cmd = exec.Command(bin, outPath)
				case "import":
					cmd = exec.Command(bin, "-window", "root", outPath)
				}
				if cmd != nil {
					core.SetHiddenConsole(cmd)
					if err := cmd.Run(); err == nil {
						if fi, statErr := os.Stat(outPath); statErr == nil && fi.Size() > 0 {
							return fmt.Sprintf("Screenshot captured via %s to %s (%d bytes)", tool, outPath, fi.Size()), nil
						}
					}
				}
			}
		}

		// macOS capture
		if runtime.GOOS == "darwin" {
			outPath := filepath.Join(os.TempDir(), fmt.Sprintf("bridge_screenshot_%d.png", time.Now().UnixNano()))
			cmd := exec.Command("screencapture", "-x", outPath)
			core.SetHiddenConsole(cmd)
			if err := cmd.Run(); err == nil {
				if fi, statErr := os.Stat(outPath); statErr == nil && fi.Size() > 0 {
					return fmt.Sprintf("Screenshot captured via screencapture to %s (%d bytes)", outPath, fi.Size()), nil
				}
			}
		}

		// Windows capture using powershell
		if runtime.GOOS == "windows" {
			outPath := filepath.Join(os.TempDir(), fmt.Sprintf("bridge_screenshot_%d.png", time.Now().UnixNano()))
			psScript := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms,System.Drawing; $b = New-Object Drawing.Bitmap([System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Width, [System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Height); $g = [Drawing.Graphics]::FromImage($b); $g.CopyFromScreen(0,0,0,0,$b.Size); $b.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png); $g.Dispose(); $b.Dispose();`, filepath.ToSlash(outPath))
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", psScript)
			core.SetHiddenConsole(cmd)
			if err := cmd.Run(); err == nil {
				if fi, statErr := os.Stat(outPath); statErr == nil && fi.Size() > 0 {
					return fmt.Sprintf("Screenshot captured via windows-screen-capture to %s (%d bytes)", outPath, fi.Size()), nil
				}
			}
		}

		return "", fmt.Errorf("screenshot capture not configured: no active display or capture utility available (grim/scrot/import/windows-screen-capture)")

	case "click":
		x, xOK := intArg(args["x"])
		y, yOK := intArg(args["y"])
		if !xOK || !yOK {
			return "", fmt.Errorf("click requires integer x and y coordinates")
		}
		if err := performClick(x, y); err != nil {
			return "", err
		}
		return fmt.Sprintf("Clicked at %d,%d", x, y), nil

	case "type":
		text, _ := args["text"].(string)
		if text == "" {
			return "", fmt.Errorf("type requires text")
		}
		if err := performType(text); err != nil {
			return "", err
		}
		return fmt.Sprintf("Typed %d characters", len([]rune(text))), nil

	default:
		return "", fmt.Errorf("unsupported computer use action: %s", action)
	}
}

// intArg accepts the number shapes JSON decoding produces for coordinates.
func intArg(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		if parsed, err := n.Int64(); err == nil {
			return int(parsed), true
		}
	}
	return 0, false
}

// splitCommand turns a command line into argv, honouring single and double quotes
// and backslash escapes. It deliberately does not expand variables, globs or pipes.
func splitCommand(line string) ([]string, error) {
	var (
		args    []string
		current strings.Builder
		quote   rune
		escaped bool
		started bool
	)
	for _, r := range line {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
			started = true
		case r == '\\' && quote != '\'':
			escaped = true
			started = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			started = true
		case r == ' ' || r == '\t' || r == '\n':
			if started {
				args = append(args, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(r)
			started = true
		}
	}
	if escaped {
		return nil, fmt.Errorf("command ends with a dangling escape")
	}
	if quote != 0 {
		return nil, fmt.Errorf("command has an unterminated quote")
	}
	if started {
		args = append(args, current.String())
	}
	return args, nil
}
