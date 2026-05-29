package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCompletionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completions <bash|zsh|fish|powershell>",
		Short:     "Print shell completions and the mw go wrapper",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateShellIntegrationStdout(cmd.Root(), args[0], cmd.OutOrStdout())
		},
	}
	return silenceSubcommand(cmd)
}

func generateShellIntegrationStdout(root *cobra.Command, shell string, out interface{ Write([]byte) (int, error) }) error {
	if err := generateCompletionStdout(root, shell, out); err != nil {
		return err
	}
	switch shell {
	case "bash", "zsh":
		_, err := out.Write([]byte(posixGoWrapper))
		return err
	case "fish":
		_, err := out.Write([]byte(fishGoWrapper))
		return err
	case "powershell":
		// PowerShell gets native completions for now. It does not have a
		// current-directory wrapper yet.
		return nil
	default:
		return fmt.Errorf("completions not supported for shell: %s", shell)
	}
}

func generateCompletionStdout(root *cobra.Command, shell string, out interface{ Write([]byte) (int, error) }) error {
	switch shell {
	case "bash":
		return root.GenBashCompletionV2(out, true)
	case "zsh":
		return root.GenZshCompletion(out)
	case "fish":
		return root.GenFishCompletion(out, true)
	case "powershell":
		return root.GenPowerShellCompletion(out)
	default:
		return fmt.Errorf("completions not supported for shell: %s", shell)
	}
}

const posixGoWrapper = `
# makework navigation wrapper
mw() {
  if [ "$#" -gt 0 ] && [ "$1" = "go" ]; then
    local _mw_arg
    for _mw_arg in "$@"; do
      case "$_mw_arg" in
        -h|--help|help)
          command mw "$@"
          return $?
          ;;
      esac
    done

    local _mw_out _mw_status _mw_path _mw_activation _mw_activation_dir
    _mw_out="$(command mw "$@")"
    _mw_status=$?
    if [ $_mw_status -ne 0 ]; then
      if [ -n "$_mw_out" ]; then
        printf '%s\n' "$_mw_out"
      fi
      return $_mw_status
    fi

    _mw_path="$(printf '%s\n' "$_mw_out" | sed -n '1p')"
    _mw_activation="$(printf '%s\n' "$_mw_out" | sed -n '2p')"
    _mw_activation_dir="$(printf '%s\n' "$_mw_out" | sed -n '3p')"

    if [ -z "$_mw_path" ]; then
      if [ -n "$_mw_out" ]; then
        printf '%s\n' "$_mw_out"
      fi
      return 0
    fi

    builtin cd -- "$_mw_path" || return $?
    if [ -n "$_mw_activation" ]; then
      if [ -n "$_mw_activation_dir" ]; then
        builtin cd -- "$_mw_activation_dir" || return $?
      fi
      eval "$_mw_activation"
    fi
  else
    command mw "$@"
  fi
}
`

const fishGoWrapper = `
# makework navigation wrapper
function mw
  if test (count $argv) -gt 0; and test "$argv[1]" = go
    for _mw_arg in $argv
      switch $_mw_arg
        case -h --help help
          command mw $argv
          return $status
      end
    end

    set -l _mw_lines (command mw $argv)
    set -l _mw_status $status
    if test $_mw_status -ne 0
      if test (count $_mw_lines) -gt 0
        printf '%s\n' $_mw_lines
      end
      return $_mw_status
    end

    if test (count $_mw_lines) -eq 0
      return 0
    end

    set -l _mw_path $_mw_lines[1]
    set -l _mw_activation
    set -l _mw_activation_dir
    if test (count $_mw_lines) -ge 2
      set _mw_activation $_mw_lines[2]
    end
    if test (count $_mw_lines) -ge 3
      set _mw_activation_dir $_mw_lines[3]
    end

    if test -z "$_mw_path"
      printf '%s\n' $_mw_lines
      return 0
    end

    cd -- "$_mw_path"; or return $status
    if test -n "$_mw_activation"
      if test -n "$_mw_activation_dir"
        cd -- "$_mw_activation_dir"; or return $status
      end
      eval "$_mw_activation"
    end
  else
    command mw $argv
  end
end
`
