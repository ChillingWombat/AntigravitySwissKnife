//go:build !windows

package process

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// findAntigravityProcessesNative scans /proc on Linux and falls back to the
// platform process list on macOS or when procfs is unavailable.
func (s *Shield) findAntigravityProcessesNative() ([]ProcessInfo, error) {
	if runtime.GOOS == "darwin" {
		return s.findAntigravityProcessesFromList()
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return s.findAntigravityProcessesFromList()
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
		var rawArgv0 string
		if nulIdx := bytes.IndexByte(cmdlineBytes, 0); nulIdx >= 0 {
			rawArgv0 = string(cmdlineBytes[:nulIdx])
		} else {
			rawArgv0 = string(cmdlineBytes)
		}
		argv0 := extractArgv0(rawArgv0)
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

	sortAntigravityProcesses(results)
	return results, nil
}

func (s *Shield) findAntigravityProcessesFromList() ([]ProcessInfo, error) {
	entries, err := ListProcesses()
	if err != nil {
		return nil, err
	}

	var results []ProcessInfo
	for _, entry := range entries {
		if entry.PID <= 0 || entry.PID == os.Getpid() || (os.Getppid() > 1 && entry.PID == os.Getppid()) {
			continue
		}
		if isZombieProcess(entry.PID) || s.IsProtected(entry.PID) {
			continue
		}
		if !isAntigravityProcess(entry.Comm, entry.Cmdline) {
			continue
		}
		results = append(results, ProcessInfo{
			PID:         entry.PID,
			Name:        entry.Comm,
			Cmdline:     strings.TrimSpace(entry.Cmdline),
			IsHostIDE:   s.protectedPID > 0 && entry.PID == s.protectedPID,
			IsProtected: s.IsProtected(entry.PID),
		})
	}
	sortAntigravityProcesses(results)
	return results, nil
}

func isAntigravityProcess(comm, cmdline string) bool {
	lowerComm := strings.ToLower(comm)
	lowerCmdline := strings.ToLower(cmdline)
	if strings.Contains(lowerComm, "swiss") || strings.Contains(lowerComm, "antigravity-swi") ||
		strings.Contains(lowerCmdline, "swiss") || strings.Contains(lowerCmdline, "antigravity-swi") {
		return false
	}
	return IsExactAntigravityExecutable(comm) || IsExactAntigravityExecutable(extractArgv0(cmdline))
}

// Prioritize primary Desktop IDE process at the front (comm == "antigravity" and not an Electron sub-process)
func sortAntigravityProcesses(results []ProcessInfo) {
	sort.SliceStable(results, func(i, j int) bool {
		isMainIDE := func(p ProcessInfo) bool {
			lower := strings.ToLower(p.Cmdline)
			return (IsExactAntigravityExecutable(p.Name) || IsExactAntigravityExecutable(extractArgv0(p.Cmdline))) && !strings.Contains(lower, "--type=")
		}
		mainI := isMainIDE(results[i])
		mainJ := isMainIDE(results[j])
		if mainI != mainJ {
			return mainI
		}
		return results[i].PID < results[j].PID
	})
}
