// Package xdgpath resolves makework's XDG directories (config, state, data).
// Reads environment variables directly for testability (adrg/xdg caches at init).
package xdgpath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrNoHome is returned when the user's home directory cannot be resolved.
var ErrNoHome = errors.New("could not determine home directory")

// ConfigDir returns $XDG_CONFIG_HOME/makework (or platform default).
func ConfigDir() (string, error) {
	base, err := configHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "makework"), nil
}

// StateDir returns $XDG_STATE_HOME/makework (or platform default).
func StateDir() (string, error) {
	base, err := stateHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "makework"), nil
}

// DataDir returns $XDG_DATA_HOME/makework (or platform default).
func DataDir() (string, error) {
	base, err := dataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "makework"), nil
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

func configHome() (string, error) {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ErrNoHome
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support"), nil
	}
	return filepath.Join(home, ".config"), nil
}

func stateHome() (string, error) {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ErrNoHome
	}
	return filepath.Join(home, ".local", "state"), nil
}

func dataHome() (string, error) {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ErrNoHome
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support"), nil
	}
	return filepath.Join(home, ".local", "share"), nil
}
