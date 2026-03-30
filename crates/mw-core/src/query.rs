use std::collections::BTreeMap;
use std::path::{Path, PathBuf};
use std::process::Command;

use crate::catalog::Catalog;
use crate::config::MakeworkConfig;
use crate::worktree::list_worktrees;

/// A single commit entry from a git log query.
#[derive(Debug, Clone)]
pub struct ActivityEntry {
    pub repo_name: String,
    pub branch: String,
    pub commit_hash: String,
    pub author: String,
    pub date: String,
    pub message: String,
    pub worktree_path: PathBuf,
}

/// Parse a single tab-separated git log line into an [`ActivityEntry`].
///
/// Expected format: `<hash>\t<author>\t<date>\t<message>`
pub fn parse_git_log_line(
    line: &str,
    repo_name: &str,
    branch: &str,
    worktree_path: &Path,
) -> Option<ActivityEntry> {
    let parts: Vec<&str> = line.splitn(4, '\t').collect();
    if parts.len() < 4 {
        return None;
    }
    Some(ActivityEntry {
        repo_name: repo_name.to_string(),
        branch: branch.to_string(),
        commit_hash: parts[0].to_string(),
        author: parts[1].to_string(),
        date: parts[2].to_string(),
        message: parts[3].to_string(),
        worktree_path: worktree_path.to_path_buf(),
    })
}

/// Deduplicate activity entries by `(repo_name, commit_hash)`.
///
/// The same commit can appear in multiple worktrees of the same repo.
/// Entries from different repos with the same hash are kept (cherry-picks).
pub fn dedup_entries(entries: &mut Vec<ActivityEntry>) {
    entries.sort_by(|a, b| {
        a.repo_name
            .cmp(&b.repo_name)
            .then(a.commit_hash.cmp(&b.commit_hash))
    });
    entries.dedup_by(|a, b| a.repo_name == b.repo_name && a.commit_hash == b.commit_hash);
}

/// Query recent git activity across all registered worktrees.
///
/// For each repo's non-bare worktrees, runs `git log --since=<since>` and
/// collects the results. Deduplicates across worktrees of the same repo.
pub fn query_activity(
    catalog: &Catalog,
    _config: &MakeworkConfig,
    since: &str,
    until: Option<&str>,
    author: Option<&str>,
) -> Vec<ActivityEntry> {
    let mut all_entries = Vec::new();

    for (repo_name, repo) in &catalog.repos {
        let worktrees = match list_worktrees(&repo.path) {
            Ok(wts) => wts,
            Err(_) => continue,
        };

        for wt in &worktrees {
            if wt.is_bare || !wt.path.exists() {
                continue;
            }

            let branch = wt.branch.as_deref().unwrap_or("(detached)");

            let mut args = vec![
                "-C".to_string(),
                wt.path.display().to_string(),
                "log".to_string(),
                format!("--since={since}"),
                "--format=%H\t%an\t%aI\t%s".to_string(),
            ];

            if let Some(u) = until {
                args.push(format!("--until={u}"));
            }
            if let Some(a) = author {
                args.push(format!("--author={a}"));
            }

            let output = Command::new("git").args(&args).output().ok();

            if let Some(output) = output
                && output.status.success()
            {
                let text = String::from_utf8_lossy(&output.stdout);
                for line in text.lines() {
                    if line.is_empty() {
                        continue;
                    }
                    if let Some(entry) = parse_git_log_line(line, repo_name, branch, &wt.path) {
                        all_entries.push(entry);
                    }
                }
            }
        }
    }

    dedup_entries(&mut all_entries);

    // Sort by date descending
    all_entries.sort_by(|a, b| b.date.cmp(&a.date));

    all_entries
}

/// Group activity entries by repo name.
pub fn query_activity_summary(entries: &[ActivityEntry]) -> BTreeMap<String, Vec<&ActivityEntry>> {
    let mut map: BTreeMap<String, Vec<&ActivityEntry>> = BTreeMap::new();
    for entry in entries {
        map.entry(entry.repo_name.clone()).or_default().push(entry);
    }
    map
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_git_log_line_valid() {
        let line = "abc1234\tTest User\t2026-03-29T10:00:00+00:00\tfix: something";
        let entry = parse_git_log_line(line, "myrepo", "main", &PathBuf::from("/tmp/wt")).unwrap();
        assert_eq!(entry.commit_hash, "abc1234");
        assert_eq!(entry.author, "Test User");
        assert_eq!(entry.date, "2026-03-29T10:00:00+00:00");
        assert_eq!(entry.message, "fix: something");
        assert_eq!(entry.repo_name, "myrepo");
        assert_eq!(entry.branch, "main");
    }

    #[test]
    fn parse_git_log_line_invalid() {
        let line = "not enough fields";
        assert!(parse_git_log_line(line, "repo", "main", &PathBuf::from("/tmp")).is_none());
    }

    #[test]
    fn dedup_entries_same_repo() {
        let mut entries = vec![
            ActivityEntry {
                repo_name: "repo".to_string(),
                branch: "main".to_string(),
                commit_hash: "abc123".to_string(),
                author: "Alice".to_string(),
                date: "2026-03-29T10:00:00+00:00".to_string(),
                message: "fix".to_string(),
                worktree_path: PathBuf::from("/wt1"),
            },
            ActivityEntry {
                repo_name: "repo".to_string(),
                branch: "feature".to_string(),
                commit_hash: "abc123".to_string(),
                author: "Alice".to_string(),
                date: "2026-03-29T10:00:00+00:00".to_string(),
                message: "fix".to_string(),
                worktree_path: PathBuf::from("/wt2"),
            },
        ];
        dedup_entries(&mut entries);
        assert_eq!(entries.len(), 1);
    }

    #[test]
    fn dedup_keeps_different_repos() {
        let mut entries = vec![
            ActivityEntry {
                repo_name: "repo-a".to_string(),
                branch: "main".to_string(),
                commit_hash: "abc123".to_string(),
                author: "Alice".to_string(),
                date: "2026-03-29T10:00:00+00:00".to_string(),
                message: "fix".to_string(),
                worktree_path: PathBuf::from("/wt1"),
            },
            ActivityEntry {
                repo_name: "repo-b".to_string(),
                branch: "main".to_string(),
                commit_hash: "abc123".to_string(),
                author: "Alice".to_string(),
                date: "2026-03-29T10:00:00+00:00".to_string(),
                message: "fix".to_string(),
                worktree_path: PathBuf::from("/wt2"),
            },
        ];
        dedup_entries(&mut entries);
        assert_eq!(entries.len(), 2);
    }

    #[test]
    fn summary_groups_by_repo() {
        let entries = vec![
            ActivityEntry {
                repo_name: "repo-a".to_string(),
                branch: "main".to_string(),
                commit_hash: "a1".to_string(),
                author: "A".to_string(),
                date: "2026-03-29".to_string(),
                message: "m1".to_string(),
                worktree_path: PathBuf::from("/a"),
            },
            ActivityEntry {
                repo_name: "repo-b".to_string(),
                branch: "main".to_string(),
                commit_hash: "b1".to_string(),
                author: "B".to_string(),
                date: "2026-03-28".to_string(),
                message: "m2".to_string(),
                worktree_path: PathBuf::from("/b"),
            },
            ActivityEntry {
                repo_name: "repo-a".to_string(),
                branch: "main".to_string(),
                commit_hash: "a2".to_string(),
                author: "A".to_string(),
                date: "2026-03-27".to_string(),
                message: "m3".to_string(),
                worktree_path: PathBuf::from("/a"),
            },
        ];
        let summary = query_activity_summary(&entries);
        assert_eq!(summary.len(), 2);
        assert_eq!(summary["repo-a"].len(), 2);
        assert_eq!(summary["repo-b"].len(), 1);
    }
}
