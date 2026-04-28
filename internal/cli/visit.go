package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jylhis/makework/internal/xdgpath"
	"github.com/spf13/cobra"
)

func newVisitCmd() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:    "visit <path>",
		Short:  "Record a visit (shell hook, not for direct use)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoName, branch, ok := resolveVisit(args[0])
			if !ok {
				return nil
			}
			recordVisit(repoName, branch)
			return nil
		},
	})
}

// resolveVisit maps a filesystem path to a (repoName, branch) pair by asking
// git for the containing repo's common directory (bare clone for worktrees,
// `.git` dir otherwise) and matching that against the repo-roots.txt cache
// written by catalog mutations and `mw repo sync`.
//
// Returns ok=false silently (no error) when the cache is missing, the path
// is not inside a git repo, or no registered root matches — the shell hook
// fires on every `cd` and must stay quiet in those cases.
func resolveVisit(visitPath string) (repoName, branch string, ok bool) {
	stateDir, err := xdgpath.StateDir()
	if err != nil {
		return "", "", false
	}
	data, err := os.ReadFile(filepath.Join(stateDir, "repo-roots.txt"))
	if err != nil {
		return "", "", false
	}

	commonDir, err := gitCommonDir(visitPath)
	if err != nil {
		return "", "", false
	}

	var matched string
	for _, line := range strings.Split(string(data), "\n") {
		root := strings.TrimSpace(line)
		if root == "" {
			continue
		}
		if samePath(commonDir, root) {
			matched = root
			break
		}
	}
	if matched == "" {
		return "", "", false
	}

	repoName = strings.TrimSuffix(filepath.Base(matched), ".git")
	branch = "unknown"
	if out, err := exec.Command("git", "-C", visitPath, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}
	return repoName, branch, true
}

// gitCommonDir returns the absolute, cleaned path to the git common directory
// for the repo containing `dir` — the shared bare clone for worktrees, or the
// `.git` directory for ordinary checkouts. Returns an error if `dir` is not
// inside a git repository.
func gitCommonDir(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", err
	}
	return filepath.Clean(strings.TrimSpace(string(out))), nil
}

// samePath reports whether a and b identify the same directory. Compares the
// cleaned strings first, then falls back to resolving symlinks so the hook
// works when either the cache or git's answer goes through a symlinked path.
func samePath(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, ea := filepath.EvalSymlinks(a)
	rb, eb := filepath.EvalSymlinks(b)
	return ea == nil && eb == nil && ra == rb
}
