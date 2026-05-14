// Package catalog manages the repo registry (catalog.toml) — loading, saving,
// syncing, adding repos, and resolving project names.
package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/jylhis/makework/internal/config"
	"github.com/jylhis/makework/internal/maintenance"
	"github.com/jylhis/makework/internal/project"
	"github.com/jylhis/makework/internal/repo"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/jylhis/makework/internal/xdgpath"
	"github.com/pelletier/go-toml/v2"
)

// Catalog is the top-level registry of bare repos.
type Catalog struct {
	Repos map[string]*repo.Repository `toml:"repos"`
}

// SyncOptions controls repo discovery.
type SyncOptions struct {
	MaxDepth uint32
	Exclude  []string
}

// InitResult describes what `Init` created or found.
type InitResult struct {
	Created        []string
	AlreadyExisted []string
}

// ResolvedProject is the result of resolving a project name.
type ResolvedProject struct {
	Repo           *repo.Repository
	SubprojectPath string
	NixConfig      *project.NixConfig
	SparsePaths    []string
}

// --- Error types ---

type ErrRepoNotFound struct{ Name string }

func (e ErrRepoNotFound) Error() string { return "repository not found: " + e.Name }

type ErrAmbiguousProject struct {
	Name  string
	Repos []string
}

func (e ErrAmbiguousProject) Error() string {
	return fmt.Sprintf("ambiguous project name '%s': found in repos %s. Use the repo name directly.",
		e.Name, strings.Join(e.Repos, ", "))
}

type ErrDuplicateSubproject struct {
	Name string
	Repo string
}

func (e ErrDuplicateSubproject) Error() string {
	return fmt.Sprintf("duplicate subproject name '%s' in repo '%s'", e.Name, e.Repo)
}

type ErrNotGitRepo struct{ Path string }

func (e ErrNotGitRepo) Error() string { return "not a git repository: " + e.Path }

// --- Path helpers ---

// CatalogPath returns the catalog.toml path.
func CatalogPath() (string, error) {
	dir, err := config.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(dir), "catalog.toml"), nil
}

// --- Load / Save ---

func Load() (*Catalog, error) {
	path, err := CatalogPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Catalog{Repos: make(map[string]*repo.Repository)}, nil
	}
	if err != nil {
		return nil, err
	}
	var cat Catalog
	if err := toml.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if cat.Repos == nil {
		cat.Repos = make(map[string]*repo.Repository)
	}
	for name, r := range cat.Repos {
		r.Name = name
	}
	if err := cat.validateUniqueSubprojects(); err != nil {
		return nil, err
	}
	return &cat, nil
}

func (c *Catalog) Save() error {
	path, err := CatalogPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.SetIndentTables(true)
	if err := enc.Encode(c); err != nil {
		return err
	}

	unlock, err := acquireSaveLock()
	if err != nil {
		return err
	}
	defer unlock()

	tmp, err := os.CreateTemp(filepath.Dir(path), ".catalog.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// acquireSaveLock takes an exclusive POSIX advisory lock on a sidecar lock
// file under $XDG_STATE_HOME/makework/. Concurrent Save calls serialise on
// this lock so the atomic temp+rename below never interleaves.
func acquireSaveLock() (func(), error) {
	stateDir, err := xdgpath.StateDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(stateDir, "catalog.toml.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

// --- Init ---

func Init(cfg *config.Config) (*InitResult, error) {
	result := &InitResult{}

	cfgDir, err := config.Path()
	if err != nil {
		return nil, err
	}
	cfgDir = filepath.Dir(cfgDir)
	trackDir(cfgDir, result)

	cfgPath, _ := config.Path()
	if fileExists(cfgPath) {
		result.AlreadyExisted = append(result.AlreadyExisted, cfgPath)
	} else {
		if err := cfg.Save(); err != nil {
			return nil, err
		}
		result.Created = append(result.Created, cfgPath)
	}

	catPath, _ := CatalogPath()
	if fileExists(catPath) {
		result.AlreadyExisted = append(result.AlreadyExisted, catPath)
	} else {
		empty := &Catalog{Repos: make(map[string]*repo.Repository)}
		if err := empty.Save(); err != nil {
			return nil, err
		}
		result.Created = append(result.Created, catPath)
	}

	trackDir(cfg.WorktreeRoot, result)
	trackDir(cfg.BareRoot, result)
	return result, nil
}

func trackDir(path string, result *InitResult) {
	if fileExists(path) {
		result.AlreadyExisted = append(result.AlreadyExisted, path)
	} else {
		_ = os.MkdirAll(path, 0o755)
		result.Created = append(result.Created, path)
	}
}

// --- Validation ---

func (c *Catalog) validateUniqueSubprojects() error {
	seen := make(map[string]string) // subproject → first repo
	for repoName, r := range c.Repos {
		for _, proj := range r.Projects {
			for subName := range proj.Subprojects {
				if prevRepo, exists := seen[subName]; exists {
					_ = prevRepo
					return ErrDuplicateSubproject{Name: subName, Repo: repoName}
				}
				seen[subName] = repoName
			}
		}
	}
	return nil
}

// --- Resolution ---

// AllProjectNames returns a sorted, deduplicated list of all navigable names.
func (c *Catalog) AllProjectNames() []string {
	set := make(map[string]struct{})
	for repoName, r := range c.Repos {
		set[repoName] = struct{}{}
		for projName, proj := range r.Projects {
			set[projName] = struct{}{}
			for subName := range proj.Subprojects {
				set[subName] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// FindProject resolves a name through repo → project → subproject. Returns the
// repo and optional subproject path.
func (c *Catalog) FindProject(name string) (*repo.Repository, string, bool) {
	if r, ok := c.Repos[name]; ok {
		return r, "", true
	}
	for _, r := range c.Repos {
		if _, ok := r.Projects[name]; ok {
			return r, "", true
		}
	}
	for _, r := range c.Repos {
		for _, proj := range r.Projects {
			if sub, ok := proj.Subprojects[name]; ok {
				return r, sub.SubprojectPath, true
			}
		}
	}
	return nil, "", false
}

// FindProjectUnambiguous resolves a name, failing on ambiguity.
func (c *Catalog) FindProjectUnambiguous(name string) (*ResolvedProject, error) {
	if r, ok := c.Repos[name]; ok {
		return &ResolvedProject{Repo: r}, nil
	}
	var projMatches []*repo.Repository
	for _, r := range c.Repos {
		if _, ok := r.Projects[name]; ok {
			projMatches = append(projMatches, r)
		}
	}
	if len(projMatches) == 1 {
		return &ResolvedProject{Repo: projMatches[0]}, nil
	}
	if len(projMatches) > 1 {
		names := make([]string, len(projMatches))
		for i, r := range projMatches {
			names[i] = r.Name
		}
		return nil, ErrAmbiguousProject{Name: name, Repos: names}
	}
	var subMatches []ResolvedProject
	for _, r := range c.Repos {
		for _, proj := range r.Projects {
			if sub, ok := proj.Subprojects[name]; ok {
				rp := ResolvedProject{
					Repo:           r,
					SubprojectPath: sub.SubprojectPath,
					NixConfig:      sub.Nix,
				}
				if sub.SparsePaths != nil {
					rp.SparsePaths = *sub.SparsePaths
				}
				subMatches = append(subMatches, rp)
			}
		}
	}
	if len(subMatches) == 1 {
		return &subMatches[0], nil
	}
	if len(subMatches) > 1 {
		names := make([]string, len(subMatches))
		for i, r := range subMatches {
			names[i] = r.Repo.Name
		}
		return nil, ErrAmbiguousProject{Name: name, Repos: names}
	}
	return nil, ErrRepoNotFound{Name: name}
}

// --- Sync ---

// Sync discovers repos under scanRoots and registers them.
func (c *Catalog) Sync(cfg *config.Config, scanRoots []string, opts SyncOptions) ([]string, error) {
	var added []string
	for _, root := range scanRoots {
		if !fileExists(root) {
			continue
		}
		repos, err := walkForRepos(root, 0, opts)
		if err != nil {
			return added, err
		}
		for _, rp := range repos {
			name, isNew, err := c.Add(rp, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", rp, err)
				continue
			}
			if isNew {
				added = append(added, name)
			}
		}
	}
	return added, nil
}

// --- Add (from local path) ---

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

	if _, exists := c.Repos[repoName]; exists {
		return repoName, false, nil
	}

	bareDest := barePath(cfg, repoName, hasParsed, &parsed)
	if !IsContainedPath(cfg.BareRoot, bareDest) {
		return "", false, fmt.Errorf("computed bare path escapes bare root: %s", bareDest)
	}

	if remoteURL != "" && !fileExists(bareDest) {
		if err := repo.CloneBare(remoteURL, bareDest); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: remote unreachable for %s: %v. Falling back to local clone.\n", sourcePath, err)
			_ = os.RemoveAll(bareDest)
			if err := repo.CloneBare(sourcePath, bareDest); err != nil {
				return "", false, err
			}
		}
	} else if !fileExists(bareDest) {
		if err := repo.CloneBare(sourcePath, bareDest); err != nil {
			return "", false, err
		}
	}

	_ = repo.Fetch(bareDest)
	mainBranch, err := repo.GetDefaultBranch(bareDest)
	if err != nil {
		mainBranch = "main"
	}

	var parsedPtr *repo.ParsedURL
	if hasParsed {
		parsedPtr = &parsed
	}
	wtPath := worktree.Path(cfg.WorktreeRoot, parsedPtr, repoName, mainBranch)
	if !IsContainedPath(cfg.WorktreeRoot, wtPath) {
		return "", false, fmt.Errorf("computed worktree path escapes worktree root: %s", wtPath)
	}
	if !fileExists(wtPath) {
		_ = worktree.Create(bareDest, mainBranch, wtPath)
	}
	_ = maintenance.Register(bareDest)

	remotes := make(map[string]repo.Remote)
	if remoteURL != "" {
		remotes["origin"] = repo.Remote{URL: remoteURL}
	}

	r := &repo.Repository{
		Name:       repoName,
		Path:       bareDest,
		MainBranch: mainBranch,
		Remotes:    remotes,
		Projects:   make(map[string]project.Project),
	}
	if remoteURL != "" {
		r.URL = &remoteURL
	}
	c.Repos[repoName] = r

	if err := c.Save(); err != nil {
		return "", false, err
	}
	return repoName, true, nil
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
	if _, exists := c.Repos[repoName]; exists {
		return repoName, false, nil
	}

	bareDest := barePath(cfg, repoName, true, &parsed)
	if !IsContainedPath(cfg.BareRoot, bareDest) {
		return "", false, fmt.Errorf("computed bare path escapes bare root: %s", bareDest)
	}
	if !fileExists(bareDest) {
		if err := repo.CloneBare(url, bareDest); err != nil {
			return "", false, err
		}
	}
	_ = repo.Fetch(bareDest)
	mainBranch, err := repo.GetDefaultBranch(bareDest)
	if err != nil {
		mainBranch = "main"
	}

	wtPath := worktree.Path(cfg.WorktreeRoot, &parsed, repoName, mainBranch)
	if !IsContainedPath(cfg.WorktreeRoot, wtPath) {
		return "", false, fmt.Errorf("computed worktree path escapes worktree root: %s", wtPath)
	}
	if !fileExists(wtPath) {
		_ = worktree.Create(bareDest, mainBranch, wtPath)
	}
	_ = maintenance.Register(bareDest)

	remotes := map[string]repo.Remote{"origin": {URL: url}}
	urlCopy := url
	r := &repo.Repository{
		Name:       repoName,
		Path:       bareDest,
		URL:        &urlCopy,
		MainBranch: mainBranch,
		Remotes:    remotes,
		Projects:   make(map[string]project.Project),
	}
	c.Repos[repoName] = r
	if err := c.Save(); err != nil {
		return "", false, err
	}
	return repoName, true, nil
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
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

func walkForRepos(dir string, depth uint32, opts SyncOptions) ([]string, error) {
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
		if !fileExists(dotGit) && fileExists(filepath.Join(path, "HEAD")) && fileExists(filepath.Join(path, "objects")) {
			continue
		}
		// Real repo
		if info, err := os.Stat(dotGit); err == nil && info.IsDir() {
			repos = append(repos, path)
			continue
		}
		if depth+1 < opts.MaxDepth {
			sub, err := walkForRepos(path, depth+1, opts)
			if err != nil {
				return repos, err
			}
			repos = append(repos, sub...)
		}
	}
	return repos, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}
