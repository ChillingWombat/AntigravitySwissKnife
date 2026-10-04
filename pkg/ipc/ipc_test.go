package ipc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIPCServerAndClientRoundtrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_ipc_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sockPath := filepath.Join(tmpDir, "test.sock")
	srv := NewServer(sockPath)

	srv.Register("test.echo", func(params json.RawMessage) (interface{}, *RPCError) {
		var p struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &RPCError{Code: InvalidParams, Message: err.Error()}
		}
		return map[string]string{"reply": p.Message}, nil
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	// Verify permissions are strictly 0600
	fi, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("socket stat error: %v", err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("expected socket permissions 0600, got %o", fi.Mode().Perm())
	}

	// Test client call
	client := NewClient(sockPath)
	var out struct {
		Reply string `json:"reply"`
	}
	err = client.Call("test.echo", map[string]string{"message": "hello go"}, &out)
	if err != nil {
		t.Fatalf("client.Call error: %v", err)
	}
	if out.Reply != "hello go" {
		t.Errorf("expected reply 'hello go', got %q", out.Reply)
	}

	// Test method not found
	err = client.Call("test.nonexistent", nil, nil)
	if err == nil {
		t.Errorf("expected error for nonexistent method")
	}
}
