package cli

import (
	"fmt"
	"os"
)

// Die prints "Error: <msg>" to stderr and exits with status 1.
// Mirrors the Rust `die()` helper used across the CLI.
func Die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

// errNotImplemented is the placeholder returned by every stub RunE until
// the corresponding command is ported in Phase 4.
type errNotImplemented struct{ cmd string }

func (e errNotImplemented) Error() string { return "not implemented: " + e.cmd }

func notImplemented(cmd string) error { return errNotImplemented{cmd: cmd} }
