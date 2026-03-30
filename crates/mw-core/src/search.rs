use std::collections::BTreeMap;
use std::path::{Path, PathBuf};
use std::process::Command;

use crate::catalog::Catalog;
use crate::config::MakeworkConfig;
use crate::repository::parse_remote_url;
use crate::worktree::worktree_path;

/// A single search result from a file match.
#[derive(Debug, Clone)]
pub struct SearchResult {
    pub repo_name: String,
    pub worktree_path: PathBuf,
    pub file_path: String,
    pub line_number: u32,
    pub line_content: String,
}

/// Options for controlling search behavior.
#[derive(Debug, Clone, Default)]
pub struct SearchOptions {
    pub file_glob: Option<String>,
    pub case_insensitive: bool,
    pub max_results: Option<usize>,
}

/// Search across all project worktrees for a pattern.
///
/// For each repo, picks the main-branch worktree and runs grep/rg.
/// Results are tagged with the repo name.
pub fn search_all(
    catalog: &Catalog,
    config: &MakeworkConfig,
    pattern: &str,
    options: &SearchOptions,
) -> Vec<SearchResult> {
    let mut all_results = Vec::new();

    for (repo_name, repo) in &catalog.repos {
        let parsed_url = repo.url.as_deref().and_then(parse_remote_url);
        let wt = worktree_path(config, parsed_url.as_ref(), repo_name, &repo.main_branch);
        if !wt.exists() {
            continue;
        }

        let results = search_worktree_grep(&wt, pattern, options, repo_name);
        all_results.extend(results);
    }

    all_results
}

/// Group search results by repo name.
pub fn search_grouped(results: &[SearchResult]) -> BTreeMap<String, Vec<&SearchResult>> {
    let mut map: BTreeMap<String, Vec<&SearchResult>> = BTreeMap::new();
    for result in results {
        map.entry(result.repo_name.clone())
            .or_default()
            .push(result);
    }
    map
}

/// Search a worktree using grep (universal fallback).
fn search_worktree_grep(
    wt_path: &Path,
    pattern: &str,
    options: &SearchOptions,
    repo_name: &str,
) -> Vec<SearchResult> {
    let mut args = vec!["-rn".to_string()];

    if options.case_insensitive {
        args.push("-i".to_string());
    }
    if let Some(ref glob) = options.file_glob {
        args.push(format!("--include={glob}"));
    }

    args.push("--".to_string());
    args.push(pattern.to_string());
    args.push(wt_path.display().to_string());

    let output = Command::new("grep").args(&args).output().ok();

    let mut results = Vec::new();
    if let Some(output) = output
        && output.status.success()
    {
        let text = String::from_utf8_lossy(&output.stdout);
        for line in text.lines() {
            if let Some(result) = parse_grep_line(line, wt_path, repo_name) {
                results.push(result);
                if let Some(max) = options.max_results
                    && results.len() >= max
                {
                    break;
                }
            }
        }
    }

    results
}

/// Parse a grep output line in the format `file:line:content`.
fn parse_grep_line(line: &str, wt_path: &Path, repo_name: &str) -> Option<SearchResult> {
    // Format: /absolute/path/file:123:content
    // We need to strip the worktree path prefix to get relative file path
    let mut parts = line.splitn(3, ':');
    let file = parts.next()?;
    let line_num_str = parts.next()?;
    let content = parts.next()?;

    let line_number: u32 = line_num_str.parse().ok()?;

    let wt_prefix = format!("{}/", wt_path.display());
    let relative = if let Some(rel) = file.strip_prefix(&wt_prefix) {
        rel.to_string()
    } else {
        file.to_string()
    };

    Some(SearchResult {
        repo_name: repo_name.to_string(),
        worktree_path: wt_path.to_path_buf(),
        file_path: relative,
        line_number,
        line_content: content.to_string(),
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_grep_line_valid() {
        let wt = PathBuf::from("/home/user/worktrees/repo/main");
        let line = "/home/user/worktrees/repo/main/src/lib.rs:42:fn hello() {}";
        let result = parse_grep_line(line, &wt, "myrepo").unwrap();
        assert_eq!(result.file_path, "src/lib.rs");
        assert_eq!(result.line_number, 42);
        assert_eq!(result.line_content, "fn hello() {}");
        assert_eq!(result.repo_name, "myrepo");
    }

    #[test]
    fn parse_grep_line_invalid() {
        let wt = PathBuf::from("/tmp/wt");
        assert!(parse_grep_line("no colons here", &wt, "repo").is_none());
    }

    #[test]
    fn search_grouped_organizes_by_repo() {
        let results = vec![
            SearchResult {
                repo_name: "a".to_string(),
                worktree_path: PathBuf::from("/a"),
                file_path: "f1".to_string(),
                line_number: 1,
                line_content: "x".to_string(),
            },
            SearchResult {
                repo_name: "b".to_string(),
                worktree_path: PathBuf::from("/b"),
                file_path: "f2".to_string(),
                line_number: 2,
                line_content: "y".to_string(),
            },
            SearchResult {
                repo_name: "a".to_string(),
                worktree_path: PathBuf::from("/a"),
                file_path: "f3".to_string(),
                line_number: 3,
                line_content: "z".to_string(),
            },
            SearchResult {
                repo_name: "b".to_string(),
                worktree_path: PathBuf::from("/b"),
                file_path: "f4".to_string(),
                line_number: 4,
                line_content: "w".to_string(),
            },
        ];
        let grouped = search_grouped(&results);
        assert_eq!(grouped.len(), 2);
        assert_eq!(grouped["a"].len(), 2);
        assert_eq!(grouped["b"].len(), 2);
    }
}
