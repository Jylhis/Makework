// Package hook runs the small set of per-worktree post-create commands
// declared in .makework.toml. By design this is intentionally minimal:
// no template engine, no lifecycle stages other than post-create, no
// pipelines. Commands execute one at a time via `sh -c`, with a short
// MW_* env overlay, in the worktree root.
//
// Security: post-create commands come from .makework.toml which is part
// of a repo's contents — running `mw go` against an untrusted repo
// therefore runs that repo's hooks. Each command is echoed to the
// supplied writer before it runs so the user sees what is about to
// execute. Higher-level gating (e.g. user opt-in, per-repo approvals)
// is the caller's responsibility.
package hook

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// RunPostCreate executes cmds in order with cwd set to wtPath. Each
// command runs via `sh -c`. The env overlay is added to the inherited
// process environment. Stops at the first command that exits non-zero
// and returns its error.
//
// out receives a one-line preamble per command ("$ <cmd>"); the
// command's own stdout/stderr is forwarded to it as well.
func RunPostCreate(wtPath string, cmds []string, env map[string]string, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	for _, cmd := range cmds {
		fmt.Fprintf(out, "$ %s\n", cmd)
		c := exec.Command("sh", "-c", cmd)
		c.Dir = wtPath
		c.Stdout = out
		c.Stderr = out
		c.Env = os.Environ()
		for k, v := range env {
			c.Env = append(c.Env, k+"="+v)
		}
		if err := c.Run(); err != nil {
			return fmt.Errorf("post-create command %q failed: %w", cmd, err)
		}
	}
	return nil
}
