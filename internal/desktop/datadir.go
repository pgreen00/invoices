// Package desktop holds the glue between the Gin app and the native Wails
// shell: where data lives, and the webview behaviours a browser would have
// provided for free.
package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DataDir returns the per-user application data directory for appName,
// creating it if needed:
//
//	macOS:   ~/Library/Application Support/<appName>
//	Windows: %AppData%\<appName>
//	Linux:   $XDG_DATA_HOME/<appName>, or ~/.local/share/<appName>
func DataDir(appName string) (string, error) {
	var base string
	switch runtime.GOOS {
	case "darwin", "windows":
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		base = dir
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(xdg) {
			base = xdg
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
	}

	dir := filepath.Join(base, appName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("could not create the data directory %s: %w", dir, err)
	}
	return dir, nil
}
