package cli

import "github.com/spf13/cobra"

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "init <shell>",
		Short:     "Generate shell hook for visit tracking",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"zsh", "bash"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("init")
		},
	}
	return silenceSubcommand(cmd)
}
