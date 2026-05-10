package worktree

import (
	"strings"
	"testing"
)

func TestCreateBranchRejectsOptionLikeBranchName(t *testing.T) {
	err := CreateBranch("/tmp/bare.git", "-D", "/tmp/wt", "main")
	if err == nil {
		t.Fatalf("expected error for option-like branch name")
	}
	if !strings.Contains(err.Error(), "must not start with '-'" ) {
		t.Fatalf("unexpected error: %v", err)
	}
}
