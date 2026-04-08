use std::collections::HashSet;
use std::path::{Path, PathBuf};
use std::process::Command;

use crate::catalog::{Catalog, CatalogError};
use crate::config::MakeworkConfig;
use crate::nix::{DetectedNix, detect_nix};
use crate::repository::{GitError, ParsedUrl, parse_remote_url};

/// Information about a single git worktree, parsed from `git worktree list --porcelain`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WorktreeInfo {
    pub path: PathBuf,
    pub branch: Option<String>,
    pub is_bare: bool,
}

pub fn worktree_path(
    config: &MakeworkConfig,
    parsed_url: Option<&ParsedUrl>,
    repo_name: &str,
    branch: &str,
) -> PathBuf {
    let mut path = config.worktree_root.clone();

    match parsed_url {
        Some(url) => {
            path.push(&url.host);
            for segment in &url.segments {
                path.push(segment);
            }
        }
        None => {
            path.push("local");
            path.push(repo_name);
        }
    }

    let sanitized = sanitize_branch_for_path(branch);
    for part in sanitized.split('/') {
        path.push(part);
    }

    path
}

/// Compute the worktree parent directory for a bare repo path.
///
/// Maps `config.bare_root / host / segments.git` to `config.worktree_root / host / segments`.
/// Strips the `.git` suffix from the final component as a string (not via
/// `Path::with_extension`) to correctly handle names containing dots like
/// `jylhis.com.git` → `jylhis.com`.
///
/// Returns `None` if `bare_path` is not under `config.bare_root`.
pub fn worktree_parent_dir(config: &MakeworkConfig, bare_path: &Path) -> Option<PathBuf> {
    let relative = bare_path.strip_prefix(&config.bare_root).ok()?;
    let file_name = relative.file_name()?.to_str()?;
    let stem = file_name.strip_suffix(".git").unwrap_or(file_name);
    let parent = relative.parent().unwrap_or(Path::new(""));
    Some(config.worktree_root.join(parent).join(stem))
}

/// Sanitize a branch/ref name for use as a filesystem path.
///
/// - Strips `refs/tags/` prefix for detached-HEAD tag refs.
/// - Keeps slashes as directory separators (they become nested dirs).
/// - Replaces filesystem-problematic characters (colons, spaces, backslashes,
///   question marks, asterisks, angle brackets, pipes, double quotes) with underscores.
/// - Collapses consecutive slashes and trims leading/trailing slashes.
pub fn sanitize_branch_for_path(ref_name: &str) -> String {
    // Strip refs/tags/ prefix for detached HEAD on tags
    let name = ref_name.strip_prefix("refs/tags/").unwrap_or(ref_name);

    let mut result = String::with_capacity(name.len());
    for ch in name.chars() {
        match ch {
            ':' | ' ' | '\\' | '?' | '*' | '<' | '>' | '|' | '"' => result.push('_'),
            _ => result.push(ch),
        }
    }

    // Collapse consecutive slashes and trim leading/trailing slashes
    result
        .split('/')
        .filter(|s| !s.is_empty())
        .collect::<Vec<_>>()
        .join("/")
}

/// Enable sparse-checkout on a worktree with the given paths using cone mode.
///
/// Runs `git -C <path> sparse-checkout init --cone` followed by
/// `git -C <path> sparse-checkout set <paths...>`.
pub fn enable_sparse_checkout(worktree_path: &Path, paths: &[String]) -> Result<(), GitError> {
    let init_cmd = format!(
        "git -C {} sparse-checkout init --cone",
        worktree_path.display()
    );
    let output = Command::new("git")
        .arg("-C")
        .arg(worktree_path)
        .args(["sparse-checkout", "init", "--cone"])
        .output()
        .map_err(|e| GitError::Command {
            cmd: init_cmd.clone(),
            stderr: e.to_string(),
        })?;
    if !output.status.success() {
        return Err(GitError::Command {
            cmd: init_cmd,
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        });
    }

    let mut args = vec!["sparse-checkout", "set"];
    args.extend(paths.iter().map(String::as_str));

    let set_cmd = format!(
        "git -C {} sparse-checkout set {}",
        worktree_path.display(),
        paths.join(" ")
    );
    let output = Command::new("git")
        .arg("-C")
        .arg(worktree_path)
        .args(&args)
        .output()
        .map_err(|e| GitError::Command {
            cmd: set_cmd.clone(),
            stderr: e.to_string(),
        })?;
    if !output.status.success() {
        return Err(GitError::Command {
            cmd: set_cmd,
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        });
    }

    Ok(())
}

/// Disable sparse-checkout on a worktree, restoring the full checkout.
pub fn disable_sparse_checkout(worktree_path: &Path) -> Result<(), GitError> {
    let cmd_str = format!("git -C {} sparse-checkout disable", worktree_path.display());
    let output = Command::new("git")
        .arg("-C")
        .arg(worktree_path)
        .args(["sparse-checkout", "disable"])
        .output()
        .map_err(|e| GitError::Command {
            cmd: cmd_str.clone(),
            stderr: e.to_string(),
        })?;
    if !output.status.success() {
        return Err(GitError::Command {
            cmd: cmd_str,
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        });
    }
    Ok(())
}

pub fn create_worktree(
    bare_path: &Path,
    branch: &str,
    worktree_path: &Path,
) -> Result<(), GitError> {
    let output = Command::new("git")
        .arg("-C")
        .arg(bare_path)
        .arg("worktree")
        .arg("add")
        .arg(worktree_path)
        .arg(branch)
        .output()
        .map_err(|e| GitError::Command {
            cmd: format!(
                "git -C {} worktree add {} {branch}",
                bare_path.display(),
                worktree_path.display()
            ),
            stderr: e.to_string(),
        })?;

    if !output.status.success() {
        return Err(GitError::Command {
            cmd: format!(
                "git -C {} worktree add {} {branch}",
                bare_path.display(),
                worktree_path.display()
            ),
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        });
    }

    Ok(())
}

/// Remove a worktree and prune stale entries.
///
/// Shells out to `git -C <bare_path> worktree remove <worktree_path>` followed by
/// `git -C <bare_path> worktree prune`.
pub fn remove_worktree(bare_path: &Path, worktree_path: &Path) -> Result<(), GitError> {
    let cmd_str = format!(
        "git -C {} worktree remove {}",
        bare_path.display(),
        worktree_path.display()
    );

    let output = Command::new("git")
        .arg("-C")
        .arg(bare_path)
        .arg("worktree")
        .arg("remove")
        .arg(worktree_path)
        .output()
        .map_err(|e| GitError::Command {
            cmd: cmd_str.clone(),
            stderr: e.to_string(),
        })?;

    if !output.status.success() {
        return Err(GitError::Command {
            cmd: cmd_str,
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        });
    }

    // Prune stale worktree entries
    let prune_cmd = format!("git -C {} worktree prune", bare_path.display());
    let prune_output = Command::new("git")
        .arg("-C")
        .arg(bare_path)
        .arg("worktree")
        .arg("prune")
        .output()
        .map_err(|e| GitError::Command {
            cmd: prune_cmd.clone(),
            stderr: e.to_string(),
        })?;

    if !prune_output.status.success() {
        return Err(GitError::Command {
            cmd: prune_cmd,
            stderr: String::from_utf8_lossy(&prune_output.stderr).into_owned(),
        });
    }

    Ok(())
}

/// List all worktrees for a bare repository by parsing `git worktree list --porcelain`.
pub fn list_worktrees(bare_path: &Path) -> Result<Vec<WorktreeInfo>, GitError> {
    let output = Command::new("git")
        .arg("-C")
        .arg(bare_path)
        .args(["worktree", "list", "--porcelain"])
        .output()
        .map_err(|e| GitError::Command {
            cmd: format!("git -C {} worktree list --porcelain", bare_path.display()),
            stderr: e.to_string(),
        })?;

    if !output.status.success() {
        return Err(GitError::Command {
            cmd: format!("git -C {} worktree list --porcelain", bare_path.display()),
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        });
    }

    let text = String::from_utf8_lossy(&output.stdout);
    Ok(parse_worktree_porcelain(&text))
}

/// Parse the porcelain output of `git worktree list --porcelain` into [`WorktreeInfo`] entries.
fn parse_worktree_porcelain(text: &str) -> Vec<WorktreeInfo> {
    let mut worktrees = Vec::new();
    let mut current_path: Option<PathBuf> = None;
    let mut current_branch: Option<String> = None;
    let mut current_bare = false;

    for line in text.lines() {
        if let Some(p) = line.strip_prefix("worktree ") {
            // Flush previous entry
            if let Some(path) = current_path.take() {
                worktrees.push(WorktreeInfo {
                    path,
                    branch: current_branch.take(),
                    is_bare: current_bare,
                });
            }
            current_path = Some(PathBuf::from(p));
            current_branch = None;
            current_bare = false;
        } else if let Some(b) = line.strip_prefix("branch ") {
            let short = b.strip_prefix("refs/heads/").unwrap_or(b);
            current_branch = Some(short.to_string());
        } else if line == "bare" {
            current_bare = true;
        }
    }
    // Flush last entry
    if let Some(path) = current_path.take() {
        worktrees.push(WorktreeInfo {
            path,
            branch: current_branch.take(),
            is_bare: current_bare,
        });
    }

    worktrees
}

/// List all worktrees across all registered repositories in the catalog.
///
/// Returns a `Vec` of `(repo_name, Vec<WorktreeInfo>)` tuples, one per repository.
pub fn list_all_worktrees(
    catalog: &Catalog,
    _config: &MakeworkConfig,
) -> Vec<(String, Vec<WorktreeInfo>)> {
    let mut result = Vec::new();
    for (name, repo) in &catalog.repos {
        match list_worktrees(&repo.path) {
            Ok(worktrees) => result.push((name.clone(), worktrees)),
            Err(_) => result.push((name.clone(), Vec::new())),
        }
    }
    result
}

/// Run `git worktree prune` on a bare repository to clean up
/// stale worktree entries whose directories no longer exist.
///
/// Returns the number of worktrees that were orphaned (and thus pruned).
pub fn prune_worktrees(bare_path: &Path) -> Result<usize, GitError> {
    let worktrees = list_worktrees(bare_path)?;
    let orphaned_count = worktrees
        .iter()
        .filter(|wt| !wt.is_bare && !wt.path.exists())
        .count();

    if orphaned_count > 0 {
        let cmd_str = format!("git -C {} worktree prune", bare_path.display());
        let output = Command::new("git")
            .arg("-C")
            .arg(bare_path)
            .args(["worktree", "prune"])
            .output()
            .map_err(|e| GitError::Command {
                cmd: cmd_str.clone(),
                stderr: e.to_string(),
            })?;

        if !output.status.success() {
            return Err(GitError::Command {
                cmd: cmd_str,
                stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
            });
        }
    }

    Ok(orphaned_count)
}

/// Prune orphaned worktrees across all registered repositories,
/// and remove worktree directories that belong to no catalog entry.
///
/// Returns a list of `(label, Result)` for repos/dirs that had orphans or errors.
pub fn prune_all_worktrees(
    catalog: &Catalog,
    config: &MakeworkConfig,
) -> Vec<(String, Result<usize, GitError>)> {
    let mut results = Vec::new();

    // Prune stale worktree entries for known repos
    for (name, repo) in &catalog.repos {
        let result = prune_worktrees(&repo.path);
        match &result {
            Ok(0) => {}
            _ => results.push((name.clone(), result)),
        }
    }

    // Find and remove orphaned worktree directories (no catalog entry)
    for dir in find_orphaned_worktree_dirs(catalog, config) {
        let label = dir
            .strip_prefix(&config.worktree_root)
            .unwrap_or(&dir)
            .display()
            .to_string();
        match std::fs::remove_dir_all(&dir) {
            Ok(()) => results.push((label, Ok(1))),
            Err(e) => results.push((
                label,
                Err(GitError::Command {
                    cmd: format!("remove {}", dir.display()),
                    stderr: e.to_string(),
                }),
            )),
        }
    }

    results
}

/// Scan `config.worktree_root` for repo-level directories with no
/// corresponding catalog entry.
pub fn find_orphaned_worktree_dirs(catalog: &Catalog, config: &MakeworkConfig) -> Vec<PathBuf> {
    let known_dirs: HashSet<PathBuf> = catalog
        .repos
        .values()
        .filter_map(|repo| worktree_parent_dir(config, &repo.path))
        .collect();

    let mut orphaned = Vec::new();
    walk_for_orphaned_dirs(&config.worktree_root, &known_dirs, &mut orphaned, 0);
    orphaned
}

fn walk_for_orphaned_dirs(
    dir: &Path,
    known_dirs: &HashSet<PathBuf>,
    orphaned: &mut Vec<PathBuf>,
    depth: u32,
) {
    if depth > 10 {
        return;
    }
    let entries = match std::fs::read_dir(dir) {
        Ok(e) => e,
        Err(_) => return,
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if !path.is_dir() {
            continue;
        }
        if known_dirs.contains(&path) {
            // Known repo worktree parent, skip
            continue;
        }
        let is_ancestor = known_dirs.iter().any(|k| k.starts_with(&path));
        if is_ancestor {
            // Intermediate directory (host or owner level), recurse
            walk_for_orphaned_dirs(&path, known_dirs, orphaned, depth + 1);
        } else {
            // Not known and not ancestor — check if it has subdirs (branch dirs)
            let has_subdirs = std::fs::read_dir(&path)
                .ok()
                .map(|entries| entries.flatten().any(|e| e.path().is_dir()))
                .unwrap_or(false);
            if has_subdirs {
                orphaned.push(path);
            }
        }
    }
}

/// Result of the [`go`] function, containing the worktree path and optional
/// nix environment activation information.
#[derive(Debug)]
pub struct GoResult {
    /// Absolute path to the worktree (or subproject directory within it).
    pub path: PathBuf,
    /// Detected nix environment activation, if any.
    pub nix_activation: Option<DetectedNix>,
    /// Directory where nix activation should run (may differ from `path` when
    /// `nix.path` is set on a subproject). `None` means use `path`.
    pub nix_activation_dir: Option<PathBuf>,
}

/// Errors produced by the [`go`] function.
#[derive(Debug)]
pub enum GoError {
    /// The project name could not be resolved in the catalog.
    ProjectNotFound(String),
    /// A git operation failed while creating a worktree.
    Git(GitError),
    /// A catalog-level error (e.g. ambiguous project name).
    Catalog(CatalogError),
}

impl std::fmt::Display for GoError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            GoError::ProjectNotFound(name) => write!(f, "project not found: {name}"),
            GoError::Git(e) => write!(f, "{e}"),
            GoError::Catalog(e) => write!(f, "{e}"),
        }
    }
}

impl std::error::Error for GoError {}

impl From<GitError> for GoError {
    fn from(e: GitError) -> Self {
        GoError::Git(e)
    }
}

impl From<CatalogError> for GoError {
    fn from(e: CatalogError) -> Self {
        GoError::Catalog(e)
    }
}

/// Navigate to a project worktree, creating it if necessary.
///
/// Resolves `project_name` via the catalog, computes the worktree path, creates
/// the worktree when it does not exist, and returns the absolute path (including
/// any subproject sub-directory).
pub fn go(
    catalog: &Catalog,
    config: &MakeworkConfig,
    project_name: &str,
    ref_: Option<&str>,
) -> Result<GoResult, GoError> {
    let resolved = catalog.find_project_unambiguous(project_name)?;

    let parsed_url = resolved.repo.url.as_deref().and_then(parse_remote_url);
    let branch = ref_.unwrap_or(resolved.repo.main_branch.as_str());
    let wt_path = worktree_path(config, parsed_url.as_ref(), &resolved.repo.name, branch);

    if !wt_path.exists() {
        create_worktree(&resolved.repo.path, branch, &wt_path)?;
    }

    // Apply sparse-checkout if configured on the subproject
    if let Some(sparse) = resolved.sparse_paths {
        enable_sparse_checkout(&wt_path, sparse)?;
    }

    // Apply template files if configured
    if let Some(ref tpl_dir) = config.template_dir {
        let _ = crate::template::apply_template(tpl_dir, &wt_path);
    }

    let final_path = match resolved.subproject_path {
        Some(sub) => wt_path.join(sub),
        None => wt_path.clone(),
    };

    // Determine nix detection directory and explicit config
    let (nix_detection_dir, explicit_nix) = if let Some(nix_cfg) = resolved.nix_config {
        // Subproject with nix config — check nix.path
        if let Some(ref nix_path) = nix_cfg.path {
            (wt_path.join(nix_path), Some(nix_cfg))
        } else {
            (wt_path.clone(), Some(nix_cfg))
        }
    } else {
        (wt_path.clone(), None)
    };

    let nix_activation = detect_nix(&nix_detection_dir, explicit_nix);
    let nix_activation_dir = if nix_activation.is_some() && nix_detection_dir != final_path {
        Some(nix_detection_dir)
    } else {
        None
    };

    Ok(GoResult {
        path: final_path,
        nix_activation,
        nix_activation_dir,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn worktree_path_with_parsed_url() {
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/data/worktrees"),
            bare_root: PathBuf::from("/data/repos"),
            scan_roots: Vec::new(),
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
            resolver: None,
        };
        let parsed = ParsedUrl {
            host: "github.com".to_string(),
            segments: vec!["user".to_string(), "repo".to_string()],
        };

        let result = worktree_path(&config, Some(&parsed), "repo", "main");
        assert_eq!(
            result,
            PathBuf::from("/data/worktrees/github.com/user/repo/main")
        );
    }

    #[test]
    fn worktree_path_with_branch_slash() {
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/data/worktrees"),
            bare_root: PathBuf::from("/data/repos"),
            scan_roots: Vec::new(),
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
            resolver: None,
        };
        let parsed = ParsedUrl {
            host: "github.com".to_string(),
            segments: vec!["user".to_string(), "repo".to_string()],
        };

        let result = worktree_path(&config, Some(&parsed), "repo", "feature/auth");
        assert_eq!(
            result,
            PathBuf::from("/data/worktrees/github.com/user/repo/feature/auth")
        );
    }

    #[test]
    fn worktree_path_local_repo() {
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/data/worktrees"),
            bare_root: PathBuf::from("/data/repos"),
            scan_roots: Vec::new(),
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
            resolver: None,
        };

        let result = worktree_path(&config, None, "my-project", "develop");
        assert_eq!(
            result,
            PathBuf::from("/data/worktrees/local/my-project/develop")
        );
    }

    #[test]
    fn worktree_path_local_with_branch_slash() {
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/data/worktrees"),
            bare_root: PathBuf::from("/data/repos"),
            scan_roots: Vec::new(),
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
            resolver: None,
        };

        let result = worktree_path(&config, None, "my-project", "feature/auth");
        assert_eq!(
            result,
            PathBuf::from("/data/worktrees/local/my-project/feature/auth")
        );
    }

    #[test]
    fn worktree_path_with_subgroup_segments() {
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/data/worktrees"),
            bare_root: PathBuf::from("/data/repos"),
            scan_roots: Vec::new(),
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
            resolver: None,
        };
        let parsed = ParsedUrl {
            host: "gitlab.com".to_string(),
            segments: vec![
                "group".to_string(),
                "subgroup".to_string(),
                "repo".to_string(),
            ],
        };

        let result = worktree_path(&config, Some(&parsed), "repo", "main");
        assert_eq!(
            result,
            PathBuf::from("/data/worktrees/gitlab.com/group/subgroup/repo/main")
        );
    }

    // --- sanitize_branch_for_path tests ---

    #[test]
    fn sanitize_simple_branch() {
        assert_eq!(sanitize_branch_for_path("main"), "main");
    }

    #[test]
    fn sanitize_branch_with_slash() {
        assert_eq!(sanitize_branch_for_path("feature/auth"), "feature/auth");
    }

    #[test]
    fn sanitize_branch_strips_refs_tags() {
        assert_eq!(sanitize_branch_for_path("refs/tags/v1.0"), "v1.0");
    }

    #[test]
    fn sanitize_branch_replaces_colons() {
        assert_eq!(sanitize_branch_for_path("fix:bug"), "fix_bug");
    }

    #[test]
    fn sanitize_branch_replaces_spaces() {
        assert_eq!(sanitize_branch_for_path("my branch name"), "my_branch_name");
    }

    #[test]
    fn sanitize_branch_replaces_problematic_chars() {
        assert_eq!(
            sanitize_branch_for_path("a\\b?c*d<e>f|g\"h"),
            "a_b_c_d_e_f_g_h"
        );
    }

    #[test]
    fn sanitize_branch_collapses_slashes() {
        assert_eq!(sanitize_branch_for_path("a//b///c"), "a/b/c");
    }

    #[test]
    fn sanitize_branch_trims_slashes() {
        assert_eq!(
            sanitize_branch_for_path("/leading/trailing/"),
            "leading/trailing"
        );
    }

    #[test]
    fn sanitize_refs_tags_nested() {
        assert_eq!(
            sanitize_branch_for_path("refs/tags/release/1.0"),
            "release/1.0"
        );
    }

    // --- parse_worktree_porcelain tests ---

    #[test]
    fn parse_porcelain_bare_and_worktree() {
        let output = "\
worktree /home/user/repos/test.git
HEAD abc123
branch refs/heads/main
bare

worktree /home/user/worktrees/test/feature
HEAD def456
branch refs/heads/feature/auth

";
        let worktrees = super::parse_worktree_porcelain(output);
        assert_eq!(worktrees.len(), 2);

        assert_eq!(
            worktrees[0].path,
            PathBuf::from("/home/user/repos/test.git")
        );
        assert_eq!(worktrees[0].branch.as_deref(), Some("main"));
        assert!(worktrees[0].is_bare);

        assert_eq!(
            worktrees[1].path,
            PathBuf::from("/home/user/worktrees/test/feature")
        );
        assert_eq!(worktrees[1].branch.as_deref(), Some("feature/auth"));
        assert!(!worktrees[1].is_bare);
    }

    #[test]
    fn parse_porcelain_detached_head() {
        let output = "\
worktree /home/user/repos/test.git
HEAD abc123
bare

worktree /home/user/worktrees/test/detached
HEAD def456
detached
";
        let worktrees = super::parse_worktree_porcelain(output);
        assert_eq!(worktrees.len(), 2);

        assert_eq!(worktrees[1].branch, None);
        assert!(!worktrees[1].is_bare);
    }

    #[test]
    fn parse_porcelain_no_trailing_newline() {
        let output = "worktree /tmp/bare.git\nHEAD abc\nbare";
        let worktrees = super::parse_worktree_porcelain(output);
        assert_eq!(worktrees.len(), 1);
        assert_eq!(worktrees[0].path, PathBuf::from("/tmp/bare.git"));
        assert!(worktrees[0].is_bare);
    }

    #[test]
    fn enable_sparse_checkout_runs_git_commands() {
        let tmp = tempfile::tempdir().unwrap();
        let repo = tmp.path().join("repo");
        std::fs::create_dir_all(&repo).unwrap();

        // git init + commit so sparse-checkout has something to work with
        Command::new("git")
            .args(["init"])
            .arg(&repo)
            .output()
            .unwrap();
        Command::new("git")
            .args(["-C"])
            .arg(&repo)
            .args(["config", "user.email", "test@test.com"])
            .output()
            .unwrap();
        Command::new("git")
            .args(["-C"])
            .arg(&repo)
            .args(["config", "user.name", "Test"])
            .output()
            .unwrap();

        // Create files in src/ and docs/
        std::fs::create_dir_all(repo.join("src")).unwrap();
        std::fs::create_dir_all(repo.join("docs")).unwrap();
        std::fs::write(repo.join("src/main.rs"), "fn main() {}").unwrap();
        std::fs::write(repo.join("docs/README.md"), "# Docs").unwrap();

        Command::new("git")
            .args(["-C"])
            .arg(&repo)
            .args(["add", "."])
            .output()
            .unwrap();
        Command::new("git")
            .args(["-C"])
            .arg(&repo)
            .args(["commit", "-m", "initial"])
            .output()
            .unwrap();

        let paths = vec!["src".to_string(), "docs".to_string()];
        enable_sparse_checkout(&repo, &paths).expect("enable_sparse_checkout should succeed");

        // Verify sparse-checkout list contains the paths
        let output = Command::new("git")
            .args(["-C"])
            .arg(&repo)
            .args(["sparse-checkout", "list"])
            .output()
            .unwrap();
        let list = String::from_utf8_lossy(&output.stdout);
        assert!(
            list.contains("src"),
            "sparse-checkout list should contain src"
        );
        assert!(
            list.contains("docs"),
            "sparse-checkout list should contain docs"
        );
    }

    #[test]
    fn disable_sparse_checkout_restores_full() {
        let tmp = tempfile::tempdir().unwrap();
        let repo = tmp.path().join("repo");
        std::fs::create_dir_all(&repo).unwrap();

        Command::new("git")
            .args(["init"])
            .arg(&repo)
            .output()
            .unwrap();
        Command::new("git")
            .args(["-C"])
            .arg(&repo)
            .args(["config", "user.email", "test@test.com"])
            .output()
            .unwrap();
        Command::new("git")
            .args(["-C"])
            .arg(&repo)
            .args(["config", "user.name", "Test"])
            .output()
            .unwrap();
        std::fs::create_dir_all(repo.join("src")).unwrap();
        std::fs::write(repo.join("src/main.rs"), "fn main() {}").unwrap();
        Command::new("git")
            .args(["-C"])
            .arg(&repo)
            .args(["add", "."])
            .output()
            .unwrap();
        Command::new("git")
            .args(["-C"])
            .arg(&repo)
            .args(["commit", "-m", "initial"])
            .output()
            .unwrap();

        let paths = vec!["src".to_string()];
        enable_sparse_checkout(&repo, &paths).unwrap();
        disable_sparse_checkout(&repo).expect("disable_sparse_checkout should succeed");

        // After disable, sparse-checkout should be off
        let output = Command::new("git")
            .args(["-C"])
            .arg(&repo)
            .args(["config", "core.sparseCheckout"])
            .output()
            .unwrap();
        let value = String::from_utf8_lossy(&output.stdout).trim().to_string();
        // sparseCheckout should be false or the config key may not exist
        assert!(
            !output.status.success() || value == "false",
            "sparse checkout should be disabled"
        );
    }
}
