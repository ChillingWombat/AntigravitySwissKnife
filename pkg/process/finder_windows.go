//go:build windows

package process

import (
	"fmt"
	"sort"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// findAntigravityProcessesNative enumerates Windows processes via the Toolhelp32
// snapshot API and returns running Antigravity Desktop IDE instances.
//
// Electron child processes (renderer, GPU, utility) are included but sorted behind
// the primary IDE process, which is identified by absence of the "--type=" switch
// in its command line (with parent-PID classification as fallback when the command
// line cannot be read).
func (s *Shield) findAntigravityProcessesNative() ([]ProcessInfo, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	type rawEntry struct {
		pid       uint32
		parentPID uint32
		exeName   string
	}

	var entries []rawEntry
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		entries = append(entries, rawEntry{
			pid:       entry.ProcessID,
			parentPID: entry.ParentProcessID,
			exeName:   windows.UTF16ToString(entry.ExeFile[:]),
		})
	}

	// Set of live Antigravity PIDs, used to classify the primary IDE process:
	// Electron sub-processes always have another Antigravity process as parent.
	agyPIDs := make(map[uint32]bool)
	for _, e := range entries {
		if IsExactAntigravityExecutable(e.exeName) {
			agyPIDs[e.pid] = true
		}
	}

	var results []ProcessInfo
	for _, e := range entries {
		if !IsExactAntigravityExecutable(e.exeName) {
			continue
		}
		pid := int(e.pid)
		if s.IsProtected(pid) {
			continue
		}

		cmdline := readProcessCommandLine(e.pid)
		lowerCmdline := strings.ToLower(cmdline)
		if strings.Contains(lowerCmdline, "swiss") || strings.Contains(lowerCmdline, "antigravity-swiss") {
			continue
		}

		isHost := s.protectedPID > 0 && pid == s.protectedPID
		results = append(results, ProcessInfo{
			PID:         pid,
			Name:        e.exeName,
			Cmdline:     cmdline,
			IsHostIDE:   isHost,
			IsProtected: s.IsProtected(pid),
		})
	}

	// Prioritize the primary Desktop IDE process.
	sort.SliceStable(results, func(i, j int) bool {
		isMainIDE := func(p ProcessInfo) bool {
			lower := strings.ToLower(p.Cmdline)
			if strings.Contains(lower, "--type=") {
				return false
			}
			if lower != "" {
				return true
			}
			// Cmdline unreadable: fall back to the parent-PID heuristic.
			return !agyPIDs[uint32(p.PID)]
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

// readProcessCommandLine reads a process command line by walking its PEB via
// NtQueryInformationProcess + ReadProcessMemory. Returns "" when the process is
// inaccessible (e.g. elevated) or already exited.
func readProcessCommandLine(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)

	var pbi windows.PROCESS_BASIC_INFORMATION
	var retLen uint32
	if err := windows.NtQueryInformationProcess(h, windows.ProcessBasicInformation, unsafe.Pointer(&pbi), uint32(unsafe.Sizeof(pbi)), &retLen); err != nil {
		return ""
	}
	if pbi.PebBaseAddress == nil {
		return ""
	}

	// PEB->ProcessParameters offset is a stable ABI constant: 0x20 on 64-bit, 0x10 on 32-bit.
	processParametersOffset := uintptr(0x20)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		processParametersOffset = 0x10
	}

	var paramsAddr uintptr
	if err := readMemory(h, uintptr(unsafe.Pointer(pbi.PebBaseAddress))+processParametersOffset, unsafe.Pointer(&paramsAddr), unsafe.Sizeof(paramsAddr)); err != nil || paramsAddr == 0 {
		return ""
	}

	var cmdLine windows.NTUnicodeString
	cmdLineOffset := unsafe.Offsetof(windows.RTL_USER_PROCESS_PARAMETERS{}.CommandLine)
	if err := readMemory(h, paramsAddr+cmdLineOffset, unsafe.Pointer(&cmdLine), unsafe.Sizeof(cmdLine)); err != nil {
		return ""
	}
	if cmdLine.Buffer == nil || cmdLine.Length == 0 {
		return ""
	}

	buf := make([]uint16, cmdLine.Length/2)
	var bytesRead uintptr
	if err := windows.ReadProcessMemory(h, uintptr(unsafe.Pointer(cmdLine.Buffer)), (*byte)(unsafe.Pointer(&buf[0])), uintptr(cmdLine.Length), &bytesRead); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:bytesRead/2])
}

// readMemory reads size bytes from a target process into out.
func readMemory(h windows.Handle, addr uintptr, out unsafe.Pointer, size uintptr) error {
	var bytesRead uintptr
	return windows.ReadProcessMemory(h, addr, (*byte)(out), size, &bytesRead)
}
