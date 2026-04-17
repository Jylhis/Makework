// Package nix detects nix environments in a directory (flake/classic/devenv)
// and picks an activation command.
package nix

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jylhis/makework/internal/project"
)

// Detected holds the outcome of DetectNix.
type Detected struct {
	NixType           project.NixType
	ActivationCommand string
}

// Detect returns the detected nix environment for dir, honoring an optional
// explicit config override that takes precedence over filesystem detection.
// Returns nil if no nix environment is found.
//
// Detection order: explicit config → flake.nix → shell.nix → devenv.nix →
// .envrc directives.
func Detect(dir string, explicit *project.NixConfig) *Detected {
	if explicit != nil && explicit.NixType != nil {
		devshell := ""
		if explicit.Devshell != nil {
			devshell = *explicit.Devshell
		}
		return &Detected{
			NixType:           *explicit.NixType,
			ActivationCommand: activationFor(*explicit.NixType, devshell),
		}
	}

	if fileExists(filepath.Join(dir, "flake.nix")) {
		return &Detected{NixType: project.NixFlake, ActivationCommand: "nix develop"}
	}
	if fileExists(filepath.Join(dir, "shell.nix")) {
		return &Detected{NixType: project.NixClassic, ActivationCommand: "nix-shell"}
	}
	if fileExists(filepath.Join(dir, "devenv.nix")) {
		return &Detected{NixType: project.NixDevenv, ActivationCommand: "devenv shell"}
	}
	if d := checkEnvrc(dir); d != nil {
		return d
	}
	return nil
}

func activationFor(nt project.NixType, devshell string) string {
	switch nt {
	case project.NixFlake:
		if devshell != "" {
			return fmt.Sprintf("nix develop .#%s", devshell)
		}
		return "nix develop"
	case project.NixClassic:
		return "nix-shell"
	case project.NixDevenv:
		return "devenv shell"
	case project.NixCustom:
		return "echo 'custom nix setup'"
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func checkEnvrc(dir string) *Detected {
	f, err := os.Open(filepath.Join(dir, ".envrc"))
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(trimmed, "use flake"), strings.HasPrefix(trimmed, "use_flake"):
			return &Detected{NixType: project.NixFlake, ActivationCommand: "nix develop"}
		case strings.HasPrefix(trimmed, "use nix"), strings.HasPrefix(trimmed, "use_nix"):
			return &Detected{NixType: project.NixClassic, ActivationCommand: "nix-shell"}
		case strings.HasPrefix(trimmed, "use devenv"), strings.HasPrefix(trimmed, "use_devenv"):
			return &Detected{NixType: project.NixDevenv, ActivationCommand: "devenv shell"}
		}
	}
	return nil
}
