package maintenance

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestRegisterAndStatus exercises Register against a bare repo and asserts
// Status flips from false → true. HOME/XDG_CONFIG_HOME are pointed at a
// scratch dir so the global git config the maintenance subcommand writes
// to does not bleed into the user's real configuration.
func TestRegisterAndStatus(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	bare := filepath.Join(home, "repo.git")
	if err := exec.Command("git", "init", "--bare", "-q", bare).Run(); err != nil {
		t.Skipf("git init unavailable: %v", err)
	}
	// Maintenance won't register a repo without at least one commit, so
	// push an empty commit through a working clone.
	wt := filepath.Join(home, "wt")
	if err := exec.Command("git", "clone", "-q", bare, wt).Run(); err != nil {
		t.Skipf("git clone unavailable: %v", err)
	}
	exec.Command("git", "-C", wt, "config", "user.email", "t@e").Run()
	exec.Command("git", "-C", wt, "config", "user.name", "t").Run()
	exec.Command("git", "-C", wt, "config", "commit.gpgsign", "false").Run()
	exec.Command("git", "-C", wt, "commit", "-q", "--allow-empty", "-m", "init").Run()
	exec.Command("git", "-C", wt, "push", "-q", "origin", "HEAD:refs/heads/main").Run()

	before, err := Status(bare)
	if err != nil {
		t.Fatalf("Status before: %v", err)
	}
	if before {
		t.Errorf("expected not-registered before Register")
	}

	if err := Register(bare); err != nil {
		t.Skipf("Register returned %v (git maintenance subcommand may be unavailable)", err)
	}

	after, err := Status(bare)
	if err != nil {
		t.Fatalf("Status after: %v", err)
	}
	if !after {
		t.Errorf("expected registered after Register")
	}

	if err := Unregister(bare); err != nil {
		t.Errorf("Unregister: %v", err)
	}
}
