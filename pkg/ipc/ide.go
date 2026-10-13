package ipc

import (
	"fmt"
	"path/filepath"
	"strings"
)

// AllowedIDEs maps supported IDE identifiers and aliases to their canonical binary name.
// Allowlist: antigravity, code, cursor, windsurf, trae, vscodium (with aliases vscode, codium, zed).
var AllowedIDEs = map[string]string{
	"antigravity": "antigravity",
	"code":        "code",
	"vscode":      "code",
	"cursor":      "cursor",
	"windsurf":    "windsurf",
	"trae":        "trae",
	"vscodium":    "vscodium",
	"codium":      "codium",
	"zed":         "zed",
}

// ValidateIDE validates and canonicalizes an IDE binary name.
// It strictly rejects arbitrary executables and prevents shell injection characters.
func ValidateIDE(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "code", nil
	}

	// Reject shell injection characters and whitespace/argument separation
	if strings.ContainsAny(trimmed, ";&|`$><\\/\n\r\t\"' ") {
		return "", fmt.Errorf("invalid IDE name: prohibited characters or arguments detected")
	}

	// Strip common executable extensions
	base := filepath.Base(trimmed)
	lower := strings.ToLower(base)
	lower = strings.TrimSuffix(lower, ".exe")

	canonical, ok := AllowedIDEs[lower]
	if !ok {
		return "", fmt.Errorf("unrecognized or disallowed IDE %q: must be one of antigravity, code, cursor, windsurf, trae, vscodium, zed", raw)
	}

	return canonical, nil
}

// IsAllowedIDE checks whether the provided name is in the recognized IDE allowlist.
func IsAllowedIDE(raw string) bool {
	_, err := ValidateIDE(raw)
	return err == nil
}
