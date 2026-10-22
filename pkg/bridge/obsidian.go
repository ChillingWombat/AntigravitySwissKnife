package bridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Obsidian vaults are plain directories of Markdown notes marked by a .obsidian
// folder. These tools exist so an agent stops answering "I cannot read Obsidian":
// discovery, search and read/write go through the vault layout (wikilinks, tags,
// note titles) instead of raw file paths it would have to guess.

const (
	obsidianVaultCacheTTL = 60 * time.Second
	obsidianMaxScanDepth  = 4
	obsidianMaxNotes      = 20000
	obsidianSnippetRunes  = 180
)

var obsidianSkipDirs = map[string]bool{
	"node_modules": true, ".git": true, ".obsidian": true, ".trash": true,
	"dist": true, "build": true, ".cache": true, "venv": true, ".venv": true,
	"__pycache__": true, ".next": true, "target": true,
}

type obsidianVault struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Notes int    `json:"notes"`
}

var (
	obsidianMu       sync.Mutex
	obsidianCache    []obsidianVault
	obsidianCachedAt time.Time
)

// obsidianVaultRoots lists the directories scanned for .obsidian markers.
func obsidianVaultRoots() []string {
	roots := []string{}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots,
			home,
			filepath.Join(home, "Documents"),
			filepath.Join(home, "Notes"),
			filepath.Join(home, "Obsidian"),
			filepath.Join(home, "vaults"),
		)
	}
	if cfg, err := os.UserConfigDir(); err == nil && cfg != "" {
		roots = append(roots, filepath.Join(cfg, "obsidian"))
	}
	roots = append(roots, "/mnt/Data", "/media")
	seen := map[string]bool{}
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		if info, err := os.Stat(r); err == nil && info.IsDir() {
			out = append(out, r)
		}
	}
	return out
}

// DiscoverObsidianVaults scans the usual locations for .obsidian folders.
func DiscoverObsidianVaults(force bool) []obsidianVault {
	obsidianMu.Lock()
	defer obsidianMu.Unlock()
	if !force && time.Since(obsidianCachedAt) < obsidianVaultCacheTTL && obsidianCache != nil {
		return append([]obsidianVault{}, obsidianCache...)
	}

	found := []obsidianVault{}
	seen := map[string]bool{}
	dedupe := map[string]bool{}
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth > obsidianMaxScanDepth || seen[dir] {
			return
		}
		seen[dir] = true
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		hasMarker := false
		subdirs := make([]string, 0, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() {
				continue
			}
			if name == ".obsidian" {
				hasMarker = true
				continue
			}
			if strings.HasPrefix(name, ".") || obsidianSkipDirs[name] {
				continue
			}
			subdirs = append(subdirs, filepath.Join(dir, name))
		}
		if hasMarker {
			// Symlinked vaults (Documents/David -> /mnt/Data/Documents/David) are the
			// same vault: report it once, under its real path.
			real := dir
			if resolved, err := filepath.EvalSymlinks(dir); err == nil && resolved != "" {
				real = resolved
			}
			key := strings.ToLower(real)
			if dedupe[key] {
				return
			}
			dedupe[key] = true
			found = append(found, obsidianVault{
				Name:  filepath.Base(real),
				Path:  real,
				Notes: countMarkdownNotes(real),
			})
			return
		}
		for _, sub := range subdirs {
			walk(sub, depth+1)
		}
	}
	for _, root := range obsidianVaultRoots() {
		walk(root, 0)
	}

	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	obsidianCache = found
	obsidianCachedAt = time.Now()
	return append([]obsidianVault{}, found...)
}

// countMarkdownNotes counts .md files below dir, bounded so a huge tree cannot
// stall discovery.
func countMarkdownNotes(dir string) int {
	count := 0
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && (strings.HasPrefix(name, ".") || obsidianSkipDirs[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			count++
			if count >= obsidianMaxNotes {
				return filepath.SkipAll
			}
		}
		return nil
	})
	return count
}

// resolveObsidianVault matches a vault by absolute path or by name.
func resolveObsidianVault(raw string) (obsidianVault, error) {
	vaults := DiscoverObsidianVaults(false)
	if len(vaults) == 0 {
		return obsidianVault{}, fmt.Errorf("no Obsidian vault found: no directory with a .obsidian folder under your home directory")
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return vaults[0], nil
	}
	for _, vault := range vaults {
		if strings.EqualFold(vault.Path, trimmed) || strings.EqualFold(vault.Name, trimmed) {
			return vault, nil
		}
	}
	for _, vault := range vaults {
		if strings.EqualFold(vault.Path, filepath.Clean(trimmed)) {
			return vault, nil
		}
	}
	names := make([]string, 0, len(vaults))
	for _, vault := range vaults {
		names = append(names, vault.Name+" ("+vault.Path+")")
	}
	return obsidianVault{}, fmt.Errorf("vault %q not found; known vaults: %s", trimmed, strings.Join(names, ", "))
}

// resolveObsidianNotePath turns a note reference into an absolute .md path,
// accepting "vault/note", a vault-relative path, or an absolute path.
func resolveObsidianNotePath(note string) (string, error) {
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return "", fmt.Errorf("path or note is required")
	}
	if filepath.IsAbs(trimmed) {
		path := filepath.Clean(trimmed)
		if strings.EqualFold(filepath.Ext(path), ".md") {
			return path, nil
		}
		return path + ".md", nil
	}
	vaultName, rel := "", trimmed
	if idx := strings.Index(trimmed, ":"); idx > 0 && !strings.ContainsAny(trimmed[:idx], `/\`) {
		vaultName, rel = trimmed[:idx], trimmed[idx+1:]
	}
	vault, err := resolveObsidianVault(vaultName)
	if err != nil {
		return "", err
	}
	rel = strings.TrimPrefix(strings.ReplaceAll(rel, `\`, "/"), "/")
	if rel == "" {
		return "", fmt.Errorf("note path is required within vault %q", vault.Name)
	}
	if !strings.EqualFold(filepath.Ext(rel), ".md") {
		rel += ".md"
	}
	return filepath.Clean(filepath.Join(vault.Path, filepath.FromSlash(rel))), nil
}

// ObsidianListVaults reports every vault found on the machine.
func (dh *DirectHandler) ObsidianListVaults(args map[string]interface{}) (string, error) {
	force := false
	if v, ok := args["refresh"].(bool); ok {
		force = v
	}
	vaults := DiscoverObsidianVaults(force)
	payload := map[string]interface{}{
		"status": "ok",
		"count":  len(vaults),
		"vaults": vaults,
	}
	if len(vaults) == 0 {
		payload["hint"] = "No .obsidian directory was found under your home directory or /mnt/Data. Pass an absolute note path to read files without a vault."
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data), nil
}

type obsidianMatch struct {
	Vault   string `json:"vault"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	Line    int    `json:"line,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

var obsidianTagPattern = regexp.MustCompile(`(^|\s)#([A-Za-z0-9_][A-Za-z0-9_/\-]*)`)
var obsidianWikiPattern = regexp.MustCompile(`\[\[([^\]|#]+)(?:#[^\]|]+)?(?:\|[^\]]+)?\]\]`)

// ObsidianSearch searches note titles and bodies, with #tag and [[wikilink]] support.
func (dh *DirectHandler) ObsidianSearch(args map[string]interface{}) (string, error) {
	query := strings.TrimSpace(argString(args, "query", ""))
	if query == "" {
		query = strings.TrimSpace(argString(args, "pattern", ""))
	}
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	limit := 30
	if v, ok := args["limit"].(float64); ok && int(v) > 0 {
		limit = int(v)
	}
	if limit > 200 {
		limit = 200
	}

	vaults, err := obsidianSearchScope(args)
	if err != nil {
		return "", err
	}

	needle := strings.ToLower(query)
	tagQuery := ""
	if strings.HasPrefix(query, "#") {
		tagQuery = strings.ToLower(strings.TrimPrefix(query, "#"))
		needle = tagQuery
	}
	linkQuery := ""
	if strings.HasPrefix(query, "[[") && strings.HasSuffix(query, "]]") {
		linkQuery = strings.ToLower(strings.Trim(query, "[]"))
	}

	matches := []obsidianMatch{}
	for _, vault := range vaults {
		_ = filepath.WalkDir(vault.Path, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if len(matches) >= limit {
				return filepath.SkipAll
			}
			if d.IsDir() {
				name := d.Name()
				if path != vault.Path && (strings.HasPrefix(name, ".") || obsidianSkipDirs[name]) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".md") {
				return nil
			}
			title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			titleHit := strings.Contains(strings.ToLower(title), needle)
			if titleHit && tagQuery == "" && linkQuery == "" {
				matches = append(matches, obsidianMatch{Vault: vault.Name, Path: path, Title: title})
				return nil
			}

			file, openErr := os.Open(path)
			if openErr != nil {
				return nil
			}
			defer file.Close()
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			lineNo := 0
			reported := false
			for scanner.Scan() {
				lineNo++
				line := scanner.Text()
				hit := false
				switch {
				case tagQuery != "":
					for _, m := range obsidianTagPattern.FindAllStringSubmatch(line, -1) {
						if strings.Contains(strings.ToLower(m[2]), tagQuery) {
							hit = true
							break
						}
					}
				case linkQuery != "":
					for _, m := range obsidianWikiPattern.FindAllStringSubmatch(line, -1) {
						if strings.Contains(strings.ToLower(strings.TrimSpace(m[1])), linkQuery) {
							hit = true
							break
						}
					}
				default:
					hit = strings.Contains(strings.ToLower(line), needle)
				}
				if hit {
					matches = append(matches, obsidianMatch{
						Vault:   vault.Name,
						Path:    path,
						Title:   title,
						Line:    lineNo,
						Snippet: truncateRunes(strings.TrimSpace(line), obsidianSnippetRunes),
					})
					reported = true
					if len(matches) >= limit {
						return filepath.SkipAll
					}
					break
				}
			}
			_ = reported
			return nil
		})
		if len(matches) >= limit {
			break
		}
	}

	payload := map[string]interface{}{
		"status":  "ok",
		"query":   query,
		"count":   len(matches),
		"matches": matches,
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data), nil
}

func obsidianSearchScope(args map[string]interface{}) ([]obsidianVault, error) {
	scope := argString(args, "vault", "")
	if scope == "" {
		vaults := DiscoverObsidianVaults(false)
		if len(vaults) == 0 {
			return nil, fmt.Errorf("no Obsidian vault found: no directory with a .obsidian folder under your home directory")
		}
		return vaults, nil
	}
	vault, err := resolveObsidianVault(scope)
	if err != nil {
		return nil, err
	}
	return []obsidianVault{vault}, nil
}

// ObsidianReadNote returns one note's markdown.
func (dh *DirectHandler) ObsidianReadNote(args map[string]interface{}) (string, error) {
	reference := argString(args, "path", "")
	if reference == "" {
		reference = argString(args, "note", "")
	}
	path, err := resolveObsidianNotePath(reference)
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return "", fmt.Errorf("note not found: %s", path)
	}
	data, err := dh.ReadFile(map[string]interface{}{"path": path})
	if err != nil {
		return "", err
	}
	payload := map[string]interface{}{
		"status":  "ok",
		"path":    path,
		"title":   strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		"content": data,
	}
	out, _ := json.MarshalIndent(payload, "", "  ")
	return string(out), nil
}

// ObsidianWriteNote creates or updates one note.
func (dh *DirectHandler) ObsidianWriteNote(args map[string]interface{}) (string, error) {
	reference := argString(args, "path", "")
	if reference == "" {
		reference = argString(args, "note", "")
	}
	path, err := resolveObsidianNotePath(reference)
	if err != nil {
		return "", err
	}
	content, _ := args["content"].(string)
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("content is required")
	}
	mode := strings.ToLower(argString(args, "mode", "overwrite"))

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("create note directory: %w", err)
		}
	}

	switch mode {
	case "append", "prepend":
		existing, readErr := os.ReadFile(path)
		if readErr != nil && !os.IsNotExist(readErr) {
			return "", fmt.Errorf("read existing note: %w", readErr)
		}
		body := string(existing)
		payload := content
		if mode == "append" {
			if body != "" && !strings.HasSuffix(body, "\n") {
				body += "\n"
			}
			payload = body + content
		} else {
			if body != "" && !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			payload = content + body
		}
		if err := os.WriteFile(path, []byte(payload), 0644); err != nil {
			return "", fmt.Errorf("write note: %w", err)
		}
	case "overwrite", "":
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("write note: %w", err)
		}
	default:
		return "", fmt.Errorf("unsupported mode %q: use overwrite, append or prepend", mode)
	}

	info, _ := os.Stat(path)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	result := map[string]interface{}{
		"status": "ok",
		"mode":   mode,
		"path":   path,
		"bytes":  size,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return string(data), nil
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
