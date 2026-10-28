// Package paths centralizes the filesystem locations used by radiocli.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigPath returns the location of config.json. It defaults to
// ~/.config/radiocli/config.json, following $XDG_CONFIG_HOME; RADIOCLI_CONFIG
// overrides the location.
func ConfigPath() string {
	return overridePath("RADIOCLI_CONFIG", "config.json")
}

// CatalogPath returns where the downloaded station list is cached, alongside
// config.json in ~/.config/radiocli. RADIOCLI_DB overrides the location.
func CatalogPath() string {
	return overridePath("RADIOCLI_DB", "allStations.json")
}

// ControlSocketPath is the mpv JSON IPC socket shared by the running instance
// and the remote-control commands.
func ControlSocketPath() string {
	return fmt.Sprintf("/tmp/radiocli-%d.sock", os.Getuid())
}

func overridePath(env, fallback string) string {
	if override := os.Getenv(env); override != "" {
		if abs, err := filepath.Abs(override); err == nil {
			return abs
		}

		return override
	}

	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "radiocli", fallback)
	}

	return fallback
}
