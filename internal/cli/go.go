package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jylhis/makework/internal/catalog"
	"github.com/jylhis/makework/internal/config"
	"github.com/jylhis/makework/internal/nix"
	"github.com/jylhis/makework/internal/resolver"
	"github.com/jylhis/makework/internal/template"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

func newGoCmd() *cobra.Command {
	var list bool
	cmd := &cobra.Command{
		Use:   "go [project] [ref]",
		Short: "Navigate to a project worktree (supports fuzzy matching)",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat := loadState()
			out := cmd.OutOrStdout()
			errOut := cmd.ErrOrStderr()

			if len(args) == 0 {
				names := cat.AllProjectNames()
				if len(names) == 0 {
					return fmt.Errorf("no projects registered. Run 'mw repo sync' or 'mw repo add' first")
				}
				fmt.Fprintln(errOut, "Available projects:")
				for _, n := range names {
					fmt.Fprintf(errOut, "  %s\n", n)
				}
				fmt.Fprintln(errOut, "\nUsage: mw go <project>")
				return fmt.Errorf("no project specified")
			}

			query := args[0]
			var refOverride string
			if len(args) > 1 {
				refOverride = args[1]
			}

			parsed, err := resolver.ParseQuery(query)
			if err != nil {
				return err
			}

			if parsed.IsExplicit() {
				resolved, err := cat.FindProjectUnambiguous(parsed.Repo)
				if err != nil {
					return err
				}
				return navigateToWorktree(cfg, resolved, parsed.Branch, out)
			}

			// Fast path: exact catalog match
			if !list {
				if resolved, err := cat.FindProjectUnambiguous(query); err == nil {
					ref := refOverride
					if ref == "" {
						ref = resolved.Repo.MainBranch
					}
					return navigateToWorktree(cfg, resolved, ref, out)
				}
			}

			// Fuzzy resolve
			resolverCfg := config.DefaultResolver()
			if cfg.Resolver != nil {
				resolverCfg = *cfg.Resolver
			}
			index := &resolver.Index{
				Targets: resolver.BuildTargets(cat),
				Visits:  loadVisits(),
			}
			ctx := resolver.DefaultContext()
			results, err := resolver.Resolve(query, index, &resolverCfg, &ctx)
			if err != nil {
				return err
			}

			if list {
				for i, t := range results {
					if i >= 10 {
						break
					}
					branch := t.Branch
					if branch == "" {
						branch = "-"
					}
					fmt.Fprintf(errOut, "  %d. %-20s %-15s %.3f  %s\n",
						i+1, t.RepoName, branch, t.Score, t.ProjectName)
				}
				return nil
			}

			if resolver.NeedsDisambiguation(results, 0.10) && isTerminal() {
				fmt.Fprintf(errOut, "Multiple matches for '%s':\n", query)
				for i, t := range results {
					if i >= 5 {
						break
					}
					branch := t.Branch
					if branch == "" {
						branch = "-"
					}
					fmt.Fprintf(errOut, "  %d. %s (%s)\n", i+1, t.RepoName, branch)
				}
				fmt.Fprintln(errOut, "Use repo@branch syntax for precise routing.")
				return fmt.Errorf("ambiguous match")
			}

			top := results[0]
			branch := refOverride
			if branch == "" {
				branch = top.Branch
				if branch == "" {
					branch = "main"
				}
			}
			suggestName := top.RepoName
			if top.ProjectName != "" {
				suggestName = top.ProjectName
			}
			fmt.Fprintf(errOut,
				"Refusing to auto-navigate on fuzzy match %q (top match: %s@%s, score %.3f).\n",
				query, suggestName, branch, top.Score,
			)
			fmt.Fprintf(errOut, "Run mw go %s to confirm.\n", shellQuote(suggestName+"@"+branch))
			return fmt.Errorf("confirmation required for fuzzy match")
		},
	}
	cmd.Flags().BoolVar(&list, "list", false, "Show all matches with scores instead of navigating")
	return silenceSubcommand(cmd)
}

func navigateToWorktree(cfg *config.Config, resolved *catalog.ResolvedProject, ref string, out io.Writer) error {
	wtPath := resolvedWorktreePath(cfg, resolved, ref)

	if !fileExistsCli(wtPath) {
		if err := worktree.Create(resolved.Repo.Path, ref, wtPath); err != nil {
			return err
		}
	}

	if len(resolved.SparsePaths) > 0 {
		_ = worktree.EnableSparseCheckout(wtPath, resolved.SparsePaths)
	}

	if cfg.TemplateDir != nil {
		_, _ = template.Apply(*cfg.TemplateDir, wtPath)
	}

	finalPath := wtPath
	if resolved.SubprojectPath != "" {
		finalPath = wtPath + "/" + resolved.SubprojectPath
	}

	nixResult := nix.Detect(wtPath, resolved.NixConfig)

	fmt.Fprintln(out, finalPath)
	if nixResult != nil {
		fmt.Fprintln(out, nixResult.ActivationCommand)
		if resolved.SubprojectPath != "" {
			fmt.Fprintln(out, wtPath)
		}
	}

	recordVisit(resolved.Repo.Name, ref)
	return nil
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// shellQuote returns s wrapped in single quotes with embedded single quotes
// escaped using the standard '\” POSIX trick. The result is safe to paste
// into a POSIX shell as a single argument.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
