package project

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestNixConfigRoundTrip(t *testing.T) {
	nt := NixFlake
	ds := "myshell"
	path := "nix/"
	c := NixConfig{NixType: &nt, Devshell: &ds, Path: &path}
	raw, err := toml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), "flake") {
		t.Errorf("expected flake literal in %q", string(raw))
	}
	var back NixConfig
	if err := toml.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.NixType == nil || *back.NixType != NixFlake {
		t.Errorf("nix_type = %v", back.NixType)
	}
	if back.Devshell == nil || *back.Devshell != "myshell" {
		t.Errorf("devshell = %v", back.Devshell)
	}
}

func TestNixConfigOmitsNone(t *testing.T) {
	c := NixConfig{}
	raw, err := toml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(raw)
	if strings.Contains(s, "type") || strings.Contains(s, "devshell") || strings.Contains(s, "path") {
		t.Errorf("nil fields should be omitted: %q", s)
	}
}

func TestProjectTagsRoundTrip(t *testing.T) {
	p := Project{Name: "myproject", Tags: []string{"go", "cli"}}
	raw, err := toml.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Project
	if err := toml.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back.Tags) != 2 || back.Tags[0] != "go" {
		t.Errorf("tags = %v", back.Tags)
	}
}

func TestProjectEmptyTagsOmitted(t *testing.T) {
	p := Project{Name: "x"}
	raw, err := toml.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "tags") {
		t.Errorf("empty tags should not appear: %q", string(raw))
	}
}

func TestSubprojectSparsePathsRoundTrip(t *testing.T) {
	paths := []string{"src/api", "libs/shared"}
	s := Subproject{
		Name:           "api",
		SubprojectPath: "services/api",
		SparsePaths:    &paths,
	}
	raw, err := toml.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Subproject
	if err := toml.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.SparsePaths == nil || len(*back.SparsePaths) != 2 {
		t.Errorf("sparse_paths = %v", back.SparsePaths)
	}

	// Nil sparse_paths should not serialize.
	s2 := Subproject{Name: "web", SubprojectPath: "services/web"}
	raw2, _ := toml.Marshal(s2)
	if strings.Contains(string(raw2), "sparse_paths") {
		t.Errorf("nil sparse_paths should be omitted: %q", string(raw2))
	}
}
