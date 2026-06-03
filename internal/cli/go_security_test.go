package cli

import "testing"

func TestEnsureSingleLineShellFieldRejectsLineBreaks(t *testing.T) {
	for _, value := range []string{
		"/tmp/worktree\ntouch /tmp/pwn",
		"/tmp/worktree\rtouch /tmp/pwn",
	} {
		if err := ensureSingleLineShellField("test field", value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestEnsureSingleLineShellFieldAllowsShellMetacharactersWithoutLineBreaks(t *testing.T) {
	value := "/tmp/worktree with spaces/$(not executed); # literal path chars"
	if err := ensureSingleLineShellField("test field", value); err != nil {
		t.Fatalf("expected single-line field to be accepted: %v", err)
	}
}
