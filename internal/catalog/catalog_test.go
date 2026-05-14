package catalog

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jylhis/makework/internal/project"
	"github.com/jylhis/makework/internal/repo"
	"github.com/pelletier/go-toml/v2"
)

func isolatedXDG(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	return home
}

func newRepo(name, path string) *repo.Repository {
	return &repo.Repository{
		Name:       name,
		Path:       path,
		MainBranch: "main",
		Remotes:    make(map[string]repo.Remote),
		Projects:   make(map[string]project.Project),
	}
}

func TestAllProjectNames(t *testing.T) {
	cat := &Catalog{Repos: map[string]*repo.Repository{
		"alpha": newRepo("alpha", "/tmp/a"),
		"beta":  newRepo("beta", "/tmp/b"),
	}}
	names := cat.AllProjectNames()
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("got %v", names)
	}
}

func TestAllProjectNamesIncludesSubprojects(t *testing.T) {
	sub := project.Subproject{Name: "api", SubprojectPath: "services/api"}
	proj := project.Project{
		Name:        "myproject",
		Subprojects: map[string]project.Subproject{"api": sub},
	}
	r := newRepo("myrepo", "/tmp/myrepo")
	r.Projects["myproject"] = proj
	cat := &Catalog{Repos: map[string]*repo.Repository{"myrepo": r}}

	names := cat.AllProjectNames()
	has := func(s string) bool {
		for _, n := range names {
			if n == s {
				return true
			}
		}
		return false
	}
	if !has("myrepo") || !has("myproject") || !has("api") {
		t.Errorf("got %v", names)
	}
}

func TestFindProject(t *testing.T) {
	sub := project.Subproject{Name: "api", SubprojectPath: "services/api"}
	proj := project.Project{
		Name:        "myproject",
		Subprojects: map[string]project.Subproject{"api": sub},
	}
	r := newRepo("myrepo", "/tmp/myrepo")
	r.Projects["myproject"] = proj
	cat := &Catalog{Repos: map[string]*repo.Repository{"myrepo": r}}

	// by repo
	rr, sp, ok := cat.FindProject("myrepo")
	if !ok || rr.Name != "myrepo" || sp != "" {
		t.Error("repo lookup failed")
	}
	// by project
	rr, sp, ok = cat.FindProject("myproject")
	if !ok || rr.Name != "myrepo" || sp != "" {
		t.Error("project lookup failed")
	}
	// by subproject
	rr, sp, ok = cat.FindProject("api")
	if !ok || rr.Name != "myrepo" || sp != "services/api" {
		t.Error("subproject lookup failed")
	}
	// not found
	_, _, ok = cat.FindProject("nope")
	if ok {
		t.Error("expected not found")
	}
}

func TestFindProjectUnambiguous(t *testing.T) {
	sub := project.Subproject{Name: "api", SubprojectPath: "services/api"}
	proj := project.Project{
		Name:        "myproject",
		Subprojects: map[string]project.Subproject{"api": sub},
	}
	r := newRepo("myrepo", "/tmp/myrepo")
	r.Projects["myproject"] = proj
	cat := &Catalog{Repos: map[string]*repo.Repository{"myrepo": r}}

	rp, err := cat.FindProjectUnambiguous("myrepo")
	if err != nil || rp.Repo.Name != "myrepo" {
		t.Error("repo lookup failed")
	}
	rp, err = cat.FindProjectUnambiguous("api")
	if err != nil || rp.SubprojectPath != "services/api" {
		t.Error("subproject lookup failed")
	}
	_, err = cat.FindProjectUnambiguous("nope")
	var notFound ErrRepoNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("expected ErrRepoNotFound, got %v", err)
	}
}

func TestFindProjectUnambiguousAmbiguous(t *testing.T) {
	shared1 := project.Project{Name: "shared"}
	shared2 := project.Project{Name: "shared"}
	r1 := newRepo("r1", "/tmp/r1")
	r1.Projects["shared"] = shared1
	r2 := newRepo("r2", "/tmp/r2")
	r2.Projects["shared"] = shared2
	cat := &Catalog{Repos: map[string]*repo.Repository{"r1": r1, "r2": r2}}

	_, err := cat.FindProjectUnambiguous("shared")
	var ambig ErrAmbiguousProject
	if !errors.As(err, &ambig) {
		t.Errorf("expected ErrAmbiguousProject, got %v", err)
	}
}

func TestValidateUniqueSubprojects(t *testing.T) {
	sub := project.Subproject{Name: "shared", SubprojectPath: "a"}
	proj1 := project.Project{
		Name:        "p1",
		Subprojects: map[string]project.Subproject{"shared": sub},
	}
	proj2 := project.Project{
		Name:        "p2",
		Subprojects: map[string]project.Subproject{"shared": {Name: "shared", SubprojectPath: "b"}},
	}
	r1 := newRepo("r1", "/tmp/r1")
	r1.Projects["p1"] = proj1
	r2 := newRepo("r2", "/tmp/r2")
	r2.Projects["p2"] = proj2
	cat := &Catalog{Repos: map[string]*repo.Repository{"r1": r1, "r2": r2}}
	err := cat.validateUniqueSubprojects()
	var dup ErrDuplicateSubproject
	if !errors.As(err, &dup) {
		t.Errorf("expected ErrDuplicateSubproject, got %v", err)
	}
}

func TestCatalogSaveConcurrent(t *testing.T) {
	isolatedXDG(t)

	const writers = 10
	const itersPerWriter = 20

	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range itersPerWriter {
				r := newRepo("repo", "/tmp/repo")
				cat := &Catalog{Repos: map[string]*repo.Repository{"repo": r}}
				if err := cat.Save(); err != nil {
					t.Errorf("writer %d iter %d: Save: %v", id, j, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load after concurrent Save: %v", err)
	}
	if len(loaded.Repos) != 1 || loaded.Repos["repo"] == nil {
		t.Fatalf("post-concurrent catalog corrupted: %+v", loaded.Repos)
	}
}

func TestIsGitRepoRejectsSpoofedHEAD(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isGitRepo(dir) {
		t.Errorf("isGitRepo accepted dir with a bare HEAD file (no git-dir)")
	}
}

func TestIsGitRepoAcceptsInitRepo(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Skipf("git init unavailable: %v", err)
	}
	if !isGitRepo(dir) {
		t.Errorf("isGitRepo rejected a real git working tree")
	}
}

func TestCatalogTOMLRoundTrip(t *testing.T) {
	url := "git@github.com:user/repo.git"
	r := &repo.Repository{
		Name:       "test-repo",
		Path:       "/tmp/repos/test.git",
		URL:        &url,
		MainBranch: "main",
		Remotes:    map[string]repo.Remote{"origin": {URL: url}},
		Projects:   make(map[string]project.Project),
	}
	cat := &Catalog{Repos: map[string]*repo.Repository{"test-repo": r}}
	raw, err := toml.Marshal(cat)
	if err != nil {
		t.Fatal(err)
	}
	var back Catalog
	if err := toml.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Repos) != 1 {
		t.Fatalf("repos = %d", len(back.Repos))
	}
	rr := back.Repos["test-repo"]
	if rr.MainBranch != "main" || rr.URL == nil || *rr.URL != url {
		t.Errorf("round-trip = %+v", rr)
	}
}
