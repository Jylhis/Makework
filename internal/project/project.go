// Package project models the per-worktree .makework.toml metadata and the
// subproject/nix config fields embedded in catalog entries.
package project

// NixType names one of the auto-detected nix environment kinds.
type NixType string

const (
	NixFlake   NixType = "flake"
	NixClassic NixType = "classic"
	NixDevenv  NixType = "devenv"
	NixCustom  NixType = "custom"
)

// NixConfig overrides the auto-detected nix activation.
type NixConfig struct {
	NixType  *NixType `toml:"type,omitempty"`
	Devshell *string  `toml:"devshell,omitempty"`
	Path     *string  `toml:"path,omitempty"`
}

// Project is the root of .makework.toml.
type Project struct {
	Name        string                `toml:"name"`
	Tags        []string              `toml:"tags,omitempty"`
	Subprojects map[string]Subproject `toml:"subprojects,omitempty"`
	Hooks       Hooks                 `toml:"hooks,omitempty"`
}

// Hooks holds the small set of lifecycle hooks supported by mw.
// PostCreate commands run via `sh -c` in the new worktree after it is
// created. No template engine; mw provides MW_WORKTREE_PATH, MW_BRANCH,
// and MW_REPO via the process environment instead.
type Hooks struct {
	PostCreate []string `toml:"post-create,omitempty"`
}

// Subproject is a nested component inside a repo (e.g. a monorepo package).
type Subproject struct {
	Name           string     `toml:"name"`
	SubprojectPath string     `toml:"subproject_path"`
	Docs           *string    `toml:"docs,omitempty"`
	Nix            *NixConfig `toml:"nix,omitempty"`
	SparsePaths    *[]string  `toml:"sparse_paths,omitempty"`
}
