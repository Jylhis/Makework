package config

import (
	"errors"
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

func TestShowBaseKeys(t *testing.T) {
	c := &Config{WorktreeRoot: "/d/w", BareRoot: "/d/r"}
	entries, err := c.Show()
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Key != "worktree_root" || entries[0].Value != "/d/w" {
		t.Errorf("entry[0] = %+v", entries[0])
	}
}

func TestShowIncludesScanRoots(t *testing.T) {
	c := &Config{
		WorktreeRoot: "/d/w",
		BareRoot:     "/d/r",
		ScanRoots:    []string{"/home/u/projects"},
	}
	entries, err := c.Show()
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("want 3 entries, got %d", len(entries))
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

func TestResolverApplyDefaults(t *testing.T) {
	r := ResolverConfig{}
	r.ApplyDefaults()
	if r.WeightFuzzy != DefaultWeightFuzzy {
		t.Errorf("fuzzy = %v", r.WeightFuzzy)
	}
	if r.WeightContext != DefaultWeightContext {
		t.Errorf("context = %v", r.WeightContext)
	}
}
