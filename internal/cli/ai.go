package cli

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed skill.md
var skillContent string

func newAiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI assistant integration",
	}
	cmd.AddCommand(newAiInitCmd())
	return silenceSubcommand(cmd)
}

func newAiInitCmd() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "init",
		Short: "Output the Claude Code skill definition for makework",
		Long:  "Prints the SKILL.md content to stdout. Pipe to a file to install:\n\n  mkdir -p .claude/skills && mw ai init > .claude/skills/makework.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.OutOrStdout(), skillContent)
			return nil
		},
	})
}
