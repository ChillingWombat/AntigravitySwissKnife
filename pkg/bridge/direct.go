package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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

	var shellCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		shellCmd = exec.CommandContext(ctx, "cmd.exe", "/c", cmdStr)
	} else {
		shellCmd = exec.CommandContext(ctx, "bash", "-c", cmdStr)
	}
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
					if err := cmd.Run(); err == nil {
						if fi, statErr := os.Stat(outPath); statErr == nil && fi.Size() > 0 {
							return fmt.Sprintf("Screenshot captured via %s to %s (%d bytes)", tool, outPath, fi.Size()), nil
						}
					}
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
			if err := cmd.Run(); err == nil {
				if fi, statErr := os.Stat(outPath); statErr == nil && fi.Size() > 0 {
					return fmt.Sprintf("Screenshot captured via windows-screen-capture to %s (%d bytes)", outPath, fi.Size()), nil
				}
			}
		}

		return "", fmt.Errorf("screenshot capture not configured: no active display or capture utility available (grim/scrot/import/windows-screen-capture)")

	case "click", "type":
		return "", fmt.Errorf("action %q not configured: UI automation driver not installed", action)

	default:
		return "", fmt.Errorf("unsupported computer use action: %s", action)
	}
}

