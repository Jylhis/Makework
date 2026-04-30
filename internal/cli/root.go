// Package cli wires the makework CLI tree using Cobra.
package cli

import (
	"fmt"
	"os"

	"github.com/jylhis/makework/internal/buildinfo"
	"github.com/spf13/cobra"
)

// newRoot constructs the root `mw` command and attaches every subcommand.
func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "mw",
		Short:         "makework — git worktree manager",
		Version:       buildinfo.Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
	}

	root.AddCommand(
		newGoCmd(),
		newSwitchCmd(),
		newRmCmd(),
		newLsCmd(),
		newPruneCmd(),
		newFetchCmd(),
		newRepoCmd(),
		newProjectCmd(),
		newMaintenanceCmd(),
		newConfigCmd(),
		newSearchCmd(),
		newQueryCmd(),
		newMcpCmd(),
		newResolverCmd(),
		newAiCmd(),
		newInitCmd(),
		newVisitCmd(),
		newManCmd(),
		newGenerateTexiCmd(),
	)
	return root
}

// Main runs the root command and returns the exit code.
// Exposed for testscript integration.
func Main() int {
	if err := newRoot().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return 1
	}
	return 0
}

// Execute runs the root command with stdin/stdout/stderr and exits with the
// appropriate status. Kept compact so `cmd/mw/main.go` stays trivial.
func Execute() {
	os.Exit(Main())
}

// silenceSubcommand marks a command as quiet on errors so our own `Die`
// output is the only thing users see. Apply to every leaf `*cobra.Command`.
func silenceSubcommand(c *cobra.Command) *cobra.Command {
	c.SilenceErrors = true
	c.SilenceUsage = true
	return c
}
