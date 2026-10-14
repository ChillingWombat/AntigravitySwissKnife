//go:build !windows

package process

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// findAntigravityProcessesNative scans Linux /proc for running antigravity instances.
func (s *Shield) findAntigravityProcessesNative() ([]ProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var results []ProcessInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		if isZombieProcess(pid) {
			continue
		}
		if s.IsProtected(pid) {
			continue
		}

		exePath, _ := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
		exeLower := strings.ToLower(exePath)
		if strings.Contains(exeLower, "swiss") || strings.Contains(exeLower, "antigravity-swiss") {
			continue
		}

		cmdlineBytes, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
		lowerCmdline := strings.ToLower(cmdline)
		if strings.Contains(lowerCmdline, "swiss") || strings.Contains(lowerCmdline, "antigravity-swiss") || strings.Contains(lowerCmdline, "antigravity-swi") {
			continue
		}

		// Determine if the process is an authentic Antigravity IDE executable:
		// Check 1: The resolved executable symlink (/proc/<pid>/exe)
		isExactExe := IsExactAntigravityExecutable(exePath)

		// Check 2: The process command name (/proc/<pid>/comm)
		commBytes, _ := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
		comm := strings.TrimSpace(string(commBytes))
		isExactComm := IsExactAntigravityExecutable(comm)

		// Check 3: The primary executable token in the command line (argv[0])
		argv0 := ""
		if fields := strings.Fields(cmdline); len(fields) > 0 {
			argv0 = fields[0]
		}
		isExactArgv0 := IsExactAntigravityExecutable(argv0)

		if !isExactExe && !isExactComm && !isExactArgv0 {
			continue
		}

		commLower := strings.ToLower(comm)
		if strings.Contains(commLower, "swiss") || strings.Contains(commLower, "antigravity-swi") || strings.HasPrefix(commLower, "antigravity-sw") {
			continue
		}

		isHost := (s.protectedPID > 0 && pid == s.protectedPID)
		results = append(results, ProcessInfo{
			PID:         pid,
			Name:        comm,
			Cmdline:     strings.TrimSpace(cmdline),
			IsHostIDE:   isHost,
			IsProtected: s.IsProtected(pid),
		})
	}

	// Prioritize primary Desktop IDE process at the front (comm == "antigravity" and not an Electron sub-process)
	sort.SliceStable(results, func(i, j int) bool {
		isMainIDE := func(p ProcessInfo) bool {
			lower := strings.ToLower(p.Cmdline)
			return (p.Name == "antigravity" || p.Name == "antigravity.exe") && !strings.Contains(lower, "--type=")
		}
		mainI := isMainIDE(results[i])
		mainJ := isMainIDE(results[j])
		if mainI != mainJ {
			return mainI
		}
		return results[i].PID < results[j].PID
	})

	return results, nil
}
