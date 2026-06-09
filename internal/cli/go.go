package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/jylhis/makework/internal/catalog"
	"github.com/jylhis/makework/internal/config"
	"github.com/jylhis/makework/internal/fsx"
	"github.com/jylhis/makework/internal/hook"
	"github.com/jylhis/makework/internal/nix"
	"github.com/jylhis/makework/internal/picker"
	"github.com/jylhis/makework/internal/project"
	"github.com/jylhis/makework/internal/refshortcut"
	"github.com/jylhis/makework/internal/repo"
	"github.com/jylhis/makework/internal/resolver"
	"github.com/jylhis/makework/internal/template"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

func newGoCmd() *cobra.Command {
	var list bool
	var allowHooks bool
	cmd := &cobra.Command{
		Use:   "go [project] [ref]",
		Short: "Navigate to a project worktree (supports fuzzy matching)",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat, err := loadState()
			if err != nil {
				return err
			}
			hooksEnabled := allowHooks || cfg.AllowHooks
			out := cmd.OutOrStdout()
			errOut := cmd.ErrOrStderr()

			if len(args) == 0 {
				names := cat.AllProjectNames()
				if len(names) == 0 {
					return fmt.Errorf("no projects registered. Run 'mw repo sync' or 'mw repo add' first")
				}
				if !isTerminal() {
					fmt.Fprintln(errOut, "Available projects:")
					for _, n := range names {
						fmt.Fprintf(errOut, "  %s\n", n)
					}
					fmt.Fprintln(errOut, "\nUsage: mw go <project>")
					return fmt.Errorf("no project specified")
				}
				items := make([]picker.Item, len(names))
				for i, n := range names {
					items[i] = picker.Item{Label: n}
				}
				sel, err := picker.Pick(items, "Pick a project:", os.Stdin, errOut)
				if err != nil {
					return err
				}
				resolved, err := cat.FindProjectUnambiguous(sel.Label)
				if err != nil {
					return err
				}
				return navigateToWorktree(cfg, resolved, resolved.Repo.MainBranch, hooksEnabled, out)
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
				return navigateToWorktree(cfg, resolved, parsed.Branch, hooksEnabled, out)
			}

			// Fast path: exact catalog match
			if !list {
				if resolved, err := cat.FindProjectUnambiguous(query); err == nil {
					ref := refOverride
					if ref == "" {
						ref = resolved.Repo.MainBranch
					}
					return navigateToWorktree(cfg, resolved, ref, hooksEnabled, out)
				}
			}

			// Fuzzy resolve
			resolverCfg := config.DefaultResolver()
			if cfg.Resolver != nil {
				resolverCfg = *cfg.Resolver
			}
			index := &resolver.Index{
				Targets: resolver.BuildTargets(cat, &resolverCfg),
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
				topN := results
				if len(topN) > 5 {
					topN = topN[:5]
				}
				items := make([]picker.Item, len(topN))
				for i, t := range topN {
					branch := t.Branch
					if branch == "" {
						branch = "-"
					}
					items[i] = picker.Item{
						Label: fmt.Sprintf("%s@%s", t.RepoName, branch),
						Sub:   fmt.Sprintf("score=%.3f", t.Score),
						Value: t,
					}
				}
				sel, err := picker.Pick(items, fmt.Sprintf("Multiple matches for '%s':", query), os.Stdin, errOut)
				if err != nil {
					return err
				}
				chosen := sel.Value.(resolver.Target)
				name := chosen.RepoName
				if chosen.ProjectName != "" {
					name = chosen.ProjectName
				}
				resolved, err := cat.FindProjectUnambiguous(name)
				if err != nil {
					return err
				}
				ref := refOverride
				if ref == "" {
					ref = chosen.Branch
					if ref == "" {
						ref = "main"
					}
				}
				return navigateToWorktree(cfg, resolved, ref, hooksEnabled, out)
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
	cmd.Flags().BoolVar(&allowHooks, "allow-hooks", false, "Run post-create hooks from .makework.toml (off by default; equivalent to config allow_hooks=true)")
	return silenceSubcommand(cmd)
}

func navigateToWorktree(cfg *config.Config, resolved *catalog.ResolvedProject, ref string, hooksEnabled bool, out io.Writer) error {
	resolvedRef, err := resolveBranchShortcut(ref, resolved)
	if err != nil {
		return err
	}
	ref = resolvedRef
	wtPath := resolvedWorktreePath(cfg, resolved, ref)
	if err := ensureSingleLineShellField("worktree path", wtPath); err != nil {
		return err
	}
	finalPath := wtPath
	if resolved.SubprojectPath != "" {
		finalPath = wtPath + "/" + resolved.SubprojectPath
	}
	if err := ensureSingleLineShellField("navigation path", finalPath); err != nil {
		return err
	}

	newlyCreated := false
	if !fsx.PathExists(wtPath) {
		if err := worktree.Create(resolved.Repo.Path, ref, wtPath); err != nil {
			return err
		}
		newlyCreated = true
	}

	if len(resolved.SparsePaths) > 0 {
		_ = worktree.EnableSparseCheckout(wtPath, resolved.SparsePaths)
	}

	if cfg.TemplateDir != nil {
		_, _ = template.Apply(*cfg.TemplateDir, wtPath)
	}

	if newlyCreated {
		runPostCreateHooks(wtPath, resolved.Repo.Name, ref, hooksEnabled, os.Stderr)
	}

	nixResult := nix.Detect(wtPath, resolved.NixConfig)
	if nixResult != nil {
		if err := ensureSingleLineShellField("activation command", nixResult.ActivationCommand); err != nil {
			return err
		}
	}

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

func ensureSingleLineShellField(name, value string) error {
	if strings.ContainsFunc(value, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
	}) {
		return fmt.Errorf("%s contains a control character and cannot be emitted to shell integration", name)
	}
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

// resolveBranchShortcut returns the resolved branch when ref is a
// pr:N or mr:N shortcut, otherwise returns ref unchanged.
func resolveBranchShortcut(ref string, resolved *catalog.ResolvedProject) (string, error) {
	slug := ""
	if resolved.Repo.URL != nil {
		if p, ok := repo.ParseRemoteURL(*resolved.Repo.URL); ok {
			slug = p.Host + "/" + strings.Join(p.Segments, "/")
		}
	}
	branch, ok, err := refshortcut.Resolve(ref, resolved.Repo.Path, slug)
	if err != nil {
		return "", err
	}
	if ok {
		return branch, nil
	}
	return ref, nil
}

// runPostCreateHooks reads .makework.toml from wtPath (if present) and
// runs the configured post-create commands. Errors are logged to out
// but do not fail the worktree creation, since the worktree itself is
// already usable.
//
// Hooks come from repo contents and are therefore untrusted by default.
// When enabled is false and the file declares any post-create commands,
// the commands are skipped with a one-line warning telling the user
// how to opt in.
func runPostCreateHooks(wtPath, repoName, branch string, enabled bool, out io.Writer) {
	data, err := os.ReadFile(filepath.Join(wtPath, ".makework.toml"))
	if err != nil {
		return
	}
	var p project.Project
	if err := toml.Unmarshal(data, &p); err != nil {
		fmt.Fprintf(out, "warning: .makework.toml parse error: %v\n", err)
		return
	}
	if len(p.Hooks.PostCreate) == 0 {
		return
	}
	if !enabled {
		fmt.Fprintf(out,
			"Skipping %d post-create hook(s) in %s (untrusted by default).\n"+
				"To run, re-invoke with --allow-hooks or set 'allow_hooks = true' in config.\n",
			len(p.Hooks.PostCreate), filepath.Join(wtPath, ".makework.toml"),
		)
		return
	}
	env := map[string]string{
		"MW_WORKTREE_PATH": wtPath,
		"MW_BRANCH":        branch,
		"MW_REPO":          repoName,
	}
	if err := hook.RunPostCreate(wtPath, p.Hooks.PostCreate, env, out); err != nil {
		fmt.Fprintf(out, "post-create hook error: %v\n", err)
	}
}
