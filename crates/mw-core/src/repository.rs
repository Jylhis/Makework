use std::collections::BTreeMap;
use std::path::PathBuf;

use serde::{Deserialize, Serialize};

use crate::project::Project;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Remote {
    pub url: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Repository {
    #[serde(skip)]
    pub name: String,
    pub path: PathBuf,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub url: Option<String>,
    pub main_branch: String,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub remotes: BTreeMap<String, Remote>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub projects: BTreeMap<String, Project>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ParsedUrl {
    pub host: String,
    pub segments: Vec<String>,
}

pub fn parse_remote_url(url: &str) -> Option<ParsedUrl> {
    let url = url.trim();
    if url.is_empty() {
        return None;
    }

    if let Some(rest) = url
        .strip_prefix("https://")
        .or_else(|| url.strip_prefix("http://"))
    {
        let (host, path) = rest.split_once('/')?;
        if host.is_empty() || path.is_empty() {
            return None;
        }
        let segments = split_path_segments(path);
        if segments.is_empty() {
            return None;
        }
        Some(ParsedUrl {
            host: host.to_string(),
            segments,
        })
    } else if let Some(rest) = url.strip_prefix("ssh://") {
        let without_user = rest.split_once('@').map(|(_, after)| after).unwrap_or(rest);
        let (host_port, path) = without_user.split_once('/')?;
        let host = host_port.split(':').next().unwrap_or(host_port);
        if host.is_empty() || path.is_empty() {
            return None;
        }
        let segments = split_path_segments(path);
        if segments.is_empty() {
            return None;
        }
        Some(ParsedUrl {
            host: host.to_string(),
            segments,
        })
    } else if let Some(rest) = url.strip_prefix("git@") {
        let (host, path) = rest.split_once(':')?;
        if host.is_empty() || path.is_empty() {
            return None;
        }
        let segments = split_path_segments(path);
        if segments.is_empty() {
            return None;
        }
        Some(ParsedUrl {
            host: host.to_string(),
            segments,
        })
    } else {
        None
    }
}

fn split_path_segments(path: &str) -> Vec<String> {
    let mut segments: Vec<String> = path
        .split('/')
        .filter(|s| !s.is_empty())
        .map(String::from)
        .collect();
    if let Some(last) = segments.last_mut()
        && let Some(stripped) = last.strip_suffix(".git")
    {
        *last = stripped.to_string();
    }
    segments.retain(|s| !s.is_empty());
    segments
}

// ---------------------------------------------------------------------------
// Git shell-out operations (T018, T019, T020)
// ---------------------------------------------------------------------------

use std::fmt;
use std::path::Path;
use std::process::Command;

/// Errors produced by git shell-out helpers and gitoxide operations.
#[derive(Debug)]
pub enum GitError {
    Command { cmd: String, stderr: String },
    Gitoxide(String),
}

impl fmt::Display for GitError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            GitError::Command { cmd, stderr } => {
                write!(f, "git command failed: `{cmd}`\n{stderr}")
            }
            GitError::Gitoxide(msg) => write!(f, "gitoxide error: {msg}"),
        }
    }
}

impl std::error::Error for GitError {}

/// T018 – Clone a bare repository.
///
/// Shells out to `git clone --bare <url> <dest>`.
pub fn clone_bare(url: &str, dest: &Path) -> Result<(), GitError> {
    let output = Command::new("git")
        .args(["clone", "--bare", url])
        .arg(dest)
        .output()
        .map_err(|e| GitError::Command {
            cmd: format!("git clone --bare {url} {}", dest.display()),
            stderr: e.to_string(),
        })?;

    if !output.status.success() {
        return Err(GitError::Command {
            cmd: format!("git clone --bare {url} {}", dest.display()),
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        });
    }

    Ok(())
}

/// T019 – Fetch all refs in a bare repository.
///
/// Shells out to `git -C <bare_path> fetch --all --prune --tags`.
pub fn fetch(bare_path: &Path) -> Result<(), GitError> {
    let output = Command::new("git")
        .arg("-C")
        .arg(bare_path)
        .args(["fetch", "--all", "--prune", "--tags"])
        .output()
        .map_err(|e| GitError::Command {
            cmd: format!("git -C {} fetch --all --prune --tags", bare_path.display()),
            stderr: e.to_string(),
        })?;

    if !output.status.success() {
        return Err(GitError::Command {
            cmd: format!("git -C {} fetch --all --prune --tags", bare_path.display()),
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        });
    }

    Ok(())
}

/// T020 – Resolve the default branch of a bare repository.
///
/// Opens the bare repo with `gix` and reads HEAD to determine the default
/// branch, returning the short name (e.g. `"main"`, not `"refs/heads/main"`).
pub fn get_default_branch(bare_path: &Path) -> Result<String, GitError> {
    let repo = gix::open(bare_path).map_err(|e| GitError::Gitoxide(e.to_string()))?;

    let head = repo.head().map_err(|e| GitError::Gitoxide(e.to_string()))?;

    let full_name = match head.kind {
        gix::head::Kind::Symbolic(ref reference) => reference.name.to_string(),
        gix::head::Kind::Unborn(ref name) => name.to_string(),
        gix::head::Kind::Detached { .. } => {
            return Err(GitError::Gitoxide(
                "HEAD is detached; cannot determine default branch".to_string(),
            ));
        }
    };

    let short = full_name.strip_prefix("refs/heads/").unwrap_or(&full_name);
    Ok(short.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_https_url() {
        let parsed = parse_remote_url("https://github.com/user/repo.git").unwrap();
        assert_eq!(parsed.host, "github.com");
        assert_eq!(parsed.segments, vec!["user", "repo"]);
    }

    #[test]
    fn parse_https_url_no_dot_git() {
        let parsed = parse_remote_url("https://github.com/user/repo").unwrap();
        assert_eq!(parsed.host, "github.com");
        assert_eq!(parsed.segments, vec!["user", "repo"]);
    }

    #[test]
    fn parse_ssh_url() {
        let parsed = parse_remote_url("git@github.com:user/repo.git").unwrap();
        assert_eq!(parsed.host, "github.com");
        assert_eq!(parsed.segments, vec!["user", "repo"]);
    }

    #[test]
    fn parse_ssh_url_no_dot_git() {
        let parsed = parse_remote_url("git@gitlab.com:group/subgroup/repo").unwrap();
        assert_eq!(parsed.host, "gitlab.com");
        assert_eq!(parsed.segments, vec!["group", "subgroup", "repo"]);
    }

    #[test]
    fn parse_ssh_protocol_url() {
        let parsed = parse_remote_url("ssh://git@github.com/user/repo.git").unwrap();
        assert_eq!(parsed.host, "github.com");
        assert_eq!(parsed.segments, vec!["user", "repo"]);
    }

    #[test]
    fn parse_empty_url_returns_none() {
        assert!(parse_remote_url("").is_none());
    }

    #[test]
    fn parse_garbage_returns_none() {
        assert!(parse_remote_url("not-a-url").is_none());
    }
}
