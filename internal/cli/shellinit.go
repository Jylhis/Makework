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
				fmt.Fprint(out, zshHook)
			case "bash":
				fmt.Fprint(out, bashHook)
			default:
				return fmt.Errorf("unsupported shell: %s (supported: zsh, bash)", args[0])
			}
			return nil
		},
	})
}

// add-zsh-hook handles idempotency: re-sourcing .zshrc won't register the
// hook twice.
const zshHook = `# Add to your .zshrc:
_makework_hook() {
  command mw visit "$PWD" 2>/dev/null &!
}
autoload -Uz add-zsh-hook
add-zsh-hook chpwd _makework_hook
`

// Bash has no equivalent to add-zsh-hook; use a sentinel variable so
// re-sourcing .bashrc doesn't duplicate the PROMPT_COMMAND entry.
const bashHook = `# Add to your .bashrc:
_makework_hook() {
  command mw visit "$PWD" 2>/dev/null & disown
}
if [ -z "${_MAKEWORK_HOOK_INSTALLED:-}" ]; then
  _MAKEWORK_HOOK_INSTALLED=1
  PROMPT_COMMAND="_makework_hook${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
`
