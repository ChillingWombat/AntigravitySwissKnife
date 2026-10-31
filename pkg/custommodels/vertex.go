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
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	httpReq, idleTimeout, err := g.prepareVertexHTTPRequest(ctx, r, model, request)
	if err != nil {
		return err
	}
	idleTimer := time.AfterFunc(idleTimeout, cancel)
	defer idleTimer.Stop()

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return vertexCallError(ctx, r.Context(), idleTimeout, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return vertexResponseError(resp)
	}
	idleTimer.Reset(idleTimeout)
	err = consumeVertexSSE(&idleReadCloser{
		ReadCloser: resp.Body,
		timer:      idleTimer,
		timeout:    idleTimeout,
	}, emit)
	return vertexStreamError(ctx, r.Context(), idleTimeout, err)
}

func (g *Gateway) prepareVertexHTTPRequest(ctx context.Context, r *http.Request, model *CustomModel, request *geminiRequest) (*http.Request, time.Duration, error) {
	endpoint, key, err := g.vertexEndpointForModel(model)
	if err != nil {
		return nil, 0, err
	}
	accessToken, err := GenerateGCPAccessToken(r.Context(), key)
	if err != nil {
		return nil, 0, fmt.Errorf("refresh GCP access token: %w", err)
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, 0, fmt.Errorf("encode Vertex request: %w", err)
	}
	idleTimeout := g.vertexIdleTimeout()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("build Vertex request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	return httpReq, idleTimeout, nil
}

func (g *Gateway) vertexEndpointForModel(model *CustomModel) (string, *ServiceAccountKey, error) {
	key, err := ParseServiceAccountKey([]byte(model.GCPServiceAccountKey))
	if err != nil {
		return "", nil, fmt.Errorf("invalid GCP service account for model %q: %w", model.ID, err)
	}
	endpoint, err := vertexStreamEndpoint(model, key)
	if err != nil {
		return "", nil, err
	}
	if _, _, err := ValidateEndpointSSRF(endpoint, ProviderGCP, g.AllowLoopback); err != nil {
		return "", nil, fmt.Errorf("reject Vertex endpoint: %w", err)
	}
	return endpoint, key, nil
}

func (g *Gateway) vertexIdleTimeout() time.Duration {
	if g.vertexStreamIdleTimeout <= 0 {
		return defaultVertexStreamIdleTimeout
	}
	return g.vertexStreamIdleTimeout
}

func vertexCallError(ctx context.Context, requestCtx context.Context, idleTimeout time.Duration, err error) error {
	if ctx.Err() != nil && requestCtx.Err() == nil {
		return fmt.Errorf("Vertex stream made no progress for %s", idleTimeout)
	}
	return fmt.Errorf("call Vertex: %w", err)
}

func vertexResponseError(resp *http.Response) error {
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("Vertex returned %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
}

func vertexStreamError(ctx context.Context, requestCtx context.Context, idleTimeout time.Duration, err error) error {
	if err != nil && ctx.Err() != nil && requestCtx.Err() == nil {
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
	n, err := r.ReadCloser.Read(p)
	r.timer.Reset(r.timeout)
	return n, err
}

func vertexStreamEndpoint(model *CustomModel, key *ServiceAccountKey) (string, error) {
	projectID, err := vertexProjectID(model, key)
	if err != nil {
		return "", err
	}
	location := vertexLocation(model)
	publisher, modelName, err := vertexTarget(model)
	if err != nil {
		return "", err
	}
	rawBase, err := vertexBaseURL(model, location)
	if err != nil {
		return "", err
	}
	endpoint, err := url.Parse(rawBase)
	if err != nil {
		return "", fmt.Errorf("parse Vertex base URL: %w", err)
	}
	endpoint.Path = fmt.Sprintf(
		"/v1/projects/%s/locations/%s/publishers/%s/models/%s:streamGenerateContent",
		projectID,
		location,
		publisher,
		modelName,
	)
	endpoint.RawPath = ""
	query := endpoint.Query()
	query.Set("alt", "sse")
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func vertexProjectID(model *CustomModel, key *ServiceAccountKey) (string, error) {
	projectID := strings.TrimSpace(model.GCPProjectID)
	if projectID == "" {
		projectID = strings.TrimSpace(key.ProjectID)
	}
	if projectID == "" {
		return "", errors.New("GCP project ID is required for Vertex")
	}
	return projectID, nil
}

func vertexLocation(model *CustomModel) string {
	location := strings.TrimSpace(model.GCPRegion)
	if location == "" {
		return "global"
	}
	return location
}

func vertexTarget(model *CustomModel) (string, string, error) {
	publisher, modelName, _ := NormalizeVertexAIModel(model.Name)
	if publisher == "" {
		publisher = "google"
	}
	if modelName == "" {
		return "", "", errors.New("Vertex model name is required")
	}
	return publisher, modelName, nil
}

func vertexBaseURL(model *CustomModel, location string) (string, error) {
	rawBase := strings.TrimRight(strings.TrimSpace(model.BaseURL), "/")
	if rawBase == "" {
		rawBase = defaultVertexBaseURL(location)
	}
	endpoint, err := url.Parse(rawBase)
	if err != nil {
		return "", fmt.Errorf("parse Vertex base URL: %w", err)
	}
	if endpoint.Scheme == "" || endpoint.Host == "" {
		return "", errors.New("Vertex base URL must include scheme and host")
	}
	return rawBase, nil
}

func defaultVertexBaseURL(location string) string {
	if location == "global" {
		return "https://aiplatform.googleapis.com"
	}
	return fmt.Sprintf("https://%s-aiplatform.googleapis.com", location)
}

func consumeVertexSSE(body io.Reader, emit func(geminiResponse) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	frames, err := consumeVertexFrames(scanner, emit)
	if err != nil {
		return err
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Vertex stream: %w", err)
	}
	if frames == 0 {
		return errors.New("Vertex returned no response frames")
	}
	return nil
}

func consumeVertexFrames(scanner *bufio.Scanner, emit func(geminiResponse) error) (int, error) {
	frames := 0
	for scanner.Scan() {
		payload, ok := parseVertexSSELine(scanner.Text())
		if !ok {
			continue
		}
		frame, err := decodeVertexSSEFrame([]byte(payload))
		if err != nil {
			return frames, err
		}
		if err := emit(frame); err != nil {
			return frames, err
		}
		frames++
	}
	return frames, nil
}

func parseVertexSSELine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "" || payload == "[DONE]" {
		return "", false
	}
	return payload, true
}

func decodeVertexSSEFrame(payload []byte) (geminiResponse, error) {
	var frame geminiResponse
	if err := json.Unmarshal(payload, &frame); err != nil {
		return frame, fmt.Errorf("decode Vertex stream: %w", err)
	}
	if frame.Error != nil {
		return frame, vertexProviderError(frame.Error)
	}
	return frame, nil
}

func vertexProviderError(providerErr *geminiError) error {
	message := strings.TrimSpace(providerErr.Message)
	if message == "" {
		message = "unknown provider error"
	}
	return fmt.Errorf("Vertex provider error: %s", message)
}
