//go:build darwin

package ls

import "path/filepath"

func logDirFromHome() string {
	return filepath.Join("Library", "Application Support", "Antigravity")
}
