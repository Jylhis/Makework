use std::fs;
use std::io;
use std::path::{Path, PathBuf};

use etcetera::BaseStrategy;
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
struct ConfigFile {
    config: MakeworkConfig,
}

/// Configuration for the workspace resolver scoring weights.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResolverConfig {
    #[serde(default = "ResolverConfig::default_weight_fuzzy")]
    pub weight_fuzzy: f64,
    #[serde(default = "ResolverConfig::default_weight_frecency")]
    pub weight_frecency: f64,
    #[serde(default = "ResolverConfig::default_weight_activity")]
    pub weight_activity: f64,
    #[serde(default = "ResolverConfig::default_weight_context")]
    pub weight_context: f64,
}

impl ResolverConfig {
    const DEFAULT_WEIGHT_FUZZY: f64 = 0.35;
    const DEFAULT_WEIGHT_FRECENCY: f64 = 0.35;
    const DEFAULT_WEIGHT_ACTIVITY: f64 = 0.15;
    const DEFAULT_WEIGHT_CONTEXT: f64 = 0.15;

    fn default_weight_fuzzy() -> f64 {
        Self::DEFAULT_WEIGHT_FUZZY
    }
    fn default_weight_frecency() -> f64 {
        Self::DEFAULT_WEIGHT_FRECENCY
    }
    fn default_weight_activity() -> f64 {
        Self::DEFAULT_WEIGHT_ACTIVITY
    }
    fn default_weight_context() -> f64 {
        Self::DEFAULT_WEIGHT_CONTEXT
    }
}

impl Default for ResolverConfig {
    fn default() -> Self {
        Self {
            weight_fuzzy: Self::DEFAULT_WEIGHT_FUZZY,
            weight_frecency: Self::DEFAULT_WEIGHT_FRECENCY,
            weight_activity: Self::DEFAULT_WEIGHT_ACTIVITY,
            weight_context: Self::DEFAULT_WEIGHT_CONTEXT,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct MakeworkConfig {
    pub worktree_root: PathBuf,
    pub bare_root: PathBuf,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub scan_roots: Vec<PathBuf>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sync_max_depth: Option<u32>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub sync_exclude: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub template_dir: Option<PathBuf>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub resolver: Option<ResolverConfig>,
}

#[derive(Debug)]
pub enum ConfigError {
    Io(io::Error),
    Toml(toml::de::Error),
    TomlSer(toml::ser::Error),
    HomeDirNotFound,
    UnknownKey(String),
}

impl std::fmt::Display for ConfigError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ConfigError::Io(e) => write!(f, "I/O error: {e}"),
            ConfigError::Toml(e) => write!(f, "TOML parse error: {e}"),
            ConfigError::TomlSer(e) => write!(f, "TOML serialization error: {e}"),
            ConfigError::HomeDirNotFound => write!(f, "could not determine home directory"),
            ConfigError::UnknownKey(key) => write!(
                f,
                "unknown config key: {key} (supported: worktree_root, bare_root, scan_roots, sync_max_depth, sync_exclude, template_dir)"
            ),
        }
    }
}

impl std::error::Error for ConfigError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            ConfigError::Io(e) => Some(e),
            ConfigError::Toml(e) => Some(e),
            ConfigError::TomlSer(e) => Some(e),
            ConfigError::HomeDirNotFound => None,
            ConfigError::UnknownKey(_) => None,
        }
    }
}

impl From<io::Error> for ConfigError {
    fn from(e: io::Error) -> Self {
        ConfigError::Io(e)
    }
}

impl From<toml::de::Error> for ConfigError {
    fn from(e: toml::de::Error) -> Self {
        ConfigError::Toml(e)
    }
}

impl From<toml::ser::Error> for ConfigError {
    fn from(e: toml::ser::Error) -> Self {
        ConfigError::TomlSer(e)
    }
}

fn base_strategy() -> Result<impl BaseStrategy, ConfigError> {
    etcetera::choose_base_strategy().map_err(|_| ConfigError::HomeDirNotFound)
}

pub fn expand_tilde(path: &Path) -> Result<PathBuf, ConfigError> {
    let s = path.to_string_lossy();
    if s == "~" {
        let strategy = base_strategy()?;
        Ok(strategy.home_dir().to_path_buf())
    } else if let Some(rest) = s.strip_prefix("~/") {
        let strategy = base_strategy()?;
        Ok(strategy.home_dir().join(rest))
    } else {
        Ok(path.to_path_buf())
    }
}

impl MakeworkConfig {
    pub fn defaults() -> Result<Self, ConfigError> {
        let strategy = base_strategy()?;
        let data_dir = strategy.data_dir().join("makework");
        Ok(Self {
            worktree_root: data_dir.join("worktrees"),
            bare_root: data_dir.join("repos"),
            scan_roots: Vec::new(),
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
            resolver: None,
        })
    }

    pub fn config_dir() -> Result<PathBuf, ConfigError> {
        let strategy = base_strategy()?;
        Ok(strategy.config_dir().join("makework"))
    }

    pub fn state_dir() -> Result<PathBuf, ConfigError> {
        let state_home = match std::env::var_os("XDG_STATE_HOME") {
            Some(dir) => PathBuf::from(dir),
            None => {
                let strategy = base_strategy()?;
                strategy.home_dir().join(".local/state")
            }
        };
        Ok(state_home.join("makework"))
    }

    fn config_file_path() -> Result<PathBuf, ConfigError> {
        Ok(Self::config_dir()?.join("config.toml"))
    }

    pub fn load() -> Result<Self, ConfigError> {
        let path = Self::config_file_path()?;

        if !path.exists() {
            return Self::defaults();
        }

        let contents = fs::read_to_string(&path)?;
        let file: ConfigFile = toml::from_str(&contents)?;
        let mut config = file.config;

        config.worktree_root = expand_tilde(&config.worktree_root)?;
        config.bare_root = expand_tilde(&config.bare_root)?;
        config.scan_roots = config
            .scan_roots
            .iter()
            .map(|p| expand_tilde(p))
            .collect::<Result<Vec<_>, _>>()?;
        if let Some(ref tpl) = config.template_dir {
            config.template_dir = Some(expand_tilde(tpl)?);
        }

        Ok(config)
    }

    pub fn save(&self) -> Result<(), ConfigError> {
        let path = Self::config_file_path()?;

        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }

        let file = ConfigFile {
            config: self.clone(),
        };
        let contents = toml::to_string_pretty(&file)?;
        fs::write(&path, contents)?;

        Ok(())
    }

    /// Return the path to the config file (for use by `mw config edit`).
    pub fn config_path() -> Result<PathBuf, ConfigError> {
        Self::config_file_path()
    }

    /// Return all config settings as `(key, value, source)` tuples.
    ///
    /// `source` is `"config-file"` when a config file exists and was read,
    /// otherwise `"default"`.
    pub fn config_show(&self) -> Result<Vec<(String, String, String)>, ConfigError> {
        let path = Self::config_file_path()?;
        let source = if path.exists() {
            "config-file"
        } else {
            "default"
        };

        let mut entries = vec![
            (
                "worktree_root".to_string(),
                self.worktree_root.display().to_string(),
                source.to_string(),
            ),
            (
                "bare_root".to_string(),
                self.bare_root.display().to_string(),
                source.to_string(),
            ),
        ];
        if !self.scan_roots.is_empty() {
            let roots: Vec<String> = self
                .scan_roots
                .iter()
                .map(|p| p.display().to_string())
                .collect();
            entries.push((
                "scan_roots".to_string(),
                roots.join(", "),
                source.to_string(),
            ));
        }
        Ok(entries)
    }

    /// Set a single config key and persist the config file.
    ///
    /// Supported keys: `worktree_root`, `bare_root`, `scan_roots`,
    /// `sync_max_depth`, `sync_exclude`, `template_dir`.
    pub fn config_set(key: &str, value: &str) -> Result<(), ConfigError> {
        let mut config = Self::load()?;

        match key {
            "worktree_root" => {
                config.worktree_root = expand_tilde(Path::new(value))?;
            }
            "bare_root" => {
                config.bare_root = expand_tilde(Path::new(value))?;
            }
            "scan_roots" => {
                config.scan_roots = value
                    .split(',')
                    .map(|s| expand_tilde(Path::new(s.trim())))
                    .collect::<Result<Vec<_>, _>>()?;
            }
            "sync_max_depth" => {
                config.sync_max_depth = Some(
                    value
                        .parse::<u32>()
                        .map_err(|e| ConfigError::UnknownKey(format!("invalid depth: {e}")))?,
                );
            }
            "sync_exclude" => {
                config.sync_exclude = value
                    .split(',')
                    .map(|s| s.trim().to_string())
                    .filter(|s| !s.is_empty())
                    .collect();
            }
            "template_dir" => {
                config.template_dir = Some(expand_tilde(Path::new(value))?);
            }
            _ => {
                return Err(ConfigError::UnknownKey(key.to_string()));
            }
        }

        config.save()?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn expand_tilde_no_tilde() {
        let path = Path::new("/absolute/path");
        let result = expand_tilde(path).unwrap();
        assert_eq!(result, PathBuf::from("/absolute/path"));
    }

    #[test]
    fn expand_tilde_with_prefix() {
        let path = Path::new("~/some/dir");
        let result = expand_tilde(path).unwrap();
        assert!(result.is_absolute());
        assert!(result.ends_with("some/dir"));
        assert!(!result.to_string_lossy().contains('~'));
    }

    #[test]
    fn expand_tilde_bare() {
        let path = Path::new("~");
        let result = expand_tilde(path).unwrap();
        assert!(result.is_absolute());
        assert!(!result.to_string_lossy().contains('~'));
    }

    #[test]
    fn expand_tilde_no_expand_mid_path() {
        let path = Path::new("/some/~/path");
        let result = expand_tilde(path).unwrap();
        assert_eq!(result, PathBuf::from("/some/~/path"));
    }

    #[test]
    fn defaults_produces_absolute_paths() {
        let config = MakeworkConfig::defaults().unwrap();
        assert!(config.worktree_root.is_absolute());
        assert!(config.bare_root.is_absolute());
        assert!(config.worktree_root.ends_with("makework/worktrees"));
        assert!(config.bare_root.ends_with("makework/repos"));
    }

    #[test]
    fn config_dir_is_absolute() {
        let dir = MakeworkConfig::config_dir().unwrap();
        assert!(dir.is_absolute());
        assert!(dir.ends_with("makework"));
    }

    #[test]
    fn state_dir_is_absolute() {
        let dir = MakeworkConfig::state_dir().unwrap();
        assert!(dir.is_absolute());
        assert!(dir.ends_with("makework"));
    }

    #[test]
    fn round_trip_toml() {
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/data/worktrees"),
            bare_root: PathBuf::from("/data/repos"),
            scan_roots: Vec::new(),
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
            resolver: None,
        };
        let file = ConfigFile {
            config: config.clone(),
        };
        let serialized = toml::to_string_pretty(&file).unwrap();
        let deserialized: ConfigFile = toml::from_str(&serialized).unwrap();
        assert_eq!(deserialized.config, config);
    }

    #[test]
    fn parse_toml_with_tilde() {
        let input = r#"
[config]
worktree_root = "~/.local/share/makework/worktrees"
bare_root = "~/.local/share/makework/repos"
"#;
        let file: ConfigFile = toml::from_str(input).unwrap();
        let worktree = expand_tilde(&file.config.worktree_root).unwrap();
        let bare = expand_tilde(&file.config.bare_root).unwrap();
        assert!(worktree.is_absolute());
        assert!(bare.is_absolute());
        assert!(!worktree.to_string_lossy().contains('~'));
        assert!(!bare.to_string_lossy().contains('~'));
    }

    #[test]
    fn load_returns_defaults_when_no_file() {
        let loaded = MakeworkConfig::load().unwrap();
        let defaults = MakeworkConfig::defaults().unwrap();
        assert_eq!(loaded, defaults);
    }

    #[test]
    fn config_show_returns_all_keys() {
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/data/worktrees"),
            bare_root: PathBuf::from("/data/repos"),
            scan_roots: Vec::new(),
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
            resolver: None,
        };
        let entries = config.config_show().unwrap();
        assert_eq!(entries.len(), 2);

        let keys: Vec<&str> = entries.iter().map(|(k, _, _)| k.as_str()).collect();
        assert!(keys.contains(&"worktree_root"));
        assert!(keys.contains(&"bare_root"));

        // Values should match the config
        for (key, value, _source) in &entries {
            match key.as_str() {
                "worktree_root" => assert_eq!(value, "/data/worktrees"),
                "bare_root" => assert_eq!(value, "/data/repos"),
                _ => panic!("unexpected key: {key}"),
            }
        }
    }

    #[test]
    fn config_set_unknown_key_returns_error() {
        let result = MakeworkConfig::config_set("unknown_key", "value");
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(
            err.to_string().contains("unknown config key"),
            "expected unknown key error, got: {err}"
        );
    }

    #[test]
    fn config_show_includes_scan_roots() {
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/data/worktrees"),
            bare_root: PathBuf::from("/data/repos"),
            scan_roots: vec![PathBuf::from("/home/user/projects")],
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
            resolver: None,
        };
        let entries = config.config_show().unwrap();
        assert_eq!(entries.len(), 3);
        let keys: Vec<&str> = entries.iter().map(|(k, _, _)| k.as_str()).collect();
        assert!(keys.contains(&"scan_roots"));
    }

    #[test]
    fn config_show_omits_scan_roots_when_empty() {
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/data/worktrees"),
            bare_root: PathBuf::from("/data/repos"),
            scan_roots: Vec::new(),
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
            resolver: None,
        };
        let entries = config.config_show().unwrap();
        assert_eq!(entries.len(), 2);
    }

    #[test]
    fn round_trip_toml_with_all_fields() {
        let config = MakeworkConfig {
            worktree_root: PathBuf::from("/data/worktrees"),
            bare_root: PathBuf::from("/data/repos"),
            scan_roots: vec![PathBuf::from("/home/user/dev")],
            sync_max_depth: Some(3),
            sync_exclude: vec!["node_modules".to_string(), ".cache".to_string()],
            template_dir: Some(PathBuf::from("/home/user/templates")),
            resolver: None,
        };
        let file = ConfigFile {
            config: config.clone(),
        };
        let serialized = toml::to_string_pretty(&file).unwrap();
        let deserialized: ConfigFile = toml::from_str(&serialized).unwrap();
        assert_eq!(deserialized.config, config);
        assert_eq!(deserialized.config.sync_max_depth, Some(3));
        assert_eq!(deserialized.config.sync_exclude.len(), 2);
        assert_eq!(
            deserialized.config.template_dir,
            Some(PathBuf::from("/home/user/templates"))
        );
    }

    #[test]
    fn config_error_display() {
        let err = ConfigError::HomeDirNotFound;
        assert_eq!(err.to_string(), "could not determine home directory");

        let err = ConfigError::UnknownKey("bad_key".to_string());
        assert!(err.to_string().contains("unknown config key: bad_key"));

        let io_err = ConfigError::Io(std::io::Error::new(std::io::ErrorKind::NotFound, "nope"));
        assert!(io_err.to_string().contains("I/O error"));
    }

    #[test]
    fn config_path_ends_with_config_toml() {
        let path = MakeworkConfig::config_path().unwrap();
        assert!(
            path.ends_with("config.toml"),
            "config path should end with config.toml, got: {}",
            path.display()
        );
    }
}
