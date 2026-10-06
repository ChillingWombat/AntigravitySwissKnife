package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var gitURLRegex = regexp.MustCompile(`(?:https://github\.com/|git@github\.com:)([^/]+)/([^/\.]+)(?:\.git)?`)

// Service provides high-level GitHub and agent task integration.
type Service struct {
	tracker *AgentTracker
	store   *Store
}

// NewService creates a new Service instance.
func NewService(tracker *AgentTracker, store *Store) *Service {
	return &Service{
		tracker: tracker,
		store:   store,
	}
}

// DetectRepository detects the GitHub repository and branch for the given workspace directory.
func (s *Service) DetectRepository(workspacePath string) (*RepoInfo, error) {
	dir := workspacePath
	if dir == "" {
		dir = "."
	}
	dir = filepath.Clean(dir)

	// 1. Get git remote URL
	cmdRemote := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	outRemote, err := cmdRemote.Output()
	if err != nil {
		return nil, fmt.Errorf("git remote get-url origin failed in %s: %w", dir, err)
	}
	remoteURL := strings.TrimSpace(string(outRemote))

	matches := gitURLRegex.FindStringSubmatch(remoteURL)
	if len(matches) < 3 {
		return nil, fmt.Errorf("unable to parse GitHub owner and repo from remote URL: %s", remoteURL)
	}
	owner := matches[1]
	name := strings.TrimSuffix(matches[2], ".git")

	// 2. Get current branch
	cmdBranch := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	outBranch, _ := cmdBranch.Output()
	branch := strings.TrimSpace(string(outBranch))

	return &RepoInfo{
		Owner:         owner,
		Name:          name,
		FullName:      fmt.Sprintf("%s/%s", owner, name),
		CurrentBranch: branch,
		RemoteURL:     remoteURL,
		WorkspacePath: dir,
	}, nil
}

// ListIssues retrieves issues for the repository and enriches them with active agent tasks.
func (s *Service) ListIssues(repo *RepoInfo, state string, search string) ([]Issue, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository info required")
	}

	st := strings.ToLower(state)
	if st == "" || (st != "open" && st != "closed" && st != "all") {
		st = "all"
	}

	args := []string{"issue", "list", "--repo", repo.FullName, "--state", st, "--limit", "100",
		"--json", "number,title,body,state,labels,assignees,author,comments,createdAt,updatedAt,url,closedAt"}
	if search != "" {
		args = append(args, "--search", search)
	}

	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh issue list failed: %s: %w", string(out), err)
	}

	type ghLabel struct {
		Name string `json:"name"`
	}
	type ghUser struct {
		Login string `json:"login"`
	}
	type ghComment struct {
		ID int64 `json:"id"`
	}
	type rawIssue struct {
		Number    int         `json:"number"`
		Title     string      `json:"title"`
		Body      string      `json:"body"`
		State     string      `json:"state"`
		Labels    []ghLabel   `json:"labels"`
		Assignees []ghUser    `json:"assignees"`
		Author    ghUser      `json:"author"`
		Comments  []ghComment `json:"comments"`
		CreatedAt time.Time   `json:"createdAt"`
		UpdatedAt time.Time   `json:"updatedAt"`
		URL       string      `json:"url"`
		ClosedAt  *time.Time  `json:"closedAt"`
	}

	var rawList []rawIssue
	if err := json.Unmarshal(out, &rawList); err != nil {
		return nil, fmt.Errorf("failed to parse gh issue list output: %w", err)
	}

	// Fetch active agent tasks for this workspace to link working agents
	var tasks []AgentTaskSummary
	if s.tracker != nil {
		tasks, _ = s.tracker.ListWorkspaceTasks(repo.WorkspacePath)
	}
	taskMap := make(map[int]AgentTaskSummary)
	for _, t := range tasks {
		if t.BoundIssueNumber > 0 {
			// Prefer currently active agent if multiple exist
			existing, exists := taskMap[t.BoundIssueNumber]
			if !exists || (t.NotFullyIdle && !existing.NotFullyIdle) {
				taskMap[t.BoundIssueNumber] = t
			}
		}
	}

	var issues []Issue
	for _, r := range rawList {
		labels := make([]string, len(r.Labels))
		for i, l := range r.Labels {
			labels[i] = l.Name
		}
		assignees := make([]string, len(r.Assignees))
		for i, a := range r.Assignees {
			assignees[i] = a.Login
		}

		iss := Issue{
			Number:        r.Number,
			Title:         r.Title,
			Body:          r.Body,
			State:         strings.ToLower(r.State),
			Labels:        labels,
			Assignees:     assignees,
			Author:        r.Author.Login,
			CommentsCount: len(r.Comments),
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
			URL:           r.URL,
			ClosedAt:      r.ClosedAt,
		}

		if agent, ok := taskMap[r.Number]; ok {
			iss.AssignedAgent = &agent
		}

		issues = append(issues, iss)
	}

	return issues, nil
}

// GetIssue retrieves detailed information and comments for a specific issue.
func (s *Service) GetIssue(repo *RepoInfo, number int) (*IssueDetail, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository info required")
	}

	args := []string{"issue", "view", strconv.Itoa(number), "--repo", repo.FullName,
		"--json", "number,title,body,state,labels,assignees,author,comments,createdAt,updatedAt,url,closedAt"}
	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh issue view failed: %s: %w", string(out), err)
	}

	type ghLabel struct {
		Name string `json:"name"`
	}
	type ghUser struct {
		Login string `json:"login"`
	}
	type rawComment struct {
		ID        string    `json:"id"`
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"createdAt"`
		Author    ghUser    `json:"author"`
	}
	type rawDetail struct {
		Number    int          `json:"number"`
		Title     string       `json:"title"`
		Body      string       `json:"body"`
		State     string       `json:"state"`
		Labels    []ghLabel    `json:"labels"`
		Assignees []ghUser     `json:"assignees"`
		Author    ghUser       `json:"author"`
		Comments  []rawComment `json:"comments"`
		CreatedAt time.Time    `json:"createdAt"`
		UpdatedAt time.Time    `json:"updatedAt"`
		URL       string       `json:"url"`
		ClosedAt  *time.Time   `json:"closedAt"`
	}

	var r rawDetail
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("failed to parse gh issue view output: %w", err)
	}

	labels := make([]string, len(r.Labels))
	for i, l := range r.Labels {
		labels[i] = l.Name
	}
	assignees := make([]string, len(r.Assignees))
	for i, a := range r.Assignees {
		assignees[i] = a.Login
	}

	comments := make([]IssueComment, len(r.Comments))
	for i, c := range r.Comments {
		comments[i] = IssueComment{
			Author:    c.Author.Login,
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
		}
	}

	detail := &IssueDetail{
		Issue: Issue{
			Number:        r.Number,
			Title:         r.Title,
			Body:          r.Body,
			State:         strings.ToLower(r.State),
			Labels:        labels,
			Assignees:     assignees,
			Author:        r.Author.Login,
			CommentsCount: len(r.Comments),
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
			URL:           r.URL,
			ClosedAt:      r.ClosedAt,
		},
		Comments: comments,
	}

	return detail, nil
}

// CreateIssue creates a new issue in the repository.
func (s *Service) CreateIssue(repo *RepoInfo, req CreateIssueRequest) (*Issue, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository info required")
	}

	args := []string{"issue", "create", "--repo", repo.FullName, "--title", req.Title, "--body", req.Body}
	for _, l := range req.Labels {
		args = append(args, "--label", l)
	}
	for _, a := range req.Assignees {
		args = append(args, "--assignee", a)
	}

	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh issue create failed: %s: %w", string(out), err)
	}

	// Output contains the created URL e.g. https://github.com/owner/repo/issues/12
	urlStr := strings.TrimSpace(string(out))
	parts := strings.Split(urlStr, "/")
	num, _ := strconv.Atoi(parts[len(parts)-1])

	return &Issue{
		Number:    num,
		Title:     req.Title,
		Body:      req.Body,
		State:     "open",
		Labels:    req.Labels,
		Assignees: req.Assignees,
		URL:       urlStr,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// UpdateIssue modifies an existing issue in-place (title, body, state, labels).
func (s *Service) UpdateIssue(repo *RepoInfo, number int, req UpdateIssueRequest) (*Issue, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository info required")
	}

	numStr := strconv.Itoa(number)

	// 1. Edit metadata
	editArgs := []string{"issue", "edit", numStr, "--repo", repo.FullName}
	needsEdit := false
	if req.Title != nil {
		editArgs = append(editArgs, "--title", *req.Title)
		needsEdit = true
	}
	if req.Body != nil {
		editArgs = append(editArgs, "--body", *req.Body)
		needsEdit = true
	}
	for _, l := range req.AddLabels {
		editArgs = append(editArgs, "--add-label", l)
		needsEdit = true
	}
	for _, l := range req.RemoveLabels {
		editArgs = append(editArgs, "--remove-label", l)
		needsEdit = true
	}
	for _, a := range req.AddAssignees {
		editArgs = append(editArgs, "--add-assignee", a)
		needsEdit = true
	}

	if needsEdit {
		cmd := exec.Command("gh", editArgs...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("gh issue edit failed: %s: %w", string(out), err)
		}
	}

	// 2. Change state if requested
	if req.State != nil {
		st := strings.ToLower(*req.State)
		if st == "closed" {
			cmd := exec.Command("gh", "issue", "close", numStr, "--repo", repo.FullName)
			if out, err := cmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("gh issue close failed: %s: %w", string(out), err)
			}
		} else if st == "open" {
			cmd := exec.Command("gh", "issue", "reopen", numStr, "--repo", repo.FullName)
			if out, err := cmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("gh issue reopen failed: %s: %w", string(out), err)
			}
		}
	}

	// 3. Return refreshed issue
	detail, err := s.GetIssue(repo, number)
	if err != nil {
		return nil, err
	}
	return &detail.Issue, nil
}

// AddIssueComment adds a comment to an issue or pull request.
func (s *Service) AddIssueComment(repo *RepoInfo, number int, body string) error {
	if repo == nil {
		return fmt.Errorf("repository info required")
	}

	cmd := exec.Command("gh", "issue", "comment", strconv.Itoa(number), "--repo", repo.FullName, "--body", body)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh issue comment failed: %s: %w", string(out), err)
	}
	return nil
}

// ListPullRequests retrieves pull requests for the repository.
func (s *Service) ListPullRequests(repo *RepoInfo, state string) ([]PullRequest, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository info required")
	}

	st := strings.ToLower(state)
	if st == "" || (st != "open" && st != "closed" && st != "merged" && st != "all") {
		st = "all"
	}

	args := []string{"pr", "list", "--repo", repo.FullName, "--state", st, "--limit", "50",
		"--json", "number,title,body,state,isDraft,headRefName,baseRefName,author,labels,assignees,reviewDecision,createdAt,updatedAt,url,mergedAt"}

	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh pr list failed: %s: %w", string(out), err)
	}

	type ghLabel struct {
		Name string `json:"name"`
	}
	type ghUser struct {
		Login string `json:"login"`
	}
	type rawPR struct {
		Number         int        `json:"number"`
		Title          string     `json:"title"`
		Body           string     `json:"body"`
		State          string     `json:"state"`
		IsDraft        bool       `json:"isDraft"`
		HeadRefName    string     `json:"headRefName"`
		BaseRefName    string     `json:"baseRefName"`
		Author         ghUser     `json:"author"`
		Labels         []ghLabel  `json:"labels"`
		Assignees      []ghUser   `json:"assignees"`
		ReviewDecision string     `json:"reviewDecision"`
		CreatedAt      time.Time  `json:"createdAt"`
		UpdatedAt      time.Time  `json:"updatedAt"`
		URL            string     `json:"url"`
		MergedAt       *time.Time `json:"mergedAt"`
	}

	var rawList []rawPR
	if err := json.Unmarshal(out, &rawList); err != nil {
		return nil, fmt.Errorf("failed to parse gh pr list output: %w", err)
	}

	var prs []PullRequest
	for _, r := range rawList {
		labels := make([]string, len(r.Labels))
		for i, l := range r.Labels {
			labels[i] = l.Name
		}
		assignees := make([]string, len(r.Assignees))
		for i, a := range r.Assignees {
			assignees[i] = a.Login
		}

		prs = append(prs, PullRequest{
			Number:         r.Number,
			Title:          r.Title,
			Body:           r.Body,
			State:          r.State,
			IsDraft:        r.IsDraft,
			HeadRef:        r.HeadRefName,
			BaseRef:        r.BaseRefName,
			Author:         r.Author.Login,
			Labels:         labels,
			Assignees:      assignees,
			ReviewDecision: r.ReviewDecision,
			CreatedAt:      r.CreatedAt,
			UpdatedAt:      r.UpdatedAt,
			URL:            r.URL,
			MergedAt:       r.MergedAt,
		})
	}

	return prs, nil
}

// GetPullRequest retrieves full pull request details and comments.
func (s *Service) GetPullRequest(repo *RepoInfo, number int) (*PullRequestDetail, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository info required")
	}

	args := []string{"pr", "view", strconv.Itoa(number), "--repo", repo.FullName,
		"--json", "number,title,body,state,isDraft,headRefName,baseRefName,author,labels,assignees,reviewDecision,comments,createdAt,updatedAt,url,mergedAt"}

	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh pr view failed: %s: %w", string(out), err)
	}

	type ghLabel struct {
		Name string `json:"name"`
	}
	type ghUser struct {
		Login string `json:"login"`
	}
	type rawComment struct {
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"createdAt"`
		Author    ghUser    `json:"author"`
	}
	type rawPRDetail struct {
		Number         int          `json:"number"`
		Title          string       `json:"title"`
		Body           string       `json:"body"`
		State          string       `json:"state"`
		IsDraft        bool         `json:"isDraft"`
		HeadRefName    string       `json:"headRefName"`
		BaseRefName    string       `json:"baseRefName"`
		Author         ghUser       `json:"author"`
		Labels         []ghLabel    `json:"labels"`
		Assignees      []ghUser     `json:"assignees"`
		ReviewDecision string       `json:"reviewDecision"`
		Comments       []rawComment `json:"comments"`
		CreatedAt      time.Time    `json:"createdAt"`
		UpdatedAt      time.Time    `json:"updatedAt"`
		URL            string       `json:"url"`
		MergedAt       *time.Time   `json:"mergedAt"`
	}

	var r rawPRDetail
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("failed to parse gh pr view output: %w", err)
	}

	labels := make([]string, len(r.Labels))
	for i, l := range r.Labels {
		labels[i] = l.Name
	}
	assignees := make([]string, len(r.Assignees))
	for i, a := range r.Assignees {
		assignees[i] = a.Login
	}
	comments := make([]IssueComment, len(r.Comments))
	for i, c := range r.Comments {
		comments[i] = IssueComment{
			Author:    c.Author.Login,
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
		}
	}

	return &PullRequestDetail{
		PullRequest: PullRequest{
			Number:         r.Number,
			Title:          r.Title,
			Body:           r.Body,
			State:          r.State,
			IsDraft:        r.IsDraft,
			HeadRef:        r.HeadRefName,
			BaseRef:        r.BaseRefName,
			Author:         r.Author.Login,
			Labels:         labels,
			Assignees:      assignees,
			ReviewDecision: r.ReviewDecision,
			CreatedAt:      r.CreatedAt,
			UpdatedAt:      r.UpdatedAt,
			URL:            r.URL,
			MergedAt:       r.MergedAt,
		},
		Comments: comments,
	}, nil
}

// ListProjects retrieves GitHub Projects (V2 or classic).
func (s *Service) ListProjects(repo *RepoInfo) ([]ProjectBoard, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository info required")
	}

	// Try owner projects first
	cmd := exec.Command("gh", "project", "list", "--owner", repo.Owner, "--limit", "30",
		"--json", "id,number,title,shortDescription,url,closed")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback to empty project list rather than hard failing if projects are unconfigured
		return []ProjectBoard{}, nil
	}

	var projects []ProjectBoard
	_ = json.Unmarshal(out, &projects)
	return projects, nil
}

// GetAgentTasks returns all active and recent conversation tasks for the workspace.
func (s *Service) GetAgentTasks(workspacePath string) ([]AgentTaskSummary, error) {
	if s.tracker == nil {
		return []AgentTaskSummary{}, nil
	}
	return s.tracker.ListWorkspaceTasks(workspacePath)
}

// BindTask associates a conversation with an issue and sets an agent label.
func (s *Service) BindTask(convID string, issueNumber int, agentLabel string) error {
	if s.store == nil {
		return fmt.Errorf("store unavailable")
	}
	if err := s.store.BindIssue(convID, issueNumber); err != nil {
		return err
	}
	if agentLabel != "" {
		if err := s.store.SetAgentLabel(convID, agentLabel); err != nil {
			return err
		}
	}
	return nil
}

// SetAgentLabel assigns a custom label/role to an agent conversation.
func (s *Service) SetAgentLabel(convID string, label string) error {
	if s.store == nil {
		return fmt.Errorf("store unavailable")
	}
	return s.store.SetAgentLabel(convID, label)
}

// FormatContextForChat produces structured markdown suitable for dragging or injecting into agent chat.
func (s *Service) FormatContextForChat(itemType string, number int, repo *RepoInfo) (string, error) {
	var buf bytes.Buffer

	if itemType == "pr" {
		pr, err := s.GetPullRequest(repo, number)
		if err != nil {
			return "", err
		}
		buf.WriteString(fmt.Sprintf("### GitHub Pull Request #%d: %s\n", pr.Number, pr.Title))
		buf.WriteString(fmt.Sprintf("- **Repository:** %s\n", repo.FullName))
		buf.WriteString(fmt.Sprintf("- **URL:** %s\n", pr.URL))
		buf.WriteString(fmt.Sprintf("- **State:** %s (Branch: `%s` -> `%s`)\n", pr.State, pr.HeadRef, pr.BaseRef))
		buf.WriteString(fmt.Sprintf("- **Author:** @%s\n", pr.Author))
		if len(pr.Labels) > 0 {
			buf.WriteString(fmt.Sprintf("- **Labels:** %s\n", strings.Join(pr.Labels, ", ")))
		}
		buf.WriteString("\n#### PR Summary:\n")
		if strings.TrimSpace(pr.Body) != "" {
			buf.WriteString(pr.Body + "\n")
		} else {
			buf.WriteString("(No description provided)\n")
		}
		return buf.String(), nil
	}

	issue, err := s.GetIssue(repo, number)
	if err != nil {
		return "", err
	}

	buf.WriteString(fmt.Sprintf("### GitHub Issue #%d: %s\n", issue.Number, issue.Title))
	buf.WriteString(fmt.Sprintf("- **Repository:** %s\n", repo.FullName))
	buf.WriteString(fmt.Sprintf("- **URL:** %s\n", issue.URL))
	buf.WriteString(fmt.Sprintf("- **State:** %s\n", issue.State))
	buf.WriteString(fmt.Sprintf("- **Author:** @%s\n", issue.Author))
	if len(issue.Labels) > 0 {
		buf.WriteString(fmt.Sprintf("- **Labels:** %s\n", strings.Join(issue.Labels, ", ")))
	}
	if len(issue.Assignees) > 0 {
		buf.WriteString(fmt.Sprintf("- **Assignees:** @%s\n", strings.Join(issue.Assignees, ", @")))
	}
	buf.WriteString("\n#### Issue Description:\n")
	if strings.TrimSpace(issue.Body) != "" {
		buf.WriteString(issue.Body + "\n")
	} else {
		buf.WriteString("(No description provided)\n")
	}

	if len(issue.Comments) > 0 {
		buf.WriteString("\n#### Recent Comments:\n")
		limit := 3
		if len(issue.Comments) < limit {
			limit = len(issue.Comments)
		}
		for _, c := range issue.Comments[len(issue.Comments)-limit:] {
			buf.WriteString(fmt.Sprintf("- **@%s** (%s):\n> %s\n\n", c.Author, c.CreatedAt.Format("2006-01-02 15:04"), strings.ReplaceAll(c.Body, "\n", "\n> ")))
		}
	}

	return buf.String(), nil
}
