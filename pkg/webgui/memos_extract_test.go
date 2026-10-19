package webgui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
)

const (
	extractTestSAKey = `{"type":"service_account","project_id":"test-project","private_key":"-----BEGIN PRIVATE KEY-----\nFAKEKEY\n-----END PRIVATE KEY-----\n","client_email":"extract-test@test-project.iam.gserviceaccount.com"}`

	extractTestPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
)

type extractUpstreamCapture struct {
	auth      string
	method    string
	path      string
	role      string
	partCount int
	prompt    string
	mimeType  string
	mediaData string
}

// extractTestEndpointPath mirrors the real generateContent path so tests can
// assert that callVertexExtract posts to the right action.
const extractTestEndpointPath = "/v1/projects/test-project/locations/global/publishers/google/models/gemini-2.5-flash:generateContent"

// captureExtractRequest decodes the outgoing Vertex request and fills capture.
// It also asserts the shape Vertex itself requires: a role on the content
// object ("user") and exactly two parts (prompt text + inline media). Tests
// that only mock the response without checking the request body would keep
// passing while every real call fails with "Please use a valid role: user,
// model.", so these assertions live on the request side.
func captureExtractRequest(t *testing.T, r *http.Request, capture *extractUpstreamCapture) {
	t.Helper()
	capture.method = r.Method
	capture.path = r.URL.Path
	capture.auth = r.Header.Get("Authorization")

	var body struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text       string `json:"text"`
				InlineData *struct {
					MIMEType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"inlineData"`
			} `json:"parts"`
		} `json:"contents"`
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Errorf("read upstream body: %v", err)
		return
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Errorf("decode upstream body %q: %v", string(raw), err)
		return
	}
	if len(body.Contents) != 1 {
		t.Errorf("contents length = %d, want 1; body = %s", len(body.Contents), string(raw))
		return
	}

	content := body.Contents[0]
	capture.role = content.Role
	capture.partCount = len(content.Parts)
	if content.Role != "user" {
		t.Errorf("contents[0].role = %q, want %q; body = %s", content.Role, "user", string(raw))
	}
	if len(content.Parts) != 2 {
		t.Errorf("contents[0].parts length = %d, want 2; body = %s", len(content.Parts), string(raw))
		return
	}

	capture.prompt = content.Parts[0].Text
	if strings.TrimSpace(capture.prompt) == "" {
		t.Errorf("contents[0].parts[0].text is empty; body = %s", string(raw))
	}
	inline := content.Parts[1].InlineData
	if inline == nil {
		t.Errorf("contents[0].parts[1].inlineData is missing; body = %s", string(raw))
		return
	}
	capture.mimeType = inline.MIMEType
	capture.mediaData = inline.Data
	if strings.TrimSpace(inline.MIMEType) == "" {
		t.Errorf("contents[0].parts[1].inlineData.mimeType is empty; body = %s", string(raw))
	}
	if strings.TrimSpace(inline.Data) == "" {
		t.Errorf("contents[0].parts[1].inlineData.data is empty; body = %s", string(raw))
	}
}

// newExtractTestServer wires an isolated Server with a temp memos file and store.
func newExtractTestServer(t *testing.T, memos []MemoItem, models []custommodels.CustomModel) (*Server, string, *custommodels.Store) {
	t.Helper()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())

	store, err := custommodels.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	for _, m := range models {
		if err := store.SaveModel(m); err != nil {
			t.Fatalf("SaveModel(%s): %v", m.ID, err)
		}
	}

	memoPath := filepath.Join(t.TempDir(), "memos.json")
	if err := writeMemosFile(memoPath, memos); err != nil {
		t.Fatalf("writeMemosFile: %v", err)
	}

	srv := &Server{customModelsStore: store}
	srv.SetGlobalMemosPath(memoPath)
	return srv, memoPath, store
}

func testGCPModel() custommodels.CustomModel {
	return custommodels.CustomModel{
		ID:                   "gcp-gemini-test",
		Name:                 "gemini-2.5-flash",
		DisplayName:          "Gemini 2.5 Flash",
		ProviderType:         custommodels.ProviderGCP,
		GCPProjectID:         "test-project",
		GCPRegion:            "us-central1",
		GCPServiceAccountKey: extractTestSAKey,
		Enabled:              true,
		QuotaManualOverride:  true,
	}
}

// stubExtractUpstream points the handler at an httptest server and stubs token minting.
func stubExtractUpstream(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	fake := httptest.NewServer(handler)
	t.Cleanup(fake.Close)

	oldClient, oldMinter, oldEndpoint := memoExtractHTTPClient, memoExtractTokenMinter, memoExtractEndpoint
	client := fake.Client()
	client.Timeout = memoExtractTimeout
	memoExtractHTTPClient = client
	memoExtractTokenMinter = func(ctx context.Context, key *custommodels.ServiceAccountKey) (string, error) {
		return "test-token", nil
	}
	memoExtractEndpoint = func(region, project, model string) string {
		return fake.URL + extractTestEndpointPath
	}
	t.Cleanup(func() {
		memoExtractHTTPClient, memoExtractTokenMinter, memoExtractEndpoint = oldClient, oldMinter, oldEndpoint
	})
	return fake
}

func postExtract(t *testing.T, srv *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/memos/extract", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.handleMemosExtract(rec, req)
	return rec
}

func postExtractWithContext(t *testing.T, srv *Server, ctx context.Context, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/memos/extract", strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.handleMemosExtract(rec, req)
	return rec
}

func decodeExtractResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return payload
}

func storedTranscript(t *testing.T, path, id string) string {
	t.Helper()
	for _, m := range readMemosFile(path) {
		if m.ID == id {
			return m.Transcript
		}
	}
	t.Fatalf("memo %q not found in %s", id, path)
	return ""
}

func TestMemosExtractImageOCRHappyPath(t *testing.T) {
	memo := MemoItem{ID: "memo-img-1", Type: "image", ImageData: extractTestPNG}
	srv, memoPath, _ := newExtractTestServer(t, []MemoItem{memo}, []custommodels.CustomModel{testGCPModel()})

	capture := &extractUpstreamCapture{}
	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		captureExtractRequest(t, r, capture)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"  hello world  "}]}}]}`))
	})

	rec := postExtract(t, srv, `{"memo_id":"memo-img-1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	payload := decodeExtractResponse(t, rec)
	if payload["success"] != true {
		t.Fatalf("expected success=true, got %v", payload)
	}
	if payload["transcript"] != "hello world" {
		t.Fatalf("transcript = %v, want trimmed text", payload["transcript"])
	}
	if capture.auth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want bearer token", capture.auth)
	}
	if capture.method != http.MethodPost {
		t.Errorf("upstream method = %q, want POST", capture.method)
	}
	if capture.path != extractTestEndpointPath {
		t.Errorf("upstream path = %q, want %q", capture.path, extractTestEndpointPath)
	}
	if capture.role != "user" {
		t.Errorf("contents[0].role = %q, want user", capture.role)
	}
	if capture.partCount != 2 {
		t.Errorf("contents[0].parts length = %d, want 2", capture.partCount)
	}
	if capture.prompt != memoExtractImagePrompt {
		t.Errorf("prompt = %q, want OCR prompt", capture.prompt)
	}
	if capture.mimeType != "image/png" {
		t.Errorf("mimeType = %q, want image/png", capture.mimeType)
	}
	if capture.mediaData == "" {
		t.Error("inlineData.data is empty")
	}
	if got := storedTranscript(t, memoPath, "memo-img-1"); got != "hello world" {
		t.Errorf("persisted transcript = %q, want hello world", got)
	}
}

func TestMemosExtractAudioUsesAudioPrompt(t *testing.T) {
	webm := base64.StdEncoding.EncodeToString([]byte{0x1A, 0x45, 0xDF, 0xA3, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm'})
	memo := MemoItem{ID: "memo-audio-1", Type: "audio", AudioData: webm}
	srv, memoPath, _ := newExtractTestServer(t, []MemoItem{memo}, []custommodels.CustomModel{testGCPModel()})

	capture := &extractUpstreamCapture{}
	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		captureExtractRequest(t, r, capture)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"spoken words"}]}}]}`))
	})

	rec := postExtract(t, srv, `{"memo_id":"memo-audio-1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if capture.prompt != memoExtractAudioPrompt {
		t.Errorf("prompt = %q, want transcription prompt", capture.prompt)
	}
	if capture.mimeType != "audio/webm" {
		t.Errorf("mimeType = %q, want audio/webm", capture.mimeType)
	}
	if got := storedTranscript(t, memoPath, "memo-audio-1"); got != "spoken words" {
		t.Errorf("persisted transcript = %q, want spoken words", got)
	}
}

func TestMemosExtractNoModelConfigured(t *testing.T) {
	memo := MemoItem{ID: "memo-img-2", Type: "image", ImageData: extractTestPNG}
	srv, _, _ := newExtractTestServer(t, []MemoItem{memo}, nil)

	called := false
	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	rec := postExtract(t, srv, `{"memo_id":"memo-img-2"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", rec.Code, rec.Body.String())
	}
	payload := decodeExtractResponse(t, rec)
	if payload["success"] != false {
		t.Errorf("success = %v, want false", payload["success"])
	}
	if payload["error"] != memoExtractNoModelMessage {
		t.Errorf("error = %q, want actionable no-model message", payload["error"])
	}
	if called {
		t.Error("upstream must not be called when no model is configured")
	}
}

func TestMemosExtractUpstreamFailureKeepsTranscript(t *testing.T) {
	memo := MemoItem{ID: "memo-img-3", Type: "image", ImageData: extractTestPNG, Transcript: "original"}
	srv, memoPath, _ := newExtractTestServer(t, []MemoItem{memo}, []custommodels.CustomModel{testGCPModel()})

	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"backend exploded"}}`))
	})

	rec := postExtract(t, srv, `{"memo_id":"memo-img-3"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body = %s", rec.Code, rec.Body.String())
	}
	payload := decodeExtractResponse(t, rec)
	if payload["success"] != false {
		t.Errorf("success = %v, want false", payload["success"])
	}
	msg, _ := payload["error"].(string)
	if !strings.Contains(msg, "HTTP 500") || !strings.Contains(msg, "backend exploded") {
		t.Errorf("error = %q, want upstream status and message", msg)
	}
	if got := storedTranscript(t, memoPath, "memo-img-3"); got != "original" {
		t.Errorf("transcript changed to %q on upstream failure", got)
	}
}

func TestMemosExtractRejectsInvalidMemo(t *testing.T) {
	cases := []struct {
		name string
		memo *MemoItem
		body string
	}{
		{name: "missing memo", body: `{"memo_id":"nope"}`},
		{name: "text memo", memo: &MemoItem{ID: "memo-text", Type: "text", Content: "hi"}, body: `{"memo_id":"memo-text"}`},
		{name: "empty media", memo: &MemoItem{ID: "memo-empty", Type: "image"}, body: `{"memo_id":"memo-empty"}`},
		{name: "missing id", body: `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var memos []MemoItem
			if tc.memo != nil {
				memos = []MemoItem{*tc.memo}
			}
			srv, _, _ := newExtractTestServer(t, memos, []custommodels.CustomModel{testGCPModel()})
			rec := postExtract(t, srv, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
			if payload := decodeExtractResponse(t, rec); payload["success"] != false {
				t.Errorf("success = %v, want false", payload["success"])
			}
		})
	}
}

func TestMemosExtractDataURLAndFormatDetection(t *testing.T) {
	cases := []struct {
		name     string
		memoType string
		raw      string
		wantMime string
	}{
		{name: "png data url", memoType: "image", raw: "data:image/png;base64," + extractTestPNG, wantMime: "image/png"},
		{name: "jpeg", memoType: "image", raw: encodeTestMedia(0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10), wantMime: "image/jpeg"},
		{name: "gif", memoType: "image", raw: encodeTestMedia('G', 'I', 'F', '8', '9', 'a'), wantMime: "image/gif"},
		{name: "webp", memoType: "image", raw: encodeTestMedia('R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'), wantMime: "image/webp"},
		{name: "wav", memoType: "audio", raw: encodeTestMedia('R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'A', 'V', 'E'), wantMime: "audio/wav"},
		{name: "ogg", memoType: "audio", raw: encodeTestMedia('O', 'g', 'g', 'S', 0, 0, 0, 0), wantMime: "audio/ogg"},
		{name: "webm", memoType: "audio", raw: encodeTestMedia(0x1A, 0x45, 0xDF, 0xA3, 0x42), wantMime: "audio/webm"},
		{name: "mp4", memoType: "audio", raw: encodeTestMedia(0, 0, 0, 0x20, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'), wantMime: "audio/mp4"},
		{name: "flac", memoType: "audio", raw: encodeTestMedia('f', 'L', 'a', 'C', 0, 0, 0, 0), wantMime: "audio/flac"},
		{name: "mp3", memoType: "audio", raw: encodeTestMedia('I', 'D', '3', 4, 0, 0, 0, 0), wantMime: "audio/mpeg"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, mimeType, err := normalizeMemoMedia(tc.raw, tc.memoType)
			if err != nil {
				t.Fatalf("normalizeMemoMedia: %v", err)
			}
			if mimeType != tc.wantMime {
				t.Errorf("mime = %q, want %q", mimeType, tc.wantMime)
			}
			if strings.Contains(payload, ",") {
				t.Errorf("payload still contains data-url prefix: %q", payload)
			}
			if payload == "" {
				t.Error("payload is empty")
			}
		})
	}
}

func TestMemosExtractRejectsUnsupportedMedia(t *testing.T) {
	memo := MemoItem{ID: "memo-bad", Type: "image", ImageData: base64.StdEncoding.EncodeToString([]byte("definitely not an image"))}
	srv, _, _ := newExtractTestServer(t, []MemoItem{memo}, []custommodels.CustomModel{testGCPModel()})

	rec := postExtract(t, srv, `{"memo_id":"memo-bad"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
}

func encodeTestMedia(b ...byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func TestMemosExtractEmptyResultKeepsExistingTranscript(t *testing.T) {
	// An image with nothing to OCR must not wipe a transcript the user already
	// has. Both an empty candidates array and an empty text part count as empty.
	cases := []struct {
		name string
		body string
	}{
		{name: "no candidates", body: `{"candidates":[]}`},
		{name: "empty text part", body: `{"candidates":[{"content":{"parts":[{"text":""}]}}]}`},
		{name: "whitespace text part", body: `{"candidates":[{"content":{"parts":[{"text":"   \n"}]}}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			memo := MemoItem{ID: "memo-img-empty", Type: "image", ImageData: extractTestPNG, Transcript: "stale"}
			srv, memoPath, _ := newExtractTestServer(t, []MemoItem{memo}, []custommodels.CustomModel{testGCPModel()})

			before, err := os.ReadFile(memoPath)
			if err != nil {
				t.Fatal(err)
			}
			stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			})

			rec := postExtract(t, srv, `{"memo_id":"memo-img-empty"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}
			payload := decodeExtractResponse(t, rec)
			if payload["success"] != true {
				t.Errorf("success = %v, want true", payload["success"])
			}
			if payload["transcript"] != "stale" {
				t.Errorf("transcript = %v, want the existing value preserved", payload["transcript"])
			}
			if payload["empty"] != true {
				t.Errorf("empty = %v, want true for an empty extraction", payload["empty"])
			}
			if got := storedTranscript(t, memoPath, "memo-img-empty"); got != "stale" {
				t.Errorf("persisted transcript = %q, want stale", got)
			}
			after, err := os.ReadFile(memoPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Errorf("memos file was rewritten on an empty extraction:\nbefore=%s\nafter=%s", before, after)
			}
		})
	}
}

func TestMemosExtractEmptyResultWithoutExistingTranscript(t *testing.T) {
	memo := MemoItem{ID: "memo-img-blank", Type: "image", ImageData: extractTestPNG}
	srv, memoPath, _ := newExtractTestServer(t, []MemoItem{memo}, []custommodels.CustomModel{testGCPModel()})

	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"candidates":[]}`))
	})

	rec := postExtract(t, srv, `{"memo_id":"memo-img-blank"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	payload := decodeExtractResponse(t, rec)
	if payload["success"] != true || payload["transcript"] != "" || payload["empty"] != true {
		t.Errorf("payload = %v, want success with empty transcript and empty=true", payload)
	}
	if got := storedTranscript(t, memoPath, "memo-img-blank"); got != "" {
		t.Errorf("persisted transcript = %q, want empty", got)
	}
}

func TestMemosExtractAudioEmptyTextIsSuccess(t *testing.T) {
	webm := base64.StdEncoding.EncodeToString([]byte{0x1A, 0x45, 0xDF, 0xA3, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm'})
	memo := MemoItem{ID: "memo-audio-empty", Type: "audio", AudioData: webm}
	srv, memoPath, _ := newExtractTestServer(t, []MemoItem{memo}, []custommodels.CustomModel{testGCPModel()})

	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[]},"finishReason":"STOP"}]}`))
	})

	rec := postExtract(t, srv, `{"memo_id":"memo-audio-empty"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	payload := decodeExtractResponse(t, rec)
	if payload["success"] != true || payload["transcript"] != "" || payload["empty"] != true {
		t.Errorf("payload = %v, want success with empty transcript and empty=true", payload)
	}
	if got := storedTranscript(t, memoPath, "memo-audio-empty"); got != "" {
		t.Errorf("persisted transcript = %q, want empty", got)
	}
}

func TestMemosExtractBlockedResponseIsBadGateway(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantInErr   string
		existingTxt string
	}{
		{
			name:        "finishReason SAFETY",
			body:        `{"candidates":[{"content":{"parts":[]},"finishReason":"SAFETY"}]}`,
			wantInErr:   "SAFETY",
			existingTxt: "original",
		},
		{
			name:        "finishReason PROHIBITED_CONTENT",
			body:        `{"candidates":[{"content":{"parts":[]},"finishReason":"PROHIBITED_CONTENT"}]}`,
			wantInErr:   "PROHIBITED_CONTENT",
			existingTxt: "original",
		},
		{
			name:        "promptFeedback blockReason",
			body:        `{"candidates":[],"promptFeedback":{"blockReason":"OTHER"}}`,
			wantInErr:   "OTHER",
			existingTxt: "original",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			memo := MemoItem{ID: "memo-img-blocked", Type: "image", ImageData: extractTestPNG, Transcript: tc.existingTxt}
			srv, memoPath, _ := newExtractTestServer(t, []MemoItem{memo}, []custommodels.CustomModel{testGCPModel()})

			before, err := os.ReadFile(memoPath)
			if err != nil {
				t.Fatal(err)
			}
			stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			})

			rec := postExtract(t, srv, `{"memo_id":"memo-img-blocked"}`)
			if rec.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want 502; body = %s", rec.Code, rec.Body.String())
			}
			payload := decodeExtractResponse(t, rec)
			if payload["success"] != false {
				t.Errorf("success = %v, want false", payload["success"])
			}
			msg, _ := payload["error"].(string)
			if !strings.Contains(msg, tc.wantInErr) {
				t.Errorf("error = %q, want it to mention %q", msg, tc.wantInErr)
			}
			if got := storedTranscript(t, memoPath, "memo-img-blocked"); got != tc.existingTxt {
				t.Errorf("transcript changed to %q on a blocked response", got)
			}
			after, err := os.ReadFile(memoPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Error("memos file was rewritten on a blocked response")
			}
		})
	}
}

func TestMemosExtractTimeoutIsGatewayTimeout(t *testing.T) {
	memo := MemoItem{ID: "memo-img-timeout", Type: "image", ImageData: extractTestPNG, Transcript: "original"}
	srv, memoPath, _ := newExtractTestServer(t, []MemoItem{memo}, []custommodels.CustomModel{testGCPModel()})

	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream must not be reached when the request context is already expired")
	})

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Minute))
	defer cancel()

	rec := postExtractWithContext(t, srv, ctx, `{"memo_id":"memo-img-timeout"}`)
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504; body = %s", rec.Code, rec.Body.String())
	}
	payload := decodeExtractResponse(t, rec)
	if payload["success"] != false {
		t.Errorf("success = %v, want false", payload["success"])
	}
	msg, _ := payload["error"].(string)
	if !strings.Contains(msg, "timed out") {
		t.Errorf("error = %q, want a timeout explanation", msg)
	}
	if got := storedTranscript(t, memoPath, "memo-img-timeout"); got != "original" {
		t.Errorf("transcript changed to %q on timeout", got)
	}
}

func TestMemosExtractMalformedUpstreamResponse(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "not json", body: `<html>gateway error</html>`},
		{name: "missing candidates field", body: `{"promptFeedback":null}`},
		{name: "candidates wrong type", body: `{"candidates":"nope"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			memo := MemoItem{ID: "memo-img-malformed", Type: "image", ImageData: extractTestPNG, Transcript: "original"}
			srv, memoPath, _ := newExtractTestServer(t, []MemoItem{memo}, []custommodels.CustomModel{testGCPModel()})

			stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			})

			rec := postExtract(t, srv, `{"memo_id":"memo-img-malformed"}`)
			if rec.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want 502; body = %s", rec.Code, rec.Body.String())
			}
			payload := decodeExtractResponse(t, rec)
			if payload["success"] != false {
				t.Errorf("success = %v, want false", payload["success"])
			}
			if got := storedTranscript(t, memoPath, "memo-img-malformed"); got != "original" {
				t.Errorf("transcript changed to %q on a malformed response", got)
			}
		})
	}
}

func TestMemosExtractPrefersConfiguredDefaultModel(t *testing.T) {
	first := testGCPModel()
	first.ID = "first-model"
	first.Name = "gemini-first"
	preferred := testGCPModel()
	preferred.ID = "preferred-model"
	preferred.Name = "gemini-preferred"

	memo := MemoItem{ID: "memo-img-default", Type: "image", ImageData: extractTestPNG}
	srv, _, _ := newExtractTestServer(t, []MemoItem{memo}, []custommodels.CustomModel{first, preferred})

	cfgPath := filepath.Join(os.Getenv("ANTIGRAVITY_SWISS_CONFIG_DIR"), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"default_custom_model":"preferred-model"}`), 0600); err != nil {
		t.Fatal(err)
	}

	calledModel := ""
	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	})
	inner := memoExtractEndpoint
	memoExtractEndpoint = func(region, project, model string) string {
		calledModel = model
		return inner(region, project, model)
	}

	rec := postExtract(t, srv, `{"memo_id":"memo-img-default"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if calledModel != "gemini-preferred" {
		t.Errorf("called model = %q, want the configured default gemini-preferred", calledModel)
	}
}

// TestCallVertexExtractSendsVertexCompatibleBody pins the exact shape Vertex
// accepts. Without contents[0].role the real API rejects every request with
// HTTP 400 "Please use a valid role: user, model.", so this test must fail if
// the role moves or disappears — even though the response is mocked.
func TestCallVertexExtractSendsVertexCompatibleBody(t *testing.T) {
	capture := &extractUpstreamCapture{}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captureExtractRequest(t, r, capture)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"transcribed"}]}}]}`))
	}))
	defer fake.Close()

	oldClient := memoExtractHTTPClient
	client := fake.Client()
	client.Timeout = memoExtractTimeout
	memoExtractHTTPClient = client
	t.Cleanup(func() { memoExtractHTTPClient = oldClient })

	text, err := callVertexExtract(context.Background(), fake.URL+extractTestEndpointPath,
		"test-token", memoExtractAudioPrompt, "audio/webm",
		encodeTestMedia(0x1A, 0x45, 0xDF, 0xA3, 0x42))
	if err != nil {
		t.Fatalf("callVertexExtract: %v", err)
	}
	if text != "transcribed" {
		t.Errorf("text = %q, want transcribed", text)
	}
	if capture.role != "user" || capture.partCount != 2 {
		t.Fatalf("request shape = role %q, %d parts; want role user and 2 parts",
			capture.role, capture.partCount)
	}
	if capture.auth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want Bearer test-token", capture.auth)
	}
	if capture.method != http.MethodPost || capture.path != extractTestEndpointPath {
		t.Errorf("request = %s %s, want POST %s", capture.method, capture.path, extractTestEndpointPath)
	}
	if capture.prompt != memoExtractAudioPrompt {
		t.Errorf("parts[0].text = %q, want the audio prompt", capture.prompt)
	}
	if capture.mimeType != "audio/webm" {
		t.Errorf("parts[1].inlineData.mimeType = %q, want audio/webm", capture.mimeType)
	}
	if capture.mediaData == "" {
		t.Error("parts[1].inlineData.data is empty")
	}
}

// TestMemoExtractEndpointHitsGenerateContent guards the real (non-mocked)
// endpoint builder: the global location must use the region-less host and the
// :generateContent action.
func TestMemoExtractEndpointHitsGenerateContent(t *testing.T) {
	endpoint := custommodels.BuildVertexAIEndpoint("global", "proj-1", "gemini-4-argon")
	want := "https://aiplatform.googleapis.com/v1/projects/proj-1/locations/global/publishers/google/models/gemini-4-argon:generateContent"
	if endpoint != want {
		t.Errorf("endpoint = %q, want %q", endpoint, want)
	}
}
