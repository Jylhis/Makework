package repo

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// GitError is returned when a git shell-out fails or produces unexpected
// output. Exposes the command and captured stderr for error messages.
type GitError struct {
	Cmd    string
	Stderr string
}

func (e *GitError) Error() string {
	return fmt.Sprintf("git command failed: `%s`\n%s", e.Cmd, e.Stderr)
}

// ErrDetachedHEAD is returned when a bare repo's HEAD is not a symbolic ref.
var ErrDetachedHEAD = errors.New("HEAD is detached; cannot determine default branch")

// RunGitCapture runs `git <args...>` and returns captured stdout trimmed of
// surrounding whitespace. On non-zero exit, returns a *GitError.
func RunGitCapture(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &GitError{
			Cmd:    "git " + strings.Join(args, " "),
			Stderr: strings.TrimSpace(stderr.String()),
		}
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// CloneBare runs `git clone --bare <url> <dest>`.
func CloneBare(url, dest string) error {
	_, err := RunGitCapture("clone", "--bare", url, dest)
	return err
}

// Fetch runs `git -C <barePath> fetch --all --prune --tags`.
func Fetch(barePath string) error {
	_, err := RunGitCapture("-C", barePath, "fetch", "--all", "--prune", "--tags")
	return err
}

// GetDefaultBranch returns the short branch name that HEAD points at in a
// bare repo. On detached HEAD, returns ErrDetachedHEAD.
func GetDefaultBranch(barePath string) (string, error) {
	out, err := RunGitCapture("-C", barePath, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		var gerr *GitError
		if errors.As(err, &gerr) && strings.Contains(gerr.Stderr, "not a symbolic ref") {
			return "", ErrDetachedHEAD
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ListBranches returns all local branch short names, sorted alphabetically.
func ListBranches(barePath string) ([]string, error) {
	out, err := RunGitCapture("-C", barePath, "for-each-ref",
		"--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	var result []string
	for _, line := range strings.Split(out, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	sort.Strings(result)
	return result, nil
}

// CheckDefaultBranchChanged returns the new branch name if the default
// branch has changed from known, or "" if unchanged.
func CheckDefaultBranchChanged(barePath, known string) (string, error) {
	current, err := GetDefaultBranch(barePath)
	if err != nil {
		return "", err
	}
	if current != known {
		return current, nil
	}
	return "", nil
}

// Rebase runs `git -C <wtPath> rebase <target>`. On failure, attempts
// `git rebase --abort` so the working tree is not left mid-rebase.
func Rebase(wtPath, target string) error {
	if _, err := RunGitCapture("-C", wtPath, "rebase", "--", target); err != nil {
		_, _ = RunGitCapture("-C", wtPath, "rebase", "--abort")
		return err
	}
	return nil
}

// IsSafeBranchName validates a short branch name and rejects option-like
// values that could be interpreted as git flags.
func IsSafeBranchName(name string) bool {
	if strings.HasPrefix(name, "-") {
		return false
	}
	_, err := RunGitCapture("check-ref-format", "--branch", name)
	return err == nil
}

// MergeFF runs `git -C <wtPath> merge --ff-only <branch>` and fails if a
// fast-forward is not possible.
func MergeFF(wtPath, branch string) error {
	_, err := RunGitCapture("-C", wtPath, "merge", "--ff-only", branch)
	return err
}

// IsAncestor reports whether a is an ancestor commit of b. Both refs
// must resolve in repoPath. A non-resolution error is returned as-is.
func IsAncestor(repoPath, a, b string) (bool, error) {
	cmd := exec.Command("git", "-C", repoPath, "merge-base", "--is-ancestor", a, b)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, &GitError{
		Cmd:    "git -C " + repoPath + " merge-base --is-ancestor " + a + " " + b,
		Stderr: strings.TrimSpace(stderr.String()),
	}
}

// DeleteBranch runs `git -C <barePath> branch -d <branch>`. With force,
// uses -D and removes unmerged branches.
func DeleteBranch(barePath, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := RunGitCapture("-C", barePath, "branch", flag, branch)
	return err
}
