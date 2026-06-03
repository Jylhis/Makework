package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wtFixture sets up an isolated XDG env, runs `mw init`, creates a
// scratch repo with a `main` commit, registers it via `mw repo add`,
// creates a starter worktree on `main`, chdirs into it, and returns
// (home, barePath, mainWtPath). The chdir is restored automatically
// via t.Cleanup.
func wtFixture(t *testing.T) (home, barePath, mainWtPath string) {
	t.Helper()
	home = setupIsolatedEnv(t)
	if _, err := captureOutput(t, "init"); err != nil {
		t.Fatalf("mw init: %v", err)
	}
	scratch := filepath.Join(home, "scratch")
	mustMkdir(t, scratch)
	runGit(t, scratch, "init", "-q", "-b", "main")
	runGit(t, scratch, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit", "-q", "--allow-empty", "-m", "init")

	if out, err := captureOutput(t, "repo", "add", scratch); err != nil {
		t.Fatalf("mw repo add: %v\n%s", err, out)
	}
	cfg, cat, _ := loadState()
	resolved, err := cat.FindProjectUnambiguous("scratch")
	if err != nil {
		t.Fatalf("FindProjectUnambiguous: %v", err)
	}
	barePath = resolved.Repo.Path
	mainWtPath = resolvedWorktreePath(cfg, resolved, "main")
	// `mw repo add` already creates the default-branch worktree.
	if _, err := os.Stat(mainWtPath); err != nil {
		t.Fatalf("expected default worktree at %s after repo add: %v", mainWtPath, err)
	}
	chdir(t, mainWtPath)
	return home, barePath, mainWtPath
}

func TestWtSwitchCreatesWorktree(t *testing.T) {
	_, barePath, _ := wtFixture(t)

	out, err := captureOutput(t, "wt", "switch", "-c", "feature")
	if err != nil {
		t.Fatalf("wt switch: %v\n%s", err, out)
	}
	path := strings.TrimSpace(out)
	if path == "" {
		t.Fatalf("expected worktree path on stdout, got %q", out)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("worktree not created at %s: %v", path, err)
	}
	if !branchExists(t, barePath, "feature") {
		t.Error("branch 'feature' was not created")
	}
}

func TestWtSwitchWithBase(t *testing.T) {
	_, barePath, _ := wtFixture(t)
	runGit(t, barePath, "branch", "dev")

	out, err := captureOutput(t, "wt", "switch", "-c", "feat-from-dev", "-b", "dev")
	if err != nil {
		t.Fatalf("wt switch -b: %v\n%s", err, out)
	}
	if !branchExists(t, barePath, "feat-from-dev") {
		t.Error("branch 'feat-from-dev' was not created")
	}
}

func TestWtListMatchesMwLs(t *testing.T) {
	wtFixture(t)
	if _, err := captureOutput(t, "wt", "switch", "-c", "feature"); err != nil {
		t.Fatalf("wt switch -c feature: %v", err)
	}

	out, err := captureOutput(t, "wt", "list")
	if err != nil {
		t.Fatalf("wt list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "feature") {
		t.Errorf("expected 'feature' in wt list output, got:\n%s", out)
	}
	if !strings.Contains(out, "main") {
		t.Errorf("expected 'main' in wt list output, got:\n%s", out)
	}
}

func TestWtListBranchesIncludesOrphan(t *testing.T) {
	_, barePath, _ := wtFixture(t)
	runGit(t, barePath, "branch", "orphan-branch")

	out, err := captureOutput(t, "wt", "list", "--branches")
	if err != nil {
		t.Fatalf("wt list --branches: %v\n%s", err, out)
	}
	if !strings.Contains(out, "orphan-branch") {
		t.Errorf("expected orphan-branch in --branches output, got:\n%s", out)
	}
}

func TestWtListRemotes(t *testing.T) {
	_, barePath, _ := wtFixture(t)
	// Synthesize a remote-tracking ref by writing directly into refs/remotes.
	runGit(t, barePath, "update-ref", "refs/remotes/origin/feat-remote", "HEAD")

	out, err := captureOutput(t, "wt", "list", "--remotes")
	if err != nil {
		t.Fatalf("wt list --remotes: %v\n%s", err, out)
	}
	if !strings.Contains(out, "origin/feat-remote") {
		t.Errorf("expected origin/feat-remote in --remotes output, got:\n%s", out)
	}
}

func TestWtRemoveByName(t *testing.T) {
	_, barePath, _ := wtFixture(t)
	if _, err := captureOutput(t, "wt", "switch", "-c", "tmp"); err != nil {
		t.Fatalf("wt switch -c tmp: %v", err)
	}

	out, err := captureOutput(t, "wt", "remove", "tmp")
	if err != nil {
		t.Fatalf("wt remove tmp: %v\n%s", err, out)
	}
	if branchExists(t, barePath, "tmp") {
		t.Error("merged branch 'tmp' was not auto-deleted")
	}
}

func TestWtRemoveNoDeleteBranch(t *testing.T) {
	_, barePath, _ := wtFixture(t)
	if _, err := captureOutput(t, "wt", "switch", "-c", "keepme"); err != nil {
		t.Fatalf("wt switch: %v", err)
	}

	out, err := captureOutput(t, "wt", "remove", "--no-delete-branch", "keepme")
	if err != nil {
		t.Fatalf("wt remove --no-delete-branch: %v\n%s", err, out)
	}
	if !branchExists(t, barePath, "keepme") {
		t.Error("branch 'keepme' was deleted despite --no-delete-branch")
	}
}

func TestWtRemoveRefusesUntrackedWithoutForce(t *testing.T) {
	wtFixture(t)
	if _, err := captureOutput(t, "wt", "switch", "-c", "dirty"); err != nil {
		t.Fatalf("wt switch -c dirty: %v", err)
	}
	cfg, cat, err := loadState()
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	resolved, err := cat.FindProjectUnambiguous("scratch")
	if err != nil {
		t.Fatalf("FindProjectUnambiguous: %v", err)
	}
	dirtyPath := resolvedWorktreePath(cfg, resolved, "dirty")
	if err := os.WriteFile(filepath.Join(dirtyPath, "junk.txt"), []byte("untracked"), 0o644); err != nil {
		t.Fatalf("write untracked: %v", err)
	}

	if out, err := captureOutput(t, "wt", "remove", "dirty"); err == nil {
		t.Fatalf("wt remove should have refused dirty worktree; got %q", out)
	}
	if _, err := os.Stat(dirtyPath); err != nil {
		t.Fatalf("worktree should still exist after refusal: %v", err)
	}

	if out, err := captureOutput(t, "wt", "remove", "--force", "dirty"); err != nil {
		t.Fatalf("wt remove --force: %v\n%s", err, out)
	}
	if _, err := os.Stat(dirtyPath); err == nil {
		t.Error("worktree should be gone after --force remove")
	}
}

func TestWtRemoveRejectsForgedGitDir(t *testing.T) {
	home, _, _ := wtFixture(t)
	if _, err := captureOutput(t, "wt", "switch", "-c", "feature"); err != nil {
		t.Fatalf("wt switch -c feature: %v", err)
	}

	cfg, cat, err := loadState()
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	resolved, err := cat.FindProjectUnambiguous("scratch")
	if err != nil {
		t.Fatalf("FindProjectUnambiguous: %v", err)
	}
	wtPath := resolvedWorktreePath(cfg, resolved, "feature")
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("expected feature worktree to exist: %v", err)
	}

	fake := filepath.Join(home, "forged")
	mustMkdir(t, fake)
	if err := os.WriteFile(filepath.Join(fake, ".git"), []byte("gitdir: "+resolved.Repo.Path+"\n"), 0o644); err != nil {
		t.Fatalf("write forged .git: %v", err)
	}
	chdir(t, fake)

	out, err := captureOutput(t, "wt", "remove", "feature")
	if err == nil {
		t.Fatalf("wt remove from forged directory should fail, got output: %s", out)
	}
	if !strings.Contains(out, "not a registered worktree") && !strings.Contains(err.Error(), "not a registered worktree") && !strings.Contains(err.Error(), "not inside a git worktree") {
		t.Fatalf("expected worktree validation failure, got output=%q err=%v", out, err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("feature worktree should not be removed: %v", err)
	}
}
