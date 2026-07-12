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
	base, err := xdgHome("XDG_CONFIG_HOME", []string{"Library", "Application Support"}, []string{".config"})
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "makework"), nil
}

// StateDir returns $XDG_STATE_HOME/makework (or platform default).
func StateDir() (string, error) {
	base, err := xdgHome("XDG_STATE_HOME", []string{".local", "state"}, []string{".local", "state"})
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "makework"), nil
}

// DataDir returns $XDG_DATA_HOME/makework (or platform default).
func DataDir() (string, error) {
	base, err := xdgHome("XDG_DATA_HOME", []string{"Library", "Application Support"}, []string{".local", "share"})
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

// xdgHome resolves an XDG base directory: it returns the value of the named
// environment variable if set, otherwise a home-relative default. darwinDefault
// is used on macOS and otherDefault on all other platforms; each is a sequence
// of path segments joined onto the home directory.
func xdgHome(envVar string, darwinDefault, otherDefault []string) (string, error) {
	if v := os.Getenv(envVar); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ErrNoHome
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(append([]string{home}, darwinDefault...)...), nil
	}
	return filepath.Join(append([]string{home}, otherDefault...)...), nil
}
