//go:build windows

package process

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ListProcesses enumerates running processes via the Toolhelp32 snapshot API and
// reads each command line from its PEB. Processes whose command line cannot be
// read (elevated/protected) are still returned with an empty Cmdline so callers
// can fall back to matching on the executable name.
func ListProcesses() ([]ProcEntry, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var entries []ProcEntry
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ProcessID == 0 {
			continue
		}
		entries = append(entries, ProcEntry{
			PID:     int(entry.ProcessID),
			Comm:    windows.UTF16ToString(entry.ExeFile[:]),
			Cmdline: readProcessCommandLine(entry.ProcessID),
		})
	}
	return entries, nil
}
