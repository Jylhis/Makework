package worktree

import (
	"os"
	"strings"

	"github.com/jylhis/makework/internal/repo"
)

// Info describes one worktree as parsed from `git worktree list --porcelain`.
type Info struct {
	Path   string
	Branch string // empty for detached HEAD
	IsBare bool
}

// Create runs `git -C <barePath> worktree add <wtPath> <branch>`.
func Create(barePath, branch, wtPath string) error {
	_, err := repo.RunGitCapture("-C", barePath, "worktree", "add", wtPath, branch)
	return err
}

// Remove runs `git -C <barePath> worktree remove <wtPath>` then prune.
func Remove(barePath, wtPath string) error {
	if _, err := repo.RunGitCapture("-C", barePath, "worktree", "remove", wtPath); err != nil {
		return err
	}
	_, err := repo.RunGitCapture("-C", barePath, "worktree", "prune")
	return err
}

// List returns all worktrees for a bare repo.
func List(barePath string) ([]Info, error) {
	out, err := repo.RunGitCapture("-C", barePath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parsePorcelain(out), nil
}

// EnableSparseCheckout enables cone-mode sparse checkout and sets the given paths.
func EnableSparseCheckout(wtPath string, paths []string) error {
	if _, err := repo.RunGitCapture("-C", wtPath, "sparse-checkout", "init", "--cone"); err != nil {
		return err
	}
	args := append([]string{"-C", wtPath, "sparse-checkout", "set"}, paths...)
	_, err := repo.RunGitCapture(args...)
	return err
}

// DisableSparseCheckout restores the full checkout.
func DisableSparseCheckout(wtPath string) error {
	_, err := repo.RunGitCapture("-C", wtPath, "sparse-checkout", "disable")
	return err
}

// Prune prunes stale worktree entries whose directories no longer exist.
// Returns the number of orphaned entries that were pruned.
func Prune(barePath string) (int, error) {
	worktrees, err := List(barePath)
	if err != nil {
		return 0, err
	}
	orphaned := 0
	for _, wt := range worktrees {
		if wt.IsBare {
			continue
		}
		if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
			orphaned++
		}
	}
	if orphaned > 0 {
		if _, err := repo.RunGitCapture("-C", barePath, "worktree", "prune"); err != nil {
			return 0, err
		}
	}
	return orphaned, nil
}

// parsePorcelain parses `git worktree list --porcelain` output.
func parsePorcelain(text string) []Info {
	var worktrees []Info
	var current *Info

	for _, line := range strings.Split(text, "\n") {
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			if current != nil {
				worktrees = append(worktrees, *current)
			}
			current = &Info{Path: path}
		} else if b, ok := strings.CutPrefix(line, "branch "); ok && current != nil {
			short := strings.TrimPrefix(b, "refs/heads/")
			current.Branch = short
		} else if line == "bare" && current != nil {
			current.IsBare = true
		}
	}
	if current != nil {
		worktrees = append(worktrees, *current)
	}
	return worktrees
}
