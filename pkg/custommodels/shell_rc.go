package custommodels

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

const (
	shellRCBeginMarker = "# >>> Antigravity Swiss Knife CLI Integration >>>"
	shellRCEndMarker   = "# <<< Antigravity Swiss Knife CLI Integration <<<"
)

type shellRCMarkerPair struct {
	begin string
	end   string
}

var shellRCMarkerPairs = []shellRCMarkerPair{
	{shellRCBeginMarker, shellRCEndMarker},
	{"# >>> Antigravity Swiss Knife >>>", "# <<< Antigravity Swiss Knife <<<"},
	{"# >>> Swiss Knife CLI Integration >>>", "# <<< Swiss Knife CLI Integration <<<"},
	{"# BEGIN Antigravity Swiss Knife CLI Integration", "# END Antigravity Swiss Knife CLI Integration"},
	{"# BEGIN Swiss Knife CLI Integration", "# END Swiss Knife CLI Integration"},
	{"# BEGIN Antigravity Swiss Knife", "# END Antigravity Swiss Knife"},
}

var legacyShellRCHeaders = map[string]struct{}{
	"# Antigravity Swiss Knife - CLI Integration Environment": {},
	"# Antigravity Swiss Knife - CLI Gateway Environment":     {},
	"# Antigravity Swiss Knife - CLI Integration":             {},
	"# Antigravity Swiss Knife - CLI Gateway":                 {},
}

var legacyShellRCExports = []string{
	"export CLOUD_CODE_URL=",
	"export AGY_LLM_GATEWAY_URL=",
	"export AGY_LLM_GATEWAY_API_KEY=",
	"export AGY_LLM_GATEWAY_MODELS=",
}

type shellRCPlan struct {
	Rewritten []byte
	Owned     bool
	Changed   bool
	Legacy    bool
}

type shellRCFilePlan struct {
	TargetPath string
	Plan       shellRCPlan
	Original   []byte
}

var commitShellRC = commitShellRCFile

// CleanShellRCHooks removes only complete Swiss Knife managed blocks from user shell RC files.
func CleanShellRCHooks() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	if home == "" {
		return nil
	}

	var plans []shellRCFilePlan
	var planErrors []error
	seenTargets := make(map[string]struct{}, 2)
	for _, name := range []string{".zshrc", ".bashrc"} {
		filePlan, exists, err := planShellRCFile(filepath.Join(home, name))
		if err != nil {
			planErrors = append(planErrors, err)
			continue
		}
		if !exists || !filePlan.Plan.Changed {
			continue
		}
		if _, seen := seenTargets[filePlan.TargetPath]; seen {
			continue
		}
		seenTargets[filePlan.TargetPath] = struct{}{}
		plans = append(plans, filePlan)
	}
	if err := errors.Join(planErrors...); err != nil {
		return err
	}

	now := time.Now()
	var committed []shellRCFilePlan
	for _, filePlan := range plans {
		if err := commitShellRC(filePlan.TargetPath, filePlan.Plan, now); err != nil {
			rollbackErrors := []error{err}
			for i := len(committed) - 1; i >= 0; i-- {
				if restoreErr := restoreShellRC(committed[i].TargetPath, committed[i].Original); restoreErr != nil {
					rollbackErrors = append(rollbackErrors, restoreErr)
				}
			}
			return errors.Join(rollbackErrors...)
		}
		committed = append(committed, filePlan)
	}
	return nil
}

func planShellRCFile(rcPath string) (shellRCFilePlan, bool, error) {
	target, err := filepath.EvalSymlinks(rcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return shellRCFilePlan{}, false, nil
		}
		return shellRCFilePlan{}, false, fmt.Errorf("resolve shell RC %s: %w", rcPath, err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return shellRCFilePlan{}, false, nil
		}
		return shellRCFilePlan{}, false, fmt.Errorf("read shell RC %s: %w", target, err)
	}
	plan, err := scanShellRC(data)
	if err != nil {
		return shellRCFilePlan{}, false, fmt.Errorf("scan shell RC %s: %w", target, err)
	}
	return shellRCFilePlan{TargetPath: target, Plan: plan, Original: data}, true, nil
}

func cleanSingleShellRC(rcPath string) error {
	filePlan, exists, err := planShellRCFile(rcPath)
	if err != nil {
		return err
	}
	if !exists || !filePlan.Plan.Changed {
		return nil
	}
	return commitShellRC(filePlan.TargetPath, filePlan.Plan, time.Now())
}

func shellRCHooked(home string) bool {
	for _, name := range []string{".zshrc", ".bashrc"} {
		data, err := os.ReadFile(filepath.Join(home, name))
		if err != nil {
			continue
		}
		plan, err := scanShellRC(data)
		if err == nil && plan.Owned {
			return true
		}
	}
	return false
}

func scanShellRC(data []byte) (shellRCPlan, error) {
	lines := splitShellRCLines(data)
	rewritten := make([]byte, 0, len(data))
	var plan shellRCPlan

	for i := 0; i < len(lines); {
		text := shellRCLineText(lines[i])
		if pair, ok := shellRCFindBegin(text); ok {
			plan.Owned = true
			foundEnd := false
			for j := i + 1; j < len(lines); j++ {
				candidate := shellRCLineText(lines[j])
				if _, nested := shellRCFindBegin(candidate); nested {
					return shellRCPlan{}, fmt.Errorf("managed shell RC block restarted before end marker %q", pair.end)
				}
				if end, isEnd := shellRCFindEnd(candidate); isEnd {
					if end != pair.end {
						return shellRCPlan{}, fmt.Errorf("managed shell RC block ended with %q, want %q", candidate, pair.end)
					}
					foundEnd = true
					i = j + 1
					break
				}
			}
			if !foundEnd {
				return shellRCPlan{}, fmt.Errorf("managed shell RC block is missing end marker %q", pair.end)
			}
			continue
		}
		if _, ok := shellRCFindEnd(text); ok {
			return shellRCPlan{}, fmt.Errorf("stray shell RC end marker %q", text)
		}
		if isLegacyShellRCHeader(text) {
			j := i + 1
			for j < len(lines) && isLegacyShellRCExport(shellRCLineText(lines[j])) {
				j++
			}
			if j > i+1 {
				plan.Owned = true
				plan.Legacy = true
				i = j
				continue
			}
		}
		rewritten = append(rewritten, lines[i]...)
		i++
	}

	plan.Rewritten = append([]byte(nil), rewritten...)
	plan.Changed = !bytes.Equal(plan.Rewritten, data)
	return plan, nil
}

func splitShellRCLines(data []byte) [][]byte {
	var lines [][]byte
	for len(data) > 0 {
		index := bytes.IndexByte(data, '\n')
		if index < 0 {
			lines = append(lines, data)
			break
		}
		lines = append(lines, data[:index+1])
		data = data[index+1:]
	}
	return lines
}

// Marker matching ignores indentation and line terminators; rewritten lines retain their raw bytes.
func shellRCLineText(raw []byte) string {
	return strings.TrimSpace(string(raw))
}

func shellRCFindBegin(line string) (shellRCMarkerPair, bool) {
	for _, pair := range shellRCMarkerPairs {
		if line == pair.begin {
			return pair, true
		}
	}
	return shellRCMarkerPair{}, false
}

func shellRCFindEnd(line string) (string, bool) {
	for _, pair := range shellRCMarkerPairs {
		if line == pair.end {
			return pair.end, true
		}
	}
	return "", false
}

func isLegacyShellRCHeader(line string) bool {
	_, ok := legacyShellRCHeaders[line]
	return ok
}

func isLegacyShellRCExport(line string) bool {
	for _, prefix := range legacyShellRCExports {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func commitShellRCFile(path string, plan shellRCPlan, now time.Time) error {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("resolve shell RC %s: %w", path, err)
	}

	lockPath := target + ".swiss.lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open shell RC lock %s: %w", lockPath, err)
	}
	if err := core.LockFile(lock); err != nil {
		_ = lock.Close()
		return fmt.Errorf("lock shell RC %s: %w", target, err)
	}
	defer func() {
		_ = core.UnlockFile(lock)
		_ = lock.Close()
	}()

	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("re-read shell RC %s: %w", target, err)
	}
	fresh, err := scanShellRC(data)
	if err != nil {
		return fmt.Errorf("re-scan shell RC %s: %w", target, err)
	}
	if !fresh.Changed {
		return nil
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("stat shell RC %s: %w", target, err)
	}
	if fresh.Legacy {
		if err := writeShellRCBackup(target, data, info, now); err != nil {
			return err
		}
	}
	return replaceShellRC(target, fresh.Rewritten, info)
}

func restoreShellRC(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat shell RC for rollback %s: %w", path, err)
	}
	if err := replaceShellRC(path, data, info); err != nil {
		return fmt.Errorf("restore shell RC %s: %w", path, err)
	}
	return nil
}

func writeShellRCBackup(path string, data []byte, info os.FileInfo, now time.Time) error {
	backupPath := path + ".swiss-backup-" + now.UTC().Format("20060102T150405.000000000Z")
	backup, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create shell RC backup %s: %w", backupPath, err)
	}
	complete := false
	defer func() {
		_ = backup.Close()
		if !complete {
			_ = os.Remove(backupPath)
		}
	}()

	if _, err := backup.Write(data); err != nil {
		return fmt.Errorf("write shell RC backup %s: %w", backupPath, err)
	}
	if err := preserveShellRCOwner(backup, info); err != nil {
		return fmt.Errorf("preserve shell RC backup owner %s: %w", backupPath, err)
	}
	if err := backup.Chmod(info.Mode()); err != nil {
		return fmt.Errorf("preserve shell RC backup mode %s: %w", backupPath, err)
	}
	if err := backup.Sync(); err != nil {
		return fmt.Errorf("sync shell RC backup %s: %w", backupPath, err)
	}
	if err := backup.Close(); err != nil {
		return fmt.Errorf("close shell RC backup %s: %w", backupPath, err)
	}
	complete = true
	return nil
}

func replaceShellRC(path string, data []byte, info os.FileInfo) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".swiss-tmp-*")
	if err != nil {
		return fmt.Errorf("create shell RC temp: %w", err)
	}
	tempPath := temp.Name()
	renamed := false
	defer func() {
		_ = temp.Close()
		if !renamed {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write shell RC temp: %w", err)
	}
	if err := preserveShellRCOwner(temp, info); err != nil {
		return fmt.Errorf("preserve shell RC owner: %w", err)
	}
	if err := temp.Chmod(info.Mode()); err != nil {
		return fmt.Errorf("preserve shell RC mode: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync shell RC temp: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close shell RC temp: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace shell RC %s: %w", path, err)
	}
	renamed = true
	if err := syncShellRCDir(dir); err != nil {
		return fmt.Errorf("sync shell RC directory %s: %w", dir, err)
	}
	return nil
}

func syncShellRCDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	handle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer handle.Close()
	return handle.Sync()
}
