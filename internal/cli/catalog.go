package cli

import "github.com/spf13/cobra"

func newCatalogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Manage the repository catalog",
	}
	cmd.AddCommand(
		newCatalogInit(),
		newCatalogAdd(),
		newCatalogList(),
		newCatalogRemove(),
		newCatalogEdit(),
		newCatalogPurge(),
	)
	return silenceSubcommand(cmd)
}

func newCatalogInit() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "init",
		Short: "Create config + state directories and empty catalog",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("catalog init")
		},
	})
}

func newCatalogAdd() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "add <source>",
		Short: "Register a repo from a URL or local path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("catalog add")
		},
	})
}

func newCatalogList() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "list",
		Short: "List registered repos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("catalog list")
		},
	})
}

func newCatalogRemove() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "remove <project>",
		Short: "Unregister a repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("catalog remove")
		},
	})
}

func newCatalogEdit() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "edit",
		Short: "Open the catalog file in $EDITOR",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("catalog edit")
		},
	})
}

func newCatalogPurge() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "purge <project>",
		Short: "Remove all worktrees and unregister a repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("catalog purge")
		},
	})
}
