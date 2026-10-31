package acpmesh

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Endpoint kinds returned by EndpointKind and reported in detection results.
const (
	KindUnix  = "unix"
	KindPipe  = "pipe"
	KindTCP   = "tcp"
	KindStdio = "stdio"
)

// EndpointKind classifies an endpoint string.
func EndpointKind(endpoint string) string {
	switch {
	case strings.HasPrefix(endpoint, "unix://"):
		return KindUnix
	case strings.HasPrefix(endpoint, "npipe://"), strings.HasPrefix(endpoint, `\\.\pipe\`):
		return KindPipe
	default:
		return KindTCP
	}
}

// ValidateEndpoint accepts the endpoint forms the daemon can dial:
// unix://<path>, \\.\pipe\<name>, npipe://<name> and host:port.
func ValidateEndpoint(endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return errors.New("endpoint is required")
	}
	switch EndpointKind(endpoint) {
	case KindUnix:
		if strings.TrimSpace(strings.TrimPrefix(endpoint, "unix://")) == "" {
			return errors.New("unix endpoints must include a socket path (unix:///path/to.sock)")
		}
		return nil
	case KindPipe:
		name := strings.TrimPrefix(strings.TrimPrefix(endpoint, "npipe://"), `\\.\pipe\`)
		if strings.TrimSpace(name) == "" {
			return errors.New("named pipe endpoints must include a pipe name (\\\\.\\pipe\\<name>)")
		}
		return nil
	default:
		host, portStr, err := net.SplitHostPort(endpoint)
		if err != nil {
			return fmt.Errorf("endpoint must be unix://<path>, \\\\.\\pipe\\<name> or host:port: %v", err)
		}
		if strings.TrimSpace(host) == "" {
			return errors.New("tcp endpoints must include a host (e.g. 127.0.0.1:4096)")
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("tcp port %q is not a valid port (1-65535)", portStr)
		}
		return nil
	}
}

// ProbeEndpoint dials the endpoint and reports reachability plus latency in
// milliseconds. Unix sockets are dialled with net.Dial (unavailable on Windows,
// which reports unreachable); named pipes are opened with os.OpenFile.
func ProbeEndpoint(endpoint string, timeout time.Duration) (bool, float64) {
	if timeout <= 0 {
		timeout = 20 * time.Millisecond
	}
	start := time.Now()

	var err error
	switch EndpointKind(endpoint) {
	case KindUnix:
		var conn net.Conn
		conn, err = net.DialTimeout("unix", strings.TrimPrefix(endpoint, "unix://"), timeout)
		if err == nil {
			_ = conn.Close()
		}
	case KindPipe:
		err = probePipe(endpoint, timeout)
	default:
		var conn net.Conn
		conn, err = net.DialTimeout("tcp", endpoint, timeout)
		if err == nil {
			_ = conn.Close()
		}
	}
	if err != nil {
		return false, 0
	}

	latency := float64(time.Since(start).Microseconds()) / 1000.0
	if latency < 0.1 {
		latency = 0.1
	}
	return true, latency
}

// Candidate is one auto-detection result.
type Candidate struct {
	Endpoint  string `json:"endpoint"`
	Kind      string `json:"kind"`
	Reachable bool   `json:"reachable"`
	Note      string `json:"note"`
}

// BuildCandidates assembles detection candidates strictly from observed inputs:
// the currently configured endpoint, the OS default for the host, TCP ports the
// running process is listening on, and socket files that exist on disk. It never
// invents an endpoint. Reachability is left false and must be filled in by the
// caller after probing.
func BuildCandidates(configured, osDefault string, listeningPorts []int, existingSocketPaths []string) []Candidate {
	out := make([]Candidate, 0, 2+len(listeningPorts)+len(existingSocketPaths))
	seen := make(map[string]bool, cap(out))

	add := func(endpoint, kind, note string) {
		endpoint = strings.TrimSpace(endpoint)
		if endpoint == "" || seen[endpoint] {
			return
		}
		seen[endpoint] = true
		out = append(out, Candidate{Endpoint: endpoint, Kind: kind, Note: note})
	}

	add(configured, EndpointKind(configured), "Currently configured endpoint")
	add(osDefault, EndpointKind(osDefault), "OS default for this host")
	for _, port := range listeningPorts {
		if port <= 0 || port > 65535 {
			continue
		}
		endpoint := fmt.Sprintf("127.0.0.1:%d", port)
		add(endpoint, KindTCP, "Listening TCP port reported by the running process")
	}
	for _, socketPath := range existingSocketPaths {
		socketPath = strings.TrimSpace(socketPath)
		if socketPath == "" {
			continue
		}
		if !strings.HasPrefix(socketPath, "unix://") {
			socketPath = "unix://" + socketPath
		}
		add(socketPath, KindUnix, "Socket file exists on disk")
	}

	return out
}
