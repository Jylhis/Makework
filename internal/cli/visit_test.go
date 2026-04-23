package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeBareWithWorktree creates a bare clone of a scratch repo and adds a
// worktree checked out at `branch`. Returns (barePath, worktreePath).
func makeBareWithWorktree(t *testing.T, home, repoName, branch string) (string, string) {
	t.Helper()
	barePath := filepath.Join(home, "bares", repoName+".git")
	wtPath := filepath.Join(home, "worktrees", repoName, branch)

	scratch := filepath.Join(home, "scratch-"+repoName)
	mustMkdir(t, scratch)
	runGit(t, scratch, "init", "-q", "-b", branch)
	runGit(t, scratch, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")

	mustMkdir(t, filepath.Dir(barePath))
	runGit(t, scratch, "clone", "-q", "--bare", ".", barePath)

	mustMkdir(t, filepath.Dir(wtPath))
	runGit(t, barePath, "worktree", "add", "-q", wtPath, branch)

	return barePath, wtPath
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s (in %s) failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func writeRepoRoots(t *testing.T, home string, roots ...string) {
	t.Helper()
	stateDir := filepath.Join(home, ".local", "state", "makework")
	mustMkdir(t, stateDir)
	content := strings.Join(roots, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(stateDir, "repo-roots.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write repo-roots: %v", err)
	}
}

func readVisitKeys(t *testing.T, home string) []string {
	t.Helper()
	path := filepath.Join(home, ".local", "state", "makework", "visits.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var db struct {
		Entries []struct {
			Key string `json:"key"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &db); err != nil {
		t.Fatalf("unmarshal visits: %v", err)
	}
	keys := make([]string, len(db.Entries))
	for i, e := range db.Entries {
		keys[i] = e.Key
	}
	return keys
}

func hasKey(keys []string, want string) bool {
	for _, k := range keys {
		if k == want {
			return true
		}
	}
	return false
}

func TestVisitFromWorktreeRecordsVisit(t *testing.T) {
	home := setupIsolatedEnv(t)
	bare, wt := makeBareWithWorktree(t, home, "myrepo", "main")
	writeRepoRoots(t, home, bare)

	if _, err := captureOutput(t, "visit", wt); err != nil {
		t.Fatalf("mw visit: %v", err)
	}
	keys := readVisitKeys(t, home)
	if !hasKey(keys, "myrepo:main") {
		t.Errorf("expected 'myrepo:main' in visits, got %v", keys)
	}
}

func TestVisitFromWorktreeSubdirRecordsVisit(t *testing.T) {
	home := setupIsolatedEnv(t)
	bare, wt := makeBareWithWorktree(t, home, "myrepo", "main")
	writeRepoRoots(t, home, bare)
	sub := filepath.Join(wt, "nested", "deeper")
	mustMkdir(t, sub)

	if _, err := captureOutput(t, "visit", sub); err != nil {
		t.Fatalf("mw visit: %v", err)
	}
	if keys := readVisitKeys(t, home); !hasKey(keys, "myrepo:main") {
		t.Errorf("expected 'myrepo:main' from worktree subdir, got %v", keys)
	}
}

func TestVisitStripsGitSuffixFromRepoName(t *testing.T) {
	home := setupIsolatedEnv(t)
	bare, wt := makeBareWithWorktree(t, home, "myrepo", "main")
	writeRepoRoots(t, home, bare)
	if _, err := captureOutput(t, "visit", wt); err != nil {
		t.Fatalf("mw visit: %v", err)
	}
	keys := readVisitKeys(t, home)
	if !hasKey(keys, "myrepo:main") {
		t.Fatalf("expected 'myrepo:main' in visits, got %v", keys)
	}
	if hasKey(keys, "myrepo.git:main") {
		t.Errorf("'.git' suffix should be stripped from repo name, got %v", keys)
	}
}

func TestVisitFromUnrelatedRepoDoesNothing(t *testing.T) {
	home := setupIsolatedEnv(t)
	bare, _ := makeBareWithWorktree(t, home, "myrepo", "main")
	writeRepoRoots(t, home, bare)

	unrelated := filepath.Join(home, "elsewhere")
	mustMkdir(t, unrelated)
	runGit(t, unrelated, "init", "-q")

	if _, err := captureOutput(t, "visit", unrelated); err != nil {
		t.Fatalf("mw visit: %v", err)
	}
	if keys := readVisitKeys(t, home); len(keys) != 0 {
		t.Errorf("expected no visits from unrelated repo, got %v", keys)
	}
}

func TestVisitFromNonRepoDoesNothing(t *testing.T) {
	home := setupIsolatedEnv(t)
	bare, _ := makeBareWithWorktree(t, home, "myrepo", "main")
	writeRepoRoots(t, home, bare)

	plain := filepath.Join(home, "plain")
	mustMkdir(t, plain)

	if _, err := captureOutput(t, "visit", plain); err != nil {
		t.Fatalf("mw visit: %v", err)
	}
	if keys := readVisitKeys(t, home); len(keys) != 0 {
		t.Errorf("expected no visits from non-repo path, got %v", keys)
	}
}

func TestVisitMissingCacheIsSilentNoop(t *testing.T) {
	home := setupIsolatedEnv(t)
	_, wt := makeBareWithWorktree(t, home, "myrepo", "main")
	// Intentionally no writeRepoRoots call.

	if out, err := captureOutput(t, "visit", wt); err != nil {
		t.Fatalf("mw visit should not error on missing cache: %v (out: %s)", err, out)
	}
	if keys := readVisitKeys(t, home); len(keys) != 0 {
		t.Errorf("expected no visits without cache, got %v", keys)
	}
}
