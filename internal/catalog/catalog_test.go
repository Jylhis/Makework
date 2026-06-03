package catalog

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jylhis/makework/internal/config"
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

func TestCatalogSavePreservesExistingMode(t *testing.T) {
	isolatedXDG(t)

	catPath, err := CatalogPath()
	if err != nil {
		t.Fatalf("CatalogPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(catPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(catPath, []byte("repos = {}\n"), 0o600); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	if err := os.Chmod(catPath, 0o600); err != nil {
		t.Fatalf("chmod catalog: %v", err)
	}

	cat := &Catalog{Repos: map[string]*repo.Repository{
		"repo": newRepo("repo", "/tmp/repo"),
	}}
	if err := cat.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	st, err := os.Stat(catPath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Fatalf("catalog mode = %o, want %o", got, 0o600)
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

func TestIsContainedPath(t *testing.T) {
	cases := []struct {
		root, path string
		want       bool
	}{
		{"/a", "/a/b", true},
		{"/a", "/a", true},
		{"/a/b", "/a", false},
		{"/a", "/other/b", false},
		{"/a", "/a/../b", false},
		{"/a/b", "/a/b/../b/c", true},
	}
	for _, tc := range cases {
		got := IsContainedPath(tc.root, tc.path)
		if got != tc.want {
			t.Errorf("IsContainedPath(%q,%q) = %v; want %v", tc.root, tc.path, got, tc.want)
		}
	}
}

func TestBarePathParsed(t *testing.T) {
	cfg := &config.Config{BareRoot: "/data/repos"}
	parsed := &repo.ParsedURL{Host: "github.com", Segments: []string{"user", "repo"}}
	got := barePath(cfg, "repo", true, parsed)
	want := "/data/repos/github.com/user/repo.git"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBarePathParsedSubgroup(t *testing.T) {
	cfg := &config.Config{BareRoot: "/data/repos"}
	parsed := &repo.ParsedURL{Host: "gitlab.com", Segments: []string{"group", "sub", "repo"}}
	got := barePath(cfg, "repo", true, parsed)
	want := "/data/repos/gitlab.com/group/sub/repo.git"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBarePathLocal(t *testing.T) {
	cfg := &config.Config{BareRoot: "/data/repos"}
	got := barePath(cfg, "myproject", false, nil)
	want := "/data/repos/local/myproject.git"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRemoveMissingRepoReturnsErr(t *testing.T) {
	isolatedXDG(t)
	cat := &Catalog{Repos: make(map[string]*repo.Repository)}
	err := cat.Remove("nope")
	var nf ErrRepoNotFound
	if !errors.As(err, &nf) {
		t.Errorf("expected ErrRepoNotFound, got %v", err)
	}
}

func TestAddLocalRepo(t *testing.T) {
	home := isolatedXDG(t)
	cfg, err := config.Defaults()
	if err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(home, "src", "myrepo")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", src, "init", "-q", "-b", "main").Run(); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	exec.Command("git", "-C", src, "config", "user.email", "t@e").Run()
	exec.Command("git", "-C", src, "config", "user.name", "t").Run()
	exec.Command("git", "-C", src, "config", "commit.gpgsign", "false").Run()
	exec.Command("git", "-C", src, "commit", "-q", "--allow-empty", "-m", "init").Run()

	cat := &Catalog{Repos: make(map[string]*repo.Repository)}
	name, isNew, err := cat.Add(src, cfg)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !isNew {
		t.Errorf("expected isNew=true")
	}
	if name != "myrepo" {
		t.Errorf("name = %q, want %q", name, "myrepo")
	}
	if _, ok := cat.Repos["myrepo"]; !ok {
		t.Errorf("repo not registered in catalog: %+v", cat.Repos)
	}

	// Idempotent: adding again returns isNew=false
	_, isNew, err = cat.Add(src, cfg)
	if err != nil {
		t.Fatalf("Add second time: %v", err)
	}
	if isNew {
		t.Errorf("expected isNew=false on duplicate")
	}

	// Remove writes the catalog without error.
	if err := cat.Remove("myrepo"); err != nil {
		t.Errorf("Remove: %v", err)
	}
	if _, ok := cat.Repos["myrepo"]; ok {
		t.Errorf("repo still present after Remove")
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
