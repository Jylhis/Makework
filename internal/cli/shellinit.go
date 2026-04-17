package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:       "init <shell>",
		Short:     "Generate shell hook for visit tracking",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"zsh", "bash"},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "zsh":
				fmt.Fprint(out, `# Add to your .zshrc:
_makework_hook() {
  command mw visit "$PWD" 2>/dev/null &!
}
chpwd_functions+=(_makework_hook)
`)
			case "bash":
				fmt.Fprint(out, `# Add to your .bashrc:
_makework_hook() {
  command mw visit "$PWD" 2>/dev/null &
}
PROMPT_COMMAND="_makework_hook;${PROMPT_COMMAND}"
`)
			default:
				return fmt.Errorf("unsupported shell: %s (supported: zsh, bash)", args[0])
			}
			return nil
		},
	})
}
