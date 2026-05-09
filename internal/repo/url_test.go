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
	for _, bad := range []string{"", "   ", "not-a-url", "https://github.com/", "https://github.com"} {
		if _, ok := ParseRemoteURL(bad); ok {
			t.Errorf("expected %q to fail", bad)
		}
	}
}

func TestParsedURLEqual(t *testing.T) {
	a := ParsedURL{Host: "github.com", Segments: []string{"u", "r"}}
	b := ParsedURL{Host: "github.com", Segments: []string{"u", "r"}}
	if !a.Equal(b) {
		t.Error("equal URLs should compare equal")
	}
	c := ParsedURL{Host: "gitlab.com", Segments: []string{"u", "r"}}
	if a.Equal(c) {
		t.Error("different host should not be equal")
	}
}


func TestParseRemoteURLRejectsTraversalSegments(t *testing.T) {
	bad := []string{
		"https://github.com/user/../repo.git",
		"https://github.com/user/./repo.git",
		"git@github.com:user/../../repo.git",
	}
	for _, u := range bad {
		if _, ok := ParseRemoteURL(u); ok {
			t.Errorf("expected %q to fail", u)
		}
	}
}
