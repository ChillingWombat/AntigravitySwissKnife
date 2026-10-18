//go:build !windows

package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ListProcesses enumerates running processes. Linux (and other hosts with a
// procfs) is read directly from /proc; hosts without procfs (macOS, BSD) fall
// back to parsing `ps`.
func ListProcesses() ([]ProcEntry, error) {
	if entries, err := listProcessesProc(); err == nil && len(entries) > 0 {
		return entries, nil
	}
	return listProcessesPS()
}

func listProcessesProc() ([]ProcEntry, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	out := make([]ProcEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		cmdBytes, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		commBytes, _ := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
		out = append(out, ProcEntry{
			PID:     pid,
			Comm:    strings.TrimSpace(string(commBytes)),
			Cmdline: strings.ReplaceAll(string(cmdBytes), "\x00", " "),
		})
	}
	return out, nil
}

func listProcessesPS() ([]ProcEntry, error) {
	cmdOut, err := exec.Command("ps", "-axo", "pid=,comm=,args=").Output()
	if err != nil {
		return nil, err
	}

	var out []ProcEntry
	for _, line := range strings.Split(string(cmdOut), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		comm := fields[1]
		cmdline := strings.Join(fields[2:], " ")
		out = append(out, ProcEntry{PID: pid, Comm: comm, Cmdline: cmdline})
	}
	return out, nil
}
