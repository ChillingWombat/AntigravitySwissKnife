package webgui

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
)

func TestExtractionPrompt(t *testing.T) {
	const audioEN = "The audio is in English. Transcribe it verbatim. Output only the transcript."
	const imageEN = "Extract all text from this image in English (OCR). Output only the text. If there is no text, output an empty string."
	const audioZH = "The audio is in Simplified Chinese. Transcribe it verbatim. Output only the transcript."
	const imageZH = "Extract all text from this image in Simplified Chinese (OCR). Output only the text. If there is no text, output an empty string."

	cases := []struct {
		name     string
		kind     string
		language string
		want     string
	}{
		{name: "audio auto", kind: "audio", language: "auto", want: memoExtractAudioPrompt},
		{name: "audio empty", kind: "audio", language: "", want: memoExtractAudioPrompt},
		{name: "audio unknown", kind: "audio", language: "xx", want: memoExtractAudioPrompt},
		{name: "audio en", kind: "audio", language: "en", want: audioEN},
		{name: "audio uppercase code", kind: "audio", language: "EN", want: audioEN},
		{name: "audio zh-Hans", kind: "audio", language: "zh-Hans", want: audioZH},
		{name: "audio zh-Hant", kind: "audio", language: "zh-Hant", want: "The audio is in Traditional Chinese. Transcribe it verbatim. Output only the transcript."},
		{name: "audio ja", kind: "audio", language: "ja", want: "The audio is in Japanese. Transcribe it verbatim. Output only the transcript."},
		{name: "image auto", kind: "image", language: "auto", want: memoExtractImagePrompt},
		{name: "image empty", kind: "image", language: "", want: memoExtractImagePrompt},
		{name: "image unknown", kind: "image", language: "klingon", want: memoExtractImagePrompt},
		{name: "image en", kind: "image", language: "en", want: imageEN},
		{name: "image zh-Hans", kind: "image", language: "zh-Hans", want: imageZH},
		{name: "image ko", kind: "image", language: "ko", want: "Extract all text from this image in Korean (OCR). Output only the text. If there is no text, output an empty string."},
		{name: "image surrounded by spaces", kind: "image", language: "  fr  ", want: "Extract all text from this image in French (OCR). Output only the text. If there is no text, output an empty string."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractionPrompt(tc.kind, tc.language); got != tc.want {
				t.Errorf("extractionPrompt(%q, %q) = %q, want %q", tc.kind, tc.language, got, tc.want)
			}
		})
	}
}

func TestExtractionPromptCoversEveryMappedLanguage(t *testing.T) {
	for code, name := range memoExtractLanguages {
		prompt := extractionPrompt("audio", code)
		want := "The audio is in " + name + ". Transcribe it verbatim. Output only the transcript."
		if prompt != want {
			t.Errorf("extractionPrompt(audio, %q) = %q, want %q", code, prompt, want)
		}
		if prompt == memoExtractAudioPrompt {
			t.Errorf("language %q fell back to the auto prompt", code)
		}
	}
}

// writeMemoTestConfig writes a config.json containing only the given memo
// settings; every other setting keeps its default.
func writeMemoTestConfig(t *testing.T, memo map[string]interface{}) {
	t.Helper()
	dir := os.Getenv("ANTIGRAVITY_SWISS_CONFIG_DIR")
	if dir == "" {
		t.Fatal("ANTIGRAVITY_SWISS_CONFIG_DIR is not set; call newExtractTestServer first")
	}
	data, err := json.Marshal(map[string]interface{}{"memo": memo})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// runAutoExtractInline makes the post-save extraction synchronous so tests can
// assert on its result without racing the goroutine.
func runAutoExtractInline(t *testing.T) {
	t.Helper()
	old := memoAutoExtractRunner
	memoAutoExtractRunner = func(run func()) { run() }
	t.Cleanup(func() { memoAutoExtractRunner = old })
}

func postSaveMemo(t *testing.T, srv *Server, memo MemoItem) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(memo)
	if err != nil {
		t.Fatalf("marshal memo: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/memos/save", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.handleMemosSave(rec, req)
	return rec
}

func testWebmAudio() string {
	return base64.StdEncoding.EncodeToString([]byte{0x1A, 0x45, 0xDF, 0xA3, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm'})
}

func TestMemoSaveAutoExtractTranscribesAudio(t *testing.T) {
	srv, memoPath, _ := newExtractTestServer(t, nil, []custommodels.CustomModel{testGCPModel()})
	writeMemoTestConfig(t, map[string]interface{}{"auto_extract": true, "voice_language": "ja"})
	runAutoExtractInline(t)

	capture := &extractUpstreamCapture{}
	calls := 0
	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		captureExtractRequest(t, r, capture)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"  hello audio  "}]}}]}`))
	})

	rec := postSaveMemo(t, srv, MemoItem{ID: "memo-auto-1", Type: "audio", AudioData: testWebmAudio()})
	if rec.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var saved struct {
		Success bool     `json:"success"`
		Memo    MemoItem `json:"memo"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("decode save response %q: %v", rec.Body.String(), err)
	}
	if !saved.Success || saved.Memo.ID != "memo-auto-1" {
		t.Fatalf("unexpected save response: %s", rec.Body.String())
	}

	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls)
	}
	if got := storedTranscript(t, memoPath, "memo-auto-1"); got != "hello audio" {
		t.Errorf("persisted transcript = %q, want hello audio", got)
	}
	if want := extractionPrompt("audio", "ja"); capture.prompt != want {
		t.Errorf("prompt = %q, want %q", capture.prompt, want)
	}
	if capture.mimeType != "audio/webm" {
		t.Errorf("mimeType = %q, want audio/webm", capture.mimeType)
	}
}

func TestMemoSaveAutoExtractUsesOCRPromptForImages(t *testing.T) {
	srv, memoPath, _ := newExtractTestServer(t, nil, []custommodels.CustomModel{testGCPModel()})
	writeMemoTestConfig(t, map[string]interface{}{"auto_extract": true, "ocr_language": "zh-Hant"})
	runAutoExtractInline(t)

	capture := &extractUpstreamCapture{}
	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		captureExtractRequest(t, r, capture)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"image text"}]}}]}`))
	})

	rec := postSaveMemo(t, srv, MemoItem{ID: "memo-auto-img", Type: "image", ImageData: extractTestPNG})
	if rec.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := storedTranscript(t, memoPath, "memo-auto-img"); got != "image text" {
		t.Errorf("persisted transcript = %q, want image text", got)
	}
	if want := extractionPrompt("image", "zh-Hant"); capture.prompt != want {
		t.Errorf("prompt = %q, want %q", capture.prompt, want)
	}
}

func TestMemoSaveAutoExtractDisabledMakesNoCall(t *testing.T) {
	srv, memoPath, _ := newExtractTestServer(t, nil, []custommodels.CustomModel{testGCPModel()})
	// No config write: auto_extract defaults to false (manual extraction).
	runAutoExtractInline(t)

	calls := 0
	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
	})

	rec := postSaveMemo(t, srv, MemoItem{ID: "memo-manual-1", Type: "audio", AudioData: testWebmAudio()})
	if rec.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if calls != 0 {
		t.Errorf("upstream calls = %d, want 0 when auto_extract is false", calls)
	}
	if got := storedTranscript(t, memoPath, "memo-manual-1"); got != "" {
		t.Errorf("transcript = %q, want empty", got)
	}
}

func TestMemoSaveAutoExtractSkipsWhenTranscriptExists(t *testing.T) {
	srv, memoPath, _ := newExtractTestServer(t, nil, []custommodels.CustomModel{testGCPModel()})
	writeMemoTestConfig(t, map[string]interface{}{"auto_extract": true})
	runAutoExtractInline(t)

	calls := 0
	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
	})

	rec := postSaveMemo(t, srv, MemoItem{
		ID:         "memo-auto-existing",
		Type:       "audio",
		AudioData:  testWebmAudio(),
		Transcript: "already transcribed",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if calls != 0 {
		t.Errorf("upstream calls = %d, want 0 when a transcript already exists", calls)
	}
	if got := storedTranscript(t, memoPath, "memo-auto-existing"); got != "already transcribed" {
		t.Errorf("transcript = %q, want the existing value untouched", got)
	}
}

func TestMemoSaveAutoExtractIgnoresMemosWithoutMedia(t *testing.T) {
	cases := []struct {
		name string
		memo MemoItem
	}{
		{name: "text memo", memo: MemoItem{ID: "memo-auto-text", Type: "text", Content: "hello"}},
		{name: "audio memo without payload", memo: MemoItem{ID: "memo-auto-noaudio", Type: "audio"}},
		{name: "image memo without payload", memo: MemoItem{ID: "memo-auto-noimage", Type: "image"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _, _ := newExtractTestServer(t, nil, []custommodels.CustomModel{testGCPModel()})
			writeMemoTestConfig(t, map[string]interface{}{"auto_extract": true})
			runAutoExtractInline(t)

			calls := 0
			stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
			})

			rec := postSaveMemo(t, srv, tc.memo)
			if rec.Code != http.StatusOK {
				t.Fatalf("save status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if calls != 0 {
				t.Errorf("upstream calls = %d, want 0 for a memo that cannot be extracted", calls)
			}
		})
	}
}

func TestMemoSaveAutoExtractFailureKeepsTheSave(t *testing.T) {
	srv, memoPath, _ := newExtractTestServer(t, nil, []custommodels.CustomModel{testGCPModel()})
	writeMemoTestConfig(t, map[string]interface{}{"auto_extract": true})
	runAutoExtractInline(t)

	stubExtractUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"backend exploded"}}`))
	})

	rec := postSaveMemo(t, srv, MemoItem{ID: "memo-auto-fail", Type: "audio", AudioData: testWebmAudio()})
	if rec.Code != http.StatusOK {
		t.Fatalf("save must succeed even when extraction fails, status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var saved struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil || !saved.Success {
		t.Fatalf("unexpected save response %q: %v", rec.Body.String(), err)
	}
	if got := storedTranscript(t, memoPath, "memo-auto-fail"); got != "" {
		t.Errorf("transcript = %q, want empty after a failed extraction", got)
	}
	memos := readMemosFile(memoPath)
	if len(memos) != 1 || memos[0].ID != "memo-auto-fail" {
		t.Errorf("saved memo was lost: %+v", memos)
	}
}

func TestMemosConfigEndpointRoundTripsExtractionSettings(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	srv := &Server{}

	get := func() map[string]interface{} {
		rec := httptest.NewRecorder()
		srv.handleMemosConfig(rec, httptest.NewRequest(http.MethodGet, "/api/memos/config", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d", rec.Code)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode GET response %q: %v", rec.Body.String(), err)
		}
		return payload
	}
	post := func(body string) map[string]interface{} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/memos/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		srv.handleMemosConfig(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode POST response %q: %v", rec.Body.String(), err)
		}
		return payload
	}

	defaults := get()
	if defaults["auto_extract"] != false {
		t.Errorf("auto_extract = %v, want false by default", defaults["auto_extract"])
	}
	if defaults["voice_language"] != core.MemoLanguageAuto || defaults["ocr_language"] != core.MemoLanguageAuto {
		t.Errorf("languages = %v/%v, want auto", defaults["voice_language"], defaults["ocr_language"])
	}

	post(`{"config":{"auto_extract":true,"voice_language":"ja","ocr_language":"zh-Hant"}}`)
	persisted := get()
	if persisted["auto_extract"] != true {
		t.Errorf("auto_extract = %v, want true after POST", persisted["auto_extract"])
	}
	if persisted["voice_language"] != "ja" || persisted["ocr_language"] != "zh-Hant" {
		t.Errorf("languages = %v/%v, want ja/zh-Hant", persisted["voice_language"], persisted["ocr_language"])
	}

	// A top-level field wins and an absent field keeps the stored value.
	post(`{"auto_extract":false,"voice_language":"ko"}`)
	partial := get()
	if partial["auto_extract"] != false {
		t.Errorf("auto_extract = %v, want false after top-level POST", partial["auto_extract"])
	}
	if partial["voice_language"] != "ko" {
		t.Errorf("voice_language = %v, want ko", partial["voice_language"])
	}
	if partial["ocr_language"] != "zh-Hant" {
		t.Errorf("ocr_language = %v, want the previous value preserved", partial["ocr_language"])
	}

	// Empty language values fall back to auto instead of being stored empty.
	post(`{"voice_language":"","ocr_language":""}`)
	cleared := get()
	if cleared["voice_language"] != core.MemoLanguageAuto || cleared["ocr_language"] != core.MemoLanguageAuto {
		t.Errorf("languages = %v/%v, want auto", cleared["voice_language"], cleared["ocr_language"])
	}

	// The persisted config must agree with what the API reported.
	cfg, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.GetMemoConfig().VoiceLanguage != core.MemoLanguageAuto {
		t.Errorf("persisted voice_language = %q, want auto", cfg.GetMemoConfig().VoiceLanguage)
	}
}
