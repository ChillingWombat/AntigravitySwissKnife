//go:build windows

package ls

import "os"

func logDirFromHome() string {
	if appData := os.Getenv("APPDATA"); appData != "" {
		return appData[len(os.Getenv("USERPROFILE"))+1:]
	}
	return "AppData\\Roaming\\Antigravity"
}
