// Package worktree manages git worktrees: path computation, creation,
// removal, listing, sparse-checkout, and pruning.
package worktree

import (
	"path/filepath"
	"strings"

	"github.com/jylhis/makework/internal/repo"
)

// Path computes the worktree directory for a (repo, branch) pair.
// When parsedURL is non-nil, the path is rooted under host/segments.
// Otherwise, under local/<repoName>.
func Path(worktreeRoot string, parsedURL *repo.ParsedURL, repoName, branch string) string {
	var parts []string
	parts = append(parts, worktreeRoot)

	if parsedURL != nil {
		parts = append(parts, parsedURL.Host)
		parts = append(parts, parsedURL.Segments...)
	} else {
		parts = append(parts, "local", repoName)
	}

	sanitized := SanitizeBranch(branch)
	for _, p := range strings.Split(sanitized, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return filepath.Join(parts...)
}

// ParentDir maps a bare repo path (under bareRoot) to its corresponding
// worktree parent directory (under worktreeRoot). Strips ".git" suffix from
// the final component as a string operation (not filepath.Ext) to correctly
// handle names like "jylhis.com.git" → "jylhis.com". Returns "" if barePath
// is not under bareRoot.
func ParentDir(worktreeRoot, bareRoot, barePath string) string {
	rel, err := filepath.Rel(bareRoot, barePath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	cleanRel := filepath.Clean(rel)
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return ""
	}
	base := filepath.Base(cleanRel)
	stem := strings.TrimSuffix(base, ".git")
	parent := filepath.Dir(cleanRel)
	if parent == "." {
		parent = ""
	}
	candidate := filepath.Join(worktreeRoot, parent, stem)
	cleanRoot := filepath.Clean(worktreeRoot)
	cleanCandidate := filepath.Clean(candidate)
	if cleanCandidate != cleanRoot && !strings.HasPrefix(cleanCandidate, cleanRoot+string(filepath.Separator)) {
		return ""
	}
	return cleanCandidate
}

// SanitizeBranch converts a ref name into a filesystem-safe path.
// Strips "refs/tags/" prefix, replaces problematic characters with "_",
// and collapses/trims slashes.
func SanitizeBranch(refName string) string {
	name := strings.TrimPrefix(refName, "refs/tags/")
	var b strings.Builder
	b.Grow(len(name))
	for _, ch := range name {
		switch ch {
		case ':', ' ', '\\', '?', '*', '<', '>', '|', '"':
			b.WriteByte('_')
		default:
			b.WriteRune(ch)
		}
	}
	parts := strings.Split(b.String(), "/")
	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, "/")
}
