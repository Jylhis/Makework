package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupIsolatedEnv creates isolated XDG dirs for testing CLI commands.
func setupIsolatedEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	return home
}

// captureOutput runs a cobra command and captures stdout.
func captureOutput(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRoot()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestInitCreatesDirectories(t *testing.T) {
	home := setupIsolatedEnv(t)
	out, err := captureOutput(t, "init")
	if err != nil {
		t.Fatalf("init failed: %v\noutput: %s", err, out)
	}

	// Should have created config dir, config.toml, catalog.toml, bare_root, worktree_root
	configDir := filepath.Join(home, ".config", "makework")
	if _, err := os.Stat(filepath.Join(configDir, "config.toml")); err != nil {
		t.Error("config.toml not created")
	}
	if _, err := os.Stat(filepath.Join(configDir, "catalog.toml")); err != nil {
		t.Error("catalog.toml not created")
	}
}

func TestInitIdempotent(t *testing.T) {
	setupIsolatedEnv(t)
	// Run twice — second should not error
	if _, err := captureOutput(t, "init"); err != nil {
		t.Fatalf("first init: %v", err)
	}
	out, err := captureOutput(t, "init")
	if err != nil {
		t.Fatalf("second init: %v", err)
	}
	if out == "" {
		t.Error("expected output on second init")
	}
}

func TestRepoListEmpty(t *testing.T) {
	setupIsolatedEnv(t)
	_, _ = captureOutput(t, "init")
	out, err := captureOutput(t, "repo", "list")
	if err != nil {
		t.Fatalf("repo list: %v", err)
	}
	if out == "" {
		t.Error("expected output from repo list")
	}
}

// initSourceRepo lays out a throwaway git repo with a single empty commit.
// Used by repo add/remove tests that need a real clone source.
func initSourceRepo(t *testing.T, home, name string) string {
	t.Helper()
	src := filepath.Join(home, name)
	mustMkdir(t, src)
	runGit(t, src, "init", "-q", "-b", "main")
	runGit(t, src, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit", "-q", "--allow-empty", "-m", "init")
	return src
}

func TestRepoAddWritesRepoRootsCache(t *testing.T) {
	home := setupIsolatedEnv(t)
	if _, err := captureOutput(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	src := initSourceRepo(t, home, "source-repo")

	if out, err := captureOutput(t, "repo", "add", src); err != nil {
		t.Fatalf("repo add: %v (out: %s)", err, out)
	}

	cachePath := filepath.Join(home, ".local", "state", "makework", "repo-roots.txt")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("repo-roots.txt should exist after repo add: %v", err)
	}
	if !bytes.Contains(data, []byte("source-repo.git")) {
		t.Errorf("expected source-repo.git bare path in cache, got %q", data)
	}
}

func TestRepoRmUpdatesRepoRootsCache(t *testing.T) {
	home := setupIsolatedEnv(t)
	if _, err := captureOutput(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	src := initSourceRepo(t, home, "source-repo")
	if _, err := captureOutput(t, "repo", "add", src); err != nil {
		t.Fatalf("repo add: %v", err)
	}
	if _, err := captureOutput(t, "repo", "rm", "source-repo"); err != nil {
		t.Fatalf("repo rm: %v", err)
	}

	cachePath := filepath.Join(home, ".local", "state", "makework", "repo-roots.txt")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("repo-roots.txt should still exist after rm: %v", err)
	}
	if bytes.Contains(data, []byte("source-repo.git")) {
		t.Errorf("expected source-repo.git purged from cache, got %q", data)
	}
}

func TestRepoPurgeUpdatesRepoRootsCache(t *testing.T) {
	home := setupIsolatedEnv(t)
	if _, err := captureOutput(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	src := initSourceRepo(t, home, "source-repo")
	if _, err := captureOutput(t, "repo", "add", src); err != nil {
		t.Fatalf("repo add: %v", err)
	}
	if _, err := captureOutput(t, "repo", "purge", "source-repo"); err != nil {
		t.Fatalf("repo purge: %v", err)
	}
	cachePath := filepath.Join(home, ".local", "state", "makework", "repo-roots.txt")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("repo-roots.txt should still exist after purge: %v", err)
	}
	if bytes.Contains(data, []byte("source-repo.git")) {
		t.Errorf("expected source-repo.git purged from cache, got %q", data)
	}
}

func TestConfigShowDefaults(t *testing.T) {
	setupIsolatedEnv(t)
	out, err := captureOutput(t, "config", "show")
	if err != nil {
		t.Fatalf("config show: %v\noutput: %s", err, out)
	}
	if out == "" {
		t.Error("expected config show output")
	}
}

func TestConfigSetAndShow(t *testing.T) {
	home := setupIsolatedEnv(t)
	_, _ = captureOutput(t, "init")
	_, err := captureOutput(t, "config", "set", "worktree_root", filepath.Join(home, "custom-wt"))
	if err != nil {
		t.Fatalf("config set: %v", err)
	}
	out, err := captureOutput(t, "config", "show")
	if err != nil {
		t.Fatalf("config show: %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("custom-wt")) {
		t.Errorf("expected custom-wt in output: %s", out)
	}
}

func TestConfigUnknownSubcommandErrors(t *testing.T) {
	setupIsolatedEnv(t)
	_, err := captureOutput(t, "config", "nonsense")
	if err == nil {
		t.Fatal("expected error for unknown config subcommand, got nil")
	}
}

func TestVersionFlag(t *testing.T) {
	out, err := captureOutput(t, "--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	if out == "" {
		t.Error("expected version output")
	}
}

func TestAiInitOutputsSkill(t *testing.T) {
	out, err := captureOutput(t, "ai", "init")
	if err != nil {
		t.Fatalf("ai init: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "name: makework") {
		t.Error("expected YAML frontmatter with 'name: makework'")
	}
	if !strings.Contains(out, "mw go") {
		t.Error("expected 'mw go' command reference in skill output")
	}
	if !strings.Contains(out, "mw repo list") {
		t.Error("expected 'mw repo list' in skill output")
	}
}

func TestAiInitContainsFrontmatter(t *testing.T) {
	out, err := captureOutput(t, "ai", "init")
	if err != nil {
		t.Fatalf("ai init: %v", err)
	}
	if !strings.HasPrefix(out, "---\n") {
		t.Error("expected output to start with YAML frontmatter delimiter")
	}
	// Should have closing frontmatter
	if !strings.Contains(out, "\n---\n") {
		t.Error("expected closing frontmatter delimiter")
	}
}
