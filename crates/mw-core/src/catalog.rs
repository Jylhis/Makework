use std::collections::{BTreeMap, HashSet};
use std::fs;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

use crate::config::MakeworkConfig;
use crate::project::{NixConfig, Subproject};
use crate::repository::Repository;

/// Options for controlling how [`Catalog::sync`] discovers repositories.
#[derive(Debug, Clone)]
pub struct SyncOptions {
    /// Maximum directory depth to recurse when scanning. Default is `1`.
    pub max_depth: u32,
    /// Directory name patterns to skip during scanning (exact match).
    pub exclude: Vec<String>,
}

impl Default for SyncOptions {
    fn default() -> Self {
        Self {
            max_depth: 1,
            exclude: Vec::new(),
        }
    }
}

/// Summary of what [`Catalog::init`] created or found.
#[derive(Debug, Default)]
pub struct InitResult {
    /// Paths that were newly created by this invocation.
    pub created: Vec<PathBuf>,
    /// Paths that already existed and were left untouched.
    pub already_existed: Vec<PathBuf>,
}

/// Result of resolving a project name through the catalog.
#[derive(Debug)]
pub struct ResolvedProject<'a> {
    pub repo: &'a Repository,
    pub subproject_path: Option<&'a str>,
    pub nix_config: Option<&'a NixConfig>,
}

#[derive(Debug, Default, Serialize, Deserialize)]
pub struct Catalog {
    #[serde(default)]
    pub repos: BTreeMap<String, Repository>,
}

#[derive(Debug, Default, Serialize, Deserialize)]
struct PerProjectConfig {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub main_branch: Option<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub tags: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub nix: Option<crate::project::NixConfig>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub subprojects: BTreeMap<String, Subproject>,
}

impl Catalog {
    pub fn path(_config: &MakeworkConfig) -> PathBuf {
        MakeworkConfig::config_dir()
            .expect("could not determine config directory")
            .join("catalog.toml")
    }

    pub fn load(config: &MakeworkConfig) -> Result<Self, CatalogError> {
        let path = Self::path(config);
        if !path.exists() {
            return Ok(Self::default());
        }
        let content = fs::read_to_string(&path).map_err(|e| CatalogError::Io(path.clone(), e))?;
        let mut catalog: Catalog =
            toml::from_str(&content).map_err(|e| CatalogError::Parse(path.clone(), e))?;

        // Populate repo name from the map key
        for (name, repo) in &mut catalog.repos {
            repo.name = name.clone();
        }

        // Validate unique subproject names across all repos
        catalog.validate_unique_subprojects()?;

        Ok(catalog)
    }

    pub fn save(&self, config: &MakeworkConfig) -> Result<(), CatalogError> {
        let path = Self::path(config);
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|e| CatalogError::Io(parent.to_path_buf(), e))?;
        }
        let content = toml::to_string_pretty(self).map_err(CatalogError::Serialize)?;
        fs::write(&path, content).map_err(|e| CatalogError::Io(path.clone(), e))?;
        Ok(())
    }

    /// Initialize makework directories and config files.
    ///
    /// Creates the config directory, default `config.toml`, empty
    /// `catalog.toml`, `bare_root`, and `worktree_root`. Idempotent:
    /// existing files are never overwritten.
    pub fn init(config: &MakeworkConfig) -> Result<InitResult, CatalogError> {
        let mut result = InitResult::default();

        // Config directory
        let config_dir =
            MakeworkConfig::config_dir().map_err(|e| CatalogError::Config(e.to_string()))?;
        create_dir_tracking(&config_dir, &mut result)?;

        // config.toml
        let config_path = config_dir.join("config.toml");
        if config_path.exists() {
            result.already_existed.push(config_path);
        } else {
            config
                .save()
                .map_err(|e| CatalogError::Config(e.to_string()))?;
            result.created.push(config_path);
        }

        // catalog.toml
        let catalog_path = Self::path(config);
        if catalog_path.exists() {
            result.already_existed.push(catalog_path);
        } else {
            Catalog::default().save(config)?;
            result.created.push(catalog_path);
        }

        // Data directories
        create_dir_tracking(&config.bare_root, &mut result)?;
        create_dir_tracking(&config.worktree_root, &mut result)?;

        Ok(result)
    }

    fn validate_unique_subprojects(&self) -> Result<(), CatalogError> {
        let mut seen: HashSet<&str> = HashSet::new();
        for (repo_name, repo) in &self.repos {
            for project in repo.projects.values() {
                for sub_name in project.subprojects.keys() {
                    if !seen.insert(sub_name.as_str()) {
                        return Err(CatalogError::DuplicateSubproject {
                            name: sub_name.clone(),
                            repo: repo_name.clone(),
                        });
                    }
                }
            }
        }
        Ok(())
    }

    pub fn merge_per_project(
        &mut self,
        repo_name: &str,
        worktree_path: &Path,
    ) -> Result<(), CatalogError> {
        let makework_toml = worktree_path.join(".makework.toml");
        if !makework_toml.exists() {
            return Ok(());
        }
        let content = fs::read_to_string(&makework_toml)
            .map_err(|e| CatalogError::Io(makework_toml.clone(), e))?;
        let per_project: PerProjectConfig =
            toml::from_str(&content).map_err(|e| CatalogError::Parse(makework_toml.clone(), e))?;

        let repo = self
            .repos
            .get_mut(repo_name)
            .ok_or_else(|| CatalogError::RepoNotFound(repo_name.to_string()))?;

        if let Some(branch) = per_project.main_branch {
            repo.main_branch = branch;
        }

        // Merge per-project into the default project entry
        let project = repo.projects.entry(repo_name.to_string()).or_default();

        if !per_project.tags.is_empty() {
            project.tags = per_project.tags;
        }

        // Subprojects: per-project wins on conflict
        for (name, sub) in per_project.subprojects {
            project.subprojects.insert(name, sub);
        }

        Ok(())
    }

    pub fn find_repo(&self, name: &str) -> Option<&Repository> {
        self.repos.get(name)
    }

    /// Alias for [`find_project`](Self::find_project).
    ///
    /// Resolution order: repo names → project names → subproject names.
    pub fn resolve_project(&self, name: &str) -> Option<(&Repository, Option<&str>)> {
        self.find_project(name)
    }

    pub fn find_project(&self, name: &str) -> Option<(&Repository, Option<&str>)> {
        // 1. Try repo names
        if let Some(repo) = self.repos.get(name) {
            return Some((repo, None));
        }
        // 2. Try project names within repos
        for repo in self.repos.values() {
            if repo.projects.contains_key(name) {
                return Some((repo, None));
            }
        }
        // 3. Try subproject names
        for repo in self.repos.values() {
            for project in repo.projects.values() {
                if let Some(sub) = project.subprojects.get(name) {
                    return Some((repo, Some(sub.subproject_path.as_str())));
                }
            }
        }
        None
    }

    /// Like [`find_project`], but returns an error when the name is ambiguous
    /// (matches in multiple repos). Also returns the subproject's [`NixConfig`]
    /// when the match is a subproject.
    pub fn find_project_unambiguous(
        &self,
        name: &str,
    ) -> Result<ResolvedProject<'_>, CatalogError> {
        // 1. Exact repo name — always unambiguous
        if let Some(repo) = self.repos.get(name) {
            return Ok(ResolvedProject {
                repo,
                subproject_path: None,
                nix_config: None,
            });
        }

        // 2. Project names within repos — collect all matches
        let project_matches: Vec<&Repository> = self
            .repos
            .values()
            .filter(|repo| repo.projects.contains_key(name))
            .collect();
        if project_matches.len() == 1 {
            return Ok(ResolvedProject {
                repo: project_matches[0],
                subproject_path: None,
                nix_config: None,
            });
        }
        if project_matches.len() > 1 {
            return Err(CatalogError::AmbiguousProject {
                name: name.to_string(),
                repos: project_matches.iter().map(|r| r.name.clone()).collect(),
            });
        }

        // 3. Subproject names — collect all matches
        let mut sub_matches: Vec<(&Repository, &str, Option<&NixConfig>)> = Vec::new();
        for repo in self.repos.values() {
            for project in repo.projects.values() {
                if let Some(sub) = project.subprojects.get(name) {
                    sub_matches.push((repo, sub.subproject_path.as_str(), sub.nix.as_ref()));
                }
            }
        }
        if sub_matches.len() == 1 {
            let (repo, path, nix) = sub_matches[0];
            return Ok(ResolvedProject {
                repo,
                subproject_path: Some(path),
                nix_config: nix,
            });
        }
        if sub_matches.len() > 1 {
            return Err(CatalogError::AmbiguousProject {
                name: name.to_string(),
                repos: sub_matches.iter().map(|(r, _, _)| r.name.clone()).collect(),
            });
        }

        Err(CatalogError::RepoNotFound(name.to_string()))
    }

    /// Scan directories for git repositories not yet in the catalog.
    ///
    /// Walks each `scan_root` up to `options.max_depth` levels deep, looking
    /// for directories that contain a `.git` directory. Skips entries matching
    /// `options.exclude` patterns, submodules (`.git` is a file), and bare
    /// repos. Calls [`catalog_add`] for each discovered repo. Returns the
    /// list of newly registered repo names.
    pub fn sync(
        &mut self,
        config: &MakeworkConfig,
        scan_roots: &[PathBuf],
        options: &SyncOptions,
    ) -> Result<Vec<String>, CatalogError> {
        let mut added = Vec::new();
        for root in scan_roots {
            if !root.exists() {
                continue;
            }
            let repos = walk_for_repos(root, 0, options)?;
            for repo_path in repos {
                match self.catalog_add(&repo_path, config) {
                    Ok(name) => {
                        if !added.contains(&name) {
                            added.push(name);
                        }
                    }
                    Err(e) => {
                        eprintln!("Warning: skipping {}: {e}", repo_path.display());
                    }
                }
            }
        }
        Ok(added)
    }

    pub fn catalog_add(
        &mut self,
        source_path: &Path,
        config: &MakeworkConfig,
    ) -> Result<String, CatalogError> {
        use crate::repository::{Remote, clone_bare, fetch, get_default_branch, parse_remote_url};
        use crate::worktree::{create_worktree, worktree_path};

        let source_path = std::fs::canonicalize(source_path)
            .map_err(|e| CatalogError::Io(source_path.to_path_buf(), e))?;

        // Detect if this is a git repo by looking for .git
        let git_dir = if source_path.join(".git").exists() || source_path.join("HEAD").exists() {
            &source_path
        } else {
            return Err(CatalogError::NotGitRepo(source_path));
        };

        // Get remote URL (origin preferred)
        let remote_url = get_origin_url(git_dir);
        let parsed = remote_url.as_deref().and_then(parse_remote_url);

        // Derive repo name
        let repo_name = if let Some(ref p) = parsed {
            p.segments.last().cloned().unwrap_or_else(|| {
                source_path
                    .file_name()
                    .unwrap_or_default()
                    .to_string_lossy()
                    .to_string()
            })
        } else {
            source_path
                .file_name()
                .unwrap_or_default()
                .to_string_lossy()
                .to_string()
        };

        // Idempotent: if already registered, return the name
        if self.repos.contains_key(&repo_name) {
            return Ok(repo_name);
        }

        // Compute bare clone destination
        let bare_dest = if let Some(ref p) = parsed {
            let mut path = config.bare_root.clone();
            path.push(&p.host);
            for seg in &p.segments {
                path.push(seg);
            }
            path.set_extension("git");
            path
        } else {
            config
                .bare_root
                .join("local")
                .join(format!("{repo_name}.git"))
        };

        // Clone bare (or register local-only)
        if let Some(ref url) = remote_url {
            if !bare_dest.exists() {
                match clone_bare(url, &bare_dest) {
                    Ok(()) => {}
                    Err(e) => {
                        eprintln!(
                            "Warning: remote unreachable for {}: {e}. Falling back to local clone.",
                            source_path.display()
                        );
                        // Clean up any partial clone attempt
                        let _ = std::fs::remove_dir_all(&bare_dest);
                        clone_bare_from_local(&source_path, &bare_dest)?;
                    }
                }
            }
        } else if !bare_dest.exists() {
            clone_bare_from_local(&source_path, &bare_dest)?;
        }

        // Fetch to ensure up to date
        let _ = fetch(&bare_dest);

        // Get default branch
        let main_branch = get_default_branch(&bare_dest).unwrap_or_else(|_| "main".to_string());

        // Create default worktree
        let wt_path = worktree_path(config, parsed.as_ref(), &repo_name, &main_branch);
        if !wt_path.exists() {
            let _ = create_worktree(&bare_dest, &main_branch, &wt_path);
        }

        // Register maintenance
        let _ = crate::maintenance::maintenance_register(&bare_dest);

        // Build remotes map
        let mut remotes = BTreeMap::new();
        if let Some(ref url) = remote_url {
            remotes.insert("origin".to_string(), Remote { url: url.clone() });
        }

        // Build repository entry
        let repo = Repository {
            name: repo_name.clone(),
            path: bare_dest,
            url: remote_url,
            main_branch,
            remotes,
            projects: BTreeMap::new(),
        };

        self.repos.insert(repo_name.clone(), repo);

        // Merge per-project config if .makework.toml exists
        let _ = self.merge_per_project(&repo_name, &source_path);

        self.save(config)?;

        Ok(repo_name)
    }

    /// Register a remote git repository by URL.
    ///
    /// Unlike [`catalog_add`](Self::catalog_add) which starts from a local
    /// path, this clones directly from the provided URL without requiring a
    /// local checkout.
    pub fn catalog_add_url(
        &mut self,
        url: &str,
        config: &MakeworkConfig,
    ) -> Result<String, CatalogError> {
        use crate::repository::{Remote, clone_bare, fetch, get_default_branch, parse_remote_url};
        use crate::worktree::{create_worktree, worktree_path};

        let parsed = parse_remote_url(url)
            .ok_or_else(|| CatalogError::Config(format!("invalid git URL: {url}")))?;

        let repo_name =
            parsed.segments.last().cloned().ok_or_else(|| {
                CatalogError::Config(format!("cannot derive repo name from: {url}"))
            })?;

        // Idempotent
        if self.repos.contains_key(&repo_name) {
            return Ok(repo_name);
        }

        // Compute bare clone destination
        let mut bare_dest = config.bare_root.clone();
        bare_dest.push(&parsed.host);
        for seg in &parsed.segments {
            bare_dest.push(seg);
        }
        bare_dest.set_extension("git");

        // Clone bare from URL
        if !bare_dest.exists() {
            clone_bare(url, &bare_dest).map_err(CatalogError::Git)?;
        }

        let _ = fetch(&bare_dest);

        let main_branch = get_default_branch(&bare_dest).unwrap_or_else(|_| "main".to_string());

        // Create default worktree
        let wt_path = worktree_path(config, Some(&parsed), &repo_name, &main_branch);
        if !wt_path.exists() {
            let _ = create_worktree(&bare_dest, &main_branch, &wt_path);
        }

        let _ = crate::maintenance::maintenance_register(&bare_dest);

        let mut remotes = BTreeMap::new();
        remotes.insert(
            "origin".to_string(),
            Remote {
                url: url.to_string(),
            },
        );

        let repo = Repository {
            name: repo_name.clone(),
            path: bare_dest,
            url: Some(url.to_string()),
            main_branch,
            remotes,
            projects: BTreeMap::new(),
        };

        self.repos.insert(repo_name.clone(), repo);
        self.save(config)?;

        Ok(repo_name)
    }
}

/// Recursively walk directories to find git repositories.
fn walk_for_repos(
    dir: &Path,
    current_depth: u32,
    options: &SyncOptions,
) -> Result<Vec<PathBuf>, CatalogError> {
    let mut repos = Vec::new();
    let entries = std::fs::read_dir(dir).map_err(|e| CatalogError::Io(dir.to_path_buf(), e))?;
    for entry in entries {
        let entry = entry.map_err(|e| CatalogError::Io(dir.to_path_buf(), e))?;
        let path = entry.path();
        if !path.is_dir() {
            continue;
        }

        // Skip entries matching exclude patterns
        if let Some(name) = path.file_name().and_then(|n| n.to_str()) {
            if options.exclude.iter().any(|pat| pat == name) {
                continue;
            }
        }

        let dot_git = path.join(".git");

        // Detect submodule: .git is a file (not dir), skip
        if dot_git.exists() && !dot_git.is_dir() {
            continue;
        }

        // Detect bare repo: HEAD + objects/ exist but no .git → skip
        if !dot_git.exists()
            && path.join("HEAD").exists()
            && path.join("objects").exists()
        {
            continue;
        }

        // Real repo: .git is a directory
        if dot_git.is_dir() {
            repos.push(path);
            // Do not recurse into repos
            continue;
        }

        // Otherwise recurse if within depth limit
        if current_depth + 1 < options.max_depth {
            let sub = walk_for_repos(&path, current_depth + 1, options)?;
            repos.extend(sub);
        }
    }
    Ok(repos)
}

fn create_dir_tracking(path: &Path, result: &mut InitResult) -> Result<(), CatalogError> {
    if path.exists() {
        result.already_existed.push(path.to_path_buf());
    } else {
        fs::create_dir_all(path).map_err(|e| CatalogError::Io(path.to_path_buf(), e))?;
        result.created.push(path.to_path_buf());
    }
    Ok(())
}

fn clone_bare_from_local(source_path: &Path, bare_dest: &Path) -> Result<(), CatalogError> {
    let output = std::process::Command::new("git")
        .args(["clone", "--bare"])
        .arg(source_path)
        .arg(bare_dest)
        .output()
        .map_err(|e| CatalogError::Io(bare_dest.to_path_buf(), e))?;
    if !output.status.success() {
        return Err(CatalogError::Git(crate::repository::GitError::Command {
            cmd: format!(
                "git clone --bare {} {}",
                source_path.display(),
                bare_dest.display()
            ),
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        }));
    }
    Ok(())
}

fn get_origin_url(git_dir: &Path) -> Option<String> {
    let output = std::process::Command::new("git")
        .args(["-C"])
        .arg(git_dir)
        .args(["remote", "get-url", "origin"])
        .output()
        .ok()?;
    if output.status.success() {
        let url = String::from_utf8_lossy(&output.stdout).trim().to_string();
        if url.is_empty() { None } else { Some(url) }
    } else {
        None
    }
}

#[derive(Debug)]
pub enum CatalogError {
    Io(PathBuf, std::io::Error),
    Parse(PathBuf, toml::de::Error),
    Serialize(toml::ser::Error),
    DuplicateSubproject { name: String, repo: String },
    RepoNotFound(String),
    AmbiguousProject { name: String, repos: Vec<String> },
    NotGitRepo(PathBuf),
    Git(crate::repository::GitError),
    Config(String),
}

impl std::fmt::Display for CatalogError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Io(path, e) => write!(f, "{}: {e}", path.display()),
            Self::Parse(path, e) => write!(f, "parse error in {}: {e}", path.display()),
            Self::Serialize(e) => write!(f, "serialization error: {e}"),
            Self::DuplicateSubproject { name, repo } => {
                write!(f, "duplicate subproject name '{name}' in repo '{repo}'")
            }
            Self::RepoNotFound(name) => write!(f, "repository not found: {name}"),
            Self::AmbiguousProject { name, repos } => {
                write!(
                    f,
                    "ambiguous project name '{name}': found in repos {}. Use the repo name directly.",
                    repos.join(", ")
                )
            }
            Self::NotGitRepo(path) => write!(f, "not a git repository: {}", path.display()),
            Self::Git(e) => write!(f, "git error: {e}"),
            Self::Config(msg) => write!(f, "config error: {msg}"),
        }
    }
}

impl std::error::Error for CatalogError {}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sync_options_default_values() {
        let opts = SyncOptions::default();
        assert_eq!(opts.max_depth, 1);
        assert!(opts.exclude.is_empty());
    }

    #[test]
    fn catalog_toml_round_trip() {
        use crate::repository::{Remote, Repository};

        let mut catalog = Catalog::default();
        let mut remotes = BTreeMap::new();
        remotes.insert(
            "origin".to_string(),
            Remote {
                url: "git@github.com:user/repo.git".to_string(),
            },
        );
        let repo = Repository {
            name: "test-repo".to_string(),
            path: PathBuf::from("/tmp/repos/test.git"),
            url: Some("git@github.com:user/repo.git".to_string()),
            main_branch: "main".to_string(),
            remotes,
            projects: BTreeMap::new(),
        };
        catalog.repos.insert("test-repo".to_string(), repo);

        let serialized = toml::to_string_pretty(&catalog).unwrap();
        let deserialized: Catalog = toml::from_str(&serialized).unwrap();
        assert_eq!(deserialized.repos.len(), 1);
        assert!(deserialized.repos.contains_key("test-repo"));
        let r = &deserialized.repos["test-repo"];
        assert_eq!(r.main_branch, "main");
        assert_eq!(r.url.as_deref(), Some("git@github.com:user/repo.git"));
    }

    #[test]
    fn duplicate_subproject_rejected() {
        use crate::project::{Project, Subproject};
        use crate::repository::Repository;

        let mut catalog = Catalog::default();

        let mut proj1 = Project {
            name: "proj1".to_string(),
            ..Default::default()
        };
        proj1.subprojects.insert(
            "shared-name".to_string(),
            Subproject {
                name: "shared-name".to_string(),
                subproject_path: "a".to_string(),
                ..Default::default()
            },
        );

        let mut proj2 = Project {
            name: "proj2".to_string(),
            ..Default::default()
        };
        proj2.subprojects.insert(
            "shared-name".to_string(),
            Subproject {
                name: "shared-name".to_string(),
                subproject_path: "b".to_string(),
                ..Default::default()
            },
        );

        let repo1 = Repository {
            name: "r1".to_string(),
            path: PathBuf::from("/tmp/r1"),
            url: None,
            main_branch: "main".to_string(),
            remotes: BTreeMap::new(),
            projects: BTreeMap::from([("proj1".to_string(), proj1)]),
        };
        let repo2 = Repository {
            name: "r2".to_string(),
            path: PathBuf::from("/tmp/r2"),
            url: None,
            main_branch: "main".to_string(),
            remotes: BTreeMap::new(),
            projects: BTreeMap::from([("proj2".to_string(), proj2)]),
        };
        catalog.repos.insert("r1".to_string(), repo1);
        catalog.repos.insert("r2".to_string(), repo2);

        let result = catalog.validate_unique_subprojects();
        assert!(result.is_err());
    }

    #[test]
    fn find_project_by_subproject_name() {
        use crate::project::{Project, Subproject};
        use crate::repository::Repository;

        let mut catalog = Catalog::default();
        let mut project = Project {
            name: "myproject".to_string(),
            ..Default::default()
        };
        project.subprojects.insert(
            "api".to_string(),
            Subproject {
                name: "api".to_string(),
                subproject_path: "services/api".to_string(),
                ..Default::default()
            },
        );

        let repo = Repository {
            name: "myrepo".to_string(),
            path: PathBuf::from("/tmp/myrepo"),
            url: None,
            main_branch: "main".to_string(),
            remotes: BTreeMap::new(),
            projects: BTreeMap::from([("myproject".to_string(), project)]),
        };
        catalog.repos.insert("myrepo".to_string(), repo);

        // Find by repo name
        let (r, sub) = catalog.find_project("myrepo").unwrap();
        assert_eq!(r.name, "myrepo");
        assert!(sub.is_none());

        // Find by project name
        let (r, sub) = catalog.find_project("myproject").unwrap();
        assert_eq!(r.name, "myrepo");
        assert!(sub.is_none());

        // Find by subproject name
        let (r, sub) = catalog.find_project("api").unwrap();
        assert_eq!(r.name, "myrepo");
        assert_eq!(sub, Some("services/api"));
    }

    #[test]
    fn find_project_unambiguous_returns_error_on_duplicate_project_name() {
        use crate::project::Project;
        use crate::repository::Repository;

        let mut catalog = Catalog::default();

        let proj1 = Project {
            name: "shared".to_string(),
            ..Default::default()
        };
        let proj2 = Project {
            name: "shared".to_string(),
            ..Default::default()
        };

        let repo1 = Repository {
            name: "r1".to_string(),
            path: PathBuf::from("/tmp/r1"),
            url: None,
            main_branch: "main".to_string(),
            remotes: BTreeMap::new(),
            projects: BTreeMap::from([("shared".to_string(), proj1)]),
        };
        let repo2 = Repository {
            name: "r2".to_string(),
            path: PathBuf::from("/tmp/r2"),
            url: None,
            main_branch: "main".to_string(),
            remotes: BTreeMap::new(),
            projects: BTreeMap::from([("shared".to_string(), proj2)]),
        };
        catalog.repos.insert("r1".to_string(), repo1);
        catalog.repos.insert("r2".to_string(), repo2);

        let result = catalog.find_project_unambiguous("shared");
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("ambiguous"));
    }

    #[test]
    fn find_project_unambiguous_succeeds_when_unique() {
        use crate::project::{Project, Subproject};
        use crate::repository::Repository;

        let mut catalog = Catalog::default();
        let mut project = Project {
            name: "myproject".to_string(),
            ..Default::default()
        };
        project.subprojects.insert(
            "api".to_string(),
            Subproject {
                name: "api".to_string(),
                subproject_path: "services/api".to_string(),
                ..Default::default()
            },
        );
        let repo = Repository {
            name: "myrepo".to_string(),
            path: PathBuf::from("/tmp/myrepo"),
            url: None,
            main_branch: "main".to_string(),
            remotes: BTreeMap::new(),
            projects: BTreeMap::from([("myproject".to_string(), project)]),
        };
        catalog.repos.insert("myrepo".to_string(), repo);

        // By repo name
        let resolved = catalog.find_project_unambiguous("myrepo").unwrap();
        assert_eq!(resolved.repo.name, "myrepo");
        assert!(resolved.subproject_path.is_none());

        // By project name
        let resolved = catalog.find_project_unambiguous("myproject").unwrap();
        assert_eq!(resolved.repo.name, "myrepo");

        // By subproject name
        let resolved = catalog.find_project_unambiguous("api").unwrap();
        assert_eq!(resolved.repo.name, "myrepo");
        assert_eq!(resolved.subproject_path, Some("services/api"));

        // Not found
        let result = catalog.find_project_unambiguous("nonexistent");
        assert!(result.is_err());
    }
}
