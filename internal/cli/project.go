package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Per-project configuration",
	}
	cmd.AddCommand(
		silenceSubcommand(&cobra.Command{
			Use:   "init",
			Short: "Create a .makework.toml in the current worktree",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				tmpl := `# main_branch = "main"
# tags = ["work"]

# [subprojects.example]
# subproject_path = "services/example"

# [subprojects.example.nix]
# type = "devenv"
# path = "services/example"
`
				if _, err := os.Stat(".makework.toml"); err == nil {
					return fmt.Errorf(".makework.toml already exists")
				}
				if err := os.WriteFile(".makework.toml", []byte(tmpl), 0o644); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Created .makework.toml")
				return nil
			},
		}),
		silenceSubcommand(&cobra.Command{
			Use:   "show [project]",
			Short: "Show project metadata resolved from catalog",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				_, cat, err := loadState()
				if err != nil {
					return err
				}
				if len(args) == 0 {
					return fmt.Errorf("please specify a project name")
				}
				name := args[0]
				resolved, err := cat.FindProjectUnambiguous(name)
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "name: %s\n", resolved.Repo.Name)
				fmt.Fprintf(out, "path: %s\n", resolved.Repo.Path)
				if resolved.Repo.URL != nil {
					fmt.Fprintf(out, "url: %s\n", *resolved.Repo.URL)
				}
				fmt.Fprintf(out, "main_branch: %s\n", resolved.Repo.MainBranch)
				for rname, remote := range resolved.Repo.Remotes {
					fmt.Fprintf(out, "remote.%s: %s\n", rname, remote.URL)
				}
				return nil
			},
		}),
	)
	return silenceSubcommand(cmd)
}
