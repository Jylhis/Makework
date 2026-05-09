package worktree

import (
	"testing"

	"github.com/jylhis/makework/internal/repo"
)

func TestPathWithParsedURL(t *testing.T) {
	url := &repo.ParsedURL{Host: "github.com", Segments: []string{"user", "repo"}}
	got := Path("/data/worktrees", url, "repo", "main")
	want := "/data/worktrees/github.com/user/repo/main"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPathBranchSlash(t *testing.T) {
	url := &repo.ParsedURL{Host: "github.com", Segments: []string{"user", "repo"}}
	got := Path("/data/worktrees", url, "repo", "feature/auth")
	want := "/data/worktrees/github.com/user/repo/feature/auth"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPathLocalRepo(t *testing.T) {
	got := Path("/data/worktrees", nil, "my-project", "develop")
	want := "/data/worktrees/local/my-project/develop"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPathSubgroupSegments(t *testing.T) {
	url := &repo.ParsedURL{Host: "gitlab.com", Segments: []string{"group", "subgroup", "repo"}}
	got := Path("/data/worktrees", url, "repo", "main")
	want := "/data/worktrees/gitlab.com/group/subgroup/repo/main"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParentDir(t *testing.T) {
	got := ParentDir("/data/worktrees", "/data/repos", "/data/repos/github.com/user/repo.git")
	want := "/data/worktrees/github.com/user/repo"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParentDirDotInName(t *testing.T) {
	got := ParentDir("/data/worktrees", "/data/repos", "/data/repos/jylhis.com.git")
	want := "/data/worktrees/jylhis.com"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}


func TestParentDirRejectsEscapingRelative(t *testing.T) {
	got := ParentDir("/data/worktrees", "/data/repos", "/data/repos/github.com/../../outside/repo.git")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestParentDirNotUnderBareRoot(t *testing.T) {
	got := ParentDir("/data/worktrees", "/data/repos", "/other/path/repo.git")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestSanitizeBranch(t *testing.T) {
	tests := []struct{ in_, want string }{
		{"main", "main"},
		{"feature/auth", "feature/auth"},
		{"refs/tags/v1.0", "v1.0"},
		{"fix:bug", "fix_bug"},
		{"my branch name", "my_branch_name"},
		{`a\b?c*d<e>f|g"h`, "a_b_c_d_e_f_g_h"},
		{"a//b///c", "a/b/c"},
		{"/leading/trailing/", "leading/trailing"},
		{"refs/tags/release/1.0", "release/1.0"},
	}
	for _, tt := range tests {
		got := SanitizeBranch(tt.in_)
		if got != tt.want {
			t.Errorf("SanitizeBranch(%q) = %q, want %q", tt.in_, got, tt.want)
		}
	}
}

func TestParsePorcelain(t *testing.T) {
	output := "worktree /home/user/repos/test.git\nHEAD abc123\nbranch refs/heads/main\nbare\n\nworktree /home/user/worktrees/test/feature\nHEAD def456\nbranch refs/heads/feature/auth\n\n"
	wts := parsePorcelain(output)
	if len(wts) != 2 {
		t.Fatalf("want 2 worktrees, got %d", len(wts))
	}
	if wts[0].Path != "/home/user/repos/test.git" || wts[0].Branch != "main" || !wts[0].IsBare {
		t.Errorf("wt[0] = %+v", wts[0])
	}
	if wts[1].Path != "/home/user/worktrees/test/feature" || wts[1].Branch != "feature/auth" || wts[1].IsBare {
		t.Errorf("wt[1] = %+v", wts[1])
	}
}

func TestParsePorcelainDetached(t *testing.T) {
	output := "worktree /tmp/bare.git\nHEAD abc123\nbare\n\nworktree /tmp/detached\nHEAD def456\ndetached\n"
	wts := parsePorcelain(output)
	if len(wts) != 2 {
		t.Fatalf("want 2, got %d", len(wts))
	}
	if wts[1].Branch != "" {
		t.Errorf("detached branch should be empty, got %q", wts[1].Branch)
	}
}

func TestParsePorcelainNoTrailingNewline(t *testing.T) {
	output := "worktree /tmp/bare.git\nHEAD abc\nbare"
	wts := parsePorcelain(output)
	if len(wts) != 1 || wts[0].Path != "/tmp/bare.git" || !wts[0].IsBare {
		t.Errorf("got %+v", wts)
	}
}
