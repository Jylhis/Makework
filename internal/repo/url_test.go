package repo

import (
	"reflect"
	"testing"
)

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   ParsedURL
		wantOK bool
	}{
		{"https with .git", "https://github.com/user/repo.git", ParsedURL{Host: "github.com", Segments: []string{"user", "repo"}}, true},
		{"https without .git", "https://github.com/user/repo", ParsedURL{Host: "github.com", Segments: []string{"user", "repo"}}, true},
		{"ssh scp-style", "git@github.com:user/repo.git", ParsedURL{Host: "github.com", Segments: []string{"user", "repo"}}, true},
		{"ssh scp-style multi segments", "git@gitlab.com:group/subgroup/repo", ParsedURL{Host: "gitlab.com", Segments: []string{"group", "subgroup", "repo"}}, true},
		{"ssh protocol", "ssh://git@github.com/user/repo.git", ParsedURL{Host: "github.com", Segments: []string{"user", "repo"}}, true},
		{"ssh protocol with port", "ssh://git@github.com:22/user/repo.git", ParsedURL{Host: "github.com", Segments: []string{"user", "repo"}}, true},
		{"ssh without user", "ssh://github.com/user/repo.git", ParsedURL{Host: "github.com", Segments: []string{"user", "repo"}}, true},
		{"http", "http://gitlab.internal/group/repo.git", ParsedURL{Host: "gitlab.internal", Segments: []string{"group", "repo"}}, true},
		{"trims whitespace", "  https://github.com/user/repo.git  ", ParsedURL{Host: "github.com", Segments: []string{"user", "repo"}}, true},
		{"https with port", "https://gitlab.example.com:8443/group/repo.git", ParsedURL{Host: "gitlab.example.com", Segments: []string{"group", "repo"}}, true},
		{"https trailing slash dot-git", "https://github.com/user/repo/.git", ParsedURL{Host: "github.com", Segments: []string{"user", "repo"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseRemoteURL(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseRemoteURLRejects(t *testing.T) {
	for _, bad := range []string{
		"", "   ", "not-a-url", "https://github.com/", "https://github.com",
		// Lexical traversal attempts via path components.
		"git@github.com:../repo.git", "git@github.com:org/../../repo.git",
		"https://../org/repo.git", "https://github.com/org/./repo.git",
		"https://github.com/../../outside/repo.git",
		"ssh://git@github.com/../evil.git",
		// Backslash separator (Windows path traversal).
		`https://github.com/org\evil/repo.git`,
		`git@github.com:org\..\evil.git`,
		// Colon in path segment (NTFS alternate data streams).
		"https://github.com/org/evil:stream/repo.git",
		// Control characters must not enter worktree paths or line-oriented
		// shell integration output.
		"http://127.0.0.1/org/\ntouch /tmp/pwn #/repo.git",
		"https://github.com/org/repo\r.git",
		"git@github.com:org/repo\tname.git",
	} {
		if _, ok := ParseRemoteURL(bad); ok {
			t.Errorf("expected %q to fail", bad)
		}
	}
}

func TestParsedURLEqual(t *testing.T) {
	a := ParsedURL{Host: "github.com", Segments: []string{"u", "r"}}
	b := ParsedURL{Host: "github.com", Segments: []string{"u", "r"}}
	if !reflect.DeepEqual(a, b) {
		t.Error("equal URLs should compare equal")
	}
	c := ParsedURL{Host: "gitlab.com", Segments: []string{"u", "r"}}
	if reflect.DeepEqual(a, c) {
		t.Error("different host should not be equal")
	}
}
