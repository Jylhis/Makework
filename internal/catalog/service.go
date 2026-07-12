package catalog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jylhis/makework/internal/config"
	"github.com/jylhis/makework/internal/fsx"
	"github.com/jylhis/makework/internal/maintenance"
	"github.com/jylhis/makework/internal/project"
	"github.com/jylhis/makework/internal/repo"
	"github.com/jylhis/makework/internal/terminal"
	"github.com/jylhis/makework/internal/worktree"
)

// --- Sync ---

// Sync discovers repos under scanRoots and registers them.
func (c *Catalog) Sync(cfg *config.Config, scanRoots []string, opts SyncOptions) ([]string, error) {
	var added []string
	progress := &syncProgress{out: opts.Progress}
	for _, root := range scanRoots {
		if !fsx.PathExists(root) {
			continue
		}
		progress.root(root)
		repos, err := walkForRepos(root, 0, opts, progress)
		if err != nil {
			return added, err
		}
		progress.discovered(root, len(repos))
		for _, rp := range repos {
			progress.registering(rp)
			name, isNew, err := c.Add(rp, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping %s: %s\n", terminal.SanitizeText(rp), terminal.SanitizeText(err.Error()))
				continue
			}
			if isNew {
				added = append(added, name)
			}
		}
	}
	return added, nil
}

// --- Add / AddURL / register ---

// addRequest is the input to register: enough to clone a bare and
// create the initial worktree without committing yet to a particular
// source flavour (local path vs URL).
type addRequest struct {
	repoName    string
	bareSource  string // path or URL passed to git clone --bare
	localFB     string // optional: fall back to cloning this path if bareSource fails
	parsedURL   *repo.ParsedURL
	originURL   string // non-empty → stored as origin remote
	warnOnLocal string // optional context for the local-fallback warning
}

// Add registers a local git repo into the catalog.
func (c *Catalog) Add(sourcePath string, cfg *config.Config) (string, bool, error) {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", false, err
	}
	sourcePath = abs

	if !isGitRepo(sourcePath) {
		return "", false, ErrNotGitRepo{Path: sourcePath}
	}

	remoteURL := getOriginURL(sourcePath)
	parsed, hasParsed := repo.ParseRemoteURL(remoteURL)

	repoName := filepath.Base(sourcePath)
	if hasParsed && len(parsed.Segments) > 0 {
		repoName = parsed.Segments[len(parsed.Segments)-1]
	}

	var parsedPtr *repo.ParsedURL
	if hasParsed {
		parsedPtr = &parsed
	}
	req := addRequest{
		repoName:    repoName,
		bareSource:  remoteURL,
		localFB:     sourcePath,
		parsedURL:   parsedPtr,
		originURL:   remoteURL,
		warnOnLocal: sourcePath,
	}
	if remoteURL == "" {
		req.bareSource = sourcePath
		req.localFB = ""
	}
	return c.register(req, cfg)
}

// AddURL registers a repo by remote URL.
func (c *Catalog) AddURL(url string, cfg *config.Config) (string, bool, error) {
	parsed, ok := repo.ParseRemoteURL(url)
	if !ok {
		return "", false, fmt.Errorf("invalid git URL: %s", url)
	}
	if len(parsed.Segments) == 0 {
		return "", false, fmt.Errorf("cannot derive repo name from: %s", url)
	}
	repoName := parsed.Segments[len(parsed.Segments)-1]
	return c.register(addRequest{
		repoName:   repoName,
		bareSource: url,
		parsedURL:  &parsed,
		originURL:  url,
	}, cfg)
}

// register is the shared core of Add and AddURL: clone a bare, fetch,
// resolve the default branch, create the initial worktree, register
// maintenance, and persist the catalog entry.
func (c *Catalog) register(req addRequest, cfg *config.Config) (string, bool, error) {
	if _, exists := c.Repos[req.repoName]; exists {
		return req.repoName, false, nil
	}

	hasParsed := req.parsedURL != nil
	var parsedVal repo.ParsedURL
	if hasParsed {
		parsedVal = *req.parsedURL
	}

	bareDest := barePath(cfg, req.repoName, hasParsed, &parsedVal)
	if !IsContainedPath(cfg.BareRoot, bareDest) {
		return "", false, fmt.Errorf("computed bare path escapes bare root: %s", bareDest)
	}

	if !fsx.PathExists(bareDest) {
		if err := repo.CloneBare(req.bareSource, bareDest); err != nil {
			if req.localFB == "" {
				return "", false, err
			}
			fmt.Fprintf(os.Stderr, "Warning: remote unreachable for %s: %s. Falling back to local clone.\n", terminal.SanitizeText(req.warnOnLocal), terminal.SanitizeText(err.Error()))
			_ = os.RemoveAll(bareDest)
			if err := repo.CloneBare(req.localFB, bareDest); err != nil {
				return "", false, err
			}
		}
	}

	_ = repo.Fetch(bareDest)
	mainBranch, err := repo.GetDefaultBranch(bareDest)
	if err != nil {
		mainBranch = "main"
	}

	wtPath := worktree.Path(cfg.WorktreeRoot, req.parsedURL, req.repoName, mainBranch)
	if !IsContainedPath(cfg.WorktreeRoot, wtPath) {
		return "", false, fmt.Errorf("computed worktree path escapes worktree root: %s", wtPath)
	}
	if !fsx.PathExists(wtPath) {
		_ = worktree.Create(bareDest, mainBranch, wtPath)
	}
	_ = maintenance.Register(bareDest)

	remotes := make(map[string]repo.Remote)
	if req.originURL != "" {
		remotes["origin"] = repo.Remote{URL: req.originURL}
	}
	r := &repo.Repository{
		Name:       req.repoName,
		Path:       bareDest,
		MainBranch: mainBranch,
		Remotes:    remotes,
		Projects:   make(map[string]project.Project),
	}
	if req.originURL != "" {
		urlCopy := req.originURL
		r.URL = &urlCopy
	}
	c.Repos[req.repoName] = r

	if err := c.Save(); err != nil {
		return "", false, err
	}
	return req.repoName, true, nil
}

// Remove deletes a repo entry from the catalog (does not delete files).
func (c *Catalog) Remove(name string) error {
	if _, ok := c.Repos[name]; !ok {
		return ErrRepoNotFound{Name: name}
	}
	delete(c.Repos, name)
	return c.Save()
}

// --- Helpers ---

func barePath(cfg *config.Config, repoName string, hasParsed bool, parsed *repo.ParsedURL) string {
	if hasParsed {
		parts := []string{cfg.BareRoot, parsed.Host}
		for i, seg := range parsed.Segments {
			if i == len(parsed.Segments)-1 {
				parts = append(parts, seg+".git")
			} else {
				parts = append(parts, seg)
			}
		}
		return filepath.Join(parts...)
	}
	return filepath.Join(cfg.BareRoot, "local", repoName+".git")
}

func getOriginURL(dir string) string {
	out, err := repo.RunGitCapture("-C", dir, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// IsContainedPath reports whether path resolves to a location inside root
// (or equal to root). It uses filepath.Rel for lexical comparison; both
// sides should already be absolute, or be safe to interpret relative to
// the current working directory.
func IsContainedPath(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

type syncProgress struct {
	out         io.Writer
	dirsScanned int
}

func (p *syncProgress) root(root string) {
	if p.out == nil {
		return
	}
	fmt.Fprintf(p.out, "Walking %s\n", terminal.SanitizeText(root))
}

func (p *syncProgress) dir(path string) {
	if p.out == nil {
		return
	}
	p.dirsScanned++
	if p.dirsScanned%100 == 0 {
		fmt.Fprintf(p.out, "  scanned %d directories; now at %s\n", p.dirsScanned, terminal.SanitizeText(path))
	}
}

func (p *syncProgress) found(path string) {
	if p.out == nil {
		return
	}
	fmt.Fprintf(p.out, "  found repo: %s\n", terminal.SanitizeText(path))
}

func (p *syncProgress) discovered(root string, count int) {
	if p.out == nil {
		return
	}
	fmt.Fprintf(p.out, "Discovered %d repo(s) under %s\n", count, terminal.SanitizeText(root))
}

func (p *syncProgress) registering(path string) {
	if p.out == nil {
		return
	}
	fmt.Fprintf(p.out, "Registering %s\n", terminal.SanitizeText(path))
}

func walkForRepos(dir string, depth uint32, opts SyncOptions, progress *syncProgress) ([]string, error) {
	if progress != nil {
		progress.dir(dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		excluded := false
		for _, pat := range opts.Exclude {
			if pat == name {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		path := filepath.Join(dir, name)
		dotGit := filepath.Join(path, ".git")

		// Submodule: .git is a file, not dir → skip
		if info, err := os.Lstat(dotGit); err == nil && !info.IsDir() {
			continue
		}
		// Bare repo: HEAD + objects/ exist but no .git → skip
		if !fsx.PathExists(dotGit) && fsx.PathExists(filepath.Join(path, "HEAD")) && fsx.PathExists(filepath.Join(path, "objects")) {
			continue
		}
		// Real repo
		if info, err := os.Stat(dotGit); err == nil && info.IsDir() {
			repos = append(repos, path)
			if progress != nil {
				progress.found(path)
			}
			continue
		}
		if depth+1 < opts.MaxDepth {
			sub, err := walkForRepos(path, depth+1, opts, progress)
			if err != nil {
				return repos, err
			}
			repos = append(repos, sub...)
		}
	}
	return repos, nil
}

// isGitRepo asks git itself whether path is a working tree or bare repo.
// Replaces a stat-based heuristic (`.git` or `HEAD` exists) that was
// trivially spoofable: any directory with an attacker-placed `HEAD`
// file would pass, and then `git clone --bare <path>` would run against
// it.
func isGitRepo(path string) bool {
	if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
		return false
	}
	_, err := repo.RunGitCapture("-C", path, "rev-parse", "--git-dir")
	return err == nil
}
