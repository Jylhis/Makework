// Package search greps across worktrees for a pattern.
package search

import (
	"os/exec"
	"strconv"
	"strings"
)

// Result is one grep match.
type Result struct {
	RepoName     string
	WorktreePath string
	FilePath     string
	LineNumber   uint32
	LineContent  string
}

// Options control grep behavior.
type Options struct {
	FileGlob        string
	CaseInsensitive bool
	MaxResults      uint
}

// Worktree greps a single worktree directory for pattern. The caller
// supplies the repoName to tag results.
func Worktree(wtPath, pattern, repoName string, opts Options) []Result {
	args := []string{"-rn"}
	if opts.CaseInsensitive {
		args = append(args, "-i")
	}
	if opts.FileGlob != "" {
		args = append(args, "--include="+opts.FileGlob)
	}
	args = append(args, "--", pattern, wtPath)

	out, err := exec.Command("grep", args...).Output()
	if err != nil {
		return nil
	}

	prefix := wtPath + "/"
	var results []Result
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		r, ok := parseGrepLine(line, prefix, repoName)
		if !ok {
			continue
		}
		results = append(results, r)
		if opts.MaxResults > 0 && uint(len(results)) >= opts.MaxResults {
			break
		}
	}
	return results
}

// Grouped organises results by RepoName.
func Grouped(results []Result) map[string][]Result {
	m := make(map[string][]Result)
	for _, r := range results {
		m[r.RepoName] = append(m[r.RepoName], r)
	}
	return m
}

func parseGrepLine(line, wtPrefix, repoName string) (Result, bool) {
	parts := strings.SplitN(line, ":", 3)
	if len(parts) < 3 {
		return Result{}, false
	}
	num, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return Result{}, false
	}
	file := parts[0]
	if rel, ok := strings.CutPrefix(file, wtPrefix); ok {
		file = rel
	}
	return Result{
		RepoName:     repoName,
		WorktreePath: strings.TrimSuffix(wtPrefix, "/"),
		FilePath:     file,
		LineNumber:   uint32(num),
		LineContent:  parts[2],
	}, true
}
