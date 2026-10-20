package enhancements

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsBuiltinExtensionID(t *testing.T) {
	builtins := []string{"browser", "files", "memos", "github", "mobile", "computer_use", "swiss-knife"}
	for _, id := range builtins {
		if !IsBuiltinExtensionID(id) {
			t.Errorf("expected %q to be recognized as builtin extension", id)
		}
	}

	nonBuiltins := []string{"my-extension", "custom-tool", "blocks", "other"}
	for _, id := range nonBuiltins {
		if IsBuiltinExtensionID(id) {
			t.Errorf("expected %q not to be recognized as builtin extension", id)
		}
	}
}

func TestCommunityExtensions_StoreOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enh-comm-ext-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// 1. Initial state should be non-nil empty slice
	exts := store.GetCommunityExtensions()
	if exts == nil {
		t.Fatalf("expected non-nil empty slice for community extensions")
	}
	if len(exts) != 0 {
		t.Fatalf("expected 0 community extensions initially, got %d", len(exts))
	}

	// 2. Validation: Empty Name
	_, err = store.SaveCommunityExtension(CommunityExtension{Name: "   "})
	if err == nil {
		t.Errorf("expected error when saving extension with empty name")
	}

	// 3. Validation: Builtin ID collision
	_, err = store.SaveCommunityExtension(CommunityExtension{Name: "Browser View", ID: "browser", SourceType: "url", URL: "http://example.com"})
	if err == nil {
		t.Errorf("expected error when saving extension with builtin ID 'browser'")
	}
	_, err = store.SaveCommunityExtension(CommunityExtension{Name: "Swiss Tool", ID: "swiss-knife", SourceType: "url", URL: "http://example.com"})
	if err == nil {
		t.Errorf("expected error when saving extension with builtin ID 'swiss-knife'")
	}

	// 4. Validation: URL source requires URL
	_, err = store.SaveCommunityExtension(CommunityExtension{Name: "My URL Ext", SourceType: "url", URL: "  "})
	if err == nil {
		t.Errorf("expected error when URL source has empty URL")
	}

	// 5. Successful Save with Auto-slug generation and defaults
	saved, err := store.SaveCommunityExtension(CommunityExtension{
		Name:     "Dev Server Preview",
		URL:      "http://localhost:3000",
		Enabled:  true,
		AuxPanel: true,
		MainPage: false,
	})
	if err != nil {
		t.Fatalf("failed to save valid community extension: %v", err)
	}
	if saved.ID != "dev-server-preview" {
		t.Errorf("expected auto-slug ID 'dev-server-preview', got %q", saved.ID)
	}
	if saved.Version != "1.0.0" {
		t.Errorf("expected default version '1.0.0', got %q", saved.Version)
	}
	if saved.Author != "Community" {
		t.Errorf("expected default author 'Community', got %q", saved.Author)
	}
	if saved.Icon != "blocks" {
		t.Errorf("expected default icon 'blocks', got %q", saved.Icon)
	}
	if saved.SourceType != "url" {
		t.Errorf("expected default source_type 'url', got %q", saved.SourceType)
	}
	if saved.CreatedAt.IsZero() || saved.UpdatedAt.IsZero() {
		t.Errorf("expected timestamps to be populated")
	}

	// Visibility map in config must be synchronized
	vis, ok := store.GetConfig().Extensions[saved.ID]
	if !ok {
		t.Errorf("expected Extensions map to have entry for %q", saved.ID)
	} else if !vis.AuxPanel || vis.MainPage {
		t.Errorf("expected visibility {AuxPanel: true, MainPage: false}, got %+v", vis)
	}

	// 6. Save HTML Extension
	htmlExt, err := store.SaveCommunityExtension(CommunityExtension{
		ID:         "calculator-widget",
		Name:       "Calculator Widget",
		SourceType: "html",
		HTML:       "<button onclick=\"calc()\">Compute</button>",
		CSS:        "button { color: red; }",
		JS:         "function calc() { window.parent.postMessage({ type: 'swiss-insert-chat', text: '42' }, '*'); }",
		Icon:       "cpu",
		Enabled:    true,
		AuxPanel:   true,
		MainPage:   true,
	})
	if err != nil {
		t.Fatalf("failed to save HTML extension: %v", err)
	}
	if htmlExt.Icon != "cpu" {
		t.Errorf("expected icon 'cpu', got %q", htmlExt.Icon)
	}
	if !store.GetConfig().Extensions["calculator-widget"].MainPage {
		t.Errorf("expected MainPage to be true in Extensions map")
	}

	// 7. Update existing extension (upsert)
	firstCreatedAt := saved.CreatedAt
	time.Sleep(10 * time.Millisecond)
	updated, err := store.SaveCommunityExtension(CommunityExtension{
		ID:         "dev-server-preview",
		Name:       "Dev Server Preview Updated",
		URL:        "http://localhost:5173",
		Version:    "1.1.0",
		Author:     "David",
		Icon:       "globe",
		Enabled:    true,
		AuxPanel:   false,
		MainPage:   true,
	})
	if err != nil {
		t.Fatalf("failed to update extension: %v", err)
	}
	if updated.Name != "Dev Server Preview Updated" || updated.URL != "http://localhost:5173" {
		t.Errorf("expected updated fields to be reflected")
	}
	if !updated.CreatedAt.Equal(firstCreatedAt) {
		t.Errorf("expected CreatedAt to be preserved across update")
	}
	if !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Errorf("expected UpdatedAt to be after CreatedAt")
	}
	visUpdated := store.GetConfig().Extensions["dev-server-preview"]
	if visUpdated.AuxPanel || !visUpdated.MainPage {
		t.Errorf("expected visibility {AuxPanel: false, MainPage: true}, got %+v", visUpdated)
	}

	// 8. Delete Extension
	if err := store.DeleteCommunityExtension(""); err == nil {
		t.Errorf("expected error when deleting empty id")
	}
	if err := store.DeleteCommunityExtension("non-existent"); err == nil {
		t.Errorf("expected error when deleting non-existent id")
	}
	if err := store.DeleteCommunityExtension("calculator-widget"); err != nil {
		t.Fatalf("failed to delete extension: %v", err)
	}
	if _, exists := store.GetConfig().Extensions["calculator-widget"]; exists {
		t.Errorf("expected deleted extension to be removed from Extensions map")
	}
	extsAfterDelete := store.GetCommunityExtensions()
	if len(extsAfterDelete) != 1 || extsAfterDelete[0].ID != "dev-server-preview" {
		t.Errorf("expected 1 remaining extension 'dev-server-preview', got %d", len(extsAfterDelete))
	}

	// 9. Persistence check across store reloads
	store2, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to load store2: %v", err)
	}
	reloadedExts := store2.GetCommunityExtensions()
	if len(reloadedExts) != 1 || reloadedExts[0].ID != "dev-server-preview" {
		t.Fatalf("expected persisted extension 'dev-server-preview' in reloaded store, got %d", len(reloadedExts))
	}
	if store2.GetConfig().Extensions["dev-server-preview"].AuxPanel != false ||
		store2.GetConfig().Extensions["dev-server-preview"].MainPage != true {
		t.Errorf("expected persisted visibility to match")
	}
}

func TestCommunityExtensions_UpdateConfigPreservesAndSyncs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enh-comm-sync-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Save a community extension
	_, err = store.SaveCommunityExtension(CommunityExtension{
		ID:         "test-ext",
		Name:       "Test Extension",
		SourceType: "url",
		URL:        "https://example.com",
		AuxPanel:   true,
		MainPage:   false,
	})
	if err != nil {
		t.Fatalf("failed to save extension: %v", err)
	}

	// Partial update omitting community_extensions
	cfg := store.GetConfig()
	cfg.CommunityExtensions = nil // simulates a client omitting community_extensions
	// Toggle visibility through Extensions map
	cfg.Extensions["test-ext"] = ExtensionVisibility{AuxPanel: false, MainPage: true}

	if err := store.UpdateConfig(cfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	updatedExts := store.GetCommunityExtensions()
	if len(updatedExts) != 1 {
		t.Fatalf("expected community extensions to be preserved, got %d", len(updatedExts))
	}
	if updatedExts[0].AuxPanel != false || updatedExts[0].MainPage != true {
		t.Errorf("expected community extension visibility to be updated from Extensions map, got AuxPanel=%v MainPage=%v",
			updatedExts[0].AuxPanel, updatedExts[0].MainPage)
	}
}
