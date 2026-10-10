package webgui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/github"
)

func (s *Server) resolveWorkspace(r *http.Request) string {
	ws := r.URL.Query().Get("workspace_path")
	if ws == "" {
		ws = r.Header.Get("X-Workspace-Path")
	}
	if ws == "" {
		ws = "."
	}
	return cleanUserPath(ws)
}

func (s *Server) handleGitHubRepo(w http.ResponseWriter, r *http.Request) {
	ws := s.resolveWorkspace(r)
	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"success": true, "repo": repo})
}

func (s *Server) handleGitHubIssues(w http.ResponseWriter, r *http.Request) {
	ws := s.resolveWorkspace(r)
	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	state := r.URL.Query().Get("state")
	search := r.URL.Query().Get("search")

	issues, err := s.githubService.ListIssues(repo, state, search)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "issues": issues, "repo": repo})
}

func (s *Server) handleGitHubIssueDetail(w http.ResponseWriter, r *http.Request) {
	ws := s.resolveWorkspace(r)
	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	numStr := r.URL.Query().Get("number")
	number, err := strconv.Atoi(numStr)
	if err != nil || number <= 0 {
		writeJSON(w, map[string]interface{}{"success": false, "error": "invalid issue number"})
		return
	}

	detail, err := s.githubService.GetIssue(repo, number)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "issue": detail})
}

func (s *Server) handleGitHubIssueUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		WorkspacePath string   `json:"workspace_path"`
		Number        int      `json:"number"`
		Title         *string  `json:"title"`
		Body          *string  `json:"body"`
		State         *string  `json:"state"`
		AddLabels     []string `json:"add_labels"`
		RemoveLabels  []string `json:"remove_labels"`
		AddAssignees  []string `json:"add_assignees"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": "invalid payload: " + err.Error()})
		return
	}

	ws := payload.WorkspacePath
	if ws == "" {
		ws = s.resolveWorkspace(r)
	}

	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	req := github.UpdateIssueRequest{
		Title:        payload.Title,
		Body:         payload.Body,
		State:        payload.State,
		AddLabels:    payload.AddLabels,
		RemoveLabels: payload.RemoveLabels,
		AddAssignees: payload.AddAssignees,
	}

	updated, err := s.githubService.UpdateIssue(repo, payload.Number, req)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "issue": updated})
}

func (s *Server) handleGitHubIssueComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		WorkspacePath string `json:"workspace_path"`
		Number        int    `json:"number"`
		Comment       string `json:"comment"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": "invalid payload: " + err.Error()})
		return
	}

	if strings.TrimSpace(payload.Comment) == "" {
		writeJSON(w, map[string]interface{}{"success": false, "error": "comment cannot be empty"})
		return
	}

	ws := payload.WorkspacePath
	if ws == "" {
		ws = s.resolveWorkspace(r)
	}

	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	if err := s.githubService.AddIssueComment(repo, payload.Number, payload.Comment); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true})
}

func (s *Server) handleGitHubIssueCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		WorkspacePath string   `json:"workspace_path"`
		Title         string   `json:"title"`
		Body          string   `json:"body"`
		Labels        []string `json:"labels"`
		Assignees     []string `json:"assignees"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": "invalid payload: " + err.Error()})
		return
	}

	if strings.TrimSpace(payload.Title) == "" {
		writeJSON(w, map[string]interface{}{"success": false, "error": "title is required"})
		return
	}

	ws := payload.WorkspacePath
	if ws == "" {
		ws = s.resolveWorkspace(r)
	}

	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	created, err := s.githubService.CreateIssue(repo, github.CreateIssueRequest{
		Title:     payload.Title,
		Body:      payload.Body,
		Labels:    payload.Labels,
		Assignees: payload.Assignees,
	})
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "issue": created})
}

func (s *Server) handleGitHubPRs(w http.ResponseWriter, r *http.Request) {
	ws := s.resolveWorkspace(r)
	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	state := r.URL.Query().Get("state")
	prs, err := s.githubService.ListPullRequests(repo, state)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "prs": prs, "repo": repo})
}

func (s *Server) handleGitHubPRDetail(w http.ResponseWriter, r *http.Request) {
	ws := s.resolveWorkspace(r)
	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	numStr := r.URL.Query().Get("number")
	number, err := strconv.Atoi(numStr)
	if err != nil || number <= 0 {
		writeJSON(w, map[string]interface{}{"success": false, "error": "invalid PR number"})
		return
	}

	detail, err := s.githubService.GetPullRequest(repo, number)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "pr": detail})
}

func (s *Server) handleGitHubProjects(w http.ResponseWriter, r *http.Request) {
	ws := s.resolveWorkspace(r)
	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	projects, err := s.githubService.ListProjects(repo)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "projects": projects})
}

func (s *Server) handleGitHubKanbanBoard(w http.ResponseWriter, r *http.Request) {
	ws := s.resolveWorkspace(r)
	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	projectNum := 0
	if pStr := r.URL.Query().Get("project_number"); pStr != "" {
		if pVal, err := strconv.Atoi(pStr); err == nil {
			projectNum = pVal
		}
	}

	board, err := s.githubService.GetKanbanBoard(repo, projectNum)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "board": board, "repo": repo})
}

func (s *Server) handleGitHubKanbanMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req github.MoveKanbanCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": "invalid payload: " + err.Error()})
		return
	}

	ws := req.WorkspacePath
	if ws == "" {
		ws = s.resolveWorkspace(r)
	}

	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	if err := s.githubService.MoveKanbanCard(repo, &req); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Card #%d moved to %s", req.Number, req.TargetColumn),
	})
}


func (s *Server) handleGitHubAgentTasks(w http.ResponseWriter, r *http.Request) {
	ws := s.resolveWorkspace(r)
	tasks, err := s.githubService.GetAgentTasks(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "tasks": tasks})
}

func (s *Server) handleGitHubAgentTaskBind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req github.BindTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": "invalid payload: " + err.Error()})
		return
	}

	if req.ConversationID == "" {
		writeJSON(w, map[string]interface{}{"success": false, "error": "conversation_id required"})
		return
	}

	if err := s.githubService.BindTask(req.ConversationID, req.IssueNumber, req.AgentLabel); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true})
}

func (s *Server) handleGitHubAgentTaskLabel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req github.SetLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": "invalid payload: " + err.Error()})
		return
	}

	if req.ConversationID == "" {
		writeJSON(w, map[string]interface{}{"success": false, "error": "conversation_id required"})
		return
	}

	if err := s.githubService.SetAgentLabel(req.ConversationID, req.AgentLabel); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true})
}

func (s *Server) handleGitHubAgentTaskReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		WorkspacePath  string `json:"workspace_path"`
		ConversationID string `json:"conversation_id"`
		WorkItem       string `json:"work_item"`
		AgentLabel     string `json:"agent_label"`
		Issues         []int  `json:"issues"`
		PRs            []int  `json:"prs"`
		Status         string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": "invalid payload: " + err.Error()})
		return
	}

	if req.ConversationID == "" {
		writeJSON(w, map[string]interface{}{"success": false, "error": "conversation_id required"})
		return
	}

	ws := req.WorkspacePath
	if ws == "" {
		ws = s.resolveWorkspace(r)
	}

	task := github.SelfReportedTask{
		ConversationID: req.ConversationID,
		WorkItem:       req.WorkItem,
		AgentLabel:     req.AgentLabel,
		Issues:         req.Issues,
		PRs:            req.PRs,
		Status:         req.Status,
	}

	if err := s.githubService.ReportAgentTask(ws, task); err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true})
}

func (s *Server) handleGitHubContext(w http.ResponseWriter, r *http.Request) {
	ws := s.resolveWorkspace(r)
	repo, err := s.githubService.DetectRepository(ws)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	itemType := strings.ToLower(r.URL.Query().Get("type"))
	if itemType == "" {
		itemType = "issue"
	}
	number, _ := strconv.Atoi(r.URL.Query().Get("number"))
	if number <= 0 {
		writeJSON(w, map[string]interface{}{"success": false, "error": "invalid number"})
		return
	}

	contextText, err := s.githubService.FormatContextForChat(itemType, number, repo)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "context": contextText})
}
