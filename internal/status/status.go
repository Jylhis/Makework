// Package status queries git worktree state (branch, dirty, ahead/behind).
package status

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jylhis/makework/internal/integration"
)

// WorktreeStatus holds the computed state of one worktree.
//
// Ahead/Behind are vs the upstream tracking branch.
// MainAhead/MainBehind are vs the repo's default branch.
// Integration classifies the branch's relationship to the default
// branch (set only when GetFull is used).
type WorktreeStatus struct {
	Path        string            `json:"path"`
	Branch      string            `json:"branch"`
	DirtyCount  uint32            `json:"dirty_count"`
	Modified    bool              `json:"modified"`
	Untracked   bool              `json:"untracked"`
	Conflicts   bool              `json:"conflicts"`
	Ahead       uint32            `json:"ahead"`
	Behind      uint32            `json:"behind"`
	MainAhead   uint32            `json:"main_ahead"`
	MainBehind  uint32            `json:"main_behind"`
	Integration integration.State `json:"integration,omitempty"`
	LastCommit  int64             `json:"last_commit_ts,omitempty"`
	IsOrphaned  bool              `json:"is_orphaned"`
}

// Get computes the basic status of a single worktree (no integration
// state or main-relative counts).
func Get(wtPath string) WorktreeStatus {
	return GetFull(wtPath, "", "")
}

// GetFull computes the full status of a single worktree. barePath and
// defaultBranch enable integration classification and main-relative
// counts; pass empty strings to skip those.
func GetFull(wtPath, barePath, defaultBranch string) WorktreeStatus {
	isOrphaned := false
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		isOrphaned = true
	}
	branch := gitOutput(wtPath, "rev-parse", "--abbrev-ref", "HEAD")
	if branch == "" {
		branch = "unknown"
	}

	dirty, mod, untr, conf := porcelainCounts(gitOutput(wtPath, "status", "--porcelain"))
	ahead, behind := aheadBehind(wtPath)
	lastTs := commitTimestamp(wtPath)

	s := WorktreeStatus{
		Path:       wtPath,
		Branch:     branch,
		DirtyCount: dirty,
		Modified:   mod,
		Untracked:  untr,
		Conflicts:  conf,
		Ahead:      ahead,
		Behind:     behind,
		LastCommit: lastTs,
		IsOrphaned: isOrphaned,
	}

	if barePath != "" && defaultBranch != "" && branch != "unknown" && !isOrphaned {
		s.MainAhead, s.MainBehind = mainAheadBehind(wtPath, defaultBranch)
		if state, err := integration.Check(barePath, branch, defaultBranch); err == nil {
			s.Integration = state
		}
	}
	return s
}

func gitOutput(dir string, args ...string) string {
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// porcelainCounts parses `git status --porcelain` output, returning the
// total dirty count plus subcategory flags. Conflict markers are XX
// (UU/AA/DD/AU/UD/DU/UA).
func porcelainCounts(s string) (dirty uint32, modified, untracked, conflicts bool) {
	if s == "" {
		return 0, false, false, false
	}
	for _, line := range strings.Split(s, "\n") {
		if len(line) < 2 {
			continue
		}
		dirty++
		x, y := line[0], line[1]
		switch {
		case x == '?' && y == '?':
			untracked = true
		case x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D'):
			conflicts = true
		default:
			modified = true
		}
	}
	return
}

// mainAheadBehind counts commits ahead/behind the default branch.
func mainAheadBehind(wtPath, defaultBranch string) (uint32, uint32) {
	out := gitOutput(wtPath, "rev-list", "--left-right", "--count",
		"HEAD..."+defaultBranch)
	parts := strings.Split(strings.TrimSpace(out), "\t")
	if len(parts) != 2 {
		return 0, 0
	}
	a, _ := strconv.ParseUint(parts[0], 10, 32)
	b, _ := strconv.ParseUint(parts[1], 10, 32)
	return uint32(a), uint32(b)
}

// commitTimestamp returns the unix timestamp of HEAD's commit.
func commitTimestamp(wtPath string) int64 {
	out := gitOutput(wtPath, "log", "-1", "--format=%ct")
	ts, _ := strconv.ParseInt(out, 10, 64)
	return ts
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
