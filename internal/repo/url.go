package repo

import (
	"strings"
	"unicode"
)

// ParsedURL is the structured form of a git remote URL, used to compute
// worktree paths (host + path segments, sans trailing .git).
type ParsedURL struct {
	Host     string
	Segments []string
}

// ParseRemoteURL parses the common git remote URL formats (https, http, ssh,
// scp-style git@host:path) and returns the host plus path segments with any
// trailing ".git" stripped. Returns false for unrecognized or malformed input.
func ParseRemoteURL(url string) (ParsedURL, bool) {
	url = strings.TrimSpace(url)
	if url == "" {
		return ParsedURL{}, false
	}

	switch {
	case strings.HasPrefix(url, "https://"):
		return parseHTTPLike(strings.TrimPrefix(url, "https://"))
	case strings.HasPrefix(url, "http://"):
		return parseHTTPLike(strings.TrimPrefix(url, "http://"))
	case strings.HasPrefix(url, "ssh://"):
		return parseSSH(strings.TrimPrefix(url, "ssh://"))
	case strings.HasPrefix(url, "git@"):
		return parseSCP(strings.TrimPrefix(url, "git@"))
	}
	return ParsedURL{}, false
}

func parseHTTPLike(rest string) (ParsedURL, bool) {
	hostPort, path, ok := cut(rest, "/")
	if !ok {
		return ParsedURL{}, false
	}
	return parseHostPath(hostPort, path)
}

func parseSSH(rest string) (ParsedURL, bool) {
	// Drop optional "user@" prefix.
	if i := strings.Index(rest, "@"); i >= 0 {
		rest = rest[i+1:]
	}
	hostPort, path, ok := cut(rest, "/")
	if !ok {
		return ParsedURL{}, false
	}
	return parseHostPath(hostPort, path)
}

func parseSCP(rest string) (ParsedURL, bool) {
	hostPort, path, ok := cut(rest, ":")
	if !ok {
		return ParsedURL{}, false
	}
	return parseHostPath(hostPort, path)
}

// parseHostPath validates a "host[:port]" and path pair (already split out by
// the format-specific parsers) into a ParsedURL. The port, if present, is
// dropped from the host.
func parseHostPath(hostPort, path string) (ParsedURL, bool) {
	if hostPort == "" || path == "" {
		return ParsedURL{}, false
	}
	host := hostPort
	if i := strings.Index(hostPort, ":"); i >= 0 {
		host = hostPort[:i]
	}
	if !isSafeHost(host) {
		return ParsedURL{}, false
	}
	segs := splitPathSegments(path)
	if len(segs) == 0 {
		return ParsedURL{}, false
	}
	return ParsedURL{Host: host, Segments: segs}, true
}

func splitPathSegments(path string) []string {
	var out []string
	for s := range strings.SplitSeq(path, "/") {
		if s == "" {
			continue
		}
		if !isSafeSegment(s) {
			return nil
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	last := out[len(out)-1]
	if stripped := strings.TrimSuffix(last, ".git"); stripped != last {
		switch {
		case stripped == "":
			// A trailing "/.git" leaves an empty segment; drop it and keep
			// the preceding segments (e.g. "user/repo/.git" -> [user, repo]).
			out = out[:len(out)-1]
		case !isSafeSegment(stripped):
			// Stripping ".git" can leave an unsafe segment (e.g. "..git"
			// becomes "."), so re-validate just it.
			return nil
		default:
			out[len(out)-1] = stripped
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// isSafeSegment reports whether s is safe to use as a single path component
// when materializing a remote URL onto the local filesystem. It rejects
// empty, "." and ".." segments, control characters that could corrupt
// line-oriented shell integration output, and any segment that contains a path
// separator on either Unix (/) or Windows (\), or a colon (which is a
// filename separator for NTFS alternate data streams).
//
// Callers that pass a raw "host:port" string must strip the port first,
// since the colon is rejected here.
func isSafeSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	return !strings.ContainsAny(s, `/\:`) && !strings.ContainsFunc(s, unicode.IsControl)
}

// isSafeHost validates a hostname using the same rules as isSafeSegment.
func isSafeHost(host string) bool {
	return isSafeSegment(host)
}

// cut is strings.Cut specialized for a single-character separator.
func cut(s, sep string) (before, after string, ok bool) {
	i := strings.Index(s, sep)
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+len(sep):], true
}
