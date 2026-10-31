package custommodels

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultVertexStreamIdleTimeout = 45 * time.Second

// runVertex executes Gemini requests against Vertex AI with the service-account
// credential managed by the custom model store.
func (g *Gateway) runVertex(r *http.Request, model *CustomModel, request *geminiRequest, emit func(geminiResponse) error) error {
	key, err := ParseServiceAccountKey([]byte(model.GCPServiceAccountKey))
	if err != nil {
		return fmt.Errorf("invalid GCP service account for model %q: %w", model.ID, err)
	}
	endpoint, err := vertexStreamEndpoint(model, key)
	if err != nil {
		return err
	}
	if _, _, err := ValidateEndpointSSRF(endpoint, ProviderGCP, g.AllowLoopback); err != nil {
		return fmt.Errorf("reject Vertex endpoint: %w", err)
	}
	accessToken, err := GenerateGCPAccessToken(r.Context(), key)
	if err != nil {
		return fmt.Errorf("refresh GCP access token: %w", err)
	}
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode Vertex request: %w", err)
	}
	idleTimeout := g.vertexStreamIdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = defaultVertexStreamIdleTimeout
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	idleTimer := time.AfterFunc(idleTimeout, cancel)
	defer idleTimer.Stop()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build Vertex request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil && r.Context().Err() == nil {
			return fmt.Errorf("Vertex stream made no progress for %s", idleTimeout)
		}
		return fmt.Errorf("call Vertex: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("Vertex returned %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}
	idleTimer.Reset(idleTimeout)
	err = consumeVertexSSE(&idleReadCloser{
		ReadCloser: resp.Body,
		timer:      idleTimer,
		timeout:    idleTimeout,
	}, emit)
	if err != nil && ctx.Err() != nil && r.Context().Err() == nil {
		return fmt.Errorf("Vertex stream made no progress for %s", idleTimeout)
	}
	return err
}

type idleReadCloser struct {
	io.ReadCloser
	timer   *time.Timer
	timeout time.Duration
}

func (r *idleReadCloser) Read(p []byte) (int, error) {
	r.timer.Reset(r.timeout)
	return r.ReadCloser.Read(p)
}

func vertexStreamEndpoint(model *CustomModel, key *ServiceAccountKey) (string, error) {
	projectID := strings.TrimSpace(model.GCPProjectID)
	if projectID == "" {
		projectID = strings.TrimSpace(key.ProjectID)
	}
	if projectID == "" {
		return "", errors.New("GCP project ID is required for Vertex")
	}
	location := strings.TrimSpace(model.GCPRegion)
	if location == "" {
		location = "global"
	}
	publisher, modelName, _ := NormalizeVertexAIModel(model.Name)
	if publisher == "" {
		publisher = "google"
	}
	if modelName == "" {
		return "", errors.New("Vertex model name is required")
	}
	action := "streamGenerateContent"

	rawBase := strings.TrimRight(strings.TrimSpace(model.BaseURL), "/")
	if rawBase == "" {
		if location == "global" {
			rawBase = "https://aiplatform.googleapis.com"
		} else {
			rawBase = fmt.Sprintf("https://%s-aiplatform.googleapis.com", location)
		}
	}
	endpoint, err := url.Parse(rawBase)
	if err != nil {
		return "", fmt.Errorf("parse Vertex base URL: %w", err)
	}
	if endpoint.Scheme == "" || endpoint.Host == "" {
		return "", errors.New("Vertex base URL must include scheme and host")
	}
	endpoint.Path = fmt.Sprintf(
		"/v1/projects/%s/locations/%s/publishers/%s/models/%s:%s",
		projectID,
		location,
		publisher,
		modelName,
		action,
	)
	endpoint.RawPath = ""
	query := endpoint.Query()
	query.Set("alt", "sse")
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func consumeVertexSSE(body io.Reader, emit func(geminiResponse) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	frames := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var frame geminiResponse
		if err := json.Unmarshal([]byte(payload), &frame); err != nil {
			return fmt.Errorf("decode Vertex stream: %w", err)
		}
		if frame.Error != nil {
			message := strings.TrimSpace(frame.Error.Message)
			if message == "" {
				message = "unknown provider error"
			}
			return fmt.Errorf("Vertex provider error: %s", message)
		}
		if err := emit(frame); err != nil {
			return err
		}
		frames++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Vertex stream: %w", err)
	}
	if frames == 0 {
		return errors.New("Vertex returned no response frames")
	}
	return nil
}
