package webgui

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"regexp"
	"strings"
	"testing"
)

var webGUIDistReferenceRE = regexp.MustCompile(`(?i)(?:^|\s)(?:src|href)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>]+))`)

func TestWebGUIDistReferencedAssets(t *testing.T) {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		t.Fatalf("open embedded web GUI dist: %v", err)
	}

	indexHTML, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		t.Fatalf("read embedded index.html: %v", err)
	}

	handler := (&Server{}).setupRoutes()
	seen := make(map[string]bool)
	references := 0
	for _, match := range webGUIDistReferenceRE.FindAllStringSubmatch(string(indexHTML), -1) {
		reference := firstNonEmpty(match[1:])
		if reference == "" || strings.HasPrefix(reference, "#") {
			continue
		}

		parsed, err := url.Parse(reference)
		if err != nil {
			t.Errorf("parse index.html reference %q: %v", reference, err)
			continue
		}
		if parsed.IsAbs() || strings.HasPrefix(reference, "//") {
			continue
		}

		assetPath := strings.TrimPrefix(path.Clean("/"+parsed.Path), "/")
		if assetPath == "" || assetPath == "." || seen[assetPath] {
			continue
		}
		seen[assetPath] = true
		references++

		if _, err := fs.Stat(dist, assetPath); err != nil {
			t.Errorf("index.html references missing dist asset %q: %v", assetPath, err)
			continue
		}

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+assetPath, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET /%s = %d, want %d", assetPath, rec.Code, http.StatusOK)
		}
	}

	if references == 0 {
		t.Fatal("index.html contains no relative src/href assets")
	}
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
