package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mergeFixture sets up XDG + mw init, registers a scratch repo with a
// feature branch that has one extra commit over main, creates the
// feature worktree, and chdirs into it. Returns barePath, wtPath.
func mergeFixture(t *testing.T, withConflict bool) (barePath, wtPath string) {
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
	// Seed a file both branches will need.
	if err := os.WriteFile(filepath.Join(scratch, "shared.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write shared.txt: %v", err)
	}
	runGit(t, scratch, "add", "shared.txt")
	runGit(t, scratch, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit", "-q", "-m", "shared")

	runGit(t, scratch, "checkout", "-q", "-b", "feature")
	if withConflict {
		if err := os.WriteFile(filepath.Join(scratch, "shared.txt"), []byte("feature\n"), 0o644); err != nil {
			t.Fatalf("write shared.txt feature: %v", err)
		}
		runGit(t, scratch, "add", "shared.txt")
	} else {
		if err := os.WriteFile(filepath.Join(scratch, "feat.txt"), []byte("feature\n"), 0o644); err != nil {
			t.Fatalf("write feat.txt: %v", err)
		}
		runGit(t, scratch, "add", "feat.txt")
	}
	runGit(t, scratch, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit", "-q", "-m", "feature commit")

	runGit(t, scratch, "checkout", "-q", "main")
	if withConflict {
		if err := os.WriteFile(filepath.Join(scratch, "shared.txt"), []byte("main\n"), 0o644); err != nil {
			t.Fatalf("write shared.txt main: %v", err)
		}
		runGit(t, scratch, "add", "shared.txt")
		runGit(t, scratch, "-c", "user.name=t", "-c", "user.email=t@t",
			"commit", "-q", "-m", "main edit")
	}

	if out, err := captureOutput(t, "repo", "add", scratch); err != nil {
		t.Fatalf("mw repo add: %v\n%s", err, out)
	}

	cfg, cat := loadState()
	resolved, err := cat.FindProjectUnambiguous("scratch")
	if err != nil {
		t.Fatalf("FindProjectUnambiguous: %v", err)
	}
	barePath = resolved.Repo.Path
	wtPath = resolvedWorktreePath(cfg, resolved, "feature")

	if out, err := captureOutput(t, "switch", "scratch", "feature"); err != nil {
		t.Fatalf("mw switch feature: %v\n%s", err, out)
	}

	chdir(t, wtPath)
	return barePath, wtPath
}

// chdir changes the working directory and restores it at test end.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Logf("restore chdir: %v", err)
		}
	})
}

func mainHead(t *testing.T, barePath string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", barePath, "rev-parse", "main").Output()
	if err != nil {
		t.Fatalf("rev-parse main: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestMergeFastForward: rebase + ff cleanup happy path. After merge,
// main equals feature's last commit, worktree is gone, branch deleted.
func TestMergeFastForward(t *testing.T) {
	barePath, wtPath := mergeFixture(t, false)

	out, err := captureOutput(t, "merge")
	if err != nil {
		t.Fatalf("mw merge: %v\n%s", err, out)
	}

	// main should now point at what feature's HEAD was.
	if mainHead(t, barePath) == "" {
		t.Error("main should have advanced")
	}
	if _, err := os.Stat(wtPath); err == nil {
		t.Errorf("worktree still exists at %s", wtPath)
	}
	if branchExists(t, barePath, "feature") {
		t.Error("branch 'feature' should have been deleted")
	}
}

// TestMergeConflictAborts: a conflict during rebase leaves the worktree
// intact and returns an error.
func TestMergeConflictAborts(t *testing.T) {
	_, wtPath := mergeFixture(t, true)

	if _, err := captureOutput(t, "merge"); err == nil {
		t.Fatal("expected mw merge to fail on conflict")
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Errorf("worktree should still exist after conflict: %v", err)
	}
	// No rebase in progress.
	gitDir := filepath.Join(wtPath, ".git")
	if fi, err := os.Stat(gitDir); err == nil && fi.IsDir() {
		for _, name := range []string{"rebase-merge", "rebase-apply"} {
			if _, err := os.Stat(filepath.Join(gitDir, name)); err == nil {
				t.Errorf(".git/%s present — rebase not aborted", name)
			}
		}
	}
}

// TestMergeRefusesDirtyWorktree: uncommitted changes block merge.
func TestMergeRefusesDirtyWorktree(t *testing.T) {
	_, wtPath := mergeFixture(t, false)
	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty.txt: %v", err)
	}
	runGit(t, wtPath, "add", "dirty.txt")

	if _, err := captureOutput(t, "merge"); err == nil {
		t.Error("mw merge should refuse a dirty worktree without --force")
	}
}

// TestMergeKeepWorktree: --keep-worktree preserves the worktree and branch.
func TestMergeKeepWorktree(t *testing.T) {
	barePath, wtPath := mergeFixture(t, false)
	if _, err := captureOutput(t, "merge", "--keep-worktree"); err != nil {
		t.Fatalf("mw merge --keep-worktree: %v", err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Errorf("worktree was removed despite --keep-worktree: %v", err)
	}
	if !branchExists(t, barePath, "feature") {
		t.Error("branch was deleted despite --keep-worktree")
	}
}

// TestMergeRejectsOptionLikeTarget: --target must not accept option-like
// values that could be interpreted as git flags.
func TestMergeRejectsOptionLikeTarget(t *testing.T) {
	_, wtPath := mergeFixture(t, false)

	_, err := captureOutput(t, "merge", "--target", "--exec=touch /tmp/mw_pwned")
	if err == nil {
		t.Fatal("expected mw merge to reject option-like --target")
	}
	if _, statErr := os.Stat(wtPath); statErr != nil {
		t.Errorf("worktree should remain after rejected target: %v", statErr)
	}
}
