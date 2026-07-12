package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestDefaultsPathsAreAbsolute(t *testing.T) {
	cfg, err := Defaults()
	if err != nil {
		t.Fatalf("Defaults: %v", err)
	}
	if !filepath.IsAbs(cfg.WorktreeRoot) {
		t.Errorf("worktree_root not absolute: %q", cfg.WorktreeRoot)
	}
	if !filepath.IsAbs(cfg.BareRoot) {
		t.Errorf("bare_root not absolute: %q", cfg.BareRoot)
	}
	if !strings.HasSuffix(cfg.WorktreeRoot, filepath.Join("makework", "worktrees")) {
		t.Errorf("unexpected worktree_root suffix: %q", cfg.WorktreeRoot)
	}
}

func TestRoundTripTOML(t *testing.T) {
	depth := uint32(3)
	tpl := "/home/u/tpl"
	c := Config{
		WorktreeRoot: "/d/w",
		BareRoot:     "/d/r",
		ScanRoots:    []string{"/home/u/dev"},
		SyncMaxDepth: &depth,
		SyncExclude:  []string{"node_modules", ".cache"},
		TemplateDir:  &tpl,
	}
	raw, err := toml.Marshal(configFile{Config: c})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back configFile
	if err := toml.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Config.WorktreeRoot != c.WorktreeRoot {
		t.Errorf("worktree_root mismatch: %q", back.Config.WorktreeRoot)
	}
	if *back.Config.SyncMaxDepth != depth {
		t.Errorf("sync_max_depth mismatch: %v", *back.Config.SyncMaxDepth)
	}
	if len(back.Config.SyncExclude) != 2 {
		t.Errorf("sync_exclude length: %d", len(back.Config.SyncExclude))
	}
}

// Show lists every settable key so `config show` mirrors `config set`.
var settableKeys = []string{
	"worktree_root", "bare_root", "scan_roots", "sync_max_depth",
	"sync_exclude", "template_dir", "allow_hooks",
}

func TestShowListsAllSettableKeys(t *testing.T) {
	c := &Config{WorktreeRoot: "/d/w", BareRoot: "/d/r"}
	entries, err := c.Show()
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if len(entries) != len(settableKeys) {
		t.Fatalf("want %d entries, got %d", len(settableKeys), len(entries))
	}
	for i, key := range settableKeys {
		if entries[i].Key != key {
			t.Errorf("entry[%d].Key = %q, want %q", i, entries[i].Key, key)
		}
	}
	if entries[0].Value != "/d/w" {
		t.Errorf("worktree_root value = %q", entries[0].Value)
	}
	// Unset optional keys render as empty / zero values.
	if got := entries[2].Value; got != "" {
		t.Errorf("scan_roots value = %q, want empty", got)
	}
	if got := entries[6].Value; got != "false" {
		t.Errorf("allow_hooks value = %q, want false", got)
	}
}

func TestShowRendersSetValues(t *testing.T) {
	depth := uint32(4)
	tpl := "/home/u/tpl"
	c := &Config{
		WorktreeRoot: "/d/w",
		BareRoot:     "/d/r",
		ScanRoots:    []string{"/home/u/projects", "/home/u/work"},
		SyncMaxDepth: &depth,
		SyncExclude:  []string{"node_modules"},
		TemplateDir:  &tpl,
		AllowHooks:   true,
	}
	entries, err := c.Show()
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	byKey := make(map[string]string, len(entries))
	for _, e := range entries {
		byKey[e.Key] = e.Value
	}
	want := map[string]string{
		"scan_roots":     "/home/u/projects, /home/u/work",
		"sync_max_depth": "4",
		"sync_exclude":   "node_modules",
		"template_dir":   "/home/u/tpl",
		"allow_hooks":    "true",
	}
	for k, v := range want {
		if byKey[k] != v {
			t.Errorf("%s = %q, want %q", k, byKey[k], v)
		}
	}
}

func TestApplySetUnknownKey(t *testing.T) {
	c := &Config{}
	err := applySet(c, "nope", "v")
	var unknown ErrUnknownKey
	if !errors.As(err, &unknown) {
		t.Fatalf("expected ErrUnknownKey, got %v", err)
	}
	if unknown.Key != "nope" {
		t.Errorf("key=%q", unknown.Key)
	}
}

func TestApplySetScanRoots(t *testing.T) {
	c := &Config{}
	if err := applySet(c, "scan_roots", " /a, /b , ,/c "); err != nil {
		t.Fatalf("applySet: %v", err)
	}
	want := []string{"/a", "/b", "/c"}
	if len(c.ScanRoots) != len(want) {
		t.Fatalf("want %v, got %v", want, c.ScanRoots)
	}
	for i, r := range want {
		if c.ScanRoots[i] != r {
			t.Errorf("ScanRoots[%d] = %q, want %q", i, c.ScanRoots[i], r)
		}
	}
}

func TestApplySetDepth(t *testing.T) {
	c := &Config{}
	if err := applySet(c, "sync_max_depth", "5"); err != nil {
		t.Fatalf("applySet: %v", err)
	}
	if c.SyncMaxDepth == nil || *c.SyncMaxDepth != 5 {
		t.Errorf("sync_max_depth = %v", c.SyncMaxDepth)
	}
	if err := applySet(c, "sync_max_depth", "not-a-number"); err == nil {
		t.Error("expected parse error")
	}
}

func TestSavePreservesExistingMode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("[config]\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cfg := &Config{WorktreeRoot: "/d/w", BareRoot: "/d/r"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want %o", got, 0o600)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if loaded.WorktreeRoot != "/d/w" || loaded.BareRoot != "/d/r" {
		t.Fatalf("round-trip mismatch: %+v", loaded)
	}
}

// --- Validate tests ---

func TestValidateOK(t *testing.T) {
	cfg, err := Defaults()
	if err != nil {
		t.Fatalf("Defaults: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate on defaults should pass, got: %v", err)
	}
}

func TestValidateEmptyWorktreeRoot(t *testing.T) {
	cfg := &Config{WorktreeRoot: "", BareRoot: "/d/r"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty worktree_root")
	}
}

func TestValidateEmptyBareRoot(t *testing.T) {
	cfg := &Config{WorktreeRoot: "/d/w", BareRoot: ""}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty bare_root")
	}
}

func TestValidateRelativeWorktreeRoot(t *testing.T) {
	cfg := &Config{WorktreeRoot: "relative/path", BareRoot: "/d/r"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for relative worktree_root")
	}
}

func TestValidateRelativeBareRoot(t *testing.T) {
	cfg := &Config{WorktreeRoot: "/d/w", BareRoot: "relative/path"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for relative bare_root")
	}
}

func TestValidateSamePaths(t *testing.T) {
	cfg := &Config{WorktreeRoot: "/d/same", BareRoot: "/d/same"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error when worktree_root == bare_root")
	}
}

func TestValidateSamePathsTrailingSlash(t *testing.T) {
	cfg := &Config{WorktreeRoot: "/d/same/", BareRoot: "/d/same"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error when paths differ only by trailing slash")
	}
}

func TestValidateResolverZeroWeights(t *testing.T) {
	cfg := &Config{
		WorktreeRoot: "/d/w",
		BareRoot:     "/d/r",
		Resolver:     &ResolverConfig{},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error when all resolver weights are zero")
	}
}

func TestValidateResolverNonUnitSum(t *testing.T) {
	cfg := &Config{
		WorktreeRoot: "/d/w",
		BareRoot:     "/d/r",
		Resolver: &ResolverConfig{
			WeightFuzzy:    0.5,
			WeightFrecency: 0.5,
			WeightActivity: 0.5,
			WeightContext:  0.5,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("non-unit sum should be allowed, got: %v", err)
	}
}

func TestValidateResolverNilIsOK(t *testing.T) {
	cfg := &Config{
		WorktreeRoot: "/d/w",
		BareRoot:     "/d/r",
		Resolver:     nil,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("nil resolver should be allowed, got: %v", err)
	}
}

func TestDefaultResolverWeights(t *testing.T) {
	r := DefaultResolver()
	if r.WeightFuzzy != DefaultWeightFuzzy {
		t.Errorf("fuzzy = %v", r.WeightFuzzy)
	}
	if r.WeightContext != DefaultWeightContext {
		t.Errorf("context = %v", r.WeightContext)
	}
}
