//go:build linux

package ls

import "path/filepath"

func logDirFromHome() string { return filepath.Join(".config", "Antigravity") }
