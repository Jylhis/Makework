package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// lsFixture creates a repo with main + feature (one extra commit),
// registers it, and switches to create the feature worktree.
func lsFixture(t *testing.T) {
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
	runGit(t, scratch, "checkout", "-q", "-b", "feature")
	runGit(t, scratch, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit", "-q", "--allow-empty", "-m", "feat")
	runGit(t, scratch, "checkout", "-q", "main")

	if out, err := captureOutput(t, "repo", "add", scratch); err != nil {
		t.Fatalf("mw repo add: %v\n%s", err, out)
	}
	if out, err := captureOutput(t, "switch", "scratch", "feature"); err != nil {
		t.Fatalf("mw switch: %v\n%s", err, out)
	}
}

// TestLsJSON: `mw ls --format=json` emits a valid JSON array with the
// expected per-entry fields.
func TestLsJSON(t *testing.T) {
	lsFixture(t)

	out, err := captureOutput(t, "ls", "--format=json")
	if err != nil {
		t.Fatalf("mw ls --format=json: %v\n%s", err, out)
	}

	var entries []lsEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one worktree")
	}
	got := entries[0]
	if got.Repo != "scratch" {
		t.Errorf("Repo = %q; want scratch", got.Repo)
	}
	if got.Status.Branch != "feature" {
		t.Errorf("Branch = %q; want feature", got.Status.Branch)
	}
	if got.Status.MainAhead == 0 {
		t.Errorf("MainAhead = 0; want > 0 (feature is ahead of main)")
	}
}

// TestLsTable: default output is a table including expected columns.
func TestLsTable(t *testing.T) {
	lsFixture(t)

	out, err := captureOutput(t, "ls")
	if err != nil {
		t.Fatalf("mw ls: %v\n%s", err, out)
	}
	for _, col := range []string{"REPO", "BRANCH", "STATUS", "MAIN", "REMOTE", "PATH"} {
		if !strings.Contains(out, col) {
			t.Errorf("missing column %q in output:\n%s", col, out)
		}
	}
	if !strings.Contains(out, "feature") {
		t.Errorf("feature branch missing from table:\n%s", out)
	}
}
