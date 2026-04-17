package xdgpath

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandTildeNoTilde(t *testing.T) {
	got, err := ExpandTilde("/absolute/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/absolute/path" {
		t.Errorf("want /absolute/path, got %q", got)
	}
}

func TestExpandTildeWithPrefix(t *testing.T) {
	got, err := ExpandTilde("~/some/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
	if !strings.HasSuffix(got, "some/dir") {
		t.Errorf("expected suffix some/dir, got %q", got)
	}
	if strings.Contains(got, "~") {
		t.Errorf("expected no tilde in %q", got)
	}
}

func TestExpandTildeBare(t *testing.T) {
	got, err := ExpandTilde("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
	if strings.Contains(got, "~") {
		t.Errorf("expected no tilde in %q", got)
	}
}

func TestExpandTildeMidPath(t *testing.T) {
	got, err := ExpandTilde("/some/~/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/some/~/path" {
		t.Errorf("mid-path tilde should not expand, got %q", got)
	}
}

func TestConfigDirSuffix(t *testing.T) {
	got, err := ConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got, "makework") {
		t.Errorf("expected suffix makework, got %q", got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

func TestStateDirSuffix(t *testing.T) {
	got, err := StateDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got, "makework") {
		t.Errorf("expected suffix makework, got %q", got)
	}
}

func TestDataDirSuffix(t *testing.T) {
	got, err := DataDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got, "makework") {
		t.Errorf("expected suffix makework, got %q", got)
	}
}
