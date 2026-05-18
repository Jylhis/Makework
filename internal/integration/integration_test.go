package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixture creates a non-bare git repo with main branch and one commit.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out, err := exec.Command("git", "init", "-b", "main", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "Test")
	git(t, dir, "config", "commit.gpgsign", "false")
	write(t, filepath.Join(dir, "README"), "init\n")
	git(t, dir, "add", "README")
	git(t, dir, "commit", "-m", "init")
	return dir
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestStateSameCommit: branch and default point at the same SHA.
func TestStateSameCommit(t *testing.T) {
	dir := fixture(t)
	git(t, dir, "branch", "twin")

	state, err := Check(dir, "twin", "main")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if state != StateSameCommit {
		t.Errorf("got %s; want %s", state, StateSameCommit)
	}
}

// TestStateAncestor: branch is at an earlier commit on main's history.
func TestStateAncestor(t *testing.T) {
	dir := fixture(t)
	git(t, dir, "branch", "feature")

	// Advance main; feature stays behind.
	write(t, filepath.Join(dir, "m.txt"), "m\n")
	git(t, dir, "add", "m.txt")
	git(t, dir, "commit", "-m", "main advances")

	state, err := Check(dir, "feature", "main")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if state != StateAncestor {
		t.Errorf("got %s; want %s", state, StateAncestor)
	}
}

// TestStateNoChanges: feature has commits ahead of merge-base, but
// their net effect on the tree is zero (e.g. an empty commit, or
// a commit that was reverted). Three-dot diff <main>...<feature>
// is therefore empty even though feature is not an ancestor of main.
func TestStateNoChanges(t *testing.T) {
	dir := fixture(t)
	git(t, dir, "checkout", "-b", "feature")
	git(t, dir, "commit", "--allow-empty", "-m", "empty feature work")

	git(t, dir, "checkout", "main")
	write(t, filepath.Join(dir, "m.txt"), "m\n")
	git(t, dir, "add", "m.txt")
	git(t, dir, "commit", "-m", "main advances")

	state, err := Check(dir, "feature", "main")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if state != StateNoChanges {
		t.Errorf("got %s; want %s", state, StateNoChanges)
	}
}

// TestStateMergedPID: feature has a commit with the same patch as a
// commit on main (squash-merge scenario). git patch-id matches.
func TestStateMergedPID(t *testing.T) {
	dir := fixture(t)
	git(t, dir, "checkout", "-b", "feature")
	write(t, filepath.Join(dir, "x.txt"), "feature change\n")
	git(t, dir, "add", "x.txt")
	git(t, dir, "commit", "-m", "feature: add x")

	// Main applies the same diff in a separate commit (simulating squash-
	// merge): add x.txt with identical content. Patch-ids match.
	git(t, dir, "checkout", "main")
	write(t, filepath.Join(dir, "x.txt"), "feature change\n")
	git(t, dir, "add", "x.txt")
	git(t, dir, "commit", "-m", "main: pick up x via squash")
	// Add an extra main-only commit so feature is not just NoChanges.
	write(t, filepath.Join(dir, "y.txt"), "main only\n")
	git(t, dir, "add", "y.txt")
	git(t, dir, "commit", "-m", "main: extra work")

	state, err := Check(dir, "feature", "main")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if state != StateMergedPID {
		t.Errorf("got %s; want %s", state, StateMergedPID)
	}
}

// TestStateDiverged: both branches have unique commits with distinct
// content; no patch-id overlap.
func TestStateDiverged(t *testing.T) {
	dir := fixture(t)
	git(t, dir, "checkout", "-b", "feature")
	write(t, filepath.Join(dir, "feat.txt"), "feature unique\n")
	git(t, dir, "add", "feat.txt")
	git(t, dir, "commit", "-m", "feature work")

	git(t, dir, "checkout", "main")
	write(t, filepath.Join(dir, "main.txt"), "main unique\n")
	git(t, dir, "add", "main.txt")
	git(t, dir, "commit", "-m", "main work")

	state, err := Check(dir, "feature", "main")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if state != StateDiverged {
		t.Errorf("got %s; want %s", state, StateDiverged)
	}
}

// TestCheckUnknownBranch returns an error for an undefined ref.
func TestCheckUnknownBranch(t *testing.T) {
	dir := fixture(t)
	if _, err := Check(dir, "nonexistent", "main"); err == nil {
		t.Error("expected error for missing branch")
	}
}

// TestStateMergedPIDWhitespaceSensitive: whitespace-different patches
// should not be treated as merged.
func TestStateMergedPIDWhitespaceSensitive(t *testing.T) {
	dir := fixture(t)
	git(t, dir, "checkout", "-b", "feature")
	write(t, filepath.Join(dir, "Makefile"), "all:\n\t@echo feature\n")
	git(t, dir, "add", "Makefile")
	git(t, dir, "commit", "-m", "feature makefile")

	git(t, dir, "checkout", "main")
	write(t, filepath.Join(dir, "Makefile"), "all:\n        @echo feature\n")
	git(t, dir, "add", "Makefile")
	git(t, dir, "commit", "-m", "main whitespace variant")

	state, err := Check(dir, "feature", "main")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if state != StateDiverged {
		t.Errorf("got %s; want %s", state, StateDiverged)
	}
}
