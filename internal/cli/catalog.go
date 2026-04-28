package cli

import (
	"fmt"
	"os"

	"github.com/jylhis/makework/internal/catalog"
	"github.com/jylhis/makework/internal/repo"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage repositories",
	}
	cmd.AddCommand(
		newRepoAdd(),
		newRepoList(),
		newRepoRm(),
		newRepoEdit(),
		newRepoPurge(),
		newSyncCmd(),
	)
	return silenceSubcommand(cmd)
}

func newRepoAdd() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "add <source>",
		Short: "Register a repo from a URL or local path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat := loadState()
			source := args[0]
			var name string
			var isNew bool
			var err error

			if _, ok := repo.ParseRemoteURL(source); ok {
				name, isNew, err = cat.AddURL(source, cfg)
			} else {
				name, isNew, err = cat.Add(source, cfg)
			}
			if err != nil {
				return err
			}
			writeRepoRootsCache(cat)
			if isNew {
				fmt.Fprintf(cmd.OutOrStdout(), "Registered repository: %s\n", name)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Already registered: %s\n", name)
			}
			return nil
		},
	})
}

func newRepoList() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "list",
		Short: "List registered repos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cat := loadState()
			out := cmd.OutOrStdout()
			if len(cat.Repos) == 0 {
				fmt.Fprintln(out, "No repositories registered.")
				return nil
			}
			fmt.Fprintf(out, "%-20s %-40s %-10s WORKTREES\n", "NAME", "URL", "BRANCH")
			for name, r := range cat.Repos {
				url := "(local)"
				if r.URL != nil {
					url = *r.URL
				}
				wtCount := 0
				if wts, err := worktree.List(r.Path); err == nil {
					for _, wt := range wts {
						if !wt.IsBare {
							wtCount++
						}
					}
				}
				fmt.Fprintf(out, "%-20s %-40s %-10s %d\n", name, url, r.MainBranch, wtCount)
			}
			return nil
		},
	})
}

func newRepoRm() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "rm <project>",
		Short: "Unregister a repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat := loadState()
			name := args[0]
			if r, ok := cat.Repos[name]; ok {
				if wts, err := worktree.List(r.Path); err == nil {
					var active []worktree.Info
					for _, wt := range wts {
						if !wt.IsBare {
							active = append(active, wt)
						}
					}
					if len(active) > 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s has %d active worktree(s):\n", name, len(active))
						for _, wt := range active {
							branch := wt.Branch
							if branch == "" {
								branch = "(detached)"
							}
							fmt.Fprintf(cmd.ErrOrStderr(), "  %s %s\n", branch, wt.Path)
						}
						fmt.Fprintln(cmd.ErrOrStderr(), "These worktrees will become orphaned.")
					}
				}
			}
			if err := cat.Remove(name); err != nil {
				return err
			}
			_ = cfg // used for save inside Remove
			writeRepoRootsCache(cat)
			fmt.Fprintf(cmd.OutOrStdout(), "Removed catalog entry: %s\n", name)
			return nil
		},
	})
}

func newRepoEdit() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "edit",
		Short: "Open the catalog file in $EDITOR",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := catalog.CatalogPath()
			if err != nil {
				return err
			}
			editFile(path)
			return nil
		},
	})
}

func newRepoPurge() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "purge <project>",
		Short: "Remove all worktrees and unregister a repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat := loadState()
			name := args[0]
			r, ok := cat.Repos[name]
			if !ok {
				return fmt.Errorf("repository not found: %s", name)
			}

			// Remove worktree directories
			wtParent := worktree.ParentDir(cfg.WorktreeRoot, cfg.BareRoot, r.Path)
			if wtParent != "" {
				if _, err := os.Stat(wtParent); err == nil {
					if err := os.RemoveAll(wtParent); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not remove worktrees at %s: %v\n", wtParent, err)
					}
				}
			}

			// Remove bare clone
			if _, err := os.Stat(r.Path); err == nil {
				if err := os.RemoveAll(r.Path); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not remove bare clone: %v\n", err)
				}
			}

			delete(cat.Repos, name)
			if err := cat.Save(); err != nil {
				return err
			}
			writeRepoRootsCache(cat)
			fmt.Fprintf(cmd.OutOrStdout(), "Purged: %s\n", name)
			return nil
		},
	})
}

func fileExistsCli(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
