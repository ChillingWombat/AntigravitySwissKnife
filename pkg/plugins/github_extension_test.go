package plugins

import (
	"os/exec"
	"strings"
	"testing"
)

func TestGenerateGitHubExtensionCSS(t *testing.T) {
	css := GenerateGitHubExtensionCSS()
	if css == "" {
		t.Fatalf("expected non-empty CSS")
	}

	requiredSelectors := []string{
		".swiss-left-nav-group",
		".swiss-left-tabs-separator",
		".swiss-left-nav-tab",
		".swiss-github-aux-view",
		".swiss-gh-repo-bar",
		".swiss-gh-kanban-board",
		".swiss-gh-kanban-col",
		".swiss-gh-kanban-card",
		"#swiss-main-stage-container",
		".swiss-main-stage-tab",
		"#swiss-main-stage-body",
		".swiss-gh-context-menu",
		".swiss-gh-menu-item",
	}

	for _, sel := range requiredSelectors {
		if !strings.Contains(css, sel) {
			t.Errorf("expected CSS to contain selector %q", sel)
		}
	}
}

func TestGenerateGitHubExtensionScript(t *testing.T) {
	js := GenerateGitHubExtensionScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	requiredIdentifiers := []string{
		"window.__swissGitHubExtInitialized",
		"fetchRepoData",
		"setupLeftNavTabs",
		"renderSwissGitHubWorkspaceView",
		"renderStageKanbanBoardHTML",
		"bindKanbanDragEvents",
		"moveKanbanCard",
		"showGitHubContextMenu",
		"getItemCurrentColumn",
		"openMainStage",
		"closeMainStage",
		"renderMainStageUI",
		"renderMainStageContent",
	}

	for _, id := range requiredIdentifiers {
		if !strings.Contains(js, id) {
			t.Errorf("expected script to contain identifier %q", id)
		}
	}

	// Verify Board is leftmost tab before Issues in the tabs row
	boardIdx := strings.Index(js, `data-tab="board">Board</button>`)
	issuesIdx := strings.Index(js, `data-tab="issues">Issues`)
	if boardIdx == -1 || issuesIdx == -1 || boardIdx > issuesIdx {
		t.Errorf("expected Board tab (idx %d) to appear before Issues tab (idx %d)", boardIdx, issuesIdx)
	}

	if nodePath, err := exec.LookPath("node"); err == nil {
		cmd := exec.Command(nodePath, "--check")
		cmd.Stdin = strings.NewReader(js)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node syntax error in GenerateGitHubExtensionScript: %v\n%s", err, string(out))
		}
	}
}
