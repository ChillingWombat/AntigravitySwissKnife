// Package ls drives the Antigravity language server, the agent runtime shared by
// the desktop app and the VS Code extension. It speaks Connect RPC over loopback
// HTTP, guarded by the CSRF token the server carries in its argv.
package ls

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface"
)

const servicePath = "/exa.language_server_pb.LanguageServerService/"

// turnTimeout bounds one turn so a stuck server cannot hang the editor.
const turnTimeout = 10 * time.Minute

// DefaultPlanModel is the model the language server expects when the client does
// not choose one. The field is required in practice: without it turns are ignored.
const DefaultPlanModel = "MODEL_PLACEHOLDER_M319"

// Surface talks to one running language server.
type Surface struct {
	BaseURL string // e.g. http://127.0.0.1:33103
	CSRF    string
	Client  *http.Client

	mu        sync.Mutex
	sessions  map[string]surface.SessionOptions
	offsets   map[string]uint32
	cancelled map[string]bool
}

func New(baseURL, csrf string) *Surface {
	return &Surface{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		CSRF:      csrf,
		Client:    &http.Client{Timeout: 60 * time.Second},
		sessions:  map[string]surface.SessionOptions{},
		offsets:   map[string]uint32{},
		cancelled: map[string]bool{},
	}
}

func (s *Surface) Name() string { return "ls" }

// Call invokes one Connect RPC and decodes the JSON reply into out.
func (s *Surface) Call(ctx context.Context, method string, body, out interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+servicePath+method, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-codeium-csrf-token", s.CSRF)
	resp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var failure bytes.Buffer
		_, _ = failure.ReadFrom(resp.Body)
		return fmt.Errorf("%s: HTTP %d: %s", method, resp.StatusCode, strings.TrimSpace(failure.String()))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (s *Surface) Start(ctx context.Context, opts surface.SessionOptions) (string, error) {
	workspace := opts.Cwd
	if workspace == "" {
		workspace = "."
	}
	var reply struct {
		CascadeID string `json:"cascadeId"`
	}
	err := s.Call(ctx, "StartCascade", map[string]interface{}{
		"source":         "CORTEX_TRAJECTORY_SOURCE_AGENT_API",
		"trajectoryType": "CORTEX_TRAJECTORY_TYPE_CASCADE",
		"workspaceUris":  []string{workspaceURI(workspace)},
	}, &reply)
	if err != nil {
		return "", err
	}
	if reply.CascadeID == "" {
		return "", fmt.Errorf("StartCascade returned no cascadeId")
	}
	s.mu.Lock()
	s.sessions[reply.CascadeID] = opts
	s.offsets[reply.CascadeID] = 0
	s.mu.Unlock()
	return reply.CascadeID, nil
}

type step struct {
	Type            string `json:"type"`
	PlannerResponse *struct {
		Response string `json:"response"`
	} `json:"plannerResponse"`
	ErrorMessage *struct {
		Error struct {
			ShortError       string `json:"shortError"`
			UserErrorMessage string `json:"userErrorMessage"`
		} `json:"error"`
		Message string `json:"message"`
	} `json:"errorMessage"`
}

func (s *Surface) Send(ctx context.Context, sessionID, prompt string, emit func(surface.Event) error) (surface.StopReason, error) {
	body := map[string]interface{}{
		"cascadeId": sessionID,
		"items":     []map[string]interface{}{{"text": prompt}},
	}
	model := s.modelFor(sessionID)
	if model == "" {
		model = DefaultPlanModel
	}
	body["cascadeConfig"] = map[string]interface{}{
		"plannerConfig": map[string]interface{}{"planModel": model},
	}
	s.mu.Lock()
	s.cancelled[sessionID] = false
	s.mu.Unlock()
	if err := s.seedOffset(ctx, sessionID); err != nil {
		return "", err
	}
	if err := s.Call(ctx, "SendUserCascadeMessage", body, nil); err != nil {
		return "", err
	}

	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	quietTicks := 0
	sawText := false
	deadline := time.Now().Add(turnTimeout)
	for {
		if time.Now().After(deadline) {
			_ = s.Cancel(context.Background(), sessionID)
			return "", fmt.Errorf("turn did not finish within %s", turnTimeout)
		}
		select {
		case <-ctx.Done():
			_ = s.Cancel(context.Background(), sessionID)
			return surface.StopCancelled, nil
		case <-ticker.C:
			if quietTicks >= 4 && sawText {
				idle, idleErr := s.isIdle(ctx, sessionID)
				if idleErr != nil {
					return "", idleErr
				}
				if idle {
					return surface.StopEndTurn, nil
				}
			}
			s.mu.Lock()
			stopped := s.cancelled[sessionID]
			s.mu.Unlock()
			if stopped {
				return surface.StopCancelled, nil
			}
			done, emitted, err := s.pollSteps(ctx, sessionID, emit)
			if err != nil {
				return "", err
			}
			if emitted > 0 {
				sawText = true
				quietTicks = 0
			} else {
				quietTicks++
			}
			if done {
				return surface.StopEndTurn, nil
			}
		}
	}
}

// pollSteps reads steps past the last offset and emits new planner text.
func (s *Surface) pollSteps(ctx context.Context, sessionID string, emit func(surface.Event) error) (bool, int, error) {
	s.mu.Lock()
	offset := s.offsets[sessionID]
	s.mu.Unlock()

	var reply struct {
		Steps      []step `json:"steps"`
		Trajectory *struct {
			Steps []step `json:"steps"`
		} `json:"trajectory"`
	}
	err := s.Call(ctx, "GetCascadeTrajectorySteps", map[string]interface{}{
		"cascadeId":  sessionID,
		"stepOffset": offset,
	}, &reply)
	if len(reply.Steps) == 0 && reply.Trajectory != nil {
		reply.Steps = reply.Trajectory.Steps
	}
	if err != nil {
		return false, 0, err
	}
	if len(reply.Steps) == 0 {
		return false, 0, nil
	}
	done := false
	emitted := 0
	for _, st := range reply.Steps {
		offset++
		switch {
		case st.PlannerResponse != nil && st.PlannerResponse.Response != "":
			if err := emit(surface.Event{Kind: surface.EventText, Text: st.PlannerResponse.Response}); err != nil {
				return false, emitted, err
			}
			emitted++
		case st.ErrorMessage != nil && errorText(st) != "":
			if err := emit(surface.Event{Kind: surface.EventError, Message: errorText(st)}); err != nil {
				return false, emitted, err
			}
			emitted++
			done = true
		}
	}
	s.mu.Lock()
	s.offsets[sessionID] = offset
	s.mu.Unlock()
	if done {
		return true, emitted, nil
	}
	// A planner response can arrive mid-turn while tools keep running, so the turn
	// ends when the cascade goes idle, not when text appears.
	idle, statusErr := s.isIdle(ctx, sessionID)
	if statusErr != nil {
		return false, emitted, statusErr
	}
	return idle && len(reply.Steps) == 0, emitted, nil
}

func (s *Surface) modelFor(sessionID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionID].Model
}

func (s *Surface) Cancel(ctx context.Context, sessionID string) error {
	return s.Call(ctx, "CancelCascadeInvocation", map[string]interface{}{
		"cascadeId":           sessionID,
		"killBackgroundTasks": true,
	}, nil)
}

func (s *Surface) List(ctx context.Context) ([]surface.SessionInfo, error) {
	// The server answers with a map keyed by cascade id, not a list.
	var reply struct {
		Summaries map[string]struct {
			StepCount        int      `json:"stepCount"`
			LastModifiedTime string   `json:"lastModifiedTime"`
			CreatedTime      string   `json:"createdTime"`
			Status           string   `json:"status"`
			WorkspaceURI     string   `json:"workspaceUri"`
			WorkspaceURIs    []string `json:"workspaceUris"`
		} `json:"trajectorySummaries"`
	}
	if err := s.Call(ctx, "GetAllCascadeTrajectories", map[string]interface{}{}, &reply); err != nil {
		return nil, err
	}
	out := make([]surface.SessionInfo, 0, len(reply.Summaries))
	for id, summary := range reply.Summaries {
		cwd := cwdFromWorkspaceURIs(summary.WorkspaceURIs)
		if cwd == "" {
			cwd = cwdFromWorkspaceURI(summary.WorkspaceURI)
		}
		if cwd == "" {
			s.mu.Lock()
			cwd = s.sessions[id].Cwd
			s.mu.Unlock()
		}
		info := surface.SessionInfo{ID: id, Cwd: cwd}
		if stamp, perr := time.Parse(time.RFC3339Nano, summary.LastModifiedTime); perr == nil {
			info.UpdatedAt = stamp.Unix()
		}
		out = append(out, info)
	}
	return out, nil
}

func cwdFromWorkspaceURIs(uris []string) string {
	for _, uri := range uris {
		if cwd := cwdFromWorkspaceURI(uri); cwd != "" {
			return cwd
		}
	}
	return ""
}

func cwdFromWorkspaceURI(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "file" {
		return ""
	}
	cwd := parsed.Path
	if parsed.Host != "" && parsed.Host != "localhost" {
		cwd = "//" + parsed.Host + cwd
	}
	if runtime.GOOS == "windows" {
		if parsed.Host != "" && parsed.Host != "localhost" {
			cwd = filepath.FromSlash(cwd)
		} else if len(cwd) >= 3 && cwd[0] == '/' && cwd[2] == ':' {
			cwd = filepath.FromSlash(cwd[1:])
		}
	} else {
		cwd = filepath.FromSlash(cwd)
	}
	return cwd
}

// History replays the planner responses already recorded in a cascade.
func (s *Surface) History(ctx context.Context, sessionID string) ([]surface.Event, error) {
	var reply struct {
		Steps      []step `json:"steps"`
		Trajectory *struct {
			Steps []step `json:"steps"`
		} `json:"trajectory"`
	}
	if err := s.Call(ctx, "GetCascadeTrajectorySteps", map[string]interface{}{"cascadeId": sessionID, "stepOffset": 0}, &reply); err != nil {
		return nil, err
	}
	steps := reply.Steps
	if len(steps) == 0 && reply.Trajectory != nil {
		steps = reply.Trajectory.Steps
	}
	events := make([]surface.Event, 0, len(steps))
	for _, st := range steps {
		if st.PlannerResponse != nil && st.PlannerResponse.Response != "" {
			events = append(events, surface.Event{Kind: surface.EventText, Text: st.PlannerResponse.Response})
		}
	}
	return events, nil
}

// SetModel changes the planner model used by the next turn of a session.
func (s *Surface) SetModel(ctx context.Context, sessionID, model string) error {
	s.mu.Lock()
	opts := s.sessions[sessionID]
	opts.Model = model
	s.sessions[sessionID] = opts
	s.mu.Unlock()
	return nil
}

func (s *Surface) Close(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	delete(s.offsets, sessionID)
	s.mu.Unlock()
	return nil
}

// Discover reads the live server's address from its log and its CSRF token from the
// process argv. The server picks an OS-assigned port, so nothing else records it.
func Discover(logPath string) (baseURL, csrf string, err error) {
	handle, openErr := os.Open(logPath)
	if openErr != nil {
		return "", "", openErr
	}
	defer handle.Close()

	scanner := bufio.NewScanner(handle)
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)
	address := ""
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "listening on random port at "); idx >= 0 {
			// The log line continues with prose ("... for HTTP"), so take the token.
			fields := strings.Fields(line[idx+len("listening on random port at "):])
			if len(fields) > 0 {
				address = strings.TrimSuffix(fields[0], ",")
			}
		}
	}
	if address == "" {
		return "", "", fmt.Errorf("no listening address in %s", logPath)
	}
	address = strings.TrimPrefix(strings.TrimPrefix(address, "http://"), "//")
	if host, port, splitErr := net.SplitHostPort(address); splitErr == nil {
		if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
			address = net.JoinHostPort("127.0.0.1", port)
		}
	} else {
		// A bare port from an older log line.
		address = net.JoinHostPort("127.0.0.1", strings.TrimPrefix(address, ":"))
	}
	token, err := csrfFromProc()
	if err != nil {
		return "", "", err
	}
	return "http://" + strings.TrimSuffix(address, "/"), token, nil
}

// DefaultLogPath is where the desktop app writes the language server log.
func DefaultLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, logDirFromHome(), "logs", "language_server.log")
}

// errorText picks the most useful message out of an error step.
func errorText(st step) string {
	if st.ErrorMessage == nil {
		return ""
	}
	if st.ErrorMessage.Error.UserErrorMessage != "" {
		return st.ErrorMessage.Error.UserErrorMessage
	}
	if st.ErrorMessage.Error.ShortError != "" {
		return st.ErrorMessage.Error.ShortError
	}
	return st.ErrorMessage.Message
}

// seedOffset moves the read cursor past everything already recorded, so a prompt
// after a restart answers the new turn instead of replaying the previous answer.
func (s *Surface) seedOffset(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	seeded := s.offsets[sessionID]
	s.mu.Unlock()
	if seeded > 0 {
		return nil
	}
	count, err := s.stepCount(ctx, sessionID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.offsets[sessionID] = count
	s.mu.Unlock()
	return nil
}

// stepCount reads how many steps a cascade already has.
func (s *Surface) stepCount(ctx context.Context, sessionID string) (uint32, error) {
	var reply struct {
		Summaries map[string]struct {
			StepCount uint32 `json:"stepCount"`
		} `json:"trajectorySummaries"`
	}
	if err := s.Call(ctx, "GetAllCascadeTrajectories", map[string]interface{}{}, &reply); err != nil {
		return 0, err
	}
	return reply.Summaries[sessionID].StepCount, nil
}

// isIdle reports whether the cascade stopped running.
func (s *Surface) isIdle(ctx context.Context, sessionID string) (bool, error) {
	var reply struct {
		Summaries map[string]struct {
			Status string `json:"status"`
		} `json:"trajectorySummaries"`
	}
	if err := s.Call(ctx, "GetAllCascadeTrajectories", map[string]interface{}{}, &reply); err != nil {
		return false, err
	}
	status, ok := reply.Summaries[sessionID]
	if !ok {
		return false, nil
	}
	return status.Status == "CASCADE_RUN_STATUS_IDLE", nil
}

func workspaceURI(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if strings.HasPrefix(cwd, "/") && !strings.HasPrefix(cwd, "//") {
		cwd = path.Clean(cwd)
	} else if abs, err := filepath.Abs(cwd); err == nil {
		cwd = abs
	}
	slashed := strings.ReplaceAll(cwd, `\`, "/")
	if strings.HasPrefix(slashed, "//") {
		trimmed := strings.TrimPrefix(slashed, "//")
		parts := strings.SplitN(trimmed, "/", 2)
		host := parts[0]
		pathPart := "/"
		if len(parts) == 2 && parts[1] != "" {
			pathPart = "/" + parts[1]
		}
		return (&url.URL{Scheme: "file", Host: host, Path: pathPart}).String()
	}
	if len(slashed) >= 2 && ((slashed[0] >= 'A' && slashed[0] <= 'Z') || (slashed[0] >= 'a' && slashed[0] <= 'z')) && slashed[1] == ':' {
		slashed = "/" + slashed
	}
	return (&url.URL{Scheme: "file", Path: slashed}).String()
}
