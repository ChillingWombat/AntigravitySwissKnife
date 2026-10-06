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
		ID interface{} `json:"id"`
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

// GetKanbanBoard returns the Kanban board for the repository.
// If projectNumber > 0 and GitHub Projects v2 is accessible, it loads project items;
// otherwise it synthesizes columns (Todo, In Progress, Review, Done) from issues and PRs.
func (s *Service) GetKanbanBoard(repo *RepoInfo, projectNumber int) (*KanbanBoard, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository info required")
	}

	if projectNumber > 0 {
		board, err := s.getProjectV2Board(repo, projectNumber)
		if err == nil && board != nil && len(board.Columns) > 0 {
			return board, nil
		}
	}

	return s.synthesizeKanbanBoard(repo)
}

// getProjectV2Board attempts to fetch a GitHub Projects v2 board via gh CLI.
func (s *Service) getProjectV2Board(repo *RepoInfo, projectNumber int) (*KanbanBoard, error) {
	if projectNumber <= 0 {
		return nil, fmt.Errorf("invalid project number")
	}

	cmd := exec.Command("gh", "project", "item-list", strconv.Itoa(projectNumber),
		"--owner", repo.Owner, "--format", "json", "--limit", "100")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh project item-list failed: %s: %w", string(out), err)
	}

	type rawProjectContent struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Type   string `json:"type"`
		URL    string `json:"url"`
	}
	type rawProjectItem struct {
		ID      string            `json:"id"`
		Content rawProjectContent `json:"content"`
		Status  string            `json:"status"`
		Title   string            `json:"title"`
		Type    string            `json:"type"`
	}
	type rawProjectResp struct {
		TotalCount int              `json:"totalCount"`
		Items      []rawProjectItem `json:"items"`
	}

	var resp rawProjectResp
	if err := json.Unmarshal(out, &resp); err != nil {
		var arr []rawProjectItem
		if errArr := json.Unmarshal(out, &arr); errArr != nil {
			return nil, fmt.Errorf("failed to parse gh project items: %w", err)
		}
		resp.Items = arr
	}

	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("no project items found")
	}

	colMap := map[string][]KanbanCard{
		"todo":        {},
		"in_progress": {},
		"review":      {},
		"done":        {},
	}

	for _, it := range resp.Items {
		num := it.Content.Number
		title := it.Content.Title
		if title == "" {
			title = it.Title
		}
		itemType := strings.ToLower(it.Content.Type)
		if itemType == "" {
			itemType = strings.ToLower(it.Type)
		}
		cardType := "issue"
		if strings.Contains(itemType, "pr") || strings.Contains(itemType, "pull") {
			cardType = "pr"
		}

		st := strings.ToLower(it.Status)
		targetCol := "todo"
		if strings.Contains(st, "progress") || strings.Contains(st, "doing") || strings.Contains(st, "working") {
			targetCol = "in_progress"
		} else if strings.Contains(st, "review") || strings.Contains(st, "qa") {
			targetCol = "review"
		} else if strings.Contains(st, "done") || strings.Contains(st, "closed") || strings.Contains(st, "completed") {
			targetCol = "done"
		}

		card := KanbanCard{
			ID:            it.ID,
			Type:          cardType,
			Number:        num,
			Title:         title,
			Body:          it.Content.Body,
			ColumnID:      targetCol,
			URL:           it.Content.URL,
			ProjectItemID: it.ID,
		}
		colMap[targetCol] = append(colMap[targetCol], card)
	}

	columns := []KanbanColumn{
		{ID: "todo", Title: "Todo", Cards: colMap["todo"]},
		{ID: "in_progress", Title: "In Progress", Cards: colMap["in_progress"]},
		{ID: "review", Title: "Review", Cards: colMap["review"]},
		{ID: "done", Title: "Done", Cards: colMap["done"]},
	}

	return &KanbanBoard{
		ProjectID:     strconv.Itoa(projectNumber),
		ProjectTitle:  fmt.Sprintf("Project #%d", projectNumber),
		IsSynthesized: false,
		Columns:       columns,
	}, nil
}

// synthesizeKanbanBoard builds Kanban columns from repository Issues, PRs, and live agent tasks.
func (s *Service) synthesizeKanbanBoard(repo *RepoInfo) (*KanbanBoard, error) {
	issues, err := s.ListIssues(repo, "all", "")
	if err != nil {
		return nil, fmt.Errorf("failed to list issues for kanban board: %w", err)
	}

	prs, err := s.ListPullRequests(repo, "all")
	if err != nil {
		prs = []PullRequest{}
	}

	colMap := map[string][]KanbanCard{
		"todo":        make([]KanbanCard, 0),
		"in_progress": make([]KanbanCard, 0),
		"review":      make([]KanbanCard, 0),
		"done":        make([]KanbanCard, 0),
	}

	isWIPLabel := func(label string) bool {
		l := strings.ToLower(strings.TrimSpace(label))
		return l == "in progress" || l == "in-progress" || l == "wip" || l == "doing" || l == "active" || l == "working"
	}
	isReviewLabel := func(label string) bool {
		l := strings.ToLower(strings.TrimSpace(label))
		return l == "review" || l == "in review" || l == "in-review" || l == "needs review" || l == "under review" || l == "qa"
	}

	// 1. Process Issues
	for _, iss := range issues {
		colID := "todo"
		if strings.EqualFold(iss.State, "closed") {
			colID = "done"
		} else {
			hasWIP := false
			hasReview := false
			for _, lbl := range iss.Labels {
				if isWIPLabel(lbl) {
					hasWIP = true
				}
				if isReviewLabel(lbl) {
					hasReview = true
				}
			}

			if iss.AssignedAgent != nil {
				colID = "in_progress"
			} else if hasWIP {
				colID = "in_progress"
			} else if hasReview {
				colID = "review"
			} else {
				colID = "todo"
			}
		}

		card := KanbanCard{
			ID:            fmt.Sprintf("issue-%d", iss.Number),
			Type:          "issue",
			Number:        iss.Number,
			Title:         iss.Title,
			Body:          iss.Body,
			State:         strings.ToLower(iss.State),
			ColumnID:      colID,
			Labels:        iss.Labels,
			Assignees:     iss.Assignees,
			Author:        iss.Author,
			URL:           iss.URL,
			CreatedAt:     iss.CreatedAt,
			UpdatedAt:     iss.UpdatedAt,
			AssignedAgent: iss.AssignedAgent,
		}
		colMap[colID] = append(colMap[colID], card)
	}

	// 2. Process Pull Requests
	for _, pr := range prs {
		colID := "review"
		if strings.EqualFold(pr.State, "closed") || strings.EqualFold(pr.State, "merged") {
			colID = "done"
		} else if pr.IsDraft {
			colID = "in_progress"
		} else {
			hasWIP := false
			for _, lbl := range pr.Labels {
				if isWIPLabel(lbl) {
					hasWIP = true
					break
				}
			}
			if hasWIP {
				colID = "in_progress"
			} else {
				colID = "review"
			}
		}

		card := KanbanCard{
			ID:            fmt.Sprintf("pr-%d", pr.Number),
			Type:          "pr",
			Number:        pr.Number,
			Title:         pr.Title,
			Body:          pr.Body,
			State:         strings.ToLower(pr.State),
			ColumnID:      colID,
			Labels:        pr.Labels,
			Assignees:     pr.Assignees,
			Author:        pr.Author,
			URL:           pr.URL,
			CreatedAt:     pr.CreatedAt,
			UpdatedAt:     pr.UpdatedAt,
			AssignedAgent: pr.AssignedAgent,
		}
		colMap[colID] = append(colMap[colID], card)
	}

	columns := []KanbanColumn{
		{ID: "todo", Title: "Todo", Cards: colMap["todo"]},
		{ID: "in_progress", Title: "In Progress", Cards: colMap["in_progress"]},
		{ID: "review", Title: "Review", Cards: colMap["review"]},
		{ID: "done", Title: "Done", Cards: colMap["done"]},
	}

	return &KanbanBoard{
		ProjectTitle:  "Repository Kanban Board",
		IsSynthesized: true,
		Columns:       columns,
	}, nil
}

// MoveKanbanCard updates a card's status and moves it to the target column.
func (s *Service) MoveKanbanCard(repo *RepoInfo, req *MoveKanbanCardRequest) error {
	if repo == nil {
		return fmt.Errorf("repository info required")
	}
	if req == nil || req.Number <= 0 {
		return fmt.Errorf("valid card number required")
	}

	targetCol := strings.ToLower(strings.TrimSpace(req.TargetColumn))
	if targetCol == "" {
		return fmt.Errorf("target column required")
	}
	if targetCol != "todo" && targetCol != "in_progress" && targetCol != "review" && targetCol != "done" {
		return fmt.Errorf("invalid target column: %s", targetCol)
	}

	cardType := strings.ToLower(strings.TrimSpace(req.CardType))
	if cardType == "" {
		if strings.HasPrefix(req.CardID, "pr-") {
			cardType = "pr"
		} else {
			cardType = "issue"
		}
	}

	// 1. If project item exists, try updating project item status via gh CLI
	if req.ProjectNumber > 0 && req.ProjectItemID != "" {
		statusValue := "Todo"
		switch targetCol {
		case "in_progress":
			statusValue = "In Progress"
		case "review":
			statusValue = "Review"
		case "done":
			statusValue = "Done"
		}
		cmdEdit := exec.Command("gh", "project", "item-edit", "--id", req.ProjectItemID,
			"--text", statusValue)
		_ = cmdEdit.Run()
	}

	// 2. Handle PR updates
	if cardType == "pr" {
		switch targetCol {
		case "done":
			cmd := exec.Command("gh", "pr", "close", strconv.Itoa(req.Number), "--repo", repo.FullName)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to close PR: %s: %w", string(out), err)
			}
		case "review":
			cmd := exec.Command("gh", "pr", "ready", strconv.Itoa(req.Number), "--repo", repo.FullName)
			_ = cmd.Run()
		case "in_progress":
			cmd := exec.Command("gh", "pr", "ready", "--undo", strconv.Itoa(req.Number), "--repo", repo.FullName)
			_ = cmd.Run()
		}
		return nil
	}

	// 3. Handle Issue updates
	iss, err := s.GetIssue(repo, req.Number)
	if err != nil {
		return fmt.Errorf("failed to get issue #%d: %w", req.Number, err)
	}

	hasLabel := func(list []string, name string) bool {
		for _, l := range list {
			if strings.EqualFold(strings.TrimSpace(l), strings.TrimSpace(name)) {
				return true
			}
		}
		return false
	}

	wipLabels := []string{"in progress", "in-progress", "wip", "doing"}
	reviewLabels := []string{"review", "in review", "in-review", "needs review", "under review"}

	var toRemove []string
	var toAdd []string
	var targetState *string

	isOpen := strings.EqualFold(iss.State, "open")

	switch targetCol {
	case "done":
		if isOpen {
			st := "closed"
			targetState = &st
		}
		for _, l := range append(wipLabels, reviewLabels...) {
			if hasLabel(iss.Labels, l) {
				toRemove = append(toRemove, l)
			}
		}

	case "todo":
		if !isOpen {
			st := "open"
			targetState = &st
		}
		for _, l := range append(wipLabels, reviewLabels...) {
			if hasLabel(iss.Labels, l) {
				toRemove = append(toRemove, l)
			}
		}

	case "in_progress":
		if !isOpen {
			st := "open"
			targetState = &st
		}
		for _, l := range reviewLabels {
			if hasLabel(iss.Labels, l) {
				toRemove = append(toRemove, l)
			}
		}
		if !hasLabel(iss.Labels, "in progress") && !hasLabel(iss.Labels, "wip") {
			toAdd = append(toAdd, "in progress")
		}

	case "review":
		if !isOpen {
			st := "open"
			targetState = &st
		}
		for _, l := range wipLabels {
			if hasLabel(iss.Labels, l) {
				toRemove = append(toRemove, l)
			}
		}
		if !hasLabel(iss.Labels, "in review") && !hasLabel(iss.Labels, "review") {
			toAdd = append(toAdd, "in review")
		}
	}

	_, err = s.UpdateIssue(repo, req.Number, UpdateIssueRequest{
		State:        targetState,
		AddLabels:    toAdd,
		RemoveLabels: toRemove,
	})
	return err
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
