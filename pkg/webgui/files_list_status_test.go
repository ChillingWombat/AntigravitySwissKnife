package webgui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type filesListResponse struct {
	Success bool       `json:"success"`
	Error   string     `json:"error"`
	Path    string     `json:"path"`
	Files   []FileItem `json:"files"`
}

func requestFilesList(t *testing.T, s *Server, rawPath string) (int, filesListResponse) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/files/list?path="+url.QueryEscape(rawPath), nil)
	rec := httptest.NewRecorder()
	s.handleFilesList(rec, req)

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	var resp filesListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}
	return rec.Code, resp
}

func TestFilesList_HTTPStatusForInvalidPaths(t *testing.T) {
	s := &Server{}
	root := t.TempDir()
	validDir := filepath.Join(root, "valid")
	if err := os.MkdirAll(validDir, 0700); err != nil {
		t.Fatalf("create valid directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "marker.txt"), []byte("ok"), 0600); err != nil {
		t.Fatalf("create marker file: %v", err)
	}
	filePath := filepath.Join(root, "not-a-directory.txt")
	if err := os.WriteFile(filePath, []byte("file"), 0600); err != nil {
		t.Fatalf("create file: %v", err)
	}

	t.Run("valid directory remains 200", func(t *testing.T) {
		status, resp := requestFilesList(t, s, validDir)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%+v", status, http.StatusOK, resp)
		}
		if !resp.Success {
			t.Fatalf("success = false, want true; error=%q", resp.Error)
		}
		if resp.Path != validDir {
			t.Errorf("path = %q, want %q", resp.Path, validDir)
		}
		if len(resp.Files) != 1 || resp.Files[0].Name != "marker.txt" {
			t.Errorf("files = %+v, want marker.txt", resp.Files)
		}
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "relative path",
			path:       filepath.Join("relative", "directory"),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "parent traversal",
			path:       filepath.Join("..", "..", "etc"),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "encoded traversal",
			path:       "%2e%2e%2fetc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "null byte",
			path:       filepath.Join(root, "bad") + "\x00suffix",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing directory",
			path:       filepath.Join(root, "missing"),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "file instead of directory",
			path:       filePath,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, resp := requestFilesList(t, s, tc.path)
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%+v", status, tc.wantStatus, resp)
			}
			if resp.Success {
				t.Errorf("success = true, want false")
			}
			if resp.Error == "" {
				t.Errorf("error is empty, want diagnostic message")
			}
			if resp.Files == nil || len(resp.Files) != 0 {
				t.Errorf("files = %+v, want non-nil empty slice", resp.Files)
			}
		})
	}
}

func TestFilesList_HTTPStatusForReadFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions are not enforced the same way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory read permissions")
	}

	dir := t.TempDir()
	if err := os.Chmod(dir, 0000); err != nil {
		t.Fatalf("chmod unreadable directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })

	status, resp := requestFilesList(t, &Server{}, dir)
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%+v", status, http.StatusForbidden, resp)
	}
	if resp.Success || resp.Error == "" {
		t.Errorf("response = %+v, want success=false with an error", resp)
	}
}
