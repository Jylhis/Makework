// Package maintenance wraps `git maintenance {register,unregister,status}`
// on bare repositories.
package maintenance

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/jylhis/makework/internal/repo"
)

// Register registers a bare repo with `git maintenance`.
func Register(barePath string) error {
	return run(barePath, "maintenance", "register")
}

// Unregister removes a bare repo from `git maintenance`.
func Unregister(barePath string) error {
	return run(barePath, "maintenance", "unregister")
}

// Status reports whether a bare repo is currently registered (i.e. has a
// `maintenance.repo` key in its config).
func Status(barePath string) (bool, error) {
	cmd := exec.Command("git", "-C", barePath, "config", "--get", "maintenance.repo")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// `git config --get` exits 1 when the key is missing; distinguish
		// that from a hard failure by checking stderr.
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, &repo.GitError{
			Cmd:    "git -C " + barePath + " config --get maintenance.repo",
			Stderr: strings.TrimSpace(stderr.String()),
		}
	}
	return true, nil
}

func run(barePath string, args ...string) error {
	cmdArgs := append([]string{"-C", barePath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return &repo.GitError{
			Cmd:    "git " + strings.Join(cmdArgs, " "),
			Stderr: strings.TrimSpace(stderr.String()),
		}
	}
	return nil
}
