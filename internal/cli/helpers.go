package cli

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jylhis/makework/internal/catalog"
	"github.com/jylhis/makework/internal/config"
	"github.com/jylhis/makework/internal/repo"
	"github.com/jylhis/makework/internal/resolver"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/jylhis/makework/internal/xdgpath"
)

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

func loadCatalog() (*catalog.Catalog, error) {
	cat, err := catalog.Load()
	if err != nil {
		return nil, fmt.Errorf("loading catalog: %w", err)
	}
	return cat, nil
}

// loadState loads the global config and catalog. Returns the error
// from either to the caller (Cobra surfaces it via RunE).
func loadState() (*config.Config, *catalog.Catalog, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, err
	}
	cat, err := loadCatalog()
	if err != nil {
		return nil, nil, err
	}
	return cfg, cat, nil
}

func visitsPath() string {
	state, err := xdgpath.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(state, "visits.json")
}

func loadVisits() resolver.VisitsDB {
	path := visitsPath()
	if path == "" {
		return resolver.NewVisitsDB()
	}
	db, err := resolver.LoadVisits(path)
	if err != nil {
		return resolver.NewVisitsDB()
	}
	return db
}

func recordVisit(repoName, branch string) {
	path := visitsPath()
	if path == "" {
		return
	}
	lockPath := path + ".lock"
	lock, err := resolver.AcquireLock(lockPath)
	if err != nil {
		slog.Warn("failed to acquire visits lock, proceeding without lock", "path", lockPath, "error", err)
	} else {
		defer func() {
			if err := lock.Close(); err != nil {
				slog.Warn("failed to release visits lock", "path", lockPath, "error", err)
			}
		}()
	}
	db, err := resolver.LoadVisits(path)
	if err != nil {
		slog.Warn("failed to load visits database", "path", path, "error", err)
		db = resolver.NewVisitsDB()
	}
	ctx := resolver.DefaultContext()
	db.RecordVisit(fmt.Sprintf("%s:%s", repoName, branch), ctx.Now)
	if err := db.Save(path); err != nil {
		slog.Warn("failed to save visits database", "path", path, "error", err)
	}
}

func writeRepoRootsCache(cat *catalog.Catalog) {
	state, err := xdgpath.StateDir()
	if err != nil {
		return
	}
	if err := os.MkdirAll(state, 0o755); err != nil {
		slog.Warn("failed to create state directory", "path", state, "error", err)
		return
	}
	var roots []string
	for _, r := range cat.Repos {
		roots = append(roots, r.Path)
	}
	path := filepath.Join(state, "repo-roots.txt")
	tmp := path + ".tmp"
	data := ""
	for _, r := range roots {
		data += r + "\n"
	}
	if err := os.WriteFile(tmp, []byte(data), 0o644); err != nil {
		slog.Warn("failed to write repo roots cache", "path", tmp, "error", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		slog.Warn("failed to rename repo roots cache", "from", tmp, "to", path, "error", err)
	}
}

func editFile(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with non-zero status: %w", err)
	}
	return nil
}

// currentRepo resolves the catalog repository that owns the current
// working directory by asking git for the common git-dir and matching
// it against cat.Repos. Returns the resolved project, the current
// branch (or "" if detached/empty), and an error.
//
// Used by `mw wt` subcommands which are scoped to the current repo
// (worktrunk-style), in contrast with the catalog-wide
// `mw switch / rm / ls`.
func currentRepo(cat *catalog.Catalog) (*catalog.ResolvedProject, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("getting working directory: %w", err)
	}
	topLevel, err := repo.RunGitCapture("-C", cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, "", fmt.Errorf("not inside a git worktree: %w", err)
	}
	topLevelClean := canonicalPath(topLevel)
	common, err := repo.RunGitCapture("-C", cwd, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, "", fmt.Errorf("not inside a git worktree: %w", err)
	}
	commonAbs := common
	if !filepath.IsAbs(commonAbs) {
		commonAbs = filepath.Join(cwd, common)
	}
	commonClean := canonicalPath(commonAbs)

	for name, r := range cat.Repos {
		barePath := canonicalPath(r.Path)
		if barePath == commonClean || barePath == filepath.Clean(commonAbs) {
			if !isRegisteredWorktree(r.Path, topLevelClean) {
				return nil, "", fmt.Errorf("current directory is not a registered worktree for catalog repo %q", r.Name)
			}
			resolved := &catalog.ResolvedProject{Repo: r}
			branch, _ := repo.RunGitCapture("-C", cwd, "symbolic-ref", "--short", "HEAD")
			return resolved, branch, nil
		}
		// Some catalogs record the repo path; the git common-dir may
		// point at the same directory's `.git` for a non-bare repo.
		if barePath == filepath.Dir(commonClean) {
			if !isRegisteredWorktree(r.Path, topLevelClean) {
				return nil, "", fmt.Errorf("current directory is not a registered worktree for catalog repo %q", r.Name)
			}
			resolved := &catalog.ResolvedProject{Repo: r}
			branch, _ := repo.RunGitCapture("-C", cwd, "symbolic-ref", "--short", "HEAD")
			return resolved, branch, nil
		}
		_ = name
	}
	return nil, "", catalog.ErrRepoNotFound{Name: commonClean}
}

func isRegisteredWorktree(repoPath, topLevel string) bool {
	out, err := repo.RunGitCapture("-C", repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		wt := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		if canonicalPath(wt) == topLevel {
			return true
		}
	}
	return false
}

func canonicalPath(p string) string {
	clean := filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		return filepath.Clean(resolved)
	}
	return clean
}

func resolvedWorktreePath(cfg *config.Config, resolved *catalog.ResolvedProject, ref string) string {
	var parsedURL *repo.ParsedURL
	if resolved.Repo.URL != nil {
		if p, ok := repo.ParseRemoteURL(*resolved.Repo.URL); ok {
			parsedURL = &p
		}
	}
	return worktree.Path(cfg.WorktreeRoot, parsedURL, resolved.Repo.Name, ref)
}
