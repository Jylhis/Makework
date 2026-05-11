package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitErrorFormat(t *testing.T) {
	err := &GitError{Cmd: "git clone", Stderr: "fatal: error"}
	msg := err.Error()
	if !strings.Contains(msg, "git clone") || !strings.Contains(msg, "fatal: error") {
		t.Errorf("unexpected message: %q", msg)
	}
}

// fixture returns a freshly initialised non-bare repo with an initial
// commit on 'main' and committer identity configured.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitInit := exec.Command("git", "init", "-b", "main", dir)
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	gitDo(t, dir, "config", "user.email", "test@example.com")
	gitDo(t, dir, "config", "user.name", "Test")
	gitDo(t, dir, "config", "commit.gpgsign", "false")
	writeFile(t, filepath.Join(dir, "README"), "init\n")
	gitDo(t, dir, "add", "README")
	gitDo(t, dir, "commit", "-m", "init")
	return dir
}

func gitDo(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func head(t *testing.T, dir string) string {
	t.Helper()
	out, err := RunGitCapture("-C", dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(out)
}

// TestIsAncestor covers the three outcomes: true, false, error.
func TestIsAncestor(t *testing.T) {
	dir := fixture(t)
	first := head(t, dir)

	writeFile(t, filepath.Join(dir, "f"), "1\n")
	gitDo(t, dir, "add", "f")
	gitDo(t, dir, "commit", "-m", "second")
	second := head(t, dir)

	ok, err := IsAncestor(dir, first, second)
	if err != nil {
		t.Fatalf("IsAncestor(first, second): %v", err)
	}
	if !ok {
		t.Errorf("expected first to be ancestor of second")
	}

	ok, err = IsAncestor(dir, second, first)
	if err != nil {
		t.Fatalf("IsAncestor(second, first): %v", err)
	}
	if ok {
		t.Errorf("expected second NOT to be ancestor of first")
	}

	if _, err := IsAncestor(dir, "nope", "main"); err == nil {
		t.Errorf("expected error for unknown ref")
	}
}

// TestRebase covers a successful linear rebase. After it runs, feature's
// new HEAD must have main's HEAD as its first parent.
func TestRebase(t *testing.T) {
	dir := fixture(t)

	// Diverge: feature branch from main, then advance main.
	gitDo(t, dir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(dir, "feat.txt"), "feature\n")
	gitDo(t, dir, "add", "feat.txt")
	gitDo(t, dir, "commit", "-m", "feature work")

	gitDo(t, dir, "checkout", "main")
	writeFile(t, filepath.Join(dir, "main.txt"), "main work\n")
	gitDo(t, dir, "add", "main.txt")
	gitDo(t, dir, "commit", "-m", "main advances")
	mainHead := head(t, dir)

	gitDo(t, dir, "checkout", "feature")
	if err := Rebase(dir, "main"); err != nil {
		t.Fatalf("Rebase: %v", err)
	}

	parent, err := RunGitCapture("-C", dir, "rev-parse", "HEAD^")
	if err != nil {
		t.Fatalf("rev-parse HEAD^: %v", err)
	}
	if strings.TrimSpace(parent) != mainHead {
		t.Errorf("after rebase, HEAD^ = %s; want main HEAD %s", parent, mainHead)
	}
}

// TestRebaseConflictAborts ensures that on conflict we get an error and
// the working tree is not left mid-rebase (rebase --abort was called).
func TestRebaseConflictAborts(t *testing.T) {
	dir := fixture(t)

	// Both branches edit the same line of the same file → conflict.
	writeFile(t, filepath.Join(dir, "conflict.txt"), "base\n")
	gitDo(t, dir, "add", "conflict.txt")
	gitDo(t, dir, "commit", "-m", "add conflict.txt")

	gitDo(t, dir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(dir, "conflict.txt"), "feature-edit\n")
	gitDo(t, dir, "add", "conflict.txt")
	gitDo(t, dir, "commit", "-m", "feature edit")

	gitDo(t, dir, "checkout", "main")
	writeFile(t, filepath.Join(dir, "conflict.txt"), "main-edit\n")
	gitDo(t, dir, "add", "conflict.txt")
	gitDo(t, dir, "commit", "-m", "main edit")

	gitDo(t, dir, "checkout", "feature")
	if err := Rebase(dir, "main"); err == nil {
		t.Fatal("expected Rebase to return an error on conflict")
	}

	// Verify no rebase in progress: .git/rebase-merge and .git/rebase-apply absent.
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(dir, ".git", name)); err == nil {
			t.Errorf("expected %s to be cleaned up after failed rebase", name)
		}
	}
}

// TestMergeFF covers a fast-forward merge into main.
func TestMergeFF(t *testing.T) {
	dir := fixture(t)
	gitDo(t, dir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(dir, "f.txt"), "f\n")
	gitDo(t, dir, "add", "f.txt")
	gitDo(t, dir, "commit", "-m", "feat")
	featureHead := head(t, dir)

	gitDo(t, dir, "checkout", "main")
	if err := MergeFF(dir, "feature"); err != nil {
		t.Fatalf("MergeFF: %v", err)
	}

	if got := head(t, dir); got != featureHead {
		t.Errorf("after ff merge, HEAD = %s; want feature %s", got, featureHead)
	}
}

// TestMergeFFRejectsNonFF: main has diverged, ff-only must fail.
func TestMergeFFRejectsNonFF(t *testing.T) {
	dir := fixture(t)
	gitDo(t, dir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(dir, "feat.txt"), "f\n")
	gitDo(t, dir, "add", "feat.txt")
	gitDo(t, dir, "commit", "-m", "feat")

	gitDo(t, dir, "checkout", "main")
	writeFile(t, filepath.Join(dir, "main.txt"), "m\n")
	gitDo(t, dir, "add", "main.txt")
	gitDo(t, dir, "commit", "-m", "main")
	mainHead := head(t, dir)

	if err := MergeFF(dir, "feature"); err == nil {
		t.Fatal("expected MergeFF to refuse non-ff merge")
	}
	if got := head(t, dir); got != mainHead {
		t.Errorf("HEAD moved after failed ff merge: %s != %s", got, mainHead)
	}
}

// TestDeleteBranch: -d works for merged, fails for unmerged; -D works always.
func TestDeleteBranch(t *testing.T) {
	dir := fixture(t)

	// Branch that's the same commit as main → -d works.
	gitDo(t, dir, "branch", "merged")
	if err := DeleteBranch(dir, "merged", false); err != nil {
		t.Fatalf("DeleteBranch(merged, force=false): %v", err)
	}
	branches, _ := RunGitCapture("-C", dir, "branch", "--list", "merged")
	if strings.TrimSpace(branches) != "" {
		t.Errorf("branch 'merged' still exists: %q", branches)
	}

	// Branch with a unique commit → -d refuses, -D succeeds.
	gitDo(t, dir, "checkout", "-b", "unmerged")
	writeFile(t, filepath.Join(dir, "u.txt"), "u\n")
	gitDo(t, dir, "add", "u.txt")
	gitDo(t, dir, "commit", "-m", "u")
	gitDo(t, dir, "checkout", "main")

	if err := DeleteBranch(dir, "unmerged", false); err == nil {
		t.Error("expected -d to refuse unmerged branch")
	}
	if err := DeleteBranch(dir, "unmerged", true); err != nil {
		t.Errorf("DeleteBranch(unmerged, force=true): %v", err)
	}
	branches, _ = RunGitCapture("-C", dir, "branch", "--list", "unmerged")
	if strings.TrimSpace(branches) != "" {
		t.Errorf("branch 'unmerged' still exists after force delete: %q", branches)
	}
}
