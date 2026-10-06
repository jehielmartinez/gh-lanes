// Package store owns lanes' local files: where they live and how they are
// read and written.
package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// Dir returns the config directory: override when set, else
// $XDG_CONFIG_HOME/lanes, else ~/.config/lanes. macOS uses ~/.config too,
// not ~/Library/Application Support, so the path is the same on every OS.
func Dir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lanes"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory for config: %w", err)
	}
	return filepath.Join(home, ".config", "lanes"), nil
}
