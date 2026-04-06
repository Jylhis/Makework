use std::fs;
use std::io;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};

use crate::catalog::{Catalog, CatalogError};
use crate::config::ResolverConfig;

// ── Data types ───────────────────────────────────────────────────

/// A resolved workspace target returned by the resolver.
#[derive(Debug, Clone)]
pub struct Target {
    pub path: PathBuf,
    pub repo_name: String,
    pub branch: Option<String>,
    pub project_name: Option<String>,
    pub score: f64,
    pub match_reason: MatchReason,
    /// Per-signal breakdown (fuzzy, frecency, activity, context).
    pub signal_scores: SignalScores,
}

/// Per-signal scoring breakdown for debugging via `mw resolver explain`.
#[derive(Debug, Clone, Default)]
pub struct SignalScores {
    pub fuzzy: f64,
    pub frecency: f64,
    pub activity: f64,
    pub context: f64,
}

/// Why this target matched the query.
#[derive(Debug, Clone, PartialEq)]
pub enum MatchReason {
    ExactName,
    FuzzyName { similarity: f64 },
    BranchMatch,
    RecentActivity,
    Frecency,
}

/// A scored visit entry (zoxide model: one entry per target, not per visit).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VisitEntry {
    pub key: String,
    pub score: f64,
    pub last_visited: u64,
}

/// The visits database persisted as JSON.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VisitsDb {
    #[serde(default)]
    pub entries: Vec<VisitEntry>,
    #[serde(default = "default_max_age")]
    pub max_age: f64,
}

fn default_max_age() -> f64 {
    10000.0
}

impl Default for VisitsDb {
    fn default() -> Self {
        Self {
            entries: Vec::new(),
            max_age: default_max_age(),
        }
    }
}

/// A scorable target derived from Catalog data.
#[derive(Debug, Clone)]
pub struct CatalogTarget {
    pub repo_name: String,
    pub branch: Option<String>,
    pub project_name: Option<String>,
    pub path: PathBuf,
}

/// Context for resolving: current working directory, current time.
#[derive(Debug, Clone)]
pub struct ResolveContext {
    pub cwd: Option<PathBuf>,
    /// Current time as unix epoch seconds. Allows injecting time in tests.
    pub now: u64,
}

impl Default for ResolveContext {
    fn default() -> Self {
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs();
        Self { cwd: None, now }
    }
}

/// The resolver's index: targets from catalog + visit history.
pub struct ResolverIndex {
    pub targets: Vec<CatalogTarget>,
    pub visits: VisitsDb,
}

/// Errors specific to the resolver.
#[derive(Debug)]
pub enum ResolverError {
    VisitsIo(io::Error),
    VisitsParseFailed(String),
    CatalogLoad(CatalogError),
    QueryTooShort,
    EmptyCatalog,
    NoMatches(String),
    InvalidAtSyntax(String),
}

impl std::fmt::Display for ResolverError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::VisitsIo(e) => write!(f, "visits I/O error: {e}"),
            Self::VisitsParseFailed(e) => write!(f, "visits.json parse error: {e}"),
            Self::CatalogLoad(e) => write!(f, "catalog load error: {e}"),
            Self::QueryTooShort => write!(f, "query too short (minimum 2 characters)"),
            Self::EmptyCatalog => {
                write!(
                    f,
                    "no projects registered. Run 'mw sync' or 'mw catalog add' first."
                )
            }
            Self::NoMatches(q) => write!(f, "no projects match '{q}'"),
            Self::InvalidAtSyntax(q) => write!(f, "invalid @-syntax in '{q}'"),
        }
    }
}

impl std::error::Error for ResolverError {}

// ── Query parsing ────────────────────────────────────────────────

/// Parsed query: either an explicit repo@branch or a fuzzy search string.
#[derive(Debug, Clone, PartialEq)]
pub enum ParsedQuery {
    /// Explicit `repo@branch` syntax. Bypasses fuzzy scoring.
    Explicit { repo: String, branch: String },
    /// Fuzzy search string.
    Fuzzy(String),
}

/// Parse a query string into structured form.
///
/// Rules:
/// - Must be at least 2 characters
/// - If contains `@`, split on the first `@` into repo and branch
/// - Empty left or right side of `@` is an error
pub fn parse_query(query: &str) -> Result<ParsedQuery, ResolverError> {
    let query = query.trim();

    if query.len() < 2 {
        return Err(ResolverError::QueryTooShort);
    }

    if let Some(at_pos) = query.find('@') {
        let repo = &query[..at_pos];
        let branch = &query[at_pos + 1..];

        if repo.is_empty() {
            return Err(ResolverError::InvalidAtSyntax(format!(
                "missing project name in '{query}'"
            )));
        }
        if branch.is_empty() {
            return Err(ResolverError::InvalidAtSyntax(format!(
                "missing branch name in '{query}'"
            )));
        }

        Ok(ParsedQuery::Explicit {
            repo: repo.to_string(),
            branch: branch.to_string(),
        })
    } else {
        Ok(ParsedQuery::Fuzzy(query.to_string()))
    }
}

// ── Visits persistence ───────────────────────────────────────────

impl VisitsDb {
    /// Load visits from a JSON file. Returns default if file doesn't exist.
    pub fn load(path: &Path) -> Result<Self, ResolverError> {
        match fs::read_to_string(path) {
            Ok(content) => serde_json::from_str(&content)
                .map_err(|e| ResolverError::VisitsParseFailed(e.to_string())),
            Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(Self::default()),
            Err(e) => Err(ResolverError::VisitsIo(e)),
        }
    }

    /// Save visits to a JSON file (atomic write: write temp, rename).
    pub fn save(&self, path: &Path) -> Result<(), ResolverError> {
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(ResolverError::VisitsIo)?;
        }
        let content = serde_json::to_string_pretty(self)
            .map_err(|e| ResolverError::VisitsIo(io::Error::other(e.to_string())))?;
        let tmp = path.with_extension("json.tmp");
        fs::write(&tmp, content).map_err(ResolverError::VisitsIo)?;
        fs::rename(&tmp, path).map_err(ResolverError::VisitsIo)?;
        Ok(())
    }

    /// Record a visit: increment score by 1.0, update timestamp, and compact if needed.
    pub fn record_visit(&mut self, key: &str, now: u64) {
        if let Some(entry) = self.entries.iter_mut().find(|e| e.key == key) {
            entry.score += 1.0;
            entry.last_visited = now;
        } else {
            self.entries.push(VisitEntry {
                key: key.to_string(),
                score: 1.0,
                last_visited: now,
            });
        }
        self.compact();
    }

    /// Compact using the zoxide algorithm:
    /// When total score exceeds max_age, divide all scores by k so total = 90% of max_age.
    /// Prune entries with score below 1.0.
    pub fn compact(&mut self) {
        let total: f64 = self.entries.iter().map(|e| e.score).sum();
        if total > self.max_age {
            let target = self.max_age * 0.9;
            let k = total / target;
            for entry in &mut self.entries {
                entry.score /= k;
            }
            self.entries.retain(|e| e.score >= 1.0);
        }
    }

    /// Compute frecency score for a key at time `now`.
    /// Returns 0.0 if no matching entry exists.
    pub fn frecency_score(&self, key: &str, now: u64) -> f64 {
        if let Some(entry) = self.entries.iter().find(|e| e.key == key) {
            let elapsed = now.saturating_sub(entry.last_visited) as f64;
            let time_weight = 1.0 / (1.0 + elapsed / 3600.0);
            entry.score * time_weight
        } else {
            0.0
        }
    }

    /// Compute frecency score for a repo prefix (sibling branches contribute at 0.5x).
    pub fn frecency_score_with_siblings(&self, key: &str, repo_prefix: &str, now: u64) -> f64 {
        let mut score = 0.0;
        for entry in &self.entries {
            if entry.key == key {
                let elapsed = now.saturating_sub(entry.last_visited) as f64;
                let time_weight = 1.0 / (1.0 + elapsed / 3600.0);
                score += entry.score * time_weight;
            } else if entry.key.starts_with(repo_prefix) {
                let elapsed = now.saturating_sub(entry.last_visited) as f64;
                let time_weight = 1.0 / (1.0 + elapsed / 3600.0);
                score += entry.score * time_weight * 0.5;
            }
        }
        score
    }
}

// ── Scoring ──────────────────────────────────────────────────────

/// Compute fuzzy match score for a query against a target.
/// Returns a value in [0.0, 1.0] with bonuses for exact prefix and substring matches.
fn fuzzy_score(query: &str, target_name: &str) -> f64 {
    if target_name.is_empty() || query.is_empty() {
        return 0.0;
    }

    let query_lower = query.to_lowercase();
    let target_lower = target_name.to_lowercase();

    // Exact match always scores 1.0 (maximum)
    if query_lower == target_lower {
        return 1.0;
    }

    let levenshtein = strsim::normalized_levenshtein(&query_lower, &target_lower);

    // Substring bonus
    let substring_bonus = if target_lower.contains(&query_lower) {
        0.2
    } else {
        0.0
    };

    // Exact prefix bonus: 1.3x multiplier on the base score
    let prefix_multiplier = if target_lower.starts_with(&query_lower) {
        1.3
    } else {
        1.0
    };

    let raw = (levenshtein + substring_bonus) * prefix_multiplier;
    // Cap at 0.95 so exact matches always win
    raw.min(0.95)
}

/// Score a single target against a query using all signals.
fn score_target(
    target: &CatalogTarget,
    query: &str,
    visits: &VisitsDb,
    config: &ResolverConfig,
    context: &ResolveContext,
) -> (f64, MatchReason, SignalScores) {
    // 1. Fuzzy matching against all name fields
    let mut best_fuzzy = 0.0f64;
    best_fuzzy = best_fuzzy.max(fuzzy_score(query, &target.repo_name));
    if let Some(ref proj) = target.project_name {
        best_fuzzy = best_fuzzy.max(fuzzy_score(query, proj));
    }
    if let Some(ref branch) = target.branch {
        best_fuzzy = best_fuzzy.max(fuzzy_score(query, branch));
    }
    // Directory basename
    if let Some(basename) = target.path.file_name().and_then(|n| n.to_str()) {
        best_fuzzy = best_fuzzy.max(fuzzy_score(query, basename));
    }

    // 2. Frecency
    let visit_key = match &target.branch {
        Some(b) => format!("{}:{}", target.repo_name, b),
        None => format!("{}:", target.repo_name),
    };
    let repo_prefix = format!("{}:", target.repo_name);
    let frecency = visits.frecency_score_with_siblings(&visit_key, &repo_prefix, context.now);

    // 3. Activity (placeholder: 0.0 until sync-time cache is implemented)
    let activity = 0.0;

    // 4. Context affinity
    let context_score = match &context.cwd {
        Some(cwd) if cwd.starts_with(&target.path) || target.path.starts_with(cwd) => 0.5,
        _ => 0.0,
    };

    let signals = SignalScores {
        fuzzy: best_fuzzy,
        frecency,
        activity,
        context: context_score,
    };

    let total = best_fuzzy * config.weight_fuzzy
        + frecency * config.weight_frecency
        + activity * config.weight_activity
        + context_score * config.weight_context;

    // Determine dominant signal for MatchReason
    let weighted_signals = [
        (
            best_fuzzy * config.weight_fuzzy,
            MatchReason::FuzzyName {
                similarity: best_fuzzy,
            },
        ),
        (frecency * config.weight_frecency, MatchReason::Frecency),
        (
            activity * config.weight_activity,
            MatchReason::RecentActivity,
        ),
    ];
    let reason = weighted_signals
        .into_iter()
        .max_by(|a, b| a.0.partial_cmp(&b.0).unwrap_or(std::cmp::Ordering::Equal))
        .map(|(_, r)| r)
        .unwrap_or(MatchReason::FuzzyName {
            similarity: best_fuzzy,
        });

    (total, reason, signals)
}

// ── Main resolve function ────────────────────────────────────────

/// Resolve a fuzzy query against the index and return ranked targets.
///
/// Returns targets sorted by score descending. The caller decides whether
/// to disambiguate (CLI) or return the full list (MCP).
pub fn resolve(
    query: &str,
    index: &ResolverIndex,
    config: &ResolverConfig,
    context: &ResolveContext,
) -> Result<Vec<Target>, ResolverError> {
    if index.targets.is_empty() {
        return Err(ResolverError::EmptyCatalog);
    }

    if query.trim().len() < 2 {
        return Err(ResolverError::QueryTooShort);
    }

    let mut results: Vec<Target> = index
        .targets
        .iter()
        .map(|t| {
            let (score, reason, signals) = score_target(t, query, &index.visits, config, context);
            Target {
                path: t.path.clone(),
                repo_name: t.repo_name.clone(),
                branch: t.branch.clone(),
                project_name: t.project_name.clone(),
                score,
                match_reason: reason,
                signal_scores: signals,
            }
        })
        .filter(|t| t.score > 0.0)
        .collect();

    results.sort_by(|a, b| {
        b.score
            .partial_cmp(&a.score)
            .unwrap_or(std::cmp::Ordering::Equal)
    });

    if results.is_empty() {
        return Err(ResolverError::NoMatches(query.to_string()));
    }

    Ok(results)
}

/// Check if the top two results are within the disambiguation threshold.
/// Returns true if disambiguation UI should be shown.
pub fn needs_disambiguation(results: &[Target], threshold: f64) -> bool {
    if results.len() < 2 {
        return false;
    }
    let top = results[0].score;
    if top == 0.0 {
        return false;
    }
    let second = results[1].score;
    (top - second) / top < threshold
}

/// Build a CatalogTarget list from a Catalog.
pub fn build_targets(catalog: &Catalog) -> Vec<CatalogTarget> {
    let mut targets = Vec::new();
    for (repo_name, repo) in &catalog.repos {
        // Main repo entry
        targets.push(CatalogTarget {
            repo_name: repo_name.clone(),
            branch: Some(repo.main_branch.clone()),
            project_name: None,
            path: repo.path.clone(),
        });
        // Project and subproject entries
        for (proj_name, project) in &repo.projects {
            if proj_name != repo_name {
                targets.push(CatalogTarget {
                    repo_name: repo_name.clone(),
                    branch: Some(repo.main_branch.clone()),
                    project_name: Some(proj_name.clone()),
                    path: repo.path.clone(),
                });
            }
            for (sub_name, sub) in &project.subprojects {
                targets.push(CatalogTarget {
                    repo_name: repo_name.clone(),
                    branch: Some(repo.main_branch.clone()),
                    project_name: Some(sub_name.clone()),
                    path: repo.path.join(&sub.subproject_path),
                });
            }
        }
    }
    targets
}

// ── Tests ────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use std::collections::BTreeMap;

    use super::*;

    fn make_target(repo: &str, branch: &str, path: &str) -> CatalogTarget {
        CatalogTarget {
            repo_name: repo.to_string(),
            branch: Some(branch.to_string()),
            project_name: None,
            path: PathBuf::from(path),
        }
    }

    fn make_target_with_project(
        repo: &str,
        branch: &str,
        project: &str,
        path: &str,
    ) -> CatalogTarget {
        CatalogTarget {
            repo_name: repo.to_string(),
            branch: Some(branch.to_string()),
            project_name: Some(project.to_string()),
            path: PathBuf::from(path),
        }
    }

    fn default_context(now: u64) -> ResolveContext {
        ResolveContext { cwd: None, now }
    }

    // ── parse_query tests ────────────────────────────────────────

    #[test]
    fn parse_query_valid_at_syntax() {
        let result = parse_query("backend@feature-auth").unwrap();
        assert_eq!(
            result,
            ParsedQuery::Explicit {
                repo: "backend".to_string(),
                branch: "feature-auth".to_string()
            }
        );
    }

    #[test]
    fn parse_query_fuzzy() {
        let result = parse_query("auth").unwrap();
        assert_eq!(result, ParsedQuery::Fuzzy("auth".to_string()));
    }

    #[test]
    fn parse_query_empty_left_at() {
        let result = parse_query("@main");
        assert!(matches!(result, Err(ResolverError::InvalidAtSyntax(_))));
    }

    #[test]
    fn parse_query_empty_right_at() {
        let result = parse_query("repo@");
        assert!(matches!(result, Err(ResolverError::InvalidAtSyntax(_))));
    }

    #[test]
    fn parse_query_too_short() {
        let result = parse_query("a");
        assert!(matches!(result, Err(ResolverError::QueryTooShort)));
    }

    #[test]
    fn parse_query_whitespace_only() {
        let result = parse_query("   ");
        assert!(matches!(result, Err(ResolverError::QueryTooShort)));
    }

    #[test]
    fn parse_query_multiple_at_splits_on_first() {
        let result = parse_query("a@b@c").unwrap();
        assert_eq!(
            result,
            ParsedQuery::Explicit {
                repo: "a".to_string(),
                branch: "b@c".to_string()
            }
        );
    }

    // ── fuzzy_score tests ────────────────────────────────────────

    #[test]
    fn fuzzy_exact_match_scores_one() {
        assert_eq!(fuzzy_score("backend", "backend"), 1.0);
    }

    #[test]
    fn fuzzy_exact_match_case_insensitive() {
        assert_eq!(fuzzy_score("Backend", "backend"), 1.0);
    }

    #[test]
    fn fuzzy_prefix_match_scores_high() {
        let prefix = fuzzy_score("back", "backend");
        let non_prefix = fuzzy_score("end", "backend");
        assert!(
            prefix > non_prefix,
            "prefix {prefix} should beat non-prefix {non_prefix}"
        );
    }

    #[test]
    fn fuzzy_substring_match_scores_bonus() {
        let substring = fuzzy_score("end", "backend");
        let no_match = fuzzy_score("xyz", "backend");
        assert!(
            substring > no_match,
            "substring {substring} should beat no-match {no_match}"
        );
    }

    #[test]
    fn fuzzy_empty_inputs() {
        assert_eq!(fuzzy_score("", "backend"), 0.0);
        assert_eq!(fuzzy_score("test", ""), 0.0);
        assert_eq!(fuzzy_score("", ""), 0.0);
    }

    // ── VisitsDb tests ───────────────────────────────────────────

    #[test]
    fn visits_record_new_entry() {
        let mut db = VisitsDb::default();
        db.record_visit("myrepo:main", 1000);
        assert_eq!(db.entries.len(), 1);
        assert_eq!(db.entries[0].score, 1.0);
        assert_eq!(db.entries[0].last_visited, 1000);
    }

    #[test]
    fn visits_record_increment_existing() {
        let mut db = VisitsDb::default();
        db.record_visit("myrepo:main", 1000);
        db.record_visit("myrepo:main", 2000);
        assert_eq!(db.entries.len(), 1);
        assert_eq!(db.entries[0].score, 2.0);
        assert_eq!(db.entries[0].last_visited, 2000);
    }

    #[test]
    fn visits_frecency_recent_beats_old() {
        let mut db = VisitsDb::default();
        db.entries.push(VisitEntry {
            key: "recent:main".to_string(),
            score: 5.0,
            last_visited: 9000,
        });
        db.entries.push(VisitEntry {
            key: "old:main".to_string(),
            score: 5.0,
            last_visited: 1000,
        });

        let now = 10000;
        let recent = db.frecency_score("recent:main", now);
        let old = db.frecency_score("old:main", now);
        assert!(
            recent > old,
            "recent {recent} should beat old {old} at same score"
        );
    }

    #[test]
    fn visits_frecency_high_score_beats_low() {
        let mut db = VisitsDb::default();
        db.entries.push(VisitEntry {
            key: "high:main".to_string(),
            score: 50.0,
            last_visited: 5000,
        });
        db.entries.push(VisitEntry {
            key: "low:main".to_string(),
            score: 2.0,
            last_visited: 5000,
        });

        let now = 10000;
        let high = db.frecency_score("high:main", now);
        let low = db.frecency_score("low:main", now);
        assert!(high > low, "high score {high} should beat low {low}");
    }

    #[test]
    fn visits_frecency_missing_key_returns_zero() {
        let db = VisitsDb::default();
        assert_eq!(db.frecency_score("nonexistent:main", 10000), 0.0);
    }

    #[test]
    fn visits_sibling_branch_contributes_half() {
        let mut db = VisitsDb::default();
        db.entries.push(VisitEntry {
            key: "repo:feature-a".to_string(),
            score: 10.0,
            last_visited: 9000,
        });

        let now = 10000;
        // Exact match for the sibling's key
        let exact = db.frecency_score_with_siblings("repo:feature-a", "repo:", now);
        // Query for a different branch in the same repo (no exact, only sibling)
        let sibling = db.frecency_score_with_siblings("repo:main", "repo:", now);

        assert!(
            exact > sibling,
            "exact {exact} should beat sibling {sibling}"
        );
        assert!(sibling > 0.0, "sibling should contribute > 0");
    }

    #[test]
    fn visits_compaction_divides_scores() {
        let mut db = VisitsDb {
            entries: vec![
                VisitEntry {
                    key: "a:main".to_string(),
                    score: 6000.0,
                    last_visited: 0,
                },
                VisitEntry {
                    key: "b:main".to_string(),
                    score: 5000.0,
                    last_visited: 0,
                },
            ],
            max_age: 10000.0,
        };
        // Total = 11000 > max_age 10000
        db.compact();
        let total: f64 = db.entries.iter().map(|e| e.score).sum();
        let target = 10000.0 * 0.9;
        assert!(
            (total - target).abs() < 1.0,
            "total {total} should be ~{target}"
        );
    }

    #[test]
    fn visits_compaction_prunes_below_one() {
        let mut db = VisitsDb {
            entries: vec![
                VisitEntry {
                    key: "big:main".to_string(),
                    score: 10000.0,
                    last_visited: 0,
                },
                VisitEntry {
                    key: "tiny:main".to_string(),
                    score: 0.5,
                    last_visited: 0,
                },
            ],
            max_age: 10000.0,
        };
        db.compact();
        // tiny entry should be pruned (0.5 / k < 1.0)
        assert!(
            !db.entries.iter().any(|e| e.key == "tiny:main"),
            "tiny entry should be pruned"
        );
    }

    #[test]
    fn visits_compaction_noop_under_threshold() {
        let mut db = VisitsDb {
            entries: vec![VisitEntry {
                key: "a:main".to_string(),
                score: 5.0,
                last_visited: 0,
            }],
            max_age: 10000.0,
        };
        db.compact();
        assert_eq!(db.entries[0].score, 5.0, "should not compact under max_age");
    }

    #[test]
    fn visits_roundtrip_json() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("visits.json");

        let mut db = VisitsDb::default();
        db.record_visit("repo:main", 1000);
        db.record_visit("repo:feature", 2000);
        db.save(&path).unwrap();

        let loaded = VisitsDb::load(&path).unwrap();
        assert_eq!(loaded.entries.len(), 2);
        assert_eq!(loaded.entries[0].key, "repo:main");
        assert_eq!(loaded.entries[0].score, 1.0);
    }

    #[test]
    fn visits_load_missing_file_returns_default() {
        let path = PathBuf::from("/nonexistent/visits.json");
        let db = VisitsDb::load(&path).unwrap();
        assert!(db.entries.is_empty());
    }

    #[test]
    fn visits_load_corrupted_returns_error() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("visits.json");
        fs::write(&path, "not valid json {{{").unwrap();
        let result = VisitsDb::load(&path);
        assert!(matches!(result, Err(ResolverError::VisitsParseFailed(_))));
    }

    // ── resolve() tests ──────────────────────────────────────────

    #[test]
    fn resolve_exact_match_scores_highest() {
        let index = ResolverIndex {
            targets: vec![
                make_target("backend", "main", "/repos/backend"),
                make_target("backend-api", "main", "/repos/backend-api"),
                make_target("frontend", "main", "/repos/frontend"),
            ],
            visits: VisitsDb::default(),
        };

        let config = ResolverConfig::default();
        let ctx = default_context(10000);
        let results = resolve("backend", &index, &config, &ctx).unwrap();

        assert_eq!(
            results[0].repo_name, "backend",
            "exact match should be first"
        );
        assert!(
            results[0].score > results[1].score,
            "exact match score {} should beat partial {}",
            results[0].score,
            results[1].score
        );
    }

    #[test]
    fn resolve_frecency_boosts_visited_target() {
        let mut visits = VisitsDb::default();
        visits.entries.push(VisitEntry {
            key: "frontend:main".to_string(),
            score: 50.0,
            last_visited: 9900,
        });

        let index = ResolverIndex {
            targets: vec![
                make_target("backend", "main", "/repos/backend"),
                make_target("frontend", "main", "/repos/frontend"),
            ],
            visits,
        };

        // "end" matches both "backend" (substring) and "frontend" (substring)
        // but frontend has high frecency
        let config = ResolverConfig::default();
        let ctx = default_context(10000);
        let results = resolve("end", &index, &config, &ctx).unwrap();

        assert_eq!(
            results[0].repo_name, "frontend",
            "frecency should boost frontend to top"
        );
    }

    #[test]
    fn resolve_empty_catalog_errors() {
        let index = ResolverIndex {
            targets: vec![],
            visits: VisitsDb::default(),
        };
        let config = ResolverConfig::default();
        let ctx = default_context(10000);
        let result = resolve("test", &index, &config, &ctx);
        assert!(matches!(result, Err(ResolverError::EmptyCatalog)));
    }

    #[test]
    fn resolve_no_matches_errors() {
        let index = ResolverIndex {
            targets: vec![make_target("backend", "main", "/repos/backend")],
            visits: VisitsDb::default(),
        };
        let config = ResolverConfig::default();
        let ctx = default_context(10000);
        // "zzz" is very dissimilar from "backend" and should score 0
        let result = resolve("zzzzzzzzz", &index, &config, &ctx);
        assert!(matches!(result, Err(ResolverError::NoMatches(_))));
    }

    #[test]
    fn resolve_query_too_short_errors() {
        let index = ResolverIndex {
            targets: vec![make_target("backend", "main", "/repos/backend")],
            visits: VisitsDb::default(),
        };
        let config = ResolverConfig::default();
        let ctx = default_context(10000);
        let result = resolve("a", &index, &config, &ctx);
        assert!(matches!(result, Err(ResolverError::QueryTooShort)));
    }

    #[test]
    fn resolve_cold_start_fuzzy_only() {
        let index = ResolverIndex {
            targets: vec![
                make_target("auth-service", "main", "/repos/auth-service"),
                make_target("billing-service", "main", "/repos/billing-service"),
            ],
            visits: VisitsDb::default(),
        };

        let config = ResolverConfig::default();
        let ctx = default_context(10000);
        let results = resolve("auth", &index, &config, &ctx).unwrap();

        assert_eq!(
            results[0].repo_name, "auth-service",
            "fuzzy should pick auth-service for 'auth'"
        );
    }

    #[test]
    fn resolve_project_name_matches() {
        let index = ResolverIndex {
            targets: vec![make_target_with_project(
                "monorepo",
                "main",
                "auth-module",
                "/repos/monorepo",
            )],
            visits: VisitsDb::default(),
        };

        let config = ResolverConfig::default();
        let ctx = default_context(10000);
        let results = resolve("auth", &index, &config, &ctx).unwrap();
        assert!(!results.is_empty(), "should match on project name");
    }

    // ── disambiguation tests ─────────────────────────────────────

    #[test]
    fn disambiguation_triggered_when_close() {
        let results = vec![
            Target {
                path: PathBuf::from("/a"),
                repo_name: "a".to_string(),
                branch: None,
                project_name: None,
                score: 0.50,
                match_reason: MatchReason::FuzzyName { similarity: 0.5 },
                signal_scores: SignalScores::default(),
            },
            Target {
                path: PathBuf::from("/b"),
                repo_name: "b".to_string(),
                branch: None,
                project_name: None,
                score: 0.48,
                match_reason: MatchReason::FuzzyName { similarity: 0.48 },
                signal_scores: SignalScores::default(),
            },
        ];
        assert!(needs_disambiguation(&results, 0.10));
    }

    #[test]
    fn disambiguation_not_triggered_when_clear_winner() {
        let results = vec![
            Target {
                path: PathBuf::from("/a"),
                repo_name: "a".to_string(),
                branch: None,
                project_name: None,
                score: 0.90,
                match_reason: MatchReason::FuzzyName { similarity: 0.9 },
                signal_scores: SignalScores::default(),
            },
            Target {
                path: PathBuf::from("/b"),
                repo_name: "b".to_string(),
                branch: None,
                project_name: None,
                score: 0.30,
                match_reason: MatchReason::FuzzyName { similarity: 0.3 },
                signal_scores: SignalScores::default(),
            },
        ];
        assert!(!needs_disambiguation(&results, 0.10));
    }

    #[test]
    fn disambiguation_single_result() {
        let results = vec![Target {
            path: PathBuf::from("/a"),
            repo_name: "a".to_string(),
            branch: None,
            project_name: None,
            score: 0.90,
            match_reason: MatchReason::FuzzyName { similarity: 0.9 },
            signal_scores: SignalScores::default(),
        }];
        assert!(!needs_disambiguation(&results, 0.10));
    }

    #[test]
    fn disambiguation_zero_score_guard() {
        let results = vec![
            Target {
                path: PathBuf::from("/a"),
                repo_name: "a".to_string(),
                branch: None,
                project_name: None,
                score: 0.0,
                match_reason: MatchReason::FuzzyName { similarity: 0.0 },
                signal_scores: SignalScores::default(),
            },
            Target {
                path: PathBuf::from("/b"),
                repo_name: "b".to_string(),
                branch: None,
                project_name: None,
                score: 0.0,
                match_reason: MatchReason::FuzzyName { similarity: 0.0 },
                signal_scores: SignalScores::default(),
            },
        ];
        // Should not panic on divide-by-zero
        assert!(!needs_disambiguation(&results, 0.10));
    }

    // ── build_targets tests ──────────────────────────────────────

    #[test]
    fn build_targets_from_catalog() {
        let mut catalog = Catalog::default();
        catalog.repos.insert(
            "backend".to_string(),
            crate::repository::Repository {
                name: "backend".to_string(),
                path: PathBuf::from("/repos/backend"),
                url: None,
                main_branch: "main".to_string(),
                remotes: BTreeMap::new(),
                projects: BTreeMap::new(),
            },
        );
        catalog.repos.insert(
            "frontend".to_string(),
            crate::repository::Repository {
                name: "frontend".to_string(),
                path: PathBuf::from("/repos/frontend"),
                url: None,
                main_branch: "main".to_string(),
                remotes: BTreeMap::new(),
                projects: BTreeMap::new(),
            },
        );

        let targets = build_targets(&catalog);
        assert_eq!(targets.len(), 2);
        let names: Vec<&str> = targets.iter().map(|t| t.repo_name.as_str()).collect();
        assert!(names.contains(&"backend"));
        assert!(names.contains(&"frontend"));
    }

    // ── ResolverConfig tests ─────────────────────────────────────

    #[test]
    fn resolver_config_defaults() {
        let config = ResolverConfig::default();
        let total = config.weight_fuzzy
            + config.weight_frecency
            + config.weight_activity
            + config.weight_context;
        assert!(
            (total - 1.0).abs() < 0.001,
            "weights should sum to 1.0, got {total}"
        );
    }
}
