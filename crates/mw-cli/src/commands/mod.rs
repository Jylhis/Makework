use clap::{Parser, Subcommand};

fn load_config() -> mw_core::config::MakeworkConfig {
    mw_core::config::MakeworkConfig::load().unwrap_or_else(|e| {
        eprintln!("Error loading config: {e}");
        std::process::exit(1);
    })
}

fn load_catalog(config: &mw_core::config::MakeworkConfig) -> mw_core::catalog::Catalog {
    mw_core::catalog::Catalog::load(config).unwrap_or_else(|e| {
        eprintln!("Error loading catalog: {e}");
        std::process::exit(1);
    })
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
    /// Navigate to a project worktree
    Go {
        /// Repository, project, or subproject name
        project: String,
        /// Branch, tag, or commit (defaults to main branch)
        #[arg(name = "ref")]
        ref_: Option<String>,
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
        /// Output format: short or full
        #[arg(long, default_value = "short")]
        format: String,
    },
    /// Generate shell completions and wrapper
    Completions {
        /// Shell type (bash, zsh, fish)
        shell: clap_complete::Shell,
    },
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

pub fn dispatch(cli: Cli) {
    match cli.command {
        None => {
            let config = load_config();
            let catalog = load_catalog(&config);
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
            Command::Go { project, ref_ } => {
                let config = match mw_core::config::MakeworkConfig::load() {
                    Ok(c) => c,
                    Err(e) => {
                        eprintln!("Error loading config: {e}");
                        std::process::exit(1);
                    }
                };
                let catalog = match mw_core::catalog::Catalog::load(&config) {
                    Ok(c) => c,
                    Err(e) => {
                        eprintln!("Error loading catalog: {e}");
                        std::process::exit(1);
                    }
                };
                match mw_core::worktree::go(&catalog, &config, &project, ref_.as_deref()) {
                    Ok(result) => {
                        println!("{}", result.path.display());
                        if let Some(ref nix) = result.nix_activation {
                            println!("{}", nix.activation_command);
                            if let Some(ref dir) = result.nix_activation_dir {
                                println!("{}", dir.display());
                            }
                        }
                    }
                    Err(e) => {
                        eprintln!("Error: {e}");
                        std::process::exit(1);
                    }
                }
            }
            Command::New { project, ref_ } => {
                let config = load_config();
                let catalog = load_catalog(&config);
                let resolved = match catalog.find_project_unambiguous(&project) {
                    Ok(r) => r,
                    Err(e) => {
                        eprintln!("Error: {e}");
                        std::process::exit(1);
                    }
                };
                let parsed_url = resolved
                    .repo
                    .url
                    .as_deref()
                    .and_then(mw_core::repository::parse_remote_url);
                let wt_path = mw_core::worktree::worktree_path(
                    &config,
                    parsed_url.as_ref(),
                    &resolved.repo.name,
                    &ref_,
                );
                match mw_core::worktree::create_worktree(&resolved.repo.path, &ref_, &wt_path) {
                    Ok(()) => println!("{}", wt_path.display()),
                    Err(e) => {
                        eprintln!("Error: {e}");
                        std::process::exit(2);
                    }
                }
            }
            Command::Rm { target } => {
                let config = load_config();
                let catalog = load_catalog(&config);
                // Parse target as <project>/<ref> or just a path
                let (project_name, ref_name) = if let Some((p, r)) = target.split_once('/') {
                    (p.to_string(), r.to_string())
                } else {
                    eprintln!("Target must be <project>/<ref> or a worktree path");
                    std::process::exit(1);
                };
                let resolved = match catalog.find_project_unambiguous(&project_name) {
                    Ok(r) => r,
                    Err(e) => {
                        eprintln!("Error: {e}");
                        std::process::exit(1);
                    }
                };
                let parsed_url = resolved
                    .repo
                    .url
                    .as_deref()
                    .and_then(mw_core::repository::parse_remote_url);
                let wt_path = mw_core::worktree::worktree_path(
                    &config,
                    parsed_url.as_ref(),
                    &resolved.repo.name,
                    &ref_name,
                );
                match mw_core::worktree::remove_worktree(&resolved.repo.path, &wt_path) {
                    Ok(()) => println!("Removed worktree: {}", wt_path.display()),
                    Err(e) => {
                        eprintln!("Error: {e}");
                        std::process::exit(2);
                    }
                }
            }
            Command::Ls { prune } => {
                let config = load_config();
                let catalog = load_catalog(&config);
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
                let config = load_config();
                let catalog = load_catalog(&config);
                let repos: Vec<_> = if let Some(ref name) = project {
                    match catalog.find_repo(name) {
                        Some(r) => vec![r],
                        None => {
                            eprintln!("Repository not found: {name}");
                            std::process::exit(1);
                        }
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
                    // Fall back to ~/Developer if it exists
                    match mw_core::config::expand_tilde(std::path::Path::new("~/Developer")) {
                        Ok(dev) if dev.exists() => vec![dev],
                        _ => vec![],
                    }
                } else {
                    config.scan_roots.clone()
                };
                if scan_roots.is_empty() {
                    eprintln!(
                        "No scan roots configured. Set with: mw config set scan_roots ~/Developer"
                    );
                    std::process::exit(1);
                }

                // Build SyncOptions from config + CLI overrides
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
                match catalog.sync(&config, &scan_roots, &options) {
                    Ok(added) => {
                        if added.is_empty() {
                            println!("No new repositories found.");
                        } else {
                            println!("Registered {} new repo(s):", added.len());
                            for name in &added {
                                println!("  {name}");
                            }
                        }
                    }
                    Err(e) => {
                        eprintln!("Error: {e}");
                        std::process::exit(1);
                    }
                }
            }
            Command::Query {
                since,
                until,
                author,
                format,
            } => {
                let config = load_config();
                let catalog = load_catalog(&config);
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
                let summary = mw_core::query::query_activity_summary(&entries);
                for (repo_name, repo_entries) in &summary {
                    println!("{repo_name}:");
                    for e in repo_entries {
                        match format.as_str() {
                            "full" => {
                                println!("  {} {} {}", e.commit_hash, e.author, e.date);
                                println!("    {}", e.message);
                            }
                            _ => {
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
                    match mw_core::catalog::Catalog::init(&config) {
                        Ok(result) => {
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
                        Err(e) => {
                            eprintln!("Error: {e}");
                            std::process::exit(1);
                        }
                    }
                }
                CatalogAction::Add { source } => {
                    let config = match mw_core::config::MakeworkConfig::load() {
                        Ok(c) => c,
                        Err(e) => {
                            eprintln!("Error loading config: {e}");
                            std::process::exit(1);
                        }
                    };
                    let mut catalog = match mw_core::catalog::Catalog::load(&config) {
                        Ok(c) => c,
                        Err(e) => {
                            eprintln!("Error loading catalog: {e}");
                            std::process::exit(1);
                        }
                    };

                    let result = if mw_core::repository::parse_remote_url(&source).is_some() {
                        catalog.catalog_add_url(&source, &config)
                    } else {
                        let path = std::path::Path::new(&source);
                        catalog.catalog_add(path, &config)
                    };

                    match result {
                        Ok(name) => {
                            println!("Registered repository: {name}");
                        }
                        Err(e) => {
                            eprintln!("Error: {e}");
                            std::process::exit(1);
                        }
                    }
                }
                CatalogAction::List => {
                    let config = load_config();
                    let catalog = load_catalog(&config);
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
                    let config = load_config();
                    let mut catalog = load_catalog(&config);
                    // Warn about active worktrees before removing
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
                    if catalog.repos.remove(&project).is_some() {
                        if let Err(e) = catalog.save(&config) {
                            eprintln!("Error saving catalog: {e}");
                            std::process::exit(1);
                        }
                        println!("Removed catalog entry: {project}");
                    } else {
                        eprintln!("Repository not found: {project}");
                        std::process::exit(1);
                    }
                }
                CatalogAction::Edit => {
                    let config = load_config();
                    let editor = std::env::var("EDITOR").unwrap_or_else(|_| "vi".to_string());
                    let path = mw_core::catalog::Catalog::path(&config);
                    let status = std::process::Command::new(&editor).arg(&path).status();
                    match status {
                        Ok(s) if s.success() => {}
                        _ => {
                            eprintln!("Editor failed");
                            std::process::exit(1);
                        }
                    }
                }
                CatalogAction::Purge { project } => {
                    let config = load_config();
                    let mut catalog = load_catalog(&config);
                    if let Some(repo) = catalog.repos.remove(&project) {
                        // Delete bare clone
                        if repo.path.exists()
                            && let Err(e) = std::fs::remove_dir_all(&repo.path)
                        {
                            eprintln!("Warning: could not remove bare clone: {e}");
                        }
                        if let Err(e) = catalog.save(&config) {
                            eprintln!("Error saving catalog: {e}");
                            std::process::exit(1);
                        }
                        println!("Purged: {project}");
                    } else {
                        eprintln!("Repository not found: {project}");
                        std::process::exit(1);
                    }
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
                    let path = std::path::Path::new(".makework.toml");
                    if path.exists() {
                        eprintln!(".makework.toml already exists");
                        std::process::exit(1);
                    }
                    std::fs::write(path, template).unwrap_or_else(|e| {
                        eprintln!("Error: {e}");
                        std::process::exit(1);
                    });
                    println!("Created .makework.toml");
                }
                ProjectAction::Show { project } => {
                    let config = load_config();
                    let catalog = load_catalog(&config);
                    let name = project.unwrap_or_else(|| {
                        eprintln!("Please specify a project name");
                        std::process::exit(1);
                    });
                    match catalog.find_project_unambiguous(&name) {
                        Ok(resolved) => {
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
                        Err(e) => {
                            eprintln!("Error: {e}");
                            std::process::exit(1);
                        }
                    }
                }
            },
            Command::Maintenance { action } => {
                let current_dir = std::env::current_dir().unwrap_or_else(|e| {
                    eprintln!("Error: {e}");
                    std::process::exit(1);
                });
                match action {
                    MaintenanceAction::Start => {
                        match mw_core::maintenance::maintenance_register(&current_dir) {
                            Ok(()) => println!("Registered for maintenance"),
                            Err(e) => {
                                eprintln!("Error: {e}");
                                std::process::exit(1);
                            }
                        }
                    }
                    MaintenanceAction::Stop => {
                        match mw_core::maintenance::maintenance_unregister(&current_dir) {
                            Ok(()) => println!("Unregistered from maintenance"),
                            Err(e) => {
                                eprintln!("Error: {e}");
                                std::process::exit(1);
                            }
                        }
                    }
                    MaintenanceAction::Status => {
                        match mw_core::maintenance::maintenance_status(&current_dir) {
                            Ok(registered) => {
                                if registered {
                                    println!("Maintenance: registered");
                                } else {
                                    println!("Maintenance: not registered");
                                }
                            }
                            Err(e) => {
                                eprintln!("Error: {e}");
                                std::process::exit(1);
                            }
                        }
                    }
                }
            }
            Command::Config { action } => match action {
                ConfigAction::Show => {
                    let config = load_config();
                    println!("{:<20} {:<50} SOURCE", "SETTING", "VALUE");
                    let defaults =
                        mw_core::config::MakeworkConfig::defaults().unwrap_or_else(|e| {
                            eprintln!("Error: {e}");
                            std::process::exit(1);
                        });
                    let wt_source = if config.worktree_root == defaults.worktree_root {
                        "default"
                    } else {
                        "config-file"
                    };
                    let br_source = if config.bare_root == defaults.bare_root {
                        "default"
                    } else {
                        "config-file"
                    };
                    println!(
                        "{:<20} {:<50} {}",
                        "worktree_root",
                        config.worktree_root.display(),
                        wt_source
                    );
                    println!(
                        "{:<20} {:<50} {}",
                        "bare_root",
                        config.bare_root.display(),
                        br_source
                    );
                }
                ConfigAction::Set { key, value } => {
                    let mut config = load_config();
                    let path = std::path::PathBuf::from(&value);
                    let expanded = mw_core::config::expand_tilde(&path).unwrap_or(path);
                    match key.as_str() {
                        "worktree_root" => config.worktree_root = expanded,
                        "bare_root" => config.bare_root = expanded,
                        _ => {
                            eprintln!("Unknown config key: {key}");
                            std::process::exit(1);
                        }
                    }
                    config.save().unwrap_or_else(|e| {
                        eprintln!("Error saving config: {e}");
                        std::process::exit(1);
                    });
                    println!("Set {key} = {value}");
                }
                ConfigAction::Edit => {
                    let editor = std::env::var("EDITOR").unwrap_or_else(|_| "vi".to_string());
                    let path = mw_core::config::MakeworkConfig::config_path().unwrap_or_else(|e| {
                        eprintln!("Error: {e}");
                        std::process::exit(1);
                    });
                    let status = std::process::Command::new(&editor).arg(&path).status();
                    match status {
                        Ok(s) if s.success() => {}
                        _ => {
                            eprintln!("Editor failed");
                            std::process::exit(1);
                        }
                    }
                }
            },
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
fn format_git_error(e: &mw_core::repository::GitError) -> String {
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
