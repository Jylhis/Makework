use std::path::{Path, PathBuf};

use clap::{Parser, Subcommand, ValueEnum};

use mw_core::catalog::Catalog;
use mw_core::config::MakeworkConfig;
use mw_core::resolver::VisitsDb;

/// Print `Error: {msg}` to stderr and exit with status 1.
fn die(msg: impl std::fmt::Display) -> ! {
    eprintln!("Error: {msg}");
    std::process::exit(1)
}

fn load_config() -> MakeworkConfig {
    MakeworkConfig::load().unwrap_or_else(|e| die(format_args!("loading config: {e}")))
}

fn load_catalog(config: &MakeworkConfig) -> Catalog {
    Catalog::load(config).unwrap_or_else(|e| die(format_args!("loading catalog: {e}")))
}

/// Load config and catalog together — most commands need both.
fn load_state() -> (MakeworkConfig, Catalog) {
    let config = load_config();
    let catalog = load_catalog(&config);
    (config, catalog)
}

/// Path to the visits database, or `None` if the state dir cannot be located.
pub fn visits_path() -> Option<PathBuf> {
    MakeworkConfig::state_dir()
        .ok()
        .map(|d| d.join("visits.json"))
}

/// Load the frecency database; returns an empty in-memory db on any failure.
pub fn load_visits() -> VisitsDb {
    visits_path()
        .as_deref()
        .map(VisitsDb::load)
        .and_then(Result::ok)
        .unwrap_or_default()
}

/// Open `path` in `$EDITOR` (defaulting to `vi`); exit on failure.
fn edit_file(path: &Path) {
    let editor = std::env::var("EDITOR").unwrap_or_else(|_| "vi".to_string());
    match std::process::Command::new(&editor).arg(path).status() {
        Ok(s) if s.success() => {}
        _ => die("editor exited with non-zero status"),
    }
}

/// Compute the worktree path for `(repo, ref_)`, parsing the remote URL if any.
pub fn resolved_worktree_path(
    config: &MakeworkConfig,
    resolved: &mw_core::catalog::ResolvedProject<'_>,
    ref_: &str,
) -> PathBuf {
    let parsed_url = resolved
        .repo
        .url
        .as_deref()
        .and_then(mw_core::repository::parse_remote_url);
    mw_core::worktree::worktree_path(config, parsed_url.as_ref(), &resolved.repo.name, ref_)
}

/// `mw go` without an argument: list available projects on stderr and exit.
///
/// Returns `!` so call sites can use it as a fallback in `unwrap_or_else`.
fn show_picker_and_exit(catalog: &Catalog) -> ! {
    use std::io::IsTerminal;
    if !std::io::stdin().is_terminal() {
        die("provide a project name or run interactively in a terminal");
    }
    let names = catalog.all_project_names();
    if names.is_empty() {
        die("no projects registered. Run 'mw sync' or 'mw catalog add' first");
    }
    eprintln!("Available projects:");
    for name in &names {
        eprintln!("  {name}");
    }
    eprintln!("\nUsage: mw go <project>");
    std::process::exit(1)
}

/// Makework — git worktree manager
#[derive(Parser)]
#[command(name = "mw", version, about)]
pub struct Cli {
    #[command(subcommand)]
    pub command: Option<Command>,
}

#[derive(Subcommand)]
pub enum Command {
    /// Navigate to a project worktree (supports fuzzy matching)
    Go {
        /// Project name, fuzzy query, or repo@branch (interactive picker if omitted)
        project: Option<String>,
        /// Branch, tag, or commit (defaults to main branch)
        #[arg(name = "ref")]
        ref_: Option<String>,
        /// Show all matches with scores instead of navigating
        #[arg(long)]
        list: bool,
    },
    /// Create a new worktree
    New {
        /// Repository name
        project: String,
        /// Branch name
        #[arg(name = "ref")]
        ref_: String,
    },
    /// Remove a worktree
    Rm {
        /// <project>/<ref> or worktree path
        target: String,
    },
    /// List all active worktrees
    Ls {
        /// Prune orphaned worktrees whose directories no longer exist
        #[arg(long)]
        prune: bool,
    },
    /// Fetch updates for one or all repos
    Fetch {
        /// Specific project to fetch (all if omitted)
        project: Option<String>,
    },
    /// Discover and register repos
    Sync {
        /// Maximum directory depth to scan (overrides config)
        #[arg(long = "depth")]
        depth: Option<u32>,
        /// Directory name patterns to skip (repeatable, merged with config)
        #[arg(long = "exclude")]
        exclude: Vec<String>,
    },
    /// Manage the repository catalog
    Catalog {
        #[command(subcommand)]
        action: CatalogAction,
    },
    /// Per-project configuration
    Project {
        #[command(subcommand)]
        action: ProjectAction,
    },
    /// Git maintenance management
    Maintenance {
        #[command(subcommand)]
        action: MaintenanceAction,
    },
    /// Manage settings
    Config {
        #[command(subcommand)]
        action: ConfigAction,
    },
    /// Search across all project worktrees
    #[command(alias = "grep")]
    Search {
        /// Regex pattern to search for
        pattern: String,
        /// File glob filter (e.g., "*.rs")
        #[arg(long)]
        glob: Option<String>,
        /// Case-insensitive search
        #[arg(short, long)]
        ignore_case: bool,
        /// Limit results per repo
        #[arg(long)]
        max: Option<usize>,
    },
    /// Query recent activity across projects
    Query {
        /// Show commits since this date (e.g., "yesterday", "7 days ago", "2026-03-01")
        #[arg(long, default_value = "7 days ago")]
        since: String,
        /// Show commits until this date (optional)
        #[arg(long)]
        until: Option<String>,
        /// Filter by author name
        #[arg(long)]
        author: Option<String>,
        /// Output format
        #[arg(long, value_enum, default_value_t = QueryFormat::Short)]
        format: QueryFormat,
    },
    /// Start MCP (Model Context Protocol) server
    Mcp,
    /// Resolver debugging tools
    Resolver {
        #[command(subcommand)]
        action: ResolverAction,
    },
    /// Generate shell hook for visit tracking
    Init {
        /// Shell type
        #[arg(value_enum)]
        shell: HookShell,
    },
    /// Record a visit (used by shell hook, not for direct use)
    #[command(hide = true)]
    Visit {
        /// Current working directory path
        path: String,
    },
    /// Generate shell completions and wrapper
    Completions {
        /// Shell type (bash, zsh, fish)
        shell: clap_complete::Shell,
    },
}

#[derive(Debug, Clone, ValueEnum)]
pub enum QueryFormat {
    Short,
    Full,
}

#[derive(Debug, Clone, ValueEnum)]
pub enum HookShell {
    Zsh,
    Bash,
}

#[derive(Subcommand)]
pub enum CatalogAction {
    /// Initialize makework directories and config files
    Init,
    /// Register a git repository
    Add {
        /// Local path or git URL (https://, git@, ssh://)
        #[arg(name = "source")]
        source: String,
    },
    /// List catalog entries
    List,
    /// Remove a catalog entry (files stay on disk)
    Remove {
        /// Repository name
        project: String,
    },
    /// Open catalog in $EDITOR
    Edit,
    /// Delete bare clone and all worktrees
    Purge {
        /// Repository name
        project: String,
    },
}

#[derive(Subcommand)]
pub enum ProjectAction {
    /// Create .makework.toml template
    Init,
    /// Show resolved configuration
    Show {
        /// Project name (defaults to current directory)
        project: Option<String>,
    },
}

#[derive(Subcommand)]
pub enum MaintenanceAction {
    /// Register repo for git maintenance
    Start,
    /// Unregister repo from git maintenance
    Stop,
    /// Check maintenance registration
    Status,
}

#[derive(Subcommand)]
pub enum ResolverAction {
    /// Show per-signal scoring breakdown for a query
    Explain {
        /// The query to explain
        query: String,
    },
}

#[derive(Subcommand)]
pub enum ConfigAction {
    /// Display current settings
    Show,
    /// Update a config setting
    Set {
        /// Setting key
        key: String,
        /// Setting value
        value: String,
    },
    /// Open config in $EDITOR
    Edit,
}

/// Print go result: path then optional nix activation lines (for shell wrapper).
pub fn print_go_result(result: &mw_core::worktree::GoResult) {
    println!("{}", result.path.display());
    if let Some(ref nix) = result.nix_activation {
        println!("{}", nix.activation_command);
        if let Some(ref dir) = result.nix_activation_dir {
            println!("{}", dir.display());
        }
    }
}

/// Record a visit to the frecency database (best-effort, never fails the caller).
pub fn record_visit(repo: &str, branch: &str) {
    let Some(path) = visits_path() else {
        return;
    };
    let mut visits = VisitsDb::load(&path).unwrap_or_default();
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    visits.record_visit(&format!("{repo}:{branch}"), now);
    let _ = visits.save(&path);
}

/// Write repo-roots.txt cache for the shell hook fast-path.
pub fn write_repo_roots_cache(catalog: &Catalog) {
    let Ok(state_dir) = MakeworkConfig::state_dir() else {
        return;
    };
    let _ = std::fs::create_dir_all(&state_dir);
    let roots: Vec<String> = catalog
        .repos
        .values()
        .map(|r| r.path.display().to_string())
        .collect();
    let path = state_dir.join("repo-roots.txt");
    let tmp = state_dir.join("repo-roots.txt.tmp");
    if std::fs::write(&tmp, roots.join("\n")).is_ok() {
        let _ = std::fs::rename(&tmp, &path);
    }
}

pub fn dispatch(cli: Cli) {
    match cli.command {
        None => {
            let (config, catalog) = load_state();
            let statuses = mw_core::status::get_all_status(&catalog, &config);
            if statuses.is_empty() {
                println!(
                    "No repositories registered. Use `mw catalog add <path>` to register one."
                );
                return;
            }
            for (repo_name, wt_statuses) in &statuses {
                println!("{repo_name}:");
                if wt_statuses.is_empty() {
                    println!("  (no worktrees)");
                }
                for s in wt_statuses {
                    let dirty = if s.dirty_count > 0 {
                        format!(" [{}M]", s.dirty_count)
                    } else {
                        String::new()
                    };
                    let sync = if s.ahead > 0 || s.behind > 0 {
                        format!(" +{}/-{}", s.ahead, s.behind)
                    } else {
                        String::new()
                    };
                    let orphan = if s.is_orphaned { " (orphaned)" } else { "" };
                    println!(
                        "  {:<30} {}{}{} {}",
                        s.branch,
                        dirty,
                        sync,
                        orphan,
                        s.path.display()
                    );
                }
            }
        }
        Some(cmd) => match cmd {
            Command::Go {
                project,
                ref_,
                list,
            } => {
                let (config, catalog) = load_state();

                let query = project.unwrap_or_else(|| show_picker_and_exit(&catalog));
                let parsed = mw_core::resolver::parse_query(&query).unwrap_or_else(|e| die(e));

                match parsed {
                    mw_core::resolver::ParsedQuery::Explicit { repo, branch } => {
                        let result = mw_core::worktree::go(&catalog, &config, &repo, Some(&branch))
                            .unwrap_or_else(|e| die(e));
                        record_visit(&repo, &branch);
                        print_go_result(&result);
                    }
                    mw_core::resolver::ParsedQuery::Fuzzy(q) => {
                        // Backward-compatible fast path: exact catalog name match
                        // skips the fuzzy resolver entirely.
                        if !list && let Ok(resolved) = catalog.find_project_unambiguous(&q) {
                            let repo_name = resolved.repo.name.clone();
                            let main_branch = resolved.repo.main_branch.clone();
                            if let Ok(result) =
                                mw_core::worktree::go(&catalog, &config, &q, ref_.as_deref())
                            {
                                let branch = ref_.as_deref().unwrap_or(&main_branch);
                                record_visit(&repo_name, branch);
                                print_go_result(&result);
                                return;
                            }
                        }

                        let resolver_config = config.resolver.clone().unwrap_or_default();
                        let index = mw_core::resolver::ResolverIndex {
                            targets: mw_core::resolver::build_targets(&catalog),
                            visits: load_visits(),
                        };
                        let ctx = mw_core::resolver::ResolveContext::default();
                        let results =
                            mw_core::resolver::resolve(&q, &index, &resolver_config, &ctx)
                                .unwrap_or_else(|e| die(e));

                        if list {
                            for (i, t) in results.iter().enumerate().take(10) {
                                let branch = t.branch.as_deref().unwrap_or("-");
                                let project = t.project_name.as_deref().unwrap_or("");
                                eprintln!(
                                    "  {}. {:<20} {:<15} {:.3}  {}",
                                    i + 1,
                                    t.repo_name,
                                    branch,
                                    t.score,
                                    project
                                );
                            }
                            return;
                        }

                        if mw_core::resolver::needs_disambiguation(&results, 0.10) {
                            use std::io::IsTerminal;
                            if std::io::stdin().is_terminal() {
                                eprintln!("Multiple matches for '{q}':");
                                for (i, t) in results.iter().enumerate().take(5) {
                                    let branch = t.branch.as_deref().unwrap_or("-");
                                    eprintln!("  {}. {} ({})", i + 1, t.repo_name, branch);
                                }
                                eprintln!("Use repo@branch syntax for precise routing.");
                                std::process::exit(1);
                            }
                        }

                        let top = &results[0];
                        let target_branch = ref_.as_deref().or(top.branch.as_deref());
                        let result =
                            mw_core::worktree::go(&catalog, &config, &top.repo_name, target_branch)
                                .unwrap_or_else(|e| die(e));
                        record_visit(&top.repo_name, top.branch.as_deref().unwrap_or("main"));
                        print_go_result(&result);
                    }
                }
            }
            Command::New { project, ref_ } => {
                let (config, catalog) = load_state();
                let resolved = catalog
                    .find_project_unambiguous(&project)
                    .unwrap_or_else(|e| die(e));
                let wt_path = resolved_worktree_path(&config, &resolved, &ref_);
                mw_core::worktree::create_worktree(&resolved.repo.path, &ref_, &wt_path)
                    .unwrap_or_else(|e| die(e));
                println!("{}", wt_path.display());
            }
            Command::Rm { target } => {
                let (config, catalog) = load_state();
                let (project_name, ref_name) = target
                    .split_once('/')
                    .map(|(p, r)| (p.to_string(), r.to_string()))
                    .unwrap_or_else(|| die("target must be <project>/<ref> or a worktree path"));
                let resolved = catalog
                    .find_project_unambiguous(&project_name)
                    .unwrap_or_else(|e| die(e));
                let wt_path = resolved_worktree_path(&config, &resolved, &ref_name);
                mw_core::worktree::remove_worktree(&resolved.repo.path, &wt_path)
                    .unwrap_or_else(|e| die(e));
                println!("Removed worktree: {}", wt_path.display());
            }
            Command::Ls { prune } => {
                let (config, catalog) = load_state();
                if prune {
                    let results = mw_core::worktree::prune_all_worktrees(&catalog, &config);
                    if results.is_empty() {
                        println!("No orphaned worktrees found.");
                    } else {
                        for (repo_name, result) in &results {
                            match result {
                                Ok(count) => {
                                    println!("{repo_name}: pruned {count} orphaned worktree(s)")
                                }
                                Err(e) => eprintln!("{repo_name}: error pruning: {e}"),
                            }
                        }
                    }
                } else {
                    let all = mw_core::worktree::list_all_worktrees(&catalog, &config);
                    if all.is_empty() {
                        println!("No active worktrees.");
                        return;
                    }
                    for (repo_name, worktrees) in &all {
                        println!("{repo_name}:");
                        for wt in worktrees {
                            if wt.is_bare {
                                continue;
                            }
                            let branch = wt.branch.as_deref().unwrap_or("(detached)");
                            let orphan = if !wt.path.exists() { " (orphaned)" } else { "" };
                            println!("  {:<30} {}{}", branch, wt.path.display(), orphan);
                        }
                    }
                }
            }
            Command::Fetch { project } => {
                let (_config, catalog) = load_state();
                let repos: Vec<_> = if let Some(ref name) = project {
                    match catalog.find_repo(name) {
                        Some(r) => vec![r],
                        None => die(format_args!("repository not found: {name}")),
                    }
                } else {
                    catalog.repos.values().collect()
                };
                let mut had_error = false;
                for repo in &repos {
                    print!("Fetching {}... ", repo.name);
                    match mw_core::repository::fetch(&repo.path) {
                        Ok(()) => {
                            println!("done");
                            if let Ok(Some(new_branch)) =
                                mw_core::repository::check_default_branch_changed(
                                    &repo.path,
                                    &repo.main_branch,
                                )
                            {
                                eprintln!(
                                    "  Note: default branch appears to have changed from '{}' to '{new_branch}'",
                                    repo.main_branch
                                );
                            }
                        }
                        Err(e) => {
                            println!("error: {}", format_git_error(&e));
                            had_error = true;
                        }
                    }
                }
                if had_error {
                    std::process::exit(1);
                }
            }
            Command::Sync { depth, exclude } => {
                let config = load_config();
                let mut catalog = load_catalog(&config);
                let scan_roots = if config.scan_roots.is_empty() {
                    // Fall back to ~/Developer if it exists.
                    match mw_core::config::expand_tilde(Path::new("~/Developer")) {
                        Ok(dev) if dev.exists() => vec![dev],
                        _ => vec![],
                    }
                } else {
                    config.scan_roots.clone()
                };
                if scan_roots.is_empty() {
                    die("no scan roots configured. Set with: mw config set scan_roots ~/Developer");
                }

                let max_depth = depth.unwrap_or_else(|| config.sync_max_depth.unwrap_or(1));
                let mut merged_exclude = config.sync_exclude.clone();
                for pat in exclude {
                    if !merged_exclude.contains(&pat) {
                        merged_exclude.push(pat);
                    }
                }
                let options = mw_core::catalog::SyncOptions {
                    max_depth,
                    exclude: merged_exclude,
                };

                println!(
                    "Scanning: {}",
                    scan_roots
                        .iter()
                        .map(|p| p.display().to_string())
                        .collect::<Vec<_>>()
                        .join(", ")
                );
                let added = catalog
                    .sync(&config, &scan_roots, &options)
                    .unwrap_or_else(|e| die(e));
                if added.is_empty() {
                    println!("No new repositories found.");
                } else {
                    println!("Registered {} new repo(s):", added.len());
                    for name in &added {
                        println!("  {name}");
                    }
                }
                write_repo_roots_cache(&catalog);
            }
            Command::Search {
                pattern,
                glob,
                ignore_case,
                max,
            } => {
                let (config, catalog) = load_state();
                let options = mw_core::search::SearchOptions {
                    file_glob: glob,
                    case_insensitive: ignore_case,
                    max_results: max,
                };
                let results = mw_core::search::search_all(&catalog, &config, &pattern, &options);
                if results.is_empty() {
                    println!("No matches found.");
                    return;
                }
                let grouped = mw_core::search::search_grouped(&results);
                for (repo_name, repo_results) in &grouped {
                    println!("{repo_name}:");
                    for r in repo_results {
                        println!("  {}:{}:{}", r.file_path, r.line_number, r.line_content);
                    }
                }
            }
            Command::Query {
                since,
                until,
                author,
                format,
            } => {
                let (config, catalog) = load_state();
                let entries = mw_core::query::query_activity(
                    &catalog,
                    &config,
                    &since,
                    until.as_deref(),
                    author.as_deref(),
                );
                if entries.is_empty() {
                    println!("No activity found.");
                    return;
                }
                for (repo_name, repo_entries) in &mw_core::query::query_activity_summary(&entries) {
                    println!("{repo_name}:");
                    for e in repo_entries {
                        match format {
                            QueryFormat::Full => {
                                println!("  {} {} {}", e.commit_hash, e.author, e.date);
                                println!("    {}", e.message);
                            }
                            QueryFormat::Short => {
                                println!(
                                    "  {} {} {}",
                                    &e.commit_hash[..7.min(e.commit_hash.len())],
                                    e.date,
                                    e.message
                                );
                            }
                        }
                    }
                }
            }
            Command::Catalog { action } => match action {
                CatalogAction::Init => {
                    let config = load_config();
                    let result = Catalog::init(&config).unwrap_or_else(|e| die(e));
                    if result.created.is_empty() {
                        println!("Already initialized. Nothing to do.");
                    } else {
                        println!("Initialized makework:");
                        for path in &result.created {
                            println!("  created {}", path.display());
                        }
                    }
                    for path in &result.already_existed {
                        println!("  exists  {}", path.display());
                    }
                }
                CatalogAction::Add { source } => {
                    let (config, mut catalog) = load_state();
                    let (name, is_new) =
                        if mw_core::repository::parse_remote_url(&source).is_some() {
                            catalog.catalog_add_url(&source, &config)
                        } else {
                            catalog.catalog_add(Path::new(&source), &config)
                        }
                        .unwrap_or_else(|e| die(e));
                    if is_new {
                        println!("Registered repository: {name}");
                    } else {
                        println!("Already registered: {name}");
                    }
                }
                CatalogAction::List => {
                    let (_config, catalog) = load_state();
                    if catalog.repos.is_empty() {
                        println!("No repositories registered.");
                        return;
                    }
                    println!("{:<20} {:<40} {:<10} WORKTREES", "NAME", "URL", "BRANCH");
                    for (name, repo) in &catalog.repos {
                        let url = repo.url.as_deref().unwrap_or("(local)");
                        let wt_count = mw_core::worktree::list_worktrees(&repo.path)
                            .map(|w| w.iter().filter(|wt| !wt.is_bare).count())
                            .unwrap_or(0);
                        println!(
                            "{:<20} {:<40} {:<10} {}",
                            name, url, repo.main_branch, wt_count
                        );
                    }
                }
                CatalogAction::Remove { project } => {
                    let (config, mut catalog) = load_state();
                    if let Some(repo) = catalog.repos.get(&project)
                        && let Ok(worktrees) = mw_core::worktree::list_worktrees(&repo.path)
                    {
                        let active: Vec<_> = worktrees.iter().filter(|wt| !wt.is_bare).collect();
                        if !active.is_empty() {
                            eprintln!(
                                "Warning: {project} has {} active worktree(s):",
                                active.len()
                            );
                            for wt in &active {
                                let branch = wt.branch.as_deref().unwrap_or("(detached)");
                                eprintln!("  {} {}", branch, wt.path.display());
                            }
                            eprintln!("These worktrees will become orphaned.");
                        }
                    }
                    if catalog.repos.remove(&project).is_none() {
                        die(format_args!("repository not found: {project}"));
                    }
                    catalog.save(&config).unwrap_or_else(|e| die(e));
                    println!("Removed catalog entry: {project}");
                }
                CatalogAction::Edit => {
                    let config = load_config();
                    edit_file(&Catalog::path(&config));
                }
                CatalogAction::Purge { project } => {
                    let (config, mut catalog) = load_state();
                    let repo = catalog
                        .repos
                        .remove(&project)
                        .unwrap_or_else(|| die(format_args!("repository not found: {project}")));

                    // Remove worktree directories first
                    if let Some(wt_parent) =
                        mw_core::worktree::worktree_parent_dir(&config, &repo.path)
                        && wt_parent.exists()
                        && let Err(e) = std::fs::remove_dir_all(&wt_parent)
                    {
                        eprintln!(
                            "Warning: could not remove worktrees at {}: {e}",
                            wt_parent.display()
                        );
                    }

                    // Remove bare clone
                    if repo.path.exists()
                        && let Err(e) = std::fs::remove_dir_all(&repo.path)
                    {
                        eprintln!("Warning: could not remove bare clone: {e}");
                    }

                    catalog.save(&config).unwrap_or_else(|e| die(e));
                    println!("Purged: {project}");
                }
            },
            Command::Project { action } => match action {
                ProjectAction::Init => {
                    let template = r#"# main_branch = "main"
# tags = ["work"]

# [subprojects.example]
# subproject_path = "services/example"

# [subprojects.example.nix]
# type = "devenv"
# path = "services/example"
"#;
                    let path = Path::new(".makework.toml");
                    if path.exists() {
                        die(".makework.toml already exists");
                    }
                    std::fs::write(path, template).unwrap_or_else(|e| die(e));
                    println!("Created .makework.toml");
                }
                ProjectAction::Show { project } => {
                    let (_config, catalog) = load_state();
                    let name = project.unwrap_or_else(|| die("please specify a project name"));
                    let resolved = catalog
                        .find_project_unambiguous(&name)
                        .unwrap_or_else(|e| die(e));
                    println!("name: {}", resolved.repo.name);
                    println!("path: {}", resolved.repo.path.display());
                    if let Some(ref url) = resolved.repo.url {
                        println!("url: {url}");
                    }
                    println!("main_branch: {}", resolved.repo.main_branch);
                    for (rname, remote) in &resolved.repo.remotes {
                        println!("remote.{rname}: {}", remote.url);
                    }
                }
            },
            Command::Maintenance { action } => {
                let current_dir = std::env::current_dir().unwrap_or_else(|e| die(e));
                match action {
                    MaintenanceAction::Start => {
                        mw_core::maintenance::maintenance_register(&current_dir)
                            .unwrap_or_else(|e| die(e));
                        println!("Registered for maintenance");
                    }
                    MaintenanceAction::Stop => {
                        mw_core::maintenance::maintenance_unregister(&current_dir)
                            .unwrap_or_else(|e| die(e));
                        println!("Unregistered from maintenance");
                    }
                    MaintenanceAction::Status => {
                        let registered = mw_core::maintenance::maintenance_status(&current_dir)
                            .unwrap_or_else(|e| die(e));
                        println!(
                            "Maintenance: {}",
                            if registered {
                                "registered"
                            } else {
                                "not registered"
                            }
                        );
                    }
                }
            }
            Command::Config { action } => match action {
                ConfigAction::Show => {
                    let config = load_config();
                    let defaults = MakeworkConfig::defaults().unwrap_or_else(|e| die(e));
                    let source = |a, b| if a == b { "default" } else { "config-file" };
                    println!("{:<20} {:<50} SOURCE", "SETTING", "VALUE");
                    println!(
                        "{:<20} {:<50} {}",
                        "worktree_root",
                        config.worktree_root.display(),
                        source(&config.worktree_root, &defaults.worktree_root)
                    );
                    println!(
                        "{:<20} {:<50} {}",
                        "bare_root",
                        config.bare_root.display(),
                        source(&config.bare_root, &defaults.bare_root)
                    );
                }
                ConfigAction::Set { key, value } => {
                    mw_core::config::MakeworkConfig::config_set(&key, &value)
                        .unwrap_or_else(|e| die(e));
                    println!("Set {key} = {value}");
                }
                ConfigAction::Edit => {
                    let path = MakeworkConfig::config_path().unwrap_or_else(|e| die(e));
                    edit_file(&path);
                }
            },
            Command::Resolver { action } => match action {
                ResolverAction::Explain { query } => {
                    let (config, catalog) = load_state();
                    let resolver_config = config.resolver.clone().unwrap_or_default();
                    let index = mw_core::resolver::ResolverIndex {
                        targets: mw_core::resolver::build_targets(&catalog),
                        visits: load_visits(),
                    };
                    let ctx = mw_core::resolver::ResolveContext::default();
                    let results =
                        mw_core::resolver::resolve(&query, &index, &resolver_config, &ctx)
                            .unwrap_or_else(|e| die(e));

                    println!("Query: '{query}'");
                    println!(
                        "Weights: fuzzy={:.2} frecency={:.2} activity={:.2} context={:.2}",
                        resolver_config.weight_fuzzy,
                        resolver_config.weight_frecency,
                        resolver_config.weight_activity,
                        resolver_config.weight_context,
                    );
                    println!();
                    println!(
                        "{:<20} {:<12} {:>6} {:>6} {:>6} {:>6} {:>8}",
                        "REPO", "BRANCH", "FUZZY", "FREC", "ACT", "CTX", "TOTAL"
                    );
                    for t in results.iter().take(10) {
                        let branch = t.branch.as_deref().unwrap_or("-");
                        println!(
                            "{:<20} {:<12} {:>6.3} {:>6.3} {:>6.3} {:>6.3} {:>8.3}",
                            t.repo_name,
                            branch,
                            t.signal_scores.fuzzy,
                            t.signal_scores.frecency,
                            t.signal_scores.activity,
                            t.signal_scores.context,
                            t.score,
                        );
                    }
                    if mw_core::resolver::needs_disambiguation(&results, 0.10) {
                        println!("\nDisambiguation: TRIGGERED (top scores within 10%)");
                    }
                }
            },
            Command::Init { shell } => match shell {
                HookShell::Zsh => {
                    println!(
                        r#"# Add to your .zshrc:
_makework_hook() {{
  command mw visit "$PWD" 2>/dev/null &!
}}
chpwd_functions+=(_makework_hook)"#
                    );
                }
                HookShell::Bash => {
                    println!(
                        r#"# Add to your .bashrc:
_makework_hook() {{
  command mw visit "$PWD" 2>/dev/null &
}}
PROMPT_COMMAND="_makework_hook;${{PROMPT_COMMAND}}""#
                    );
                }
            },
            Command::Visit { path } => {
                // Hot path called from a shell hook on every `cd`. Failures
                // are intentionally silent so the user's shell never sees errors.
                let Ok(state_dir) = MakeworkConfig::state_dir() else {
                    return;
                };
                let Ok(roots) = std::fs::read_to_string(state_dir.join("repo-roots.txt")) else {
                    return;
                };
                let visit_path = Path::new(&path);
                let Some(root) = roots
                    .lines()
                    .find(|root| !root.is_empty() && visit_path.starts_with(root))
                else {
                    return;
                };
                let repo_name = Path::new(root)
                    .file_name()
                    .and_then(|n| n.to_str())
                    .unwrap_or("unknown");
                let branch = std::process::Command::new("git")
                    .args(["-C", &path, "rev-parse", "--abbrev-ref", "HEAD"])
                    .output()
                    .ok()
                    .filter(|o| o.status.success())
                    .and_then(|o| String::from_utf8(o.stdout).ok())
                    .map(|s| s.trim().to_string())
                    .unwrap_or_else(|| "unknown".to_string());
                record_visit(repo_name, &branch);
            }
            Command::Mcp => {
                mw_mcp::server::run_stdio();
            }
            Command::Completions { shell } => {
                use clap::CommandFactory;
                use clap_complete::generate;
                let mut cmd = Cli::command();
                let name = cmd.get_name().to_string();
                generate(shell, &mut cmd, &name, &mut std::io::stdout());

                let wrapper = match shell {
                    clap_complete::Shell::Bash | clap_complete::Shell::Zsh => {
                        r#"
mw() {
  local output
  output=$(command mw "$@") || return $?
  case "$1" in
    go)
      local path nix_cmd nix_dir
      path=$(printf '%s' "$output" | sed -n '1p')
      nix_cmd=$(printf '%s' "$output" | sed -n '2p')
      nix_dir=$(printf '%s' "$output" | sed -n '3p')
      [ -n "$path" ] && cd "$path" || return $?
      if [ -n "$nix_cmd" ]; then
        cd "${nix_dir:-$path}" && eval "$nix_cmd"
      fi
      ;;
    *) [ -n "$output" ] && echo "$output" ;;
  esac
}
"#
                    }
                    clap_complete::Shell::Fish => {
                        r#"
function mw
  set -l output (command mw $argv); or return $status
  switch $argv[1]
    case go
      set -l path (echo "$output" | sed -n '1p')
      set -l nix_cmd (echo "$output" | sed -n '2p')
      set -l nix_dir (echo "$output" | sed -n '3p')
      test -n "$path"; and cd $path; or return $status
      if test -n "$nix_cmd"
        cd (test -n "$nix_dir" && echo "$nix_dir" || echo "$path")
        eval $nix_cmd
      end
    case '*'
      test -n "$output"; and echo $output
  end
end
"#
                    }
                    _ => "",
                };
                print!("{wrapper}");
            }
        },
    }
}

/// Format a git error for display, adding hints for common issues like lock files.
pub fn format_git_error(e: &mw_core::repository::GitError) -> String {
    let msg = e.to_string();
    if msg.contains(".lock'") || msg.contains(".lock:") {
        format!(
            "{msg}\nHint: a git lock file exists. Another git process may be running, \
             or a previous process crashed. Remove the lock file to proceed."
        )
    } else {
        msg
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use mw_core::nix::DetectedNix;
    use mw_core::project::NixType;
    use mw_core::repository::{GitError, Repository};
    use mw_core::worktree::GoResult;
    use std::collections::BTreeMap;

    fn temp_repo(name: &str, url: Option<&str>) -> Repository {
        Repository {
            name: name.to_string(),
            path: PathBuf::from(format!("/tmp/{name}")),
            url: url.map(String::from),
            main_branch: "main".to_string(),
            remotes: BTreeMap::new(),
            projects: BTreeMap::new(),
        }
    }

    #[test]
    fn format_git_error_lock_file_adds_hint() {
        let err = GitError::Command {
            cmd: "git fetch".to_string(),
            stderr: "error: could not lock '/repo/.git/HEAD.lock'".to_string(),
        };
        let msg = format_git_error(&err);
        assert!(msg.contains("Hint:"), "expected lock-file hint, got: {msg}");
        assert!(msg.contains("git lock file"));
    }

    #[test]
    fn format_git_error_normal_error_no_hint() {
        let err = GitError::Command {
            cmd: "git fetch".to_string(),
            stderr: "fatal: unable to access remote".to_string(),
        };
        let msg = format_git_error(&err);
        assert!(!msg.contains("Hint:"));
        assert!(msg.contains("unable to access"));
    }

    #[test]
    fn resolved_worktree_path_with_url_uses_host_segments() {
        let repo = temp_repo("makework", Some("https://github.com/Jylhis/Makework"));
        let resolved = mw_core::catalog::ResolvedProject {
            repo: &repo,
            subproject_path: None,
            nix_config: None,
            sparse_paths: None,
        };
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/wt"),
            bare_root: PathBuf::from("/bare"),
            scan_roots: vec![],
            sync_max_depth: None,
            sync_exclude: vec![],
            template_dir: None,
            resolver: None,
        };
        let path = resolved_worktree_path(&config, &resolved, "main");
        assert!(path.starts_with("/wt"));
        assert!(path.to_string_lossy().contains("github.com"));
    }

    #[test]
    fn resolved_worktree_path_local_repo_uses_name() {
        let repo = temp_repo("local-repo", None);
        let resolved = mw_core::catalog::ResolvedProject {
            repo: &repo,
            subproject_path: None,
            nix_config: None,
            sparse_paths: None,
        };
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/wt"),
            bare_root: PathBuf::from("/bare"),
            scan_roots: vec![],
            sync_max_depth: None,
            sync_exclude: vec![],
            template_dir: None,
            resolver: None,
        };
        let path = resolved_worktree_path(&config, &resolved, "feature");
        assert!(path.to_string_lossy().contains("local-repo"));
        assert!(path.to_string_lossy().contains("feature"));
    }

    #[test]
    fn print_go_result_runs_without_panic() {
        // Output goes to stdout; we just exercise both branches.
        let plain = GoResult {
            path: PathBuf::from("/wt/main"),
            nix_activation: None,
            nix_activation_dir: None,
        };
        print_go_result(&plain);

        let with_nix = GoResult {
            path: PathBuf::from("/wt/main"),
            nix_activation: Some(DetectedNix {
                nix_type: NixType::Devenv,
                activation_command: "devenv shell".to_string(),
            }),
            nix_activation_dir: Some(PathBuf::from("/wt/main/api")),
        };
        print_go_result(&with_nix);
    }

    #[test]
    fn write_repo_roots_cache_writes_file() {
        let tmp = tempfile::tempdir().unwrap();
        let _guard = ENV_LOCK.lock().unwrap_or_else(|p| p.into_inner());
        // SAFETY: ENV_LOCK serializes env-mutating tests in this module.
        unsafe {
            std::env::set_var("XDG_STATE_HOME", tmp.path());
        }

        let mut catalog = Catalog::default();
        catalog.repos.insert(
            "demo".to_string(),
            temp_repo("demo", Some("git@github.com:demo/repo")),
        );
        write_repo_roots_cache(&catalog);

        let roots = std::fs::read_to_string(tmp.path().join("makework/repo-roots.txt")).unwrap();
        assert!(roots.contains("/tmp/demo"));

        unsafe {
            std::env::remove_var("XDG_STATE_HOME");
        }
    }

    #[test]
    fn record_visit_persists_to_visits_db() {
        let tmp = tempfile::tempdir().unwrap();
        let _guard = ENV_LOCK.lock().unwrap_or_else(|p| p.into_inner());
        unsafe {
            std::env::set_var("XDG_STATE_HOME", tmp.path());
        }

        record_visit("demo", "main");
        let visits = VisitsDb::load(&tmp.path().join("makework/visits.json")).unwrap_or_default();
        assert!(visits.entries.iter().any(|e| e.key == "demo:main"));

        unsafe {
            std::env::remove_var("XDG_STATE_HOME");
        }
    }

    #[test]
    fn load_visits_returns_default_when_no_state() {
        let tmp = tempfile::tempdir().unwrap();
        let _guard = ENV_LOCK.lock().unwrap_or_else(|p| p.into_inner());
        unsafe {
            std::env::set_var("XDG_STATE_HOME", tmp.path());
        }

        let visits = load_visits();
        assert!(visits.entries.is_empty());

        unsafe {
            std::env::remove_var("XDG_STATE_HOME");
        }
    }

    #[test]
    fn visits_path_is_under_state_dir() {
        let tmp = tempfile::tempdir().unwrap();
        let _guard = ENV_LOCK.lock().unwrap_or_else(|p| p.into_inner());
        unsafe {
            std::env::set_var("XDG_STATE_HOME", tmp.path());
        }
        let path = visits_path().unwrap();
        assert!(path.ends_with("makework/visits.json"));
        unsafe {
            std::env::remove_var("XDG_STATE_HOME");
        }
    }

    // Mutex serializing env-var-mutating tests inside this module.
    static ENV_LOCK: std::sync::Mutex<()> = std::sync::Mutex::new(());
}
