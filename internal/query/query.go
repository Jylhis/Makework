// Package query collects recent git log entries across worktrees.
package query

import (
	"os/exec"
	"sort"
	"strings"
	"unicode"
)

// Entry is one parsed commit from git log.
type Entry struct {
	RepoName     string
	Branch       string
	CommitHash   string
	Author       string
	Date         string
	Message      string
	WorktreePath string
}

// ParseLogLine parses a tab-separated git log line:
// <hash>\t<author>\t<date>\t<message>
func ParseLogLine(line, repoName, branch, wtPath string) (Entry, bool) {
	parts := strings.SplitN(line, "\t", 4)
	if len(parts) < 4 {
		return Entry{}, false
	}
	return Entry{
		RepoName:     repoName,
		Branch:       branch,
		CommitHash:   parts[0],
		Author:       sanitizeTerminalText(parts[1]),
		Date:         parts[2],
		Message:      sanitizeTerminalText(parts[3]),
		WorktreePath: wtPath,
	}, true
}

func sanitizeTerminalText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// Dedup removes duplicate entries sharing the same (RepoName, CommitHash).
// Cherry-picks across repos are preserved.
func Dedup(entries []Entry) []Entry {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].RepoName != entries[j].RepoName {
			return entries[i].RepoName < entries[j].RepoName
		}
		return entries[i].CommitHash < entries[j].CommitHash
	})
	out := entries[:0]
	for i, e := range entries {
		if i > 0 && e.RepoName == entries[i-1].RepoName && e.CommitHash == entries[i-1].CommitHash {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Summary groups entries by RepoName.
func Summary(entries []Entry) map[string][]Entry {
	m := make(map[string][]Entry)
	for _, e := range entries {
		m[e.RepoName] = append(m[e.RepoName], e)
	}
	return m
}

// LogWorktree runs `git log --since=<since> --format=...` in the given
// worktree and returns parsed entries.
func LogWorktree(wtPath, branch, repoName, since string, until, author *string) []Entry {
	args := []string{"-C", wtPath, "log", "--since=" + since, "--format=%H\t%an\t%aI\t%s"}
	if until != nil && *until != "" {
		args = append(args, "--until="+*until)
	}
	if author != nil && *author != "" {
		args = append(args, "--author="+*author)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil
	}
	var entries []Entry
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		if e, ok := ParseLogLine(line, repoName, branch, wtPath); ok {
			entries = append(entries, e)
		}
	}
	return entries
}
