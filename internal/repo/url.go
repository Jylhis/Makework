package repo

import "strings"

// ParsedURL is the structured form of a git remote URL, used to compute
// worktree paths (host + path segments, sans trailing .git).
type ParsedURL struct {
	Host     string
	Segments []string
}

// Equal reports whether two ParsedURLs are equivalent.
func (p ParsedURL) Equal(other ParsedURL) bool {
	if p.Host != other.Host || len(p.Segments) != len(other.Segments) {
		return false
	}
	for i, s := range p.Segments {
		if s != other.Segments[i] {
			return false
		}
	}
	return true
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
	host, path, ok := cut(rest, "/")
	if !ok || host == "" || path == "" {
		return ParsedURL{}, false
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

func parseSSH(rest string) (ParsedURL, bool) {
	// Drop optional "user@" prefix.
	if i := strings.Index(rest, "@"); i >= 0 {
		rest = rest[i+1:]
	}
	hostPort, path, ok := cut(rest, "/")
	if !ok || hostPort == "" || path == "" {
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
	if host == "" || len(segs) == 0 {
		return ParsedURL{}, false
	}
	return ParsedURL{Host: host, Segments: segs}, true
}

func parseSCP(rest string) (ParsedURL, bool) {
	host, path, ok := cut(rest, ":")
	if !ok || host == "" || path == "" {
		return ParsedURL{}, false
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
	for _, s := range strings.Split(path, "/") {
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
		out[len(out)-1] = stripped
	}
	// Re-filter in case the strip left an empty segment (".git" alone).
	filtered := out[:0]
	for _, s := range out {
		if s == "" {
			continue
		}
		if !isSafeSegment(s) {
			return nil
		}
		filtered = append(filtered, s)
	}
	return filtered
}

func isSafeSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	return !strings.Contains(s, `/`) && !strings.Contains(s, `\\`)
}

func isSafeHost(host string) bool {
	if host == "" || host == "." || host == ".." {
		return false
	}
	return !strings.Contains(host, "/") && !strings.Contains(host, `\\`)
}

// cut is strings.Cut specialized for a single-character separator.
func cut(s, sep string) (before, after string, ok bool) {
	i := strings.Index(s, sep)
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+len(sep):], true
}
