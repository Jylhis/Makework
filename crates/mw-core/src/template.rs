use std::fs;
use std::path::{Path, PathBuf};

/// Apply template files from `template_dir` into `worktree_dir`.
///
/// Copies all files from `template_dir` recursively into `worktree_dir`,
/// preserving directory structure. Existing files in the worktree are
/// NOT overwritten (skipped). Returns the list of files that were copied.
///
/// If `template_dir` does not exist, returns an empty vec (graceful no-op).
pub fn apply_template(
    template_dir: &Path,
    worktree_dir: &Path,
) -> Result<Vec<PathBuf>, TemplateError> {
    if !template_dir.exists() {
        return Ok(Vec::new());
    }

    let mut copied = Vec::new();
    copy_recursive(template_dir, template_dir, worktree_dir, &mut copied)?;
    Ok(copied)
}

fn copy_recursive(
    base: &Path,
    current: &Path,
    dest_root: &Path,
    copied: &mut Vec<PathBuf>,
) -> Result<(), TemplateError> {
    let entries = fs::read_dir(current).map_err(|e| TemplateError::Io(current.to_path_buf(), e))?;

    for entry in entries {
        let entry = entry.map_err(|e| TemplateError::Io(current.to_path_buf(), e))?;
        let path = entry.path();
        let relative = path.strip_prefix(base).expect("path should be under base");
        let dest = dest_root.join(relative);

        if path.is_dir() {
            copy_recursive(base, &path, dest_root, copied)?;
        } else if !dest.exists() {
            if let Some(parent) = dest.parent() {
                fs::create_dir_all(parent)
                    .map_err(|e| TemplateError::Io(parent.to_path_buf(), e))?;
            }
            fs::copy(&path, &dest).map_err(|e| TemplateError::Io(path.clone(), e))?;
            copied.push(dest);
        }
    }
    Ok(())
}

#[derive(Debug)]
pub enum TemplateError {
    Io(PathBuf, std::io::Error),
}

impl std::fmt::Display for TemplateError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Io(path, e) => write!(f, "{}: {e}", path.display()),
        }
    }
}

impl std::error::Error for TemplateError {}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn apply_template_copies_files() {
        let tmp = tempfile::tempdir().unwrap();
        let tpl = tmp.path().join("templates");
        let wt = tmp.path().join("worktree");
        fs::create_dir_all(&tpl).unwrap();
        fs::create_dir_all(&wt).unwrap();

        fs::write(tpl.join(".envrc"), "use flake").unwrap();
        fs::write(tpl.join(".editorconfig"), "root = true").unwrap();

        let copied = apply_template(&tpl, &wt).unwrap();
        assert_eq!(copied.len(), 2);
        assert_eq!(fs::read_to_string(wt.join(".envrc")).unwrap(), "use flake");
        assert_eq!(
            fs::read_to_string(wt.join(".editorconfig")).unwrap(),
            "root = true"
        );
    }

    #[test]
    fn apply_template_does_not_overwrite() {
        let tmp = tempfile::tempdir().unwrap();
        let tpl = tmp.path().join("templates");
        let wt = tmp.path().join("worktree");
        fs::create_dir_all(&tpl).unwrap();
        fs::create_dir_all(&wt).unwrap();

        fs::write(tpl.join(".envrc"), "template").unwrap();
        fs::write(wt.join(".envrc"), "existing").unwrap();

        let copied = apply_template(&tpl, &wt).unwrap();
        assert_eq!(copied.len(), 0);
        assert_eq!(fs::read_to_string(wt.join(".envrc")).unwrap(), "existing");
    }

    #[test]
    fn apply_template_copies_nested_dirs() {
        let tmp = tempfile::tempdir().unwrap();
        let tpl = tmp.path().join("templates");
        let wt = tmp.path().join("worktree");
        fs::create_dir_all(tpl.join(".config")).unwrap();
        fs::create_dir_all(&wt).unwrap();

        fs::write(tpl.join(".config/settings.json"), "{}").unwrap();

        let copied = apply_template(&tpl, &wt).unwrap();
        assert_eq!(copied.len(), 1);
        assert!(wt.join(".config/settings.json").exists());
    }

    #[test]
    fn apply_template_noop_when_dir_missing() {
        let tmp = tempfile::tempdir().unwrap();
        let wt = tmp.path().join("worktree");
        fs::create_dir_all(&wt).unwrap();
        let nonexistent = tmp.path().join("nonexistent");

        let copied = apply_template(&nonexistent, &wt).unwrap();
        assert!(copied.is_empty());
    }
}
