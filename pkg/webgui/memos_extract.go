package webgui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
)

const (
	memoExtractAudioPrompt = "Transcribe this audio verbatim. Output only the transcript."
	memoExtractImagePrompt = "Extract all text from this image (OCR). Output only the text. If there is no text, output an empty string."

	memoExtractNoModelMessage = "no multimodal model configured — add a GCP custom model with a service-account key to enable transcription/OCR"

	memoExtractTimeout     = 60 * time.Second
	memoExtractMaxBodySize = 4 << 20
)

// Injectable dependencies so tests can exercise the handler without reaching Google.
var (
	memoExtractHTTPClient  = &http.Client{Timeout: memoExtractTimeout}
	memoExtractTokenMinter = custommodels.GenerateGCPAccessToken
	memoExtractEndpoint    = custommodels.BuildVertexAIEndpoint
)

// memoAutoExtractRunner launches the post-save extraction off the save path.
// Tests replace it with a synchronous runner so assertions cannot race the
// background goroutine.
var memoAutoExtractRunner = func(run func()) { go run() }

// memoExtractLanguages maps the stored language codes to the human-readable
// names used in prompts. Keys are lower case so lookups stay case-insensitive.
var memoExtractLanguages = map[string]string{
	"en":      "English",
	"zh-hans": "Simplified Chinese",
	"zh-hant": "Traditional Chinese",
	"ja":      "Japanese",
	"ko":      "Korean",
	"es":      "Spanish",
	"fr":      "French",
	"de":      "German",
	"pt":      "Portuguese",
	"ru":      "Russian",
	"ar":      "Arabic",
	"hi":      "Hindi",
	"it":      "Italian",
	"nl":      "Dutch",
	"tr":      "Turkish",
	"vi":      "Vietnamese",
	"th":      "Thai",
	"id":      "Indonesian",
}

// memoExtractLanguageName returns the prompt-facing name for a stored code, or
// "" when the code means automatic detection, is empty, or is unknown.
func memoExtractLanguageName(code string) string {
	normalized := strings.ToLower(strings.TrimSpace(code))
	if normalized == "" || normalized == core.MemoLanguageAuto {
		return ""
	}
	return memoExtractLanguages[normalized]
}

// extractionPrompt builds the instruction sent to the model for a memo kind
// ("audio" or "image"). "auto", empty and unknown language codes keep the
// language-agnostic prompt; an explicit, known code names the language.
func extractionPrompt(kind, languageCode string) string {
	language := memoExtractLanguageName(languageCode)
	switch kind {
	case "audio":
		if language == "" {
			return memoExtractAudioPrompt
		}
		return fmt.Sprintf("The audio is in %s. Transcribe it verbatim. Output only the transcript.", language)
	case "image":
		if language == "" {
			return memoExtractImagePrompt
		}
		return fmt.Sprintf(
			"Extract all text from this image in %s (OCR). Output only the text. If there is no text, output an empty string.",
			language)
	}
	return ""
}

// memoNeedsExtraction reports whether a memo holds media that can be extracted.
func memoNeedsExtraction(memo MemoItem) bool {
	switch memo.Type {
	case "audio":
		return strings.TrimSpace(memo.AudioData) != ""
	case "image":
		return strings.TrimSpace(memo.ImageData) != ""
	}
	return false
}

// scheduleMemoAutoExtract starts a background extraction when automatic
// extraction is enabled and the just-saved memo has media but no transcript.
// Call it only after the save response has been written: the save must never
// block on or fail because of extraction.
func (s *Server) scheduleMemoAutoExtract(memo MemoItem, storagePath string, memoCfg core.MemoConfig) {
	if !memoCfg.AutoExtract || strings.TrimSpace(memo.Transcript) != "" || !memoNeedsExtraction(memo) {
		return
	}
	memoAutoExtractRunner(func() {
		s.autoExtractMemo(memo.ID, storagePath, memoCfg)
	})
}

// autoExtractMemo re-reads the memo from storage before extracting so a memo
// that gained a transcript (or was deleted) since the save is skipped.
func (s *Server) autoExtractMemo(id, storagePath string, memoCfg core.MemoConfig) {
	s.memoMu.RLock()
	memos := readMemosFile(storagePath)
	s.memoMu.RUnlock()

	var memo *MemoItem
	for i := range memos {
		if memos[i].ID == id {
			found := memos[i]
			memo = &found
			break
		}
	}
	if memo == nil || strings.TrimSpace(memo.Transcript) != "" || !memoNeedsExtraction(*memo) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), memoExtractTimeout)
	defer cancel()
	if _, _, err := s.extractMemo(ctx, memo, storagePath, memoCfg); err != nil {
		log.Printf("[memos] automatic extraction failed for memo %s: %v", id, err)
	}
}

// handleMemosExtract transcribes an audio memo or OCRs an image memo with a GCP
// multimodal model and persists the result into the memo's transcript field.
func (s *Server) handleMemosExtract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeExtractError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		MemoID string `json:"memo_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeExtractError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	memoID := strings.TrimSpace(req.MemoID)
	if memoID == "" {
		writeExtractError(w, http.StatusBadRequest, "memo_id is required")
		return
	}

	memo, storagePath, err := s.findMemoByID(memoID, s.resolveWorkspacePath(r))
	if err != nil {
		writeExtractError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), memoExtractTimeout)
	defer cancel()

	transcript, empty, err := s.extractMemo(ctx, memo, storagePath, loadMemoConfig())
	if err != nil {
		var upstream *upstreamExtractError
		if errors.As(err, &upstream) {
			writeExtractError(w, upstream.Status, upstream.Message)
			return
		}
		writeExtractError(w, http.StatusBadGateway, err.Error())
		return
	}

	if empty {
		// Never overwrite an existing transcript with nothing. When the model
		// legitimately finds no text (e.g. an image without text) or returns no
		// candidates at all, the memo's current transcript is the best data we
		// have, so report it back unchanged and leave the file untouched.
		writeJSON(w, map[string]interface{}{
			"success":    true,
			"transcript": memo.Transcript,
			"empty":      true,
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"success":    true,
		"transcript": transcript,
	})
}

// loadMemoConfig returns the persisted memo settings, falling back to defaults
// when the config cannot be read.
func loadMemoConfig() core.MemoConfig {
	cfg, err := core.LoadConfig()
	if err != nil || cfg == nil {
		return core.DefaultConfig().GetMemoConfig()
	}
	return cfg.GetMemoConfig()
}

// extractMemo is the shared extraction pipeline used by the HTTP handler and the
// post-save automatic path: it validates the media, selects the configured
// model, authenticates, calls the model and persists the transcript. empty
// reports a model response without text, in which case nothing is written.
func (s *Server) extractMemo(ctx context.Context, memo *MemoItem, storagePath string, memoCfg core.MemoConfig) (transcript string, empty bool, err error) {
	var media, prompt string
	switch memo.Type {
	case "audio":
		media = memo.AudioData
		prompt = extractionPrompt("audio", memoCfg.VoiceLanguage)
	case "image":
		media = memo.ImageData
		prompt = extractionPrompt("image", memoCfg.OCRLanguage)
	default:
		return "", false, &upstreamExtractError{
			Status: http.StatusBadRequest,
			Message: fmt.Sprintf("memo %q has type %q; only audio and image memos can be extracted",
				memo.ID, memo.Type),
		}
	}
	if strings.TrimSpace(media) == "" {
		return "", false, &upstreamExtractError{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("memo %q has no %s data to extract", memo.ID, memo.Type),
		}
	}

	payload, mimeType, err := normalizeMemoMedia(media, memo.Type)
	if err != nil {
		return "", false, &upstreamExtractError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	model, err := selectMemoExtractModel(s.getCustomModelsStore())
	if err != nil {
		return "", false, &upstreamExtractError{Status: http.StatusUnprocessableEntity, Message: err.Error()}
	}

	saKey, err := custommodels.ParseServiceAccountKey([]byte(model.GCPServiceAccountKey))
	if err != nil {
		return "", false, &upstreamExtractError{
			Status:  http.StatusBadGateway,
			Message: fmt.Sprintf("configured model %q has an invalid service-account key: %v", model.Name, err),
		}
	}

	projectID := strings.TrimSpace(model.GCPProjectID)
	if projectID == "" {
		projectID = saKey.ProjectID
	}

	token, err := memoExtractTokenMinter(ctx, saKey)
	if err != nil {
		return "", false, &upstreamExtractError{
			Status:  http.StatusBadGateway,
			Message: shortExtractError("could not authenticate with Google Cloud", err),
		}
	}

	endpoint := memoExtractEndpoint(model.GCPRegion, projectID, model.Name)
	text, err := callVertexExtract(ctx, endpoint, token, prompt, mimeType, payload)
	if err != nil {
		var upstream *upstreamExtractError
		if errors.As(err, &upstream) {
			return "", false, upstream
		}
		return "", false, &upstreamExtractError{Status: http.StatusBadGateway, Message: err.Error()}
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", true, nil
	}

	if err := s.persistMemoTranscript(memo.ID, storagePath, text); err != nil {
		return "", false, &upstreamExtractError{
			Status:  http.StatusInternalServerError,
			Message: "failed to persist transcript: " + err.Error(),
		}
	}
	return text, false, nil
}

// findMemoByID locates a memo in the current workspace, known workspaces, or the
// global store and returns the file it was found in so the transcript can be
// written back to the same location.
func (s *Server) findMemoByID(id, ws string) (*MemoItem, string, error) {
	var candidates []string
	seen := make(map[string]struct{})
	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		candidates = append(candidates, p)
	}

	if ws != "" {
		add(filepath.Join(ws, ".antigravity", "memos.json"))
	}

	s.memoMu.RLock()
	for known := range s.knownWorkspaces {
		if known != "" {
			add(filepath.Join(known, ".antigravity", "memos.json"))
		}
	}
	add(s.getGlobalMemosPath())
	s.memoMu.RUnlock()

	for _, path := range candidates {
		memos := readMemosFile(path)
		for i := range memos {
			if memos[i].ID == id {
				memo := memos[i]
				return &memo, path, nil
			}
		}
	}
	return nil, "", fmt.Errorf("memo %q not found", id)
}

// persistMemoTranscript writes the transcript back to the file the memo lives in.
func (s *Server) persistMemoTranscript(id, storagePath, transcript string) error {
	s.memoMu.Lock()
	defer s.memoMu.Unlock()

	memos := readMemosFile(storagePath)
	for i := range memos {
		if memos[i].ID == id {
			memos[i].Transcript = transcript
			return writeMemosFile(storagePath, memos)
		}
	}
	return fmt.Errorf("memo %q no longer exists", id)
}

// selectMemoExtractModel picks the configured default custom model when it is a
// GCP model with a service-account key, otherwise the first such enabled model.
func selectMemoExtractModel(store *custommodels.Store) (*custommodels.CustomModel, error) {
	noModel := errors.New(memoExtractNoModelMessage)
	if store == nil {
		return nil, noModel
	}

	cfg := store.GetConfig()
	eligible := func(m *custommodels.CustomModel) bool {
		if m.ProviderType != custommodels.ProviderGCP || !m.Enabled {
			return false
		}
		key := strings.TrimSpace(m.GCPServiceAccountKey)
		return key != "" && key != "[REDACTED]"
	}

	defaultID := ""
	if coreCfg, err := core.LoadConfig(); err == nil {
		defaultID = strings.TrimSpace(coreCfg.DefaultCustomModel)
	}
	if defaultID == "" {
		defaultID = strings.TrimSpace(cfg.ActiveModelID)
	}

	for i := range cfg.Models {
		if eligible(&cfg.Models[i]) && defaultID != "" &&
			(cfg.Models[i].ID == defaultID || cfg.Models[i].Name == defaultID) {
			pick := cfg.Models[i]
			return &pick, nil
		}
	}
	for i := range cfg.Models {
		if eligible(&cfg.Models[i]) && cfg.Models[i].IsDefault {
			pick := cfg.Models[i]
			return &pick, nil
		}
	}
	for i := range cfg.Models {
		if eligible(&cfg.Models[i]) {
			pick := cfg.Models[i]
			return &pick, nil
		}
	}
	return nil, noModel
}

// normalizeMemoMedia strips an optional data-URL wrapper, returns canonical
// base64 and the mime type detected from the payload's magic bytes.
func normalizeMemoMedia(raw, memoType string) (payload, mimeType string, err error) {
	s := strings.TrimSpace(raw)
	var declaredMime string
	if strings.HasPrefix(strings.ToLower(s), "data:") {
		if comma := strings.Index(s, ","); comma != -1 {
			meta := s[len("data:"):comma]
			if semi := strings.Index(meta, ";"); semi != -1 {
				meta = meta[:semi]
			}
			declaredMime = strings.TrimSpace(meta)
			s = s[comma+1:]
		}
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	if s == "" {
		return "", "", errors.New("media payload is empty")
	}

	head, err := decodeBase64Prefix(s, 4096)
	if err != nil {
		return "", "", errors.New("media payload is not valid base64")
	}

	mimeType = detectMemoMediaMime(head)
	if mimeType == "" && (strings.HasPrefix(declaredMime, "audio/") || strings.HasPrefix(declaredMime, "image/")) {
		mimeType = declaredMime
	}
	if mimeType == "" {
		return "", "", fmt.Errorf("could not detect the %s format from the payload; unsupported media type", memoType)
	}
	return s, mimeType, nil
}

// decodeBase64Prefix decodes at most maxChars characters of a base64 string.
func decodeBase64Prefix(s string, maxChars int) ([]byte, error) {
	if len(s) > maxChars {
		s = s[:maxChars]
	}
	if rem := len(s) % 4; rem != 0 {
		s = s[:len(s)-rem]
	}
	if s == "" {
		return nil, errors.New("empty base64 payload")
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
}

// detectMemoMediaMime inspects the leading bytes of a decoded media payload.
func detectMemoMediaMime(head []byte) string {
	switch {
	case len(head) >= 8 && bytes.Equal(head[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return "image/png"
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return "image/jpeg"
	case len(head) >= 6 && (string(head[:6]) == "GIF87a" || string(head[:6]) == "GIF89a"):
		return "image/gif"
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return "image/webp"
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		return "audio/wav"
	case len(head) >= 4 && string(head[:4]) == "OggS":
		return "audio/ogg"
	case len(head) >= 4 && bytes.Equal(head[:4], []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return "audio/webm"
	case len(head) >= 12 && string(head[4:8]) == "ftyp":
		return "audio/mp4"
	case len(head) >= 4 && string(head[:4]) == "fLaC":
		return "audio/flac"
	case len(head) >= 3 && string(head[:3]) == "ID3":
		return "audio/mpeg"
	case len(head) >= 2 && head[0] == 0xFF && (head[1]&0xF6) == 0xF0:
		return "audio/aac"
	default:
		return ""
	}
}

// upstreamExtractError carries the HTTP status an upstream failure maps to.
type upstreamExtractError struct {
	Status  int
	Message string
}

func (e *upstreamExtractError) Error() string { return e.Message }

type vertexExtractPart struct {
	Text       string               `json:"text,omitempty"`
	InlineData *vertexExtractInline `json:"inlineData,omitempty"`
}

type vertexExtractInline struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"`
}

type vertexExtractContent struct {
	Role  string              `json:"role"`
	Parts []vertexExtractPart `json:"parts"`
}

type vertexExtractRequest struct {
	Contents []vertexExtractContent `json:"contents"`
}

// callVertexExtract posts the prompt and media to a Vertex AI generateContent
// endpoint and returns the first candidate's text.
func callVertexExtract(ctx context.Context, endpoint, token, prompt, mimeType, payload string) (string, error) {
	// Vertex rejects contents without an explicit role ("Please use a valid
	// role: user, model."), so the role must sit alongside parts on the content
	// object, not on the individual parts.
	reqBody := vertexExtractRequest{
		Contents: []vertexExtractContent{
			{
				Role: "user",
				Parts: []vertexExtractPart{
					{Text: prompt},
					{InlineData: &vertexExtractInline{MIMEType: mimeType, Data: payload}},
				},
			},
		},
	}
	encoded, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := memoExtractHTTPClient.Do(req)
	if err != nil {
		if isTimeoutError(ctx, err) {
			return "", &upstreamExtractError{
				Status:  http.StatusGatewayTimeout,
				Message: "the model request timed out after 60 seconds; try again",
			}
		}
		return "", &upstreamExtractError{
			Status:  http.StatusBadGateway,
			Message: "failed to reach the configured model endpoint; check the GCP region and connectivity",
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, memoExtractMaxBodySize))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &upstreamExtractError{
			Status: http.StatusBadGateway,
			Message: fmt.Sprintf("model request failed (HTTP %d): %s",
				resp.StatusCode, summarizeVertexErrorBody(body)),
		}
	}

	var parsed struct {
		Candidates *[]struct {
			FinishReason string `json:"finishReason"`
			Content      struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		PromptFeedback *struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", &upstreamExtractError{
			Status:  http.StatusBadGateway,
			Message: "model returned an unreadable response",
		}
	}
	if parsed.Candidates == nil {
		return "", &upstreamExtractError{
			Status:  http.StatusBadGateway,
			Message: "model returned an unreadable response: no candidates field",
		}
	}

	for _, candidate := range *parsed.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				return part.Text, nil
			}
		}
	}

	if parsed.PromptFeedback != nil {
		if reason := strings.TrimSpace(parsed.PromptFeedback.BlockReason); reason != "" {
			return "", &upstreamExtractError{
				Status: http.StatusBadGateway,
				Message: fmt.Sprintf(
					"the model blocked this request (%s); nothing was saved — try a different file or review the content policy",
					reason),
			}
		}
	}
	for _, candidate := range *parsed.Candidates {
		reason := strings.ToUpper(strings.TrimSpace(candidate.FinishReason))
		if memoExtractBlockedFinishReasons[reason] {
			return "", &upstreamExtractError{
				Status: http.StatusBadGateway,
				Message: fmt.Sprintf(
					"the model refused to process this media (%s); nothing was saved — try a different file or review the content policy",
					reason),
			}
		}
	}
	return "", nil
}

// memoExtractBlockedFinishReasons are finish reasons that mean the model
// declined to produce output for policy or safety reasons rather than simply
// finding no text. They must surface as an error, not as an empty transcript.
var memoExtractBlockedFinishReasons = map[string]bool{
	"SAFETY":             true,
	"RECITATION":         true,
	"BLOCKLIST":          true,
	"PROHIBITED_CONTENT": true,
}

func isTimeoutError(ctx context.Context, err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func summarizeVertexErrorBody(body []byte) string {
	var apiErr struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	msg := ""
	if json.Unmarshal(body, &apiErr) == nil {
		msg = strings.TrimSpace(apiErr.Error.Message)
	}
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	if msg == "" {
		msg = "empty error response"
	}
	runes := []rune(msg)
	if len(runes) > 300 {
		msg = string(runes[:300]) + "…"
	}
	return msg
}

func shortExtractError(prefix string, err error) string {
	msg := err.Error()
	runes := []rune(msg)
	if len(runes) > 240 {
		msg = string(runes[:240]) + "…"
	}
	return prefix + ": " + msg
}

func writeExtractError(w http.ResponseWriter, status int, msg string) {
	writeJSONStatus(w, status, map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}
