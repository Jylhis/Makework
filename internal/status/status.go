// Package status queries git worktree state (branch, dirty, ahead/behind)
// and caches results as JSON.
package status

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jylhis/makework/internal/xdgpath"
)

// WorktreeStatus holds the computed state of one worktree.
type WorktreeStatus struct {
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	DirtyCount uint32 `json:"dirty_count"`
	Ahead      uint32 `json:"ahead"`
	Behind     uint32 `json:"behind"`
	IsOrphaned bool   `json:"is_orphaned"`
}

// RepoStatusCache is the JSON on-disk format under $XDG_STATE_HOME/makework/cache/<repo>.json.
type RepoStatusCache struct {
	RepoName  string           `json:"repo_name"`
	UpdatedAt string           `json:"updated_at"`
	Worktrees []WorktreeStatus `json:"worktrees"`
}

func cacheDir() (string, error) {
	state, err := xdgpath.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(state, "cache"), nil
}

// ReadCache loads the cached status for a repo or returns nil.
func ReadCache(repoName string) *RepoStatusCache {
	dir, err := cacheDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, repoName+".json"))
	if err != nil {
		return nil
	}
	var cache RepoStatusCache
	if json.Unmarshal(data, &cache) != nil {
		return nil
	}
	return &cache
}

// WriteCache persists a repo status cache to disk.
func WriteCache(cache *RepoStatusCache) error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, cache.RepoName+".json"), data, 0o644)
}

// Get computes the status of a single worktree.
func Get(wtPath string) WorktreeStatus {
	isOrphaned := false
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		isOrphaned = true
	}
	branch := gitOutput(wtPath, "rev-parse", "--abbrev-ref", "HEAD")
	if branch == "" {
		branch = "unknown"
	}
	dirty := countLines(gitOutput(wtPath, "status", "--porcelain"))
	ahead, behind := aheadBehind(wtPath)

	return WorktreeStatus{
		Path:       wtPath,
		Branch:     branch,
		DirtyCount: dirty,
		Ahead:      ahead,
		Behind:     behind,
		IsOrphaned: isOrphaned,
	}
}

func gitOutput(dir string, args ...string) string {
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func countLines(s string) uint32 {
	if s == "" {
		return 0
	}
	var n uint32
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			n++
		}
	}
	return n
}

func aheadBehind(dir string) (uint32, uint32) {
	out := gitOutput(dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	parts := strings.Split(strings.TrimSpace(out), "\t")
	if len(parts) != 2 {
		return 0, 0
	}
	a, _ := strconv.ParseUint(parts[0], 10, 32)
	b, _ := strconv.ParseUint(parts[1], 10, 32)
	return uint32(a), uint32(b)
}
