// Package repo models catalog repository entries and wraps git shell-outs.
package repo

import "github.com/jylhis/makework/internal/project"

// Remote is one named remote on a bare repository.
type Remote struct {
	URL string `toml:"url"`
}

// Repository is one entry in the catalog.toml [repos] table.
type Repository struct {
	Name       string                     `toml:"-"`
	Path       string                     `toml:"path"`
	URL        *string                    `toml:"url,omitempty"`
	MainBranch string                     `toml:"main_branch"`
	Remotes    map[string]Remote          `toml:"remotes,omitempty"`
	Projects   map[string]project.Project `toml:"projects,omitempty"`
}
