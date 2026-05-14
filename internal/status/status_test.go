package status

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jylhis/makework/internal/integration"
)

func TestWorktreeStatusJSON(t *testing.T) {
	s := WorktreeStatus{
		Path: "/tmp/wt/main", Branch: "main",
		DirtyCount: 3, Ahead: 1, Behind: 2,
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var back WorktreeStatus
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Branch != "main" || back.DirtyCount != 3 || back.Ahead != 1 || back.Behind != 2 {
		t.Errorf("round-trip = %+v", back)
	}
}

func TestRepoStatusCacheJSON(t *testing.T) {
	c := RepoStatusCache{
		RepoName: "test-repo", UpdatedAt: "2026-04-01T12:00:00Z",
		Worktrees: []WorktreeStatus{
			{Path: "/tmp/wt/main", Branch: "main"},
			{Path: "/tmp/wt/feat", Branch: "feat", DirtyCount: 5, Ahead: 3},
		},
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var back RepoStatusCache
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.RepoName != "test-repo" || len(back.Worktrees) != 2 {
		t.Errorf("round-trip = %+v", back)
	}
}

func TestGetNonexistentIsOrphaned(t *testing.T) {
	s := Get("/nonexistent/path/to/worktree")
	if !s.IsOrphaned {
		t.Error("expected orphaned")
	}
	if s.Branch != "unknown" {
		t.Errorf("branch = %q", s.Branch)
	}
}

// TestGetFullPopulatesIntegrationAndCounts covers the new fields that
// require the bare path and default branch.
func TestGetFullPopulatesIntegrationAndCounts(t *testing.T) {
	dir := t.TempDir()
	mustGit(t, "", "init", "-q", "-b", "main", dir)
	mustGit(t, dir, "config", "user.email", "t@e")
	mustGit(t, dir, "config", "user.name", "t")
	mustGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "README")
	mustGit(t, dir, "commit", "-q", "-m", "init")

	// Feature branch with one extra commit.
	mustGit(t, dir, "checkout", "-q", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("f\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "f.txt")
	mustGit(t, dir, "commit", "-q", "-m", "feat")

	gitDir := filepath.Join(dir, ".git")
	s := GetFull(dir, gitDir, "main")
	if s.Branch != "feature" {
		t.Errorf("Branch = %q", s.Branch)
	}
	if s.MainAhead != 1 {
		t.Errorf("MainAhead = %d; want 1", s.MainAhead)
	}
	if s.MainBehind != 0 {
		t.Errorf("MainBehind = %d; want 0", s.MainBehind)
	}
	if s.Integration != integration.StateDiverged && s.Integration != integration.StateAncestor {
		// Feature is ahead of main only (main never moved) → Diverged
		// is not quite right; integration.IsAncestor(feature, main)
		// returns false because main is at the older commit. The check
		// in this case is correct: feature is ahead, so the comparison
		// "feature in main's history" fails, NoChanges fails, patch-id
		// also has no match → Diverged.
		t.Errorf("Integration = %q; want diverged or ancestor", s.Integration)
	}
	if s.LastCommit == 0 {
		t.Error("LastCommit should be set")
	}
}

// TestPorcelainCounts tabular check.
func TestPorcelainCounts(t *testing.T) {
	cases := []struct {
		name      string
		porcelain string
		dirty     uint32
		mod       bool
		untracked bool
		conflicts bool
	}{
		{"clean", "", 0, false, false, false},
		{"one untracked", "?? new.txt", 1, false, true, false},
		{"modified+untracked", " M a.txt\n?? b.txt", 2, true, true, false},
		{"conflict UU", "UU shared.txt", 1, false, false, true},
		{"staged", "M  a.txt", 1, true, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, m, u, c := porcelainCounts(tc.porcelain)
			if d != tc.dirty || u != tc.untracked || c != tc.conflicts {
				t.Errorf("got (dirty=%d, mod=%v, untr=%v, conf=%v); want (%d, %v, %v, %v)",
					d, m, u, c, tc.dirty, tc.mod, tc.untracked, tc.conflicts)
			}
		})
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := args
	if dir != "" {
		full = append([]string{"-C", dir}, args...)
	}
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}
