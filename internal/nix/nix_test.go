package nix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jylhis/makework/internal/project"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectFlakeNix(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "flake.nix", "{}")
	got := Detect(d, nil)
	if got == nil || got.NixType != project.NixFlake {
		t.Fatalf("want flake, got %+v", got)
	}
	if got.ActivationCommand != "nix develop" {
		t.Errorf("activation = %q", got.ActivationCommand)
	}
}

func TestDetectShellNix(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "shell.nix", "{}")
	got := Detect(d, nil)
	if got == nil || got.NixType != project.NixClassic {
		t.Fatalf("want classic, got %+v", got)
	}
}

func TestDetectDevenvNix(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "devenv.nix", "{}")
	got := Detect(d, nil)
	if got == nil || got.NixType != project.NixDevenv {
		t.Fatalf("want devenv, got %+v", got)
	}
}

func TestDetectEnvrc(t *testing.T) {
	for _, tt := range []struct {
		body string
		want project.NixType
	}{
		{"use flake\n", project.NixFlake},
		{"use_flake\n", project.NixFlake},
		{"use nix\n", project.NixClassic},
		{"use_nix\n", project.NixClassic},
		{"use devenv\n", project.NixDevenv},
		{"use_devenv\n", project.NixDevenv},
		{"# comment\n\nuse flake\n", project.NixFlake},
	} {
		d := t.TempDir()
		writeFile(t, d, ".envrc", tt.body)
		got := Detect(d, nil)
		if got == nil || got.NixType != tt.want {
			t.Errorf("body=%q: want %v, got %+v", tt.body, tt.want, got)
		}
	}
}

func TestExplicitConfigPrecedence(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "flake.nix", "{}")
	devenv := project.NixDevenv
	cfg := &project.NixConfig{NixType: &devenv}
	got := Detect(d, cfg)
	if got == nil || got.NixType != project.NixDevenv {
		t.Errorf("explicit should override flake, got %+v", got)
	}
}

func TestExplicitConfigWithDevshell(t *testing.T) {
	d := t.TempDir()
	flake := project.NixFlake
	ds := "myshell"
	cfg := &project.NixConfig{NixType: &flake, Devshell: &ds}
	got := Detect(d, cfg)
	if got.ActivationCommand != "nix develop .#myshell" {
		t.Errorf("activation = %q", got.ActivationCommand)
	}
}

func TestExplicitCustom(t *testing.T) {
	d := t.TempDir()
	ct := project.NixCustom
	got := Detect(d, &project.NixConfig{NixType: &ct})
	if got == nil || got.NixType != project.NixCustom {
		t.Fatalf("want custom, got %+v", got)
	}
	if got.ActivationCommand != "echo 'custom nix setup'" {
		t.Errorf("activation = %q", got.ActivationCommand)
	}
}

func TestExplicitWithNoTypeFallsThrough(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "shell.nix", "{}")
	got := Detect(d, &project.NixConfig{})
	if got == nil || got.NixType != project.NixClassic {
		t.Errorf("want classic fall-through, got %+v", got)
	}
}

func TestNoNixReturnsNil(t *testing.T) {
	d := t.TempDir()
	if got := Detect(d, nil); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

func TestEnvrcWithoutDirectivesReturnsNil(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, ".envrc", "export FOO=bar\n")
	if got := Detect(d, nil); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

func TestFlakeWinsOverShell(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "flake.nix", "{}")
	writeFile(t, d, "shell.nix", "{}")
	got := Detect(d, nil)
	if got == nil || got.NixType != project.NixFlake {
		t.Errorf("want flake precedence, got %+v", got)
	}
}
