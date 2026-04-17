package query

import "testing"

func TestParseLogLineValid(t *testing.T) {
	e, ok := ParseLogLine("abc1234\tTest User\t2026-03-29T10:00:00+00:00\tfix: something", "myrepo", "main", "/tmp/wt")
	if !ok {
		t.Fatal("expected ok")
	}
	if e.CommitHash != "abc1234" {
		t.Errorf("hash = %q", e.CommitHash)
	}
	if e.Author != "Test User" {
		t.Errorf("author = %q", e.Author)
	}
	if e.Message != "fix: something" {
		t.Errorf("msg = %q", e.Message)
	}
}

func TestParseLogLineInvalid(t *testing.T) {
	if _, ok := ParseLogLine("not enough fields", "r", "m", "/tmp"); ok {
		t.Error("expected !ok")
	}
}

func TestParseLogLineTabsInMessage(t *testing.T) {
	e, ok := ParseLogLine("abc\tAlice\t2026-01-01\tfix: a\tb\tc", "r", "m", "/tmp")
	if !ok {
		t.Fatal("expected ok")
	}
	if e.Message != "fix: a\tb\tc" {
		t.Errorf("msg = %q", e.Message)
	}
}

func TestParseLogLineThreeFields(t *testing.T) {
	if _, ok := ParseLogLine("hash\tauthor\tdate", "r", "m", "/tmp"); ok {
		t.Error("expected !ok for 3 fields")
	}
}

func TestDedupSameRepo(t *testing.T) {
	entries := []Entry{
		{RepoName: "r", CommitHash: "abc"},
		{RepoName: "r", CommitHash: "abc"},
	}
	got := Dedup(entries)
	if len(got) != 1 {
		t.Errorf("want 1, got %d", len(got))
	}
}

func TestDedupDifferentRepos(t *testing.T) {
	entries := []Entry{
		{RepoName: "a", CommitHash: "abc"},
		{RepoName: "b", CommitHash: "abc"},
	}
	got := Dedup(entries)
	if len(got) != 2 {
		t.Errorf("want 2, got %d", len(got))
	}
}

func TestDedupEmpty(t *testing.T) {
	got := Dedup(nil)
	if len(got) != 0 {
		t.Error("expected empty")
	}
}

func TestSummary(t *testing.T) {
	entries := []Entry{
		{RepoName: "a"}, {RepoName: "b"}, {RepoName: "a"},
	}
	s := Summary(entries)
	if len(s) != 2 || len(s["a"]) != 2 || len(s["b"]) != 1 {
		t.Errorf("summary = %v", s)
	}
}
