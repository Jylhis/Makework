// Package config loads, saves, and mutates the makework config.toml.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jylhis/makework/internal/xdgpath"
	"github.com/pelletier/go-toml/v2"
)

// Default resolver weights (match Rust constants).
const (
	DefaultWeightFuzzy    = 0.35
	DefaultWeightFrecency = 0.35
	DefaultWeightActivity = 0.15
	DefaultWeightContext  = 0.15
)

// ResolverConfig holds resolver scoring weights. Zero-valued struct is NOT
// equivalent to defaults; use (*ResolverConfig).ApplyDefaults.
type ResolverConfig struct {
	WeightFuzzy    float64 `toml:"weight_fuzzy"`
	WeightFrecency float64 `toml:"weight_frecency"`
	WeightActivity float64 `toml:"weight_activity"`
	WeightContext  float64 `toml:"weight_context"`
}

// DefaultResolver returns the default weights.
func DefaultResolver() ResolverConfig {
	return ResolverConfig{
		WeightFuzzy:    DefaultWeightFuzzy,
		WeightFrecency: DefaultWeightFrecency,
		WeightActivity: DefaultWeightActivity,
		WeightContext:  DefaultWeightContext,
	}
}

// ApplyDefaults fills in any zero weight with its default.
func (r *ResolverConfig) ApplyDefaults() {
	if r.WeightFuzzy == 0 {
		r.WeightFuzzy = DefaultWeightFuzzy
	}
	if r.WeightFrecency == 0 {
		r.WeightFrecency = DefaultWeightFrecency
	}
	if r.WeightActivity == 0 {
		r.WeightActivity = DefaultWeightActivity
	}
	if r.WeightContext == 0 {
		r.WeightContext = DefaultWeightContext
	}
}

// Config is the deserialized makework config.
type Config struct {
	WorktreeRoot string          `toml:"worktree_root"`
	BareRoot     string          `toml:"bare_root"`
	ScanRoots    []string        `toml:"scan_roots,omitempty"`
	SyncMaxDepth *uint32         `toml:"sync_max_depth,omitempty"`
	SyncExclude  []string        `toml:"sync_exclude,omitempty"`
	TemplateDir  *string         `toml:"template_dir,omitempty"`
	AllowHooks   bool            `toml:"allow_hooks,omitempty"`
	Resolver     *ResolverConfig `toml:"resolver,omitempty"`
}

// configFile wraps Config under the [config] table (matches Rust layout).
type configFile struct {
	Config Config `toml:"config"`
}

// ErrUnknownKey is returned by Set for unknown config keys.
type ErrUnknownKey struct{ Key string }

func (e ErrUnknownKey) Error() string {
	return fmt.Sprintf("unknown config key: %s (supported: worktree_root, bare_root, scan_roots, sync_max_depth, sync_exclude, template_dir, allow_hooks)", e.Key)
}

// Defaults returns the default config, deriving paths from XDG.
func Defaults() (*Config, error) {
	dataDir, err := xdgpath.DataDir()
	if err != nil {
		return nil, err
	}
	return &Config{
		WorktreeRoot: filepath.Join(dataDir, "worktrees"),
		BareRoot:     filepath.Join(dataDir, "repos"),
	}, nil
}

// Path returns the absolute config file path.
func Path() (string, error) {
	dir, err := xdgpath.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// Load reads the config file and returns defaults if absent.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults()
	}
	if err != nil {
		return nil, err
	}
	var file configFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	cfg := file.Config
	if err := expandTildePaths(&cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return &cfg, nil
}

func expandTildePaths(c *Config) error {
	var err error
	if c.WorktreeRoot, err = xdgpath.ExpandTilde(c.WorktreeRoot); err != nil {
		return err
	}
	if c.BareRoot, err = xdgpath.ExpandTilde(c.BareRoot); err != nil {
		return err
	}
	for i, r := range c.ScanRoots {
		c.ScanRoots[i], err = xdgpath.ExpandTilde(r)
		if err != nil {
			return err
		}
	}
	if c.TemplateDir != nil {
		expanded, err := xdgpath.ExpandTilde(*c.TemplateDir)
		if err != nil {
			return err
		}
		c.TemplateDir = &expanded
	}
	return nil
}

// Validate checks cross-field constraints on the config.
func (c *Config) Validate() error {
	if c.WorktreeRoot == "" {
		return errors.New("worktree_root must not be empty")
	}
	if c.BareRoot == "" {
		return errors.New("bare_root must not be empty")
	}
	if !filepath.IsAbs(c.WorktreeRoot) {
		return fmt.Errorf("worktree_root must be an absolute path, got %q", c.WorktreeRoot)
	}
	if !filepath.IsAbs(c.BareRoot) {
		return fmt.Errorf("bare_root must be an absolute path, got %q", c.BareRoot)
	}
	if filepath.Clean(c.WorktreeRoot) == filepath.Clean(c.BareRoot) {
		return fmt.Errorf("worktree_root and bare_root must differ (both are %q)", c.WorktreeRoot)
	}
	if c.Resolver != nil {
		sum := c.Resolver.WeightFuzzy + c.Resolver.WeightFrecency +
			c.Resolver.WeightActivity + c.Resolver.WeightContext
		if sum < 0.01 {
			return fmt.Errorf("resolver weights sum to %.3f; at least one weight must be positive", sum)
		}
	}
	return nil
}

// Save writes the config to disk (creating parent dirs).
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.SetIndentTables(true)
	if err := enc.Encode(configFile{Config: *c}); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// ShowEntry is one row in the `mw config show` output.
type ShowEntry struct {
	Key    string
	Value  string
	Source string
}

// Show returns the settings that should be displayed to the user.
func (c *Config) Show() ([]ShowEntry, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	source := "default"
	if _, err := os.Stat(path); err == nil {
		source = "config-file"
	}
	entries := []ShowEntry{
		{"worktree_root", c.WorktreeRoot, source},
		{"bare_root", c.BareRoot, source},
	}
	if len(c.ScanRoots) > 0 {
		entries = append(entries, ShowEntry{"scan_roots", strings.Join(c.ScanRoots, ", "), source})
	}
	return entries, nil
}

// Set applies a key=value mutation, loading/persisting the config.
// Supported keys: worktree_root, bare_root, scan_roots, sync_max_depth,
// sync_exclude, template_dir.
func Set(key, value string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	if err := applySet(cfg, key, value); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation after set: %w", err)
	}
	return cfg.Save()
}

func applySet(c *Config, key, value string) error {
	switch key {
	case "worktree_root":
		v, err := xdgpath.ExpandTilde(value)
		if err != nil {
			return err
		}
		c.WorktreeRoot = v
	case "bare_root":
		v, err := xdgpath.ExpandTilde(value)
		if err != nil {
			return err
		}
		c.BareRoot = v
	case "scan_roots":
		parts := strings.Split(value, ",")
		var roots []string
		for _, p := range parts {
			t := strings.TrimSpace(p)
			if t == "" {
				continue
			}
			exp, err := xdgpath.ExpandTilde(t)
			if err != nil {
				return err
			}
			roots = append(roots, exp)
		}
		c.ScanRoots = roots
	case "sync_max_depth":
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid depth: %w", err)
		}
		d := uint32(n)
		c.SyncMaxDepth = &d
	case "sync_exclude":
		var excl []string
		for _, p := range strings.Split(value, ",") {
			if t := strings.TrimSpace(p); t != "" {
				excl = append(excl, t)
			}
		}
		c.SyncExclude = excl
	case "template_dir":
		v, err := xdgpath.ExpandTilde(value)
		if err != nil {
			return err
		}
		c.TemplateDir = &v
	case "allow_hooks":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool: %w", err)
		}
		c.AllowHooks = b
	default:
		return ErrUnknownKey{Key: key}
	}
	return nil
}
