package search

import "testing"

func TestParseGrepLine(t *testing.T) {
	r, ok := parseGrepLine("/home/u/wt/src/lib.go:42:func hello() {}", "/home/u/wt/", "myrepo")
	if !ok {
		t.Fatal("expected ok")
	}
	if r.FilePath != "src/lib.go" {
		t.Errorf("file = %q", r.FilePath)
	}
	if r.LineNumber != 42 {
		t.Errorf("line = %d", r.LineNumber)
	}
	if r.LineContent != "func hello() {}" {
		t.Errorf("content = %q", r.LineContent)
	}
}

func TestParseGrepLineInvalid(t *testing.T) {
	if _, ok := parseGrepLine("no colons here", "/tmp/wt/", "r"); ok {
		t.Error("expected !ok")
	}
}

func TestParseGrepLineColonsInContent(t *testing.T) {
	r, ok := parseGrepLine("/tmp/wt/config.toml:10:key = \"value:with:colons\"", "/tmp/wt/", "r")
	if !ok {
		t.Fatal("expected ok")
	}
	if r.LineContent != "key = \"value:with:colons\"" {
		t.Errorf("content = %q", r.LineContent)
	}
}

func TestParseGrepLineNonNumeric(t *testing.T) {
	if _, ok := parseGrepLine("/tmp/wt/f:abc:content", "/tmp/wt/", "r"); ok {
		t.Error("expected !ok for non-numeric line number")
	}
}

func TestGrouped(t *testing.T) {
	results := []Result{
		{RepoName: "a"}, {RepoName: "b"}, {RepoName: "a"}, {RepoName: "b"},
	}
	g := Grouped(results)
	if len(g) != 2 || len(g["a"]) != 2 || len(g["b"]) != 2 {
		t.Errorf("grouped = %v", g)
	}
}
