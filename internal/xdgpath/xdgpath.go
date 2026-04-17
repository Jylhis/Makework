// Package xdgpath resolves makework's XDG directories (config, state, data)
// using adrg/xdg defaults — no macOS override.
package xdgpath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

// ErrNoHome is returned when the user's home directory cannot be resolved.
var ErrNoHome = errors.New("could not determine home directory")

// ConfigDir returns $XDG_CONFIG_HOME/makework.
func ConfigDir() (string, error) {
	if xdg.ConfigHome == "" {
		return "", ErrNoHome
	}
	return filepath.Join(xdg.ConfigHome, "makework"), nil
}

// StateDir returns $XDG_STATE_HOME/makework. `adrg/xdg` exposes StateHome.
func StateDir() (string, error) {
	if xdg.StateHome == "" {
		return "", ErrNoHome
	}
	return filepath.Join(xdg.StateHome, "makework"), nil
}

// DataDir returns $XDG_DATA_HOME/makework.
func DataDir() (string, error) {
	if xdg.DataHome == "" {
		return "", ErrNoHome
	}
	return filepath.Join(xdg.DataHome, "makework"), nil
}

// ExpandTilde expands a leading "~" or "~/" against $HOME. Mid-path "~"
// characters are left unchanged, matching the Rust etcetera behavior.
func ExpandTilde(path string) (string, error) {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrNoHome, err)
		}
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrNoHome, err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
