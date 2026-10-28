package quota

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

const (
	languageServerUserStatusMethod         = "/exa.language_server_pb.LanguageServerService/GetUserStatus"
	languageServerQuotaSummaryMethod       = "/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary"
	languageServerSetCloudCodeURLMethod    = "/exa.language_server_pb.LanguageServerService/SetCloudCodeURL"
	languageServerResponseMaxBytes   int64 = 8 << 20
)

type languageServerEndpoint struct {
	origin    string
	csrfToken string
}

type languageServerClientModelConfig struct {
	ModelID        string                        `json:"modelId"`
	ModelIDSnake   string                        `json:"model_id"`
	Label          string                        `json:"label"`
	QuotaInfo      *languageServerModelQuotaInfo `json:"quotaInfo"`
	QuotaInfoSnake *languageServerModelQuotaInfo `json:"quota_info"`
}

type languageServerModelQuotaInfo struct {
	RemainingFraction *float64 `json:"remainingFraction"`
	RemainingFractAlt *float64 `json:"remaining_fraction"`
	ResetTime         string   `json:"resetTime"`
	ResetTimeSnake    string   `json:"reset_time"`
}

func (q *languageServerModelQuotaInfo) fraction() (float64, bool) {
	if q == nil {
		return 0, false
	}
	if q.RemainingFraction != nil {
		return *q.RemainingFraction, true
	}
	if q.RemainingFractAlt != nil {
		return *q.RemainingFractAlt, true
	}
	return 0, false
}

func (q *languageServerModelQuotaInfo) resetTime() string {
	if q == nil {
		return ""
	}
	if q.ResetTime != "" {
		return q.ResetTime
	}
	return q.ResetTimeSnake
}

type languageServerUserStatusResponse struct {
	UserStatus struct {
		Email                  string                           `json:"email"`
		CascadeModelConfigData struct {
			ClientModelConfigs []languageServerClientModelConfig `json:"clientModelConfigs"`
		} `json:"cascadeModelConfigData"`
		CascadeModelConfigSnake struct {
			ClientModelConfigs []languageServerClientModelConfig `json:"client_model_configs"`
		} `json:"cascade_model_config_data"`
	} `json:"userStatus"`
	UserStatusSnake struct {
		Email                  string                           `json:"email"`
		CascadeModelConfigData struct {
			ClientModelConfigs []languageServerClientModelConfig `json:"clientModelConfigs"`
		} `json:"cascadeModelConfigData"`
		CascadeModelConfigSnake struct {
			ClientModelConfigs []languageServerClientModelConfig `json:"client_model_configs"`
		} `json:"cascade_model_config_data"`
	} `json:"user_status"`
}

func (s languageServerUserStatusResponse) email() string {
	if s.UserStatus.Email != "" {
		return s.UserStatus.Email
	}
	return s.UserStatusSnake.Email
}

func (s languageServerUserStatusResponse) extractArgonQuota(now time.Time) *languageServerArgonQuota {
	configs := s.UserStatus.CascadeModelConfigData.ClientModelConfigs
	if len(configs) == 0 {
		configs = s.UserStatus.CascadeModelConfigSnake.ClientModelConfigs
	}
	if len(configs) == 0 {
		configs = s.UserStatusSnake.CascadeModelConfigData.ClientModelConfigs
	}
	if len(configs) == 0 {
		configs = s.UserStatusSnake.CascadeModelConfigSnake.ClientModelConfigs
	}
	for _, m := range configs {
		modelID := m.ModelID
		if modelID == "" {
			modelID = m.ModelIDSnake
		}
		if !isArgonQuotaIdentifier(modelID, m.Label) {
			continue
		}
		info := m.QuotaInfo
		if info == nil {
			info = m.QuotaInfoSnake
		}
		fract, ok := info.fraction()
		if !ok {
			continue
		}
		remaining := ClampFraction(fract)
		resetAt := parseResetTime(info.resetTime())
		return &languageServerArgonQuota{
			Weekly:          &remaining,
			WeeklyResetTime: resetAt,
		}
	}
	return nil
}
type languageServerQuotaBucket struct {
	BucketID          string   `json:"bucketId"`
	BucketIDSnake     string   `json:"bucket_id"`
	DisplayName       string   `json:"displayName"`
	DisplayNameSnake  string   `json:"display_name"`
	Window            string   `json:"window"`
	RemainingFraction *float64 `json:"remainingFraction"`
	RemainingFractAlt *float64 `json:"remaining_fraction"`
	RemainingAmount   *int64   `json:"remainingAmount"`
	RemainingAmtAlt   *int64   `json:"remaining_amount"`
	ResetTime         string   `json:"resetTime"`
	ResetTimeSnake    string   `json:"reset_time"`
}

func (b languageServerQuotaBucket) bucketID() string {
	if b.BucketID != "" {
		return b.BucketID
	}
	return b.BucketIDSnake
}

func (b languageServerQuotaBucket) displayName() string {
	if b.DisplayName != "" {
		return b.DisplayName
	}
	return b.DisplayNameSnake
}

func (b languageServerQuotaBucket) resetTime() string {
	if b.ResetTime != "" {
		return b.ResetTime
	}
	return b.ResetTimeSnake
}

func (b languageServerQuotaBucket) fraction() (float64, bool) {
	if b.RemainingFraction != nil {
		return *b.RemainingFraction, true
	}
	if b.RemainingFractAlt != nil {
		return *b.RemainingFractAlt, true
	}
	if b.RemainingAmount != nil {
		return float64(*b.RemainingAmount) / 100.0, true
	}
	if b.RemainingAmtAlt != nil {
		return float64(*b.RemainingAmtAlt) / 100.0, true
	}
	return 0, false
}

type languageServerQuotaGroup struct {
	DisplayName      string                      `json:"displayName"`
	DisplayNameSnake string                      `json:"display_name"`
	Description      string                      `json:"description"`
	Buckets          []languageServerQuotaBucket `json:"buckets"`
}

func (g languageServerQuotaGroup) displayName() string {
	if g.DisplayName != "" {
		return g.DisplayName
	}
	return g.DisplayNameSnake
}

type languageServerQuotaPayload struct {
	Groups  []languageServerQuotaGroup  `json:"groups"`
	Buckets []languageServerQuotaBucket `json:"buckets"`
}

type languageServerQuotaResponse struct {
	Response *languageServerQuotaPayload `json:"response"`
	Groups   []languageServerQuotaGroup  `json:"groups"`
	Buckets  []languageServerQuotaBucket `json:"buckets"`
}

func (r *languageServerQuotaResponse) allGroups() []languageServerQuotaGroup {
	if r == nil {
		return nil
	}
	if r.Response != nil && len(r.Response.Groups) > 0 {
		return r.Response.Groups
	}
	return r.Groups
}

func (r *languageServerQuotaResponse) allBuckets() []languageServerQuotaBucket {
	if r == nil {
		return nil
	}
	if r.Response != nil && len(r.Response.Buckets) > 0 {
		return r.Response.Buckets
	}
	return r.Buckets
}

type languageServerArgonQuota struct {
	FiveHour          *float64
	FiveHourResetTime time.Time
	Weekly            *float64
	WeeklyResetTime   time.Time
}

// fetchLanguageServerArgonQuota reads the quota group that Antigravity's local
// language server exposes but the public CloudCode summary endpoint omits.
func fetchLanguageServerArgonQuota(expectedEmail string, now time.Time) (*languageServerArgonQuota, error) {
	return fetchLanguageServerArgonQuotaWithProject(expectedEmail, "", now)
}

func fetchLanguageServerArgonQuotaWithProject(expectedEmail, project string, now time.Time) (*languageServerArgonQuota, error) {
	if core.IsRunningTests() {
		return nil, fmt.Errorf("local language server lookup disabled during tests")
	}

	for _, endpoint := range discoverLanguageServerEndpoints() {
		quotaResult, err := fetchArgonQuotaFromEndpointWithProject(endpoint, expectedEmail, project, now)
		if err == nil {
			return quotaResult, nil
		}
	}
	return nil, fmt.Errorf("Gemini 4 Argon quota is unavailable from the local Antigravity language server")
}

func fetchArgonQuotaFromEndpoint(endpoint languageServerEndpoint, expectedEmail string, now time.Time) (*languageServerArgonQuota, error) {
	return fetchArgonQuotaFromEndpointWithProject(endpoint, expectedEmail, "", now)
}

func fetchArgonQuotaFromEndpointWithProject(endpoint languageServerEndpoint, expectedEmail, project string, now time.Time) (*languageServerArgonQuota, error) {
	var status languageServerUserStatusResponse
	if err := postLanguageServerRPC(endpoint, languageServerUserStatusMethod, map[string]any{}, &status); err != nil {
		return nil, err
	}
	if expectedEmail != "" && !strings.EqualFold(strings.TrimSpace(status.email()), strings.TrimSpace(expectedEmail)) {
		return nil, fmt.Errorf("local language server account does not match")
	}

	payload := map[string]any{
		"forceRefresh": true,
	}
	if project != "" {
		payload["project"] = project
		payload["request"] = map[string]any{
			"project": project,
		}
	}

	var summary languageServerQuotaResponse
	summaryErr := postLanguageServerRPC(endpoint, languageServerQuotaSummaryMethod, payload, &summary)

	result := &languageServerArgonQuota{}
	if summaryErr == nil {
		processBucket := func(groupName string, bucket languageServerQuotaBucket) {
			bucketID := bucket.bucketID()
			displayName := bucket.displayName()
			if !isArgonQuotaIdentifier(groupName, bucketID, displayName) {
				return
			}
			fract, ok := bucket.fraction()
			if !ok {
				return
			}
			resetTime := parseResetTime(bucket.resetTime())
			remaining := ClampFraction(fract)
			switch classifyQuotaWindow(bucket.Window, bucketID, displayName, resetTime, now) {
			case "weekly":
				result.Weekly = &remaining
				result.WeeklyResetTime = resetTime
			case "5h":
				result.FiveHour = &remaining
				result.FiveHourResetTime = resetTime
			}
		}

		for _, group := range summary.allGroups() {
			for _, bucket := range group.Buckets {
				processBucket(group.displayName(), bucket)
			}
		}
		for _, bucket := range summary.allBuckets() {
			processBucket("", bucket)
		}
	}

	if result.FiveHour == nil && result.Weekly == nil {
		if fallback := status.extractArgonQuota(now); fallback != nil {
			result = fallback
		}
	}

	if result.FiveHour == nil && result.Weekly == nil {
		if summaryErr != nil {
			return nil, summaryErr
		}
		return nil, fmt.Errorf("Gemini 4 Argon quota buckets not found")
	}
	return result, nil
}

func applyLanguageServerArgon(breakdown *LiveQuotaBreakdown, result *languageServerArgonQuota, now time.Time) {
	if breakdown == nil || result == nil {
		return
	}

	if result.FiveHour != nil {
		remaining := ClampFraction(*result.FiveHour)
		breakdown.Quota5hArgon = &remaining
		breakdown.ResetTime5hArgon = result.FiveHourResetTime
		breakdown.ResetHorizon5hArgonText = FormatResetHorizon(result.FiveHourResetTime, now)
		breakdown.ResetSeconds5hArgon = 0
		if !result.FiveHourResetTime.IsZero() && result.FiveHourResetTime.After(now) {
			breakdown.ResetSeconds5hArgon = result.FiveHourResetTime.Sub(now).Seconds()
		}
		updateArgonModel(breakdown, "Gemini 4 Argon 5H", remaining, result.FiveHourResetTime, now)
	}
	if result.Weekly != nil {
		remaining := ClampFraction(*result.Weekly)
		breakdown.QuotaWeeklyArgon = &remaining
		breakdown.ResetTimeWeeklyArgon = result.WeeklyResetTime
		breakdown.ResetHorizonWeeklyArgonText = FormatResetHorizon(result.WeeklyResetTime, now)
		breakdown.ResetSecondsWeeklyArgon = 0
		if !result.WeeklyResetTime.IsZero() && result.WeeklyResetTime.After(now) {
			breakdown.ResetSecondsWeeklyArgon = result.WeeklyResetTime.Sub(now).Seconds()
		}
		updateArgonModel(breakdown, "Gemini 4 Argon Weekly", remaining, result.WeeklyResetTime, now)
	}
}

func updateArgonModel(breakdown *LiveQuotaBreakdown, name string, remaining float64, resetTime time.Time, now time.Time) {
	isWeekly := strings.Contains(strings.ToLower(name), "weekly")
	for i := range breakdown.Models {
		existingName := strings.ToLower(breakdown.Models[i].ModelName)
		existingWeekly := strings.Contains(existingName, "weekly") || strings.Contains(existingName, "7d")
		if !isArgonQuotaIdentifier(breakdown.Models[i].ModelName) || existingWeekly != isWeekly {
			continue
		}
		breakdown.Models[i].ModelName = name
		breakdown.Models[i].Fraction = remaining
		breakdown.Models[i].ResetTime = resetTime
		breakdown.Models[i].ResetText = FormatResetHorizon(resetTime, now)
		breakdown.Models[i].HealthStatus = ComputeHealth(remaining)
		return
	}

	breakdown.Models = append(breakdown.Models, ModelQuota{
		ModelName:    name,
		Fraction:     remaining,
		ResetTime:    resetTime,
		ResetText:    FormatResetHorizon(resetTime, now),
		HealthStatus: ComputeHealth(remaining),
	})
}

func discoverLanguageServerEndpoints() []languageServerEndpoint {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var endpoints []languageServerEndpoint
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}

		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		args := splitNullArgs(cmdline)
		csrfToken, explicitPort, ok := parseLanguageServerCommand(args)
		if !ok || csrfToken == "" {
			continue
		}

		ports := listeningPortsForPID(pid)
		if explicitPort > 0 {
			ports = append([]int{explicitPort}, ports...)
		}
		for _, port := range uniquePorts(ports) {
			for _, scheme := range []string{"https", "http"} {
				endpoints = append(endpoints, languageServerEndpoint{
					origin:    fmt.Sprintf("%s://127.0.0.1:%d", scheme, port),
					csrfToken: csrfToken,
				})
			}
		}
	}
	return endpoints
}

func languageServerEndpointsForPorts(csrfToken string, ports []int) []languageServerEndpoint {
	endpoints := make([]languageServerEndpoint, 0, len(ports)*2)
	for _, port := range uniquePorts(ports) {
		for _, scheme := range []string{"https", "http"} {
			endpoints = append(endpoints, languageServerEndpoint{
				origin:    fmt.Sprintf("%s://127.0.0.1:%d", scheme, port),
				csrfToken: csrfToken,
			})
		}
	}
	return endpoints
}

func splitNullArgs(cmdline []byte) []string {
	parts := bytes.Split(cmdline, []byte{0})
	args := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) > 0 {
			args = append(args, string(part))
		}
	}
	return args
}

func parseLanguageServerCommand(args []string) (csrfToken string, explicitPort int, ok bool) {
	if len(args) == 0 {
		return "", 0, false
	}

	joined := strings.ToLower(strings.Join(args, " "))
	executable := strings.ToLower(filepath.Base(args[0]))
	if !strings.Contains(executable, "language_server") && !strings.Contains(joined, "language_server") {
		return "", 0, false
	}
	hasHubSubclient := strings.Contains(joined, "subclient_type hub") ||
		strings.Contains(joined, "subclient_type=hub") ||
		strings.Contains(joined, "subclient-type=hub")
	if !strings.Contains(joined, "antigravity") && !hasHubSubclient {
		return "", 0, false
	}

	for i, arg := range args {
		switch {
		case arg == "--csrf_token" || arg == "--csrf-token":
			if i+1 < len(args) {
				csrfToken = strings.TrimSpace(args[i+1])
			}
		case strings.HasPrefix(arg, "--csrf_token="):
			csrfToken = strings.TrimSpace(strings.TrimPrefix(arg, "--csrf_token="))
		case strings.HasPrefix(arg, "--csrf-token="):
			csrfToken = strings.TrimSpace(strings.TrimPrefix(arg, "--csrf-token="))
		}

		switch {
		case arg == "--https_server_port" || arg == "--https-server-port":
			if i+1 < len(args) {
				explicitPort, _ = strconv.Atoi(strings.TrimSpace(args[i+1]))
			}
		case strings.HasPrefix(arg, "--https_server_port="):
			explicitPort, _ = strconv.Atoi(strings.TrimPrefix(arg, "--https_server_port="))
		case strings.HasPrefix(arg, "--https-server-port="):
			explicitPort, _ = strconv.Atoi(strings.TrimPrefix(arg, "--https-server-port="))
		}
	}
	return csrfToken, explicitPort, true
}

func listeningPortsForPID(pid int) []int {
	socketInodes := make(map[string]struct{})
	fdEntries, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(pid), "fd"))
	if err != nil {
		return nil
	}
	for _, entry := range fdEntries {
		target, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "fd", entry.Name()))
		if err != nil || !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
			continue
		}
		socketInodes[strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")] = struct{}{}
	}

	var ports []int
	for _, name := range []string{"tcp", "tcp6"} {
		data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "net", name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 10 || fields[3] != "0A" {
				continue
			}
			if _, owned := socketInodes[fields[9]]; !owned {
				continue
			}
			separator := strings.LastIndex(fields[1], ":")
			if separator < 0 {
				continue
			}
			portHex := fields[1][separator+1:]
			port, err := strconv.ParseInt(portHex, 16, 32)
			if err == nil && port > 0 && port <= 65535 {
				ports = append(ports, int(port))
			}
		}
	}
	sort.Ints(ports)
	return uniquePorts(ports)
}

func uniquePorts(ports []int) []int {
	seen := make(map[int]struct{}, len(ports))
	result := make([]int, 0, len(ports))
	for _, port := range ports {
		if port <= 0 || port > 65535 {
			continue
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		result = append(result, port)
	}
	return result
}

func postLanguageServerRPC(endpoint languageServerEndpoint, method string, payload any, out any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	frame := grpcWebFrame(payloadJSON)

	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint.origin, "/")+method, bytes.NewReader(frame))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/grpc-web+json")
	req.Header.Set("Accept", "application/grpc-web+json")
	req.Header.Set("X-Grpc-Web", "1")
	req.Header.Set("x-codeium-csrf-token", endpoint.csrfToken)
	req.Header.Set("Origin", endpoint.origin)
	req.Header.Set("Referer", strings.TrimRight(endpoint.origin, "/")+"/")

	resp, err := newLanguageServerHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, languageServerResponseMaxBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("local language server returned HTTP %d", resp.StatusCode)
	}

	message, err := decodeGRPCWebMessage(body)
	if err != nil {
		return err
	}
	var envelope struct {
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(message, &envelope); err == nil && len(envelope.Response) > 0 {
		message = envelope.Response
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(message, out)
}

var (
	languageServerHTTPClientOnce   sync.Once
	sharedLanguageServerHTTPClient *http.Client
)

func newLanguageServerHTTPClient() *http.Client {
	languageServerHTTPClientOnce.Do(func() {
		sharedLanguageServerHTTPClient = &http.Client{
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				ForceAttemptHTTP2:   true,
				IdleConnTimeout:     30 * time.Second,
				MaxIdleConnsPerHost: 4,
				TLSClientConfig: &tls.Config{
					// The quota API is bound to loopback and uses an ephemeral
					// self-signed certificate generated by the local process.
					InsecureSkipVerify: true,
				},
				DialContext: (&net.Dialer{
					Timeout:   400 * time.Millisecond,
					KeepAlive: 15 * time.Second,
				}).DialContext,
				ResponseHeaderTimeout: 1800 * time.Millisecond,
			},
		}
	})
	return sharedLanguageServerHTTPClient
}

func grpcWebFrame(payload []byte) []byte {
	frame := make([]byte, 5+len(payload))
	frame[0] = 0
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(payload)))
	copy(frame[5:], payload)
	return frame
}

func decodeGRPCWebMessage(data []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return trimmed, nil
	}
	for len(data) >= 5 {
		flag := data[0]
		size := int(binary.BigEndian.Uint32(data[1:5]))
		data = data[5:]
		if size < 0 || size > len(data) {
			return nil, fmt.Errorf("invalid local language server frame")
		}
		payload := data[:size]
		data = data[size:]
		if flag&0x80 != 0 {
			continue
		}
		if flag != 0 {
			return nil, fmt.Errorf("unsupported compressed local language server frame")
		}
		return payload, nil
	}
	return nil, fmt.Errorf("empty local language server response")
}

// SetLanguageServerCloudCodeURLForEndpoint instructs a specific Antigravity language_server
// endpoint to route Cloud Code API requests to targetURL.
func SetLanguageServerCloudCodeURLForEndpoint(endpoint languageServerEndpoint, targetURL string) error {
	payload := map[string]any{
		"url":            targetURL,
		"cloudCodeUrl":   targetURL,
		"cloud_code_url": targetURL,
	}
	return postLanguageServerRPC(endpoint, languageServerSetCloudCodeURLMethod, payload, nil)
}

// SetLanguageServerCloudCodeURL instructs all discovered running Antigravity
// language_server instances to route their Cloud Code requests to targetURL
// (e.g. "http://127.0.0.1:8765").
func SetLanguageServerCloudCodeURL(targetURL string) error {
	endpoints := discoverLanguageServerEndpoints()
	if len(endpoints) == 0 {
		return fmt.Errorf("no running Antigravity language_server instances discovered")
	}
	var lastErr error
	successCount := 0
	for _, endpoint := range endpoints {
		if err := SetLanguageServerCloudCodeURLForEndpoint(endpoint, targetURL); err != nil {
			lastErr = err
			continue
		}
		successCount++
	}
	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("failed to set cloud code url on language server: %w", lastErr)
	}
	return nil
}
