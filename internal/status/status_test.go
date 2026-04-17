package status

import (
	"encoding/json"
	"testing"
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
