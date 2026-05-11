package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestRunPostCreateRunsInCwd: each command runs with cwd=wtPath and
// sees the configured env vars.
func TestRunPostCreateRunsInCwd(t *testing.T) {
	wt := t.TempDir()
	cmds := []string{
		`touch ran.txt`,
		`echo $MW_BRANCH > branch.txt`,
		`echo $MW_REPO > repo.txt`,
		`echo $MW_WORKTREE_PATH > wt.txt`,
	}
	var buf bytes.Buffer
	env := map[string]string{
		"MW_WORKTREE_PATH": wt,
		"MW_BRANCH":        "feature",
		"MW_REPO":          "scratch",
	}
	if err := RunPostCreate(wt, cmds, env, &buf); err != nil {
		t.Fatalf("RunPostCreate: %v\n%s", err, buf.String())
	}

	for _, want := range []struct{ name, content string }{
		{"ran.txt", ""},
		{"branch.txt", "feature\n"},
		{"repo.txt", "scratch\n"},
		{"wt.txt", wt + "\n"},
	} {
		got, err := os.ReadFile(filepath.Join(wt, want.name))
		if err != nil {
			t.Errorf("missing %s: %v", want.name, err)
			continue
		}
		if want.content != "" && string(got) != want.content {
			t.Errorf("%s = %q; want %q", want.name, string(got), want.content)
		}
	}
}

// TestRunPostCreateStopsOnError: when one command fails, subsequent
// commands are NOT run and an error is returned.
func TestRunPostCreateStopsOnError(t *testing.T) {
	wt := t.TempDir()
	cmds := []string{
		`exit 1`,
		`touch should-not-exist.txt`,
	}
	var buf bytes.Buffer
	err := RunPostCreate(wt, cmds, nil, &buf)
	if err == nil {
		t.Fatal("expected error from failing command")
	}
	if _, err := os.Stat(filepath.Join(wt, "should-not-exist.txt")); err == nil {
		t.Error("second command should not have run after first failed")
	}
}

// TestRunPostCreateNoCmdsIsNoop: empty list returns nil and writes no
// output.
func TestRunPostCreateNoCmdsIsNoop(t *testing.T) {
	wt := t.TempDir()
	var buf bytes.Buffer
	if err := RunPostCreate(wt, nil, nil, &buf); err != nil {
		t.Errorf("expected nil for empty cmds, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}
