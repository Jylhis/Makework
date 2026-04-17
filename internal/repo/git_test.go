package repo

import (
	"strings"
	"testing"
)

func TestGitErrorFormat(t *testing.T) {
	err := &GitError{Cmd: "git clone", Stderr: "fatal: error"}
	msg := err.Error()
	if !strings.Contains(msg, "git clone") || !strings.Contains(msg, "fatal: error") {
		t.Errorf("unexpected message: %q", msg)
	}
}
