package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMcpCmd() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:    "mcp",
		Short:  "MCP server (deprecated — use Claude Code skills instead)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "The MCP server has been replaced by Claude Code skills.")
			fmt.Fprintln(cmd.ErrOrStderr(), "See .claude/skills/makework.md for the skill definition.")
			return fmt.Errorf("mcp server removed; use Claude Code skills instead")
		},
	})
}
