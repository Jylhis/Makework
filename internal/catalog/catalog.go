// Package catalog manages the repo registry (catalog.toml) — loading, saving,
// syncing, adding repos, and resolving project names.
package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/jylhis/makework/internal/config"
	"github.com/jylhis/makework/internal/fsx"
	"github.com/jylhis/makework/internal/project"
	"github.com/jylhis/makework/internal/repo"
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
	if fsx.PathExists(cfgPath) {
		result.AlreadyExisted = append(result.AlreadyExisted, cfgPath)
	} else {
		if err := cfg.Save(); err != nil {
			return nil, err
		}
		result.Created = append(result.Created, cfgPath)
	}

	catPath, _ := CatalogPath()
	if fsx.PathExists(catPath) {
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
	if fsx.PathExists(path) {
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
	slices.Sort(names)
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
