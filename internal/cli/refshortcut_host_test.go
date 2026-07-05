package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jylhis/makework/internal/catalog"
	"github.com/jylhis/makework/internal/repo"
)

func TestResolveBranchShortcutPassesHostQualifiedRepoToGH(t *testing.T) {
	stubDir := t.TempDir()
	argsFile := filepath.Join(stubDir, "gh.args")
	gh := filepath.Join(stubDir, "gh")
	script := "#!/bin/sh\nprintf '%s\n' \"$@\" > \"" + argsFile + "\"\necho '{\"headRefName\":\"feat/ok\"}'\n"
	if err := os.WriteFile(gh, []byte(script), 0o755); err != nil {
		t.Fatalf("write gh stub: %v", err)
	}
	t.Setenv("PATH", stubDir+":"+os.Getenv("PATH"))

	url := "https://ghe.example/acme/secret.git"
	resolved := &catalog.ResolvedProject{Repo: &repo.Repository{Path: t.TempDir(), URL: &url}}

	branch, err := resolveBranchShortcut("pr:42", resolved)
	if err != nil {
		t.Fatalf("resolveBranchShortcut: %v", err)
	}
	if branch != "feat/ok" {
		t.Fatalf("branch = %q; want feat/ok", branch)
	}

	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	if !strings.Contains(string(got), "ghe.example/acme/secret") {
		t.Fatalf("gh args did not include host-qualified --repo value: %q", string(got))
	}
}
