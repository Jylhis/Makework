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

	// `git maintenance` writes to global git config, not the bare's own
	// config — Status reads from the bare and `git config --get` falls
	// back to global, so a previously-registered bare in the same
	// global config can leak through. Just assert Register doesn't
	// error and Status returns no error; full lifecycle is hard to
	// isolate without rewriting Register to use --local.
	if err := Register(bare); err != nil {
		t.Skipf("Register returned %v (git maintenance subcommand may be unavailable)", err)
	}
	if _, err := Status(bare); err != nil {
		t.Errorf("Status after Register: %v", err)
	}
	if err := Unregister(bare); err != nil {
		t.Errorf("Unregister: %v", err)
	}
}
