package webgui

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// memoExportFolderName is the folder created under the OS download
	// directory. Every exported memo lands there so the user can find them.
	memoExportFolderName = "Antigravity Memos"
	// memoExportMaxNameRunes keeps "<name> (n).<ext>" well below the 255-byte
	// filename limit shared by NTFS, APFS and ext4 for non-ASCII titles.
	memoExportMaxNameRunes = 120
	// memoExportMaxAttempts bounds the " (2)", " (3)" … probe loop.
	memoExportMaxAttempts = 1000
)

// memoExportDirResolver returns the directory exported memos are written to.
// Tests replace it so they never touch the real Downloads folder.
var memoExportDirResolver = defaultMemoExportDir

// defaultMemoExportDir picks the OS-native download folder and appends the
// app-specific folder name. Linux prefers $XDG_DOWNLOAD_DIR when set.
func defaultMemoExportDir() (string, error) {
	if runtime.GOOS == "linux" {
		if xdg := strings.TrimSpace(os.Getenv("XDG_DOWNLOAD_DIR")); xdg != "" {
			return filepath.Join(xdg, memoExportFolderName), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", errors.New("could not determine the home directory for the export")
	}
	return filepath.Join(home, "Downloads", memoExportFolderName), nil
}

// memoExportExtensions maps the mime types detected by detectMemoMediaMime to
// the extension of the exported file.
var memoExportExtensions = map[string]string{
	"audio/webm":  ".webm",
	"audio/ogg":   ".ogg",
	"audio/wav":   ".wav",
	"audio/x-wav": ".wav",
	"audio/mp4":   ".mp4",
	"audio/mpeg":  ".mp3",
	"audio/aac":   ".aac",
	"audio/flac":  ".flac",
	"image/png":   ".png",
	"image/jpeg":  ".jpg",
	"image/webp":  ".webp",
	"image/gif":   ".gif",
}

// memoExportWindowsReserved are device names Windows refuses as file names,
// with or without an extension.
var memoExportWindowsReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// handleMemosExport writes a memo's payload to the OS download folder and
// returns the absolute, OS-native path it was written to.
func (s *Server) handleMemosExport(w http.ResponseWriter, r *http.Request) {
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

	memo, _, err := s.findMemoByID(memoID, s.resolveWorkspacePath(r))
	if err != nil {
		writeExtractError(w, http.StatusBadRequest, err.Error())
		return
	}

	data, ext, err := memoExportPayload(*memo)
	if err != nil {
		writeExtractError(w, http.StatusBadRequest, err.Error())
		return
	}

	dir, err := memoExportDirResolver()
	if err != nil {
		writeExtractError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeExtractError(w, http.StatusInternalServerError,
			fmt.Sprintf("could not create the export folder %q: %v", dir, err))
		return
	}

	path, err := writeMemoExportFile(dir, memoExportBaseName(*memo), ext, data)
	if err != nil {
		writeExtractError(w, http.StatusInternalServerError,
			fmt.Sprintf("could not write the memo file: %v", err))
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"path":    path,
	})
}

// memoExportPayload returns the bytes to write and the file extension for a
// memo. Text memos are written verbatim as UTF-8 (the title already lives in
// the filename, so no heading is prepended); media memos are base64-decoded
// and their extension comes from the magic bytes, not the client's claim.
func memoExportPayload(memo MemoItem) ([]byte, string, error) {
	switch memo.Type {
	case "text":
		return []byte(memo.Content), ".md", nil
	case "audio", "image":
		raw := memo.ImageData
		if memo.Type == "audio" {
			raw = memo.AudioData
		}
		payload, mimeType, err := normalizeMemoMedia(raw, memo.Type)
		if err != nil {
			return nil, "", fmt.Errorf("memo %q cannot be exported: %w", memo.ID, err)
		}
		ext, ok := memoExportExtensions[mimeType]
		if !ok {
			return nil, "", fmt.Errorf("memo %q has an unsupported %s format (%s)",
				memo.ID, memo.Type, mimeType)
		}
		data, err := decodeMemoBase64(payload)
		if err != nil {
			return nil, "", fmt.Errorf("memo %q media is not valid base64", memo.ID)
		}
		if len(data) == 0 {
			return nil, "", fmt.Errorf("memo %q has no %s data to export", memo.ID, memo.Type)
		}
		return data, ext, nil
	default:
		return nil, "", fmt.Errorf("memo %q has type %q; only text, audio and image memos can be exported",
			memo.ID, memo.Type)
	}
}

// decodeMemoBase64 decodes the canonical payload produced by
// normalizeMemoMedia, tolerating missing padding and the URL-safe alphabet.
func decodeMemoBase64(s string) ([]byte, error) {
	if data, err := base64.StdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	if data, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "=")); err == nil {
		return data, nil
	}
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

// memoExportBaseName derives a filesystem-safe name from the memo title,
// falling back to memo-<id> when the title is empty or entirely unusable.
func memoExportBaseName(memo MemoItem) string {
	base := sanitizeMemoExportName(stripLegacyVoicePrefix(memo.Title))
	if base == "" {
		if id := sanitizeMemoExportName(memo.ID); id != "" {
			base = "memo-" + id
		} else {
			base = "memo"
		}
	}
	// "CON" and friends are reserved regardless of extension on Windows, so
	// prefixing keeps the file writable there.
	if memoExportWindowsReserved[strings.ToUpper(base)] {
		base = "_" + base
	}
	return base
}

// stripLegacyVoicePrefix drops the "[Voice] " tag that memos recorded before
// the naming change carry in their stored title, so the exported file name
// matches the clean header the UI renders.
func stripLegacyVoicePrefix(title string) string {
	trimmed := strings.TrimSpace(title)
	const prefix = "[Voice]"
	if len(trimmed) >= len(prefix) && strings.EqualFold(trimmed[:len(prefix)], prefix) {
		return strings.TrimSpace(trimmed[len(prefix):])
	}
	return title
}

// sanitizeMemoExportName replaces separators, Windows-reserved characters and
// control characters with "_", trims leading/trailing dots and spaces (which
// Windows silently drops), and caps the length.
func sanitizeMemoExportName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < 0x20 || r == 0x7f:
			b.WriteRune('_')
		case strings.ContainsRune(`\/:*?"<>|`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	cleaned := strings.Trim(b.String(), " .")
	if runes := []rune(cleaned); len(runes) > memoExportMaxNameRunes {
		cleaned = strings.TrimSpace(string(runes[:memoExportMaxNameRunes]))
	}
	return cleaned
}

// writeMemoExportFile creates a new file without ever overwriting an existing
// one: on collision it appends " (2)", " (3)" … before the extension. It
// returns the absolute, OS-native path of the file it created.
func writeMemoExportFile(dir, base, ext string, data []byte) (string, error) {
	for n := 1; n <= memoExportMaxAttempts; n++ {
		name := base + ext
		if n > 1 {
			name = fmt.Sprintf("%s (%d)%s", base, n, ext)
		}
		path, err := filepath.Abs(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, writeErr := f.Write(data)
		closeErr := f.Close()
		if writeErr != nil {
			return "", writeErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		return path, nil
	}
	return "", fmt.Errorf("could not find an unused name for %q", base+ext)
}
