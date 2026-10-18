//go:build windows

package acpmesh

import (
	"net"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// DetectListeningPorts returns the TCP ports the given PID is listening on,
// parsed from netstat. It only reports ports that were actually observed; an
// empty result means detection failed or the process holds no TCP listener.
func DetectListeningPorts(pid int) []int {
	if pid <= 0 {
		return nil
	}
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return nil
	}

	want := strconv.Itoa(pid)
	seen := make(map[int]bool)
	var ports []int
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.EqualFold(fields[0], "TCP") {
			continue
		}
		if !strings.EqualFold(fields[len(fields)-2], "LISTENING") || fields[len(fields)-1] != want {
			continue
		}
		_, portStr, err := net.SplitHostPort(fields[1])
		if err != nil {
			continue
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 || seen[port] {
			continue
		}
		seen[port] = true
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports
}
