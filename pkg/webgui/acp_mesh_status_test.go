package webgui

import (
	"net"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/acpmesh"
)

// A detected process whose endpoint never answers is installed, not connected, and
// nothing about it may look negotiated.
func TestDetectedProcessWithoutEndpointIsNotConnected(t *testing.T) {
	agent := acpmesh.Resolved{
		ID:                "antigravity",
		Name:              "Google Antigravity 2.0",
		Endpoint:          "unix:///run/user/1000/antigravity-acp.sock",
		Version:           "v1.2.0-draft",
		Tools:             []string{"read_file", "write_file"},
		StatusWhenRunning: "active_hosting",
	}
	node := buildACPNode(agent, true, 4242, false)

	if node["status"] != "installed_idle" {
		t.Errorf("status = %v, want installed_idle", node["status"])
	}
	if node["ping_latency_ms"] != nil {
		t.Errorf("latency = %v, want nil when the endpoint is silent", node["ping_latency_ms"])
	}
	if node["acp_version"] != "" {
		t.Errorf("version = %v, want empty without a handshake", node["acp_version"])
	}
	if tools, _ := node["supported_tools"].([]string); len(tools) != 0 {
		t.Errorf("tools = %v, want none without a handshake", tools)
	}
	if node["last_handshake"] != "never" {
		t.Errorf("handshake = %v, want never", node["last_handshake"])
	}
}

// A reachable endpoint reports the measured latency and the declared metadata.
func TestReachableEndpointReportsMeasuredLatency(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no loopback listener available")
	}
	defer listener.Close()

	agent := acpmesh.Resolved{
		ID:                "pi",
		Name:              "Pi",
		Endpoint:          listener.Addr().String(),
		Version:           "v1.1.2",
		Tools:             []string{"chat_eval"},
		StatusWhenRunning: "connected",
	}
	node := buildACPNode(agent, true, 1, true)

	if node["status"] != "connected" {
		t.Errorf("status = %v, want connected", node["status"])
	}
	if node["ping_latency_ms"] == nil {
		t.Error("latency missing for a reachable endpoint")
	}
	if node["acp_version"] != "v1.1.2" {
		t.Errorf("version = %v", node["acp_version"])
	}
}

// Nothing running and nothing reachable must not read as a healthy daemon.
func TestAbsentAgentReportsNotFound(t *testing.T) {
	node := buildACPNode(acpmesh.Resolved{ID: "ghost", Endpoint: "127.0.0.1:1"}, false, 0, false)
	if node["status"] != "not_found" {
		t.Errorf("status = %v, want not_found", node["status"])
	}
}
