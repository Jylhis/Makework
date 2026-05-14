package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// rmFixture sets up an isolated XDG env, runs `mw init`, creates a
// scratch repo with `main` (one commit) and one extra branch shaped
// according to `diverge`, then registers it with `mw repo add` and
// switches to create the worktree.
//
// When diverge is false the branch points at the same commit as main
// (StateSameCommit). When true the branch has an additional commit
// (StateDiverged).
//
// Returns barePath and the worktree path for the chosen branch.
func rmFixture(t *testing.T, branch string, diverge bool) (barePath, wtPath string) {
	t.Helper()
	home := setupIsolatedEnv(t)

	if _, err := captureOutput(t, "init"); err != nil {
		t.Fatalf("mw init: %v", err)
	}

	scratch := filepath.Join(home, "scratch")
	mustMkdir(t, scratch)
	runGit(t, scratch, "init", "-q", "-b", "main")
	runGit(t, scratch, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit", "-q", "--allow-empty", "-m", "init")
	runGit(t, scratch, "checkout", "-q", "-b", branch)
	if diverge {
		runGit(t, scratch, "-c", "user.name=t", "-c", "user.email=t@t",
			"commit", "-q", "--allow-empty", "-m", "feature work")
	}
	runGit(t, scratch, "checkout", "-q", "main")

	if out, err := captureOutput(t, "repo", "add", scratch); err != nil {
		t.Fatalf("mw repo add: %v\n%s", err, out)
	}

	cfg, cat := loadState()
	resolved, err := cat.FindProjectUnambiguous("scratch")
	if err != nil {
		t.Fatalf("FindProjectUnambiguous: %v", err)
	}
	barePath = resolved.Repo.Path
	wtPath = resolvedWorktreePath(cfg, resolved, branch)

	if out, err := captureOutput(t, "switch", "scratch", branch); err != nil {
		t.Fatalf("mw switch %s: %v\n%s", branch, err, out)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree not created at %s: %v", wtPath, err)
	}
	return barePath, wtPath
}

func branchExists(t *testing.T, barePath, branch string) bool {
	t.Helper()
	out, err := exec.Command("git", "-C", barePath, "branch", "--list", branch).CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out)) != ""
}

// TestRmAutoDeletesMergedBranch: a branch at the same commit as main
// is auto-deleted after rm.
func TestRmAutoDeletesMergedBranch(t *testing.T) {
	barePath, wtPath := rmFixture(t, "merged", false)

	out, err := captureOutput(t, "rm", "scratch/merged")
	if err != nil {
		t.Fatalf("mw rm: %v\n%s", err, out)
	}
	if _, err := os.Stat(wtPath); err == nil {
		t.Errorf("worktree still exists at %s", wtPath)
	}
	if branchExists(t, barePath, "merged") {
		t.Error("branch 'merged' was not auto-deleted")
	}
}

// TestRmKeepsBranchWithKeepFlag: --keep-branch preserves the branch
// even when it is integrated.
func TestRmKeepsBranchWithKeepFlag(t *testing.T) {
	barePath, _ := rmFixture(t, "kept", false)

	out, err := captureOutput(t, "rm", "scratch/kept", "--keep-branch")
	if err != nil {
		t.Fatalf("mw rm: %v\n%s", err, out)
	}
	if !branchExists(t, barePath, "kept") {
		t.Error("branch 'kept' was deleted despite --keep-branch")
	}
}

// TestRmKeepsDivergedBranch: an unmerged branch is preserved without -D.
func TestRmKeepsDivergedBranch(t *testing.T) {
	barePath, _ := rmFixture(t, "diverged", true)

	out, err := captureOutput(t, "rm", "scratch/diverged")
	if err != nil {
		t.Fatalf("mw rm: %v\n%s", err, out)
	}
	if !branchExists(t, barePath, "diverged") {
		t.Error("diverged branch should be kept without -D")
	}
}

// TestRmForceDeleteRemovesDivergedBranch: -D deletes even unmerged branches.
func TestRmForceDeleteRemovesDivergedBranch(t *testing.T) {
	barePath, _ := rmFixture(t, "diverged", true)

	out, err := captureOutput(t, "rm", "scratch/diverged", "-D")
	if err != nil {
		t.Fatalf("mw rm -D: %v\n%s", err, out)
	}
	if branchExists(t, barePath, "diverged") {
		t.Error("diverged branch should have been force-deleted with -D")
	}
}
