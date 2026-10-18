//go:build !windows

package acpmesh

import (
	"net"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// DetectListeningPorts returns the TCP ports the given PID is listening on.
// It prefers `ss -ltnp` (procps) and falls back to `lsof -nP -iTCP -sTCP:LISTEN`.
// Only observed ports are reported; an empty result means neither tool was able
// to attribute a listener to the process.
func DetectListeningPorts(pid int) []int {
	if pid <= 0 {
		return nil
	}
	if ports := portsFromSS(pid); len(ports) > 0 {
		return ports
	}
	return portsFromLsof(pid)
}

func portsFromSS(pid int) []int {
	out, err := exec.Command("ss", "-ltnp").Output()
	if err != nil {
		return nil
	}
	needle := "pid=" + strconv.Itoa(pid) + ","
	seen := make(map[int]bool)
	var ports []int
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		// ss -ltnp columns: State Recv-Q Send-Q Local:Port Peer:Port Process
		if port := portFromAddr(fields[3]); port > 0 && !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	sort.Ints(ports)
	return ports
}

func portsFromLsof(pid int) []int {
	out, err := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-a", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil
	}
	seen := make(map[int]bool)
	var ports []int
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		// lsof: COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME (LISTEN)
		if port := portFromAddr(fields[len(fields)-2]); port > 0 && !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	sort.Ints(ports)
	return ports
}

func portFromAddr(addr string) int {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return 0
	}
	return port
}
