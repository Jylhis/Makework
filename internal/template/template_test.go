package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyCopiesFiles(t *testing.T) {
	tmp := t.TempDir()
	tpl := filepath.Join(tmp, "templates")
	wt := filepath.Join(tmp, "worktree")
	os.MkdirAll(tpl, 0o755)
	os.MkdirAll(wt, 0o755)
	os.WriteFile(filepath.Join(tpl, ".envrc"), []byte("use flake"), 0o644)
	os.WriteFile(filepath.Join(tpl, ".editorconfig"), []byte("root = true"), 0o644)

	copied, err := Apply(tpl, wt)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied) != 2 {
		t.Fatalf("want 2 copied, got %d", len(copied))
	}
	got, _ := os.ReadFile(filepath.Join(wt, ".envrc"))
	if string(got) != "use flake" {
		t.Errorf("envrc = %q", got)
	}
}

func TestApplyDoesNotOverwrite(t *testing.T) {
	tmp := t.TempDir()
	tpl := filepath.Join(tmp, "templates")
	wt := filepath.Join(tmp, "worktree")
	os.MkdirAll(tpl, 0o755)
	os.MkdirAll(wt, 0o755)
	os.WriteFile(filepath.Join(tpl, ".envrc"), []byte("template"), 0o644)
	os.WriteFile(filepath.Join(wt, ".envrc"), []byte("existing"), 0o644)

	copied, err := Apply(tpl, wt)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied) != 0 {
		t.Fatalf("want 0 copied, got %d", len(copied))
	}
	got, _ := os.ReadFile(filepath.Join(wt, ".envrc"))
	if string(got) != "existing" {
		t.Errorf("envrc = %q, want existing", got)
	}
}

func TestApplyNestedDirs(t *testing.T) {
	tmp := t.TempDir()
	tpl := filepath.Join(tmp, "templates")
	wt := filepath.Join(tmp, "worktree")
	os.MkdirAll(filepath.Join(tpl, ".config"), 0o755)
	os.MkdirAll(wt, 0o755)
	os.WriteFile(filepath.Join(tpl, ".config", "settings.json"), []byte("{}"), 0o644)

	copied, err := Apply(tpl, wt)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied) != 1 {
		t.Fatalf("want 1 copied, got %d", len(copied))
	}
	if _, err := os.Stat(filepath.Join(wt, ".config", "settings.json")); err != nil {
		t.Error("settings.json not found")
	}
}

func TestApplyNoopWhenDirMissing(t *testing.T) {
	wt := t.TempDir()
	copied, err := Apply("/nonexistent/template/dir", wt)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied) != 0 {
		t.Errorf("want 0 copied, got %d", len(copied))
	}
}

func TestApplySkipsSymlinkedDestinationAncestor(t *testing.T) {
	tmp := t.TempDir()
	tpl := filepath.Join(tmp, "templates")
	wt := filepath.Join(tmp, "worktree")
	escape := filepath.Join(tmp, "escape")
	os.MkdirAll(filepath.Join(tpl, ".config"), 0o755)
	os.MkdirAll(wt, 0o755)
	os.MkdirAll(escape, 0o755)
	os.WriteFile(filepath.Join(tpl, ".config", "settings.json"), []byte("{}"), 0o644)
	if err := os.Symlink(escape, filepath.Join(wt, ".config")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := Apply(tpl, wt)
	if err == nil {
		t.Fatal("expected error for symlinked destination path")
	}
	if _, err := os.Stat(filepath.Join(escape, "settings.json")); err == nil {
		t.Fatal("unexpected write outside worktree")
	}
}
