package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPostCreateHookRunsOnSwitch: when mw creates a worktree (via
// `mw go`), the post-create hooks declared in the worktree's
// .makework.toml run with the MW_* env overlay set.
func TestPostCreateHookRunsOnSwitch(t *testing.T) {
	home := setupIsolatedEnv(t)
	if _, err := captureOutput(t, "init"); err != nil {
		t.Fatalf("mw init: %v", err)
	}

	scratch := filepath.Join(home, "scratch")
	mustMkdir(t, scratch)
	runGit(t, scratch, "init", "-q", "-b", "main")
	runGit(t, scratch, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit", "-q", "--allow-empty", "-m", "init")

	// Commit .makework.toml on main with a post-create hook that
	// writes a sentinel file containing MW_BRANCH.
	toml := `name = "scratch"

[hooks]
post-create = ["echo $MW_BRANCH > HOOK_RAN.txt"]
`
	if err := os.WriteFile(filepath.Join(scratch, ".makework.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write .makework.toml: %v", err)
	}
	runGit(t, scratch, "add", ".makework.toml")
	runGit(t, scratch, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit", "-q", "-m", "add hooks")

	// Create the feature branch BEFORE mw clones into a bare so the
	// bare has it.
	runGit(t, scratch, "checkout", "-q", "-b", "feature")
	runGit(t, scratch, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit", "-q", "--allow-empty", "-m", "feat")
	runGit(t, scratch, "checkout", "-q", "main")

	if out, err := captureOutput(t, "repo", "add", scratch); err != nil {
		t.Fatalf("mw repo add: %v\n%s", err, out)
	}

	cfg, cat := loadState()
	resolved, _ := cat.FindProjectUnambiguous("scratch")
	wtPath := resolvedWorktreePath(cfg, resolved, "feature")

	if out, err := captureOutput(t, "go", "--allow-post-create-hooks", "scratch@feature"); err != nil {
		t.Fatalf("mw go scratch@feature: %v\n%s", err, out)
	}

	got, err := os.ReadFile(filepath.Join(wtPath, "HOOK_RAN.txt"))
	if err != nil {
		t.Fatalf("HOOK_RAN.txt not created: %v", err)
	}
	if string(got) != "feature\n" {
		t.Errorf("HOOK_RAN.txt contents = %q; want %q", got, "feature\n")
	}
}
