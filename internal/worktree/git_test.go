package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// setupBare initialises an empty bare repo seeded with one commit on
// main via a temporary working clone. Returns the bare path.
func setupBare(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	bare := filepath.Join(home, "repo.git")
	if err := exec.Command("git", "init", "--bare", "-q", bare).Run(); err != nil {
		t.Skipf("git init unavailable: %v", err)
	}
	seed := filepath.Join(home, "seed")
	if err := exec.Command("git", "clone", "-q", bare, seed).Run(); err != nil {
		t.Skipf("git clone unavailable: %v", err)
	}
	for _, cfg := range [][]string{
		{"user.email", "t@e"},
		{"user.name", "t"},
		{"commit.gpgsign", "false"},
	} {
		if err := exec.Command("git", "-C", seed, "config", cfg[0], cfg[1]).Run(); err != nil {
			t.Fatalf("git config: %v", err)
		}
	}
	if err := exec.Command("git", "-C", seed, "commit", "-q", "--allow-empty", "-m", "init").Run(); err != nil {
		t.Fatalf("seed commit: %v", err)
	}
	if err := exec.Command("git", "-C", seed, "push", "-q", "origin", "HEAD:refs/heads/main").Run(); err != nil {
		t.Fatalf("seed push: %v", err)
	}
	return bare
}

func TestCreateRemoveList(t *testing.T) {
	bare := setupBare(t)
	wtPath := filepath.Join(t.TempDir(), "wt-main")

	if err := Create(bare, "main", wtPath); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir missing: %v", err)
	}

	wts, err := List(bare)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !slices.ContainsFunc(wts, func(i Info) bool { return !i.IsBare && i.Branch == "main" }) {
		t.Errorf("List did not include a non-bare main worktree: %+v", wts)
	}

	if err := Remove(bare, wtPath); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree dir not removed: %v", err)
	}
}

func TestCreateBranch(t *testing.T) {
	bare := setupBare(t)
	wtPath := filepath.Join(t.TempDir(), "wt-feat")

	if err := CreateBranch(bare, "feature", wtPath, "main"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	wts, err := List(bare)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !slices.ContainsFunc(wts, func(i Info) bool { return i.Branch == "feature" }) {
		t.Errorf("feature branch not in list: %+v", wts)
	}
}

func TestPruneOrphan(t *testing.T) {
	bare := setupBare(t)
	wtPath := filepath.Join(t.TempDir(), "wt-orphan")

	if err := Create(bare, "main", wtPath); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Simulate orphaning: nuke the worktree dir behind git's back.
	if err := os.RemoveAll(wtPath); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	n, err := Prune(bare)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Errorf("orphaned count = %d; want 1", n)
	}
}

func TestEnableSparseCheckout(t *testing.T) {
	bare := setupBare(t)
	wtPath := filepath.Join(t.TempDir(), "wt-sparse")
	if err := Create(bare, "main", wtPath); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := EnableSparseCheckout(wtPath, []string{"some/dir"}); err != nil {
		t.Fatalf("EnableSparseCheckout: %v", err)
	}
	cfg, err := exec.Command("git", "-C", wtPath, "config", "core.sparseCheckout").Output()
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(cfg) == "" {
		t.Errorf("core.sparseCheckout not set")
	}
}

func TestCreateBranchRejectsOptionLikeBranchName(t *testing.T) {
	err := CreateBranch("/tmp/bare.git", "-D", "/tmp/wt", "main")
	if err == nil {
		t.Fatalf("expected error for option-like branch name")
	}
	if !strings.Contains(err.Error(), "must not start with '-'") {
		t.Fatalf("unexpected error: %v", err)
	}
}
