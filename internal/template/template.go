// Package template copies template files into newly created worktrees.
package template

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Apply copies all files from templateDir into worktreeDir recursively,
// preserving directory structure. Existing files in the worktree are NOT
// overwritten. Returns the list of files that were copied.
// If templateDir does not exist, returns an empty slice (graceful no-op).
func Apply(templateDir, worktreeDir string) ([]string, error) {
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		return nil, nil
	}

	var copied []string
	err := filepath.WalkDir(templateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(worktreeDir, rel)
		if err := ensureSafeDestination(worktreeDir, dest); err != nil {
			return err
		}
		if _, err := os.Lstat(dest); err == nil {
			return nil // skip existing
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("%s: %w", filepath.Dir(dest), err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("%s: %w", dest, err)
		}
		copied = append(copied, dest)
		return nil
	})
	return copied, err
}

func ensureSafeDestination(root, dest string) error {
	root = filepath.Clean(root)
	parent := filepath.Dir(dest)
	rel, err := filepath.Rel(root, parent)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s: destination escapes worktree", dest)
	}

	cur := root
	if rel == "." {
		return nil
	}
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("%s: %w", cur, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s: destination path contains symlink", cur)
		}
	}
	return nil
}
