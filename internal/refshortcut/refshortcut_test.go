package refshortcut

import (
	"os"
	"path/filepath"
	"testing"
)

// installStub writes a shell stub at <stubDir>/<name> and prepends
// stubDir to PATH for the test's duration.
func installStub(t *testing.T, name, script string) string {
	t.Helper()
	stubDir := t.TempDir()
	stubBin := filepath.Join(stubDir, name)
	full := "#!/bin/sh\n" + script
	if err := os.WriteFile(stubBin, []byte(full), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", stubDir+":"+oldPath)
	return stubBin
}

// TestResolveNonShortcutReturnsEmpty: a query that does NOT match
// pr:N or mr:N returns ("", false, nil).
func TestResolveNonShortcutReturnsEmpty(t *testing.T) {
	for _, q := range []string{"main", "feature", "pr:abc", "mr:", "PR:1", ""} {
		t.Run(q, func(t *testing.T) {
			branch, ok, err := Resolve(q, "", "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok || branch != "" {
				t.Errorf("Resolve(%q) = %q, %v; want ('', false, nil)", q, branch, ok)
			}
		})
	}
}

// TestResolvePRViaStub: gh stub returns headRefName JSON; Resolve
// extracts and returns it.
func TestResolvePRViaStub(t *testing.T) {
	installStub(t, "gh", `
if [ "$1" = "pr" ] && [ "$2" = "view" ] && [ "$3" = "42" ]; then
    echo '{"headRefName":"user/cool-feature"}'
    exit 0
fi
exit 99
`)

	branch, ok, err := Resolve("pr:42", "", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ok {
		t.Fatal("expected shortcut to match")
	}
	if branch != "user/cool-feature" {
		t.Errorf("branch = %q; want user/cool-feature", branch)
	}
}

// TestResolveMRViaStub: glab stub for mr:N.
func TestResolveMRViaStub(t *testing.T) {
	installStub(t, "glab", `
if [ "$1" = "mr" ] && [ "$2" = "show" ] && [ "$3" = "101" ]; then
    echo '{"source_branch":"hotfix/auth"}'
    exit 0
fi
exit 99
`)

	branch, ok, err := Resolve("mr:101", "", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ok {
		t.Fatal("expected shortcut to match")
	}
	if branch != "hotfix/auth" {
		t.Errorf("branch = %q; want hotfix/auth", branch)
	}
}

// TestResolvePRMissingTool: pr:N when gh is absent → clear error.
func TestResolvePRMissingTool(t *testing.T) {
	// Shadow gh with a non-existent path by setting PATH to an empty dir.
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	if _, _, err := Resolve("pr:42", "", ""); err == nil {
		t.Error("expected error when gh is not on PATH")
	}
}

// TestResolveRejectsNonDigitN: pr:abc is treated as a non-shortcut
// (handled by TestResolveNonShortcutReturnsEmpty), but extra-large or
// shell-metachar inputs should never reach the subprocess.
func TestResolveRejectsInjection(t *testing.T) {
	installStub(t, "gh", `echo SHOULD_NOT_RUN`)
	for _, q := range []string{"pr:42;rm -rf /", "pr:42 && echo bad", "pr:42$(touch x)"} {
		_, ok, _ := Resolve(q, "", "")
		if ok {
			t.Errorf("Resolve(%q) was accepted as a shortcut", q)
		}
	}
}
