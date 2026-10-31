package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/acpmesh"
)

type acpMeshResponse struct {
	Status          string                   `json:"status"`
	MeshNodes       int                      `json:"mesh_nodes"`
	ProtocolVersion string                   `json:"protocol_version"`
	Agents          []map[string]interface{} `json:"agents"`
}

func decodeACPMesh(t *testing.T, resp *http.Response) acpMeshResponse {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
	var out acpMeshResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	return out
}

func agentByID(t *testing.T, mesh acpMeshResponse, id string) map[string]interface{} {
	t.Helper()
	for _, a := range mesh.Agents {
		if a["id"] == id {
			return a
		}
	}
	t.Fatalf("agent %q missing from response", id)
	return nil
}

func TestACPAgentRegistryEndpoints(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())

	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()
	baseURL := "http://" + srv.Addr()

	getMesh := func() acpMeshResponse {
		resp, err := http.Get(baseURL + "/api/utilities/acp")
		if err != nil {
			t.Fatalf("GET /api/utilities/acp failed: %v", err)
		}
		return decodeACPMesh(t, resp)
	}

	post := func(body string) *http.Response {
		resp, err := http.Post(baseURL+"/api/utilities/acp/agents", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("POST /api/utilities/acp/agents failed: %v", err)
		}
		return resp
	}

	deleteAgent := func(id string) *http.Response {
		req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/utilities/acp/agents?id="+id, nil)
		if err != nil {
			t.Fatalf("building DELETE failed: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("DELETE failed: %v", err)
		}
		return resp
	}

	t.Run("presets are returned with OS defaults and are editable", func(t *testing.T) {
		mesh := getMesh()
		if mesh.MeshNodes != len(acpmesh.Presets()) || len(mesh.Agents) != len(acpmesh.Presets()) {
			t.Fatalf("expected %d presets, got %d", len(acpmesh.Presets()), len(mesh.Agents))
		}
		antigravity := agentByID(t, mesh, acpmesh.PresetAntigravity)
		if antigravity["editable"] != true {
			t.Fatal("every node must be editable")
		}
		if antigravity["os_default"] != true {
			t.Fatal("a preset without override must report os_default=true")
		}
		if antigravity["custom"] != false {
			t.Fatal("a preset must not be reported as custom")
		}
	})

	t.Run("preset override is persisted and wins over the OS default", func(t *testing.T) {
		resp := post(`{"id":"agent-devin","name":"Devin","endpoint":"127.0.0.1:19000","acp_version":"v9.9.9"}`)
		mesh := decodeACPMesh(t, resp)
		devin := agentByID(t, mesh, acpmesh.PresetDevin)
		if devin["port_socket"] != "127.0.0.1:19000" {
			t.Fatalf("override endpoint not applied: %v", devin["port_socket"])
		}
		if devin["os_default"] != false {
			t.Fatal("override endpoint must report os_default=false")
		}
		if devin["acp_version"] != "v9.9.9" {
			t.Fatalf("override version not applied: %v", devin["acp_version"])
		}
		if devin["custom"] != false {
			t.Fatal("preset override must not be flagged custom")
		}

		// Reloading from disk must keep the override.
		reloaded := getMesh()
		if agentByID(t, reloaded, acpmesh.PresetDevin)["port_socket"] != "127.0.0.1:19000" {
			t.Fatal("override was not persisted")
		}
	})

	t.Run("custom agent is appended and deletable", func(t *testing.T) {
		resp := post(`{"name":"My Test Agent","type":"Test","endpoint":"127.0.0.1:19001","tools":["shell","shell",""]}`)
		mesh := decodeACPMesh(t, resp)
		custom := agentByID(t, mesh, "custom-my-test-agent")
		if custom["custom"] != true {
			t.Fatal("new agent must be flagged custom")
		}
		if custom["editable"] != true {
			t.Fatal("custom agent must be editable")
		}
		tools, _ := custom["supported_tools"].([]interface{})
		if len(tools) != 1 || tools[0] != "shell" {
			t.Fatalf("tools not normalised: %v", custom["supported_tools"])
		}

		// Editing the custom agent must update it in place, not duplicate it.
		edited := decodeACPMesh(t, post(`{"id":"custom-my-test-agent","name":"My Test Agent","endpoint":"127.0.0.1:19002"}`))
		customCount := 0
		for _, a := range edited.Agents {
			if a["custom"] == true {
				customCount += 1
			}
		}
		if customCount != 1 {
			t.Fatalf("expected exactly one custom agent after edit, got %d", customCount)
		}
		if agentByID(t, edited, "custom-my-test-agent")["port_socket"] != "127.0.0.1:19002" {
			t.Fatal("custom agent edit was not applied")
		}

		afterDelete := decodeACPMesh(t, deleteAgent("custom-my-test-agent"))
		for _, a := range afterDelete.Agents {
			if a["id"] == "custom-my-test-agent" {
				t.Fatal("custom agent still present after DELETE")
			}
		}
	})

	t.Run("deleting a preset override resets it to the OS default", func(t *testing.T) {
		mesh := decodeACPMesh(t, deleteAgent(acpmesh.PresetDevin))
		devin := agentByID(t, mesh, acpmesh.PresetDevin)
		if devin["os_default"] != true {
			t.Fatal("preset must fall back to its OS default after DELETE")
		}
		// The preset's version is a declaration, not a handshake result: it is only
		// reported once the endpoint answers. With no override and no probe, the node
		// must not present one.
		if devin["acp_version"] != "" {
			t.Fatalf("unreachable preset must not report a version, got %v", devin["acp_version"])
		}
	})

	t.Run("invalid submissions are rejected", func(t *testing.T) {
		for _, body := range []string{
			`{"name":"Bad","endpoint":"not-an-endpoint"}`,
			`{"endpoint":"127.0.0.1:19002"}`,
			`{"id":"agent-opencode","endpoint":""}`,
			`not json`,
		} {
			resp := post(body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("body %q: expected 400, got %d", body, resp.StatusCode)
			}
		}
	})

	t.Run("deleting an unknown agent returns 404", func(t *testing.T) {
		resp := deleteAgent("custom-does-not-exist")
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})
}

func TestACPAgentDetectEndpoint(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())

	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()
	baseURL := "http://" + srv.Addr()

	post := func(body string) *http.Response {
		resp, err := http.Post(baseURL+"/api/utilities/acp/detect", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("POST detect failed: %v", err)
		}
		return resp
	}

	t.Run("detect returns observed candidates only", func(t *testing.T) {
		resp := post(`{"id":"agent-opencode"}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var out struct {
			Candidates []acpmesh.Candidate `json:"candidates"`
			OSDefault  string              `json:"os_default"`
			PID        int                 `json:"pid"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if out.OSDefault != "127.0.0.1:4096" {
			t.Fatalf("unexpected os_default %q", out.OSDefault)
		}
		if len(out.Candidates) == 0 {
			t.Fatal("expected at least the configured endpoint candidate")
		}
		found := false
		for _, c := range out.Candidates {
			if c.Endpoint == "127.0.0.1:4096" {
				found = true
			}
			if c.Endpoint == "" || c.Note == "" {
				t.Fatalf("candidate must be described: %+v", c)
			}
			if strings.HasPrefix(c.Endpoint, "http") {
				t.Fatalf("candidate %q was not observed", c.Endpoint)
			}
		}
		if !found {
			t.Fatal("configured endpoint must be listed as a candidate")
		}
	})

	t.Run("detect rejects unknown and empty ids", func(t *testing.T) {
		for _, body := range []string{`{"id":"agent-nope"}`, `{}`, `{"id":""}`} {
			resp := post(body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("body %q: expected 400/404, got %d", body, resp.StatusCode)
			}
		}
	})

	t.Run("stdio presets report a transport candidate", func(t *testing.T) {
		for _, id := range []string{acpmesh.PresetPi, acpmesh.PresetCodex} {
			resp := post(`{"id":"` + id + `"}`)
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				t.Fatalf("%s: expected 200, got %d", id, resp.StatusCode)
			}

			var out struct {
				Candidates []acpmesh.Candidate `json:"candidates"`
				OSDefault  string              `json:"os_default"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				resp.Body.Close()
				t.Fatalf("%s: decode failed: %v", id, err)
			}
			resp.Body.Close()

			if out.OSDefault != "" {
				t.Fatalf("%s: os_default = %q, want stdio", id, out.OSDefault)
			}
			if len(out.Candidates) == 0 ||
				out.Candidates[0].Endpoint != "stdio" ||
				out.Candidates[0].Kind != acpmesh.KindStdio ||
				out.Candidates[0].Note == "" {
				t.Fatalf("%s: missing stdio candidate: %+v", id, out.Candidates)
			}
		}
	})
}
