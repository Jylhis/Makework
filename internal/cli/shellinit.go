package cli

import (
	"fmt"

	"github.com/jylhis/makework/internal/catalog"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:       "init [shell]",
		Short:     "Initialize makework or generate shell integration",
		Long:      "Without arguments: create config, catalog, and state directories.\nWith a shell argument: output shell completions, the go wrapper, and visit-tracking hook.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"zsh", "bash", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runCatalogInit(cmd)
			}
			return runShellInit(cmd, args[0])
		},
	})
}

func runCatalogInit(cmd *cobra.Command) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	result, err := catalog.Init(cfg)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(result.Created) == 0 {
		fmt.Fprintln(out, "Already initialized. Nothing to do.")
	} else {
		fmt.Fprintln(out, "Initialized makework:")
		for _, p := range result.Created {
			fmt.Fprintf(out, "  created %s\n", p)
		}
	}
	for _, p := range result.AlreadyExisted {
		fmt.Fprintf(out, "  exists  %s\n", p)
	}
	return nil
}

func runShellInit(cmd *cobra.Command, shell string) error {
	out := cmd.OutOrStdout()
	root := cmd.Root()

	// Output completions and the navigation wrapper.
	if err := generateShellIntegrationStdout(root, shell, out); err != nil {
		return fmt.Errorf("generating shell integration for %s: %w", shell, err)
	}

	// Output visit-tracking hook
	switch shell {
	case "zsh":
		fmt.Fprint(out, zshHook)
	case "bash":
		fmt.Fprint(out, bashHook)
	case "fish", "powershell":
		// Visit hooks not yet implemented for these shells;
		// completions are still output above.
	default:
		return fmt.Errorf("unsupported shell: %s (supported: zsh, bash, fish, powershell)", shell)
	}
	return nil
}

// add-zsh-hook handles idempotency: re-sourcing .zshrc won't register the
// hook twice.
const zshHook = `
# makework visit tracking
_makework_hook() {
  command mw visit "$PWD" 2>/dev/null &!
}
autoload -Uz add-zsh-hook
add-zsh-hook chpwd _makework_hook
`

// Bash has no equivalent to add-zsh-hook; use a sentinel variable so
// re-sourcing .bashrc doesn't duplicate the PROMPT_COMMAND entry.
const bashHook = `
# makework visit tracking
_makework_hook() {
  command mw visit "$PWD" 2>/dev/null & disown "$!" 2>/dev/null || true
}
if [ -z "${_MAKEWORK_HOOK_INSTALLED:-}" ]; then
  _MAKEWORK_HOOK_INSTALLED=1
  PROMPT_COMMAND="_makework_hook${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
`
