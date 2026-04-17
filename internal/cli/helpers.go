package cli

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jylhis/makework/internal/catalog"
	"github.com/jylhis/makework/internal/config"
	"github.com/jylhis/makework/internal/repo"
	"github.com/jylhis/makework/internal/resolver"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/jylhis/makework/internal/xdgpath"
)

func loadConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		Die("loading config: %v", err)
	}
	return cfg
}

func loadCatalog() *catalog.Catalog {
	cat, err := catalog.Load()
	if err != nil {
		Die("loading catalog: %v", err)
	}
	return cat
}

func loadState() (*config.Config, *catalog.Catalog) {
	return loadConfig(), loadCatalog()
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

func editFile(path string) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		Die("editor exited with non-zero status")
	}
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
