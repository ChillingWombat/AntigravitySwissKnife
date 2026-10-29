package webgui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/bridge"
)

// handleA2AAgentCard serves the discoverable A2A Agent Card descriptor.
func (s *Server) handleA2AAgentCard(w http.ResponseWriter, r *http.Request) {
	bridgeSvc := s.getBridgeService()
	var gatewayURL string
	if bridgeSvc != nil {
		gatewayURL = bridgeSvc.GetStatus().GatewayURL
	}
	baseURL := bridge.ResolveExternalBaseURL(r, gatewayURL)

	var card bridge.A2AAgentCard
	if bridgeSvc != nil {
		card = bridgeSvc.AgentCard(baseURL)
	} else {
		card = bridge.DefaultA2AAgentCard(baseURL)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	writeJSON(w, card)
}

// handleA2ATasks handles task creation, status polling, cancellation, and SSE events.
func (s *Server) handleA2ATasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Swiss-KeepAlive")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	bridgeSvc := s.getBridgeService()
	if bridgeSvc == nil {
		http.Error(w, `{"error":"bridge service not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	// Authenticate request using Bridge AuthManager (Bearer token, pairing PIN, or app password)
	granted, reason := bridgeSvc.ValidateRequest(r)
	if !granted {
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="Antigravity Swiss Knife A2A", resource_metadata="%s/.well-known/agent.json"`, bridge.ResolveExternalBaseURL(r, bridgeSvc.GetStatus().GatewayURL)))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "unauthorized",
			"message": reason,
		})
		return
	}

	taskMgr := bridgeSvc.A2ATaskManager()
	if taskMgr == nil {
		http.Error(w, `{"error":"A2A task manager not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/bridge")
	path = strings.TrimPrefix(path, "/a2a/tasks")
	path = strings.Trim(path, "/")

	// If path is empty, it's either POST /tasks (create) or GET /tasks (list)
	if path == "" {
		if r.Method == http.MethodPost {
			var payload struct {
				Skill  string `json:"skill"`
				Prompt string `json:"prompt"`
				Title  string `json:"title"`
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(payload.Prompt) == "" {
				http.Error(w, `{"error":"prompt is required"}`, http.StatusBadRequest)
				return
			}

			task, err := taskMgr.CreateTask(payload.Skill, payload.Prompt, payload.Title)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, task)
			return
		}

		if r.Method == http.MethodGet {
			status := r.URL.Query().Get("status")
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			tasks := taskMgr.ListTasks(status, limit)
			w.Header().Set("Content-Type", "application/json")
			writeJSON(w, map[string]interface{}{
				"tasks": tasks,
			})
			return
		}

		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Sub-paths: {id}, {id}/send, {id}/cancel, {id}/events
	parts := strings.Split(path, "/")
	taskID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "":
		// GET /a2a/tasks/{id}
		if r.Method == http.MethodGet {
			task, err := taskMgr.GetTask(taskID)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			writeJSON(w, task)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)

	case "send", "messages":
		// POST /a2a/tasks/{id}/send
		if r.Method == http.MethodPost {
			var payload struct {
				Message string `json:"message"`
			}
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &payload)
			if strings.TrimSpace(payload.Message) == "" {
				http.Error(w, `{"error":"message is required"}`, http.StatusBadRequest)
				return
			}
			task, err := taskMgr.SendTaskMessage(taskID, payload.Message)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			writeJSON(w, task)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)

	case "cancel":
		// POST /a2a/tasks/{id}/cancel
		if r.Method == http.MethodPost {
			task, err := taskMgr.CancelTask(taskID)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			writeJSON(w, task)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)

	case "events":
		// GET /a2a/tasks/{id}/events (Server-Sent Events)
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		events, unsub := taskMgr.SubscribeTaskEvents(taskID)
		defer unsub()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				data, _ := json.Marshal(ev)
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, string(data))
				flusher.Flush()
				if ev.Type == "task.completed" || ev.Type == "task.failed" || ev.Type == "task.canceled" {
					return
				}
			}
		}

	default:
		http.Error(w, `{"error":"unknown action"}`, http.StatusNotFound)
	}
}
