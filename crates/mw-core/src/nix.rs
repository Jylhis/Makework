use std::path::Path;

use crate::project::{NixConfig, NixType};

#[derive(Debug, Clone)]
pub struct DetectedNix {
    pub nix_type: NixType,
    pub activation_command: String,
}

/// Detect Nix environment in a directory.
///
/// Detection order (FR-011):
/// 1. Explicit `.makework.toml` config (passed as parameter)
/// 2. `flake.nix` → NixType::Flake
/// 3. `shell.nix` → NixType::Classic
/// 4. `devenv.nix` → NixType::Devenv
/// 5. `.envrc` containing nix directives
pub fn detect_nix(dir: &Path, explicit_config: Option<&NixConfig>) -> Option<DetectedNix> {
    // 1. Explicit config takes precedence
    if let Some(config) = explicit_config
        && let Some(ref nix_type) = config.nix_type
    {
        return Some(DetectedNix {
            activation_command: activation_for_type(nix_type, config.devshell.as_deref()),
            nix_type: nix_type.clone(),
        });
    }

    // 2. flake.nix
    if dir.join("flake.nix").exists() {
        return Some(DetectedNix {
            nix_type: NixType::Flake,
            activation_command: "nix develop".to_string(),
        });
    }

    // 3. shell.nix
    if dir.join("shell.nix").exists() {
        return Some(DetectedNix {
            nix_type: NixType::Classic,
            activation_command: "nix-shell".to_string(),
        });
    }

    // 4. devenv.nix
    if dir.join("devenv.nix").exists() {
        return Some(DetectedNix {
            nix_type: NixType::Devenv,
            activation_command: "devenv shell".to_string(),
        });
    }

    // 5. .envrc with nix directives
    if let Some(detected) = check_envrc(dir) {
        return Some(detected);
    }

    None
}

fn activation_for_type(nix_type: &NixType, devshell: Option<&str>) -> String {
    match nix_type {
        NixType::Flake => match devshell {
            Some(name) => format!("nix develop .#{name}"),
            None => "nix develop".to_string(),
        },
        NixType::Classic => "nix-shell".to_string(),
        NixType::Devenv => "devenv shell".to_string(),
        NixType::Custom => "echo 'custom nix setup'".to_string(),
    }
}

fn check_envrc(dir: &Path) -> Option<DetectedNix> {
    let envrc = dir.join(".envrc");
    let content = std::fs::read_to_string(envrc).ok()?;

    for line in content.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with("use flake") || trimmed.starts_with("use_flake") {
            return Some(DetectedNix {
                nix_type: NixType::Flake,
                activation_command: "nix develop".to_string(),
            });
        }
        if trimmed.starts_with("use nix") || trimmed.starts_with("use_nix") {
            return Some(DetectedNix {
                nix_type: NixType::Classic,
                activation_command: "nix-shell".to_string(),
            });
        }
        if trimmed.starts_with("use devenv") || trimmed.starts_with("use_devenv") {
            return Some(DetectedNix {
                nix_type: NixType::Devenv,
                activation_command: "devenv shell".to_string(),
            });
        }
    }

    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detect_flake_nix() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join("flake.nix"), "{}").unwrap();
        let result = detect_nix(tmp.path(), None).unwrap();
        assert_eq!(result.nix_type, NixType::Flake);
        assert_eq!(result.activation_command, "nix develop");
    }

    #[test]
    fn detect_shell_nix() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join("shell.nix"), "{}").unwrap();
        let result = detect_nix(tmp.path(), None).unwrap();
        assert_eq!(result.nix_type, NixType::Classic);
    }

    #[test]
    fn detect_devenv_nix() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join("devenv.nix"), "{}").unwrap();
        let result = detect_nix(tmp.path(), None).unwrap();
        assert_eq!(result.nix_type, NixType::Devenv);
    }

    #[test]
    fn detect_envrc_use_flake() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join(".envrc"), "use flake\n").unwrap();
        let result = detect_nix(tmp.path(), None).unwrap();
        assert_eq!(result.nix_type, NixType::Flake);
    }

    #[test]
    fn detect_envrc_use_devenv() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join(".envrc"), "use devenv\n").unwrap();
        let result = detect_nix(tmp.path(), None).unwrap();
        assert_eq!(result.nix_type, NixType::Devenv);
    }

    #[test]
    fn explicit_config_takes_precedence() {
        let tmp = tempfile::tempdir().unwrap();
        // Both flake.nix and explicit config exist
        std::fs::write(tmp.path().join("flake.nix"), "{}").unwrap();
        let config = NixConfig {
            nix_type: Some(NixType::Devenv),
            devshell: None,
            path: None,
        };
        let result = detect_nix(tmp.path(), Some(&config)).unwrap();
        assert_eq!(result.nix_type, NixType::Devenv);
    }

    #[test]
    fn no_nix_returns_none() {
        let tmp = tempfile::tempdir().unwrap();
        assert!(detect_nix(tmp.path(), None).is_none());
    }

    #[test]
    fn explicit_config_with_devshell() {
        let tmp = tempfile::tempdir().unwrap();
        let config = NixConfig {
            nix_type: Some(NixType::Flake),
            devshell: Some("myshell".to_string()),
            path: None,
        };
        let result = detect_nix(tmp.path(), Some(&config)).unwrap();
        assert_eq!(result.nix_type, NixType::Flake);
        assert_eq!(result.activation_command, "nix develop .#myshell");
    }

    #[test]
    fn explicit_config_custom_type() {
        let tmp = tempfile::tempdir().unwrap();
        let config = NixConfig {
            nix_type: Some(NixType::Custom),
            devshell: None,
            path: None,
        };
        let result = detect_nix(tmp.path(), Some(&config)).unwrap();
        assert_eq!(result.nix_type, NixType::Custom);
        assert_eq!(result.activation_command, "echo 'custom nix setup'");
    }

    #[test]
    fn envrc_use_nix_classic() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join(".envrc"), "use nix\n").unwrap();
        let result = detect_nix(tmp.path(), None).unwrap();
        assert_eq!(result.nix_type, NixType::Classic);
        assert_eq!(result.activation_command, "nix-shell");
    }

    #[test]
    fn envrc_use_flake_underscore_variant() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join(".envrc"), "use_flake\n").unwrap();
        let result = detect_nix(tmp.path(), None).unwrap();
        assert_eq!(result.nix_type, NixType::Flake);
    }

    #[test]
    fn envrc_use_nix_underscore_variant() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join(".envrc"), "use_nix\n").unwrap();
        let result = detect_nix(tmp.path(), None).unwrap();
        assert_eq!(result.nix_type, NixType::Classic);
    }

    #[test]
    fn envrc_use_devenv_underscore_variant() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join(".envrc"), "use_devenv\n").unwrap();
        let result = detect_nix(tmp.path(), None).unwrap();
        assert_eq!(result.nix_type, NixType::Devenv);
    }

    #[test]
    fn envrc_with_comments_and_blank_lines() {
        let tmp = tempfile::tempdir().unwrap();
        let content = "# This is a comment\n\n# Another comment\nuse flake\n";
        std::fs::write(tmp.path().join(".envrc"), content).unwrap();
        let result = detect_nix(tmp.path(), None).unwrap();
        assert_eq!(result.nix_type, NixType::Flake);
    }

    #[test]
    fn envrc_without_nix_directives_returns_none() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join(".envrc"), "export FOO=bar\n").unwrap();
        assert!(detect_nix(tmp.path(), None).is_none());
    }

    #[test]
    fn explicit_config_without_type_falls_through() {
        let tmp = tempfile::tempdir().unwrap();
        // Config with no nix_type set should fall through to file detection
        let config = NixConfig {
            nix_type: None,
            devshell: None,
            path: None,
        };
        std::fs::write(tmp.path().join("shell.nix"), "{}").unwrap();
        let result = detect_nix(tmp.path(), Some(&config)).unwrap();
        assert_eq!(result.nix_type, NixType::Classic);
    }

    #[test]
    fn precedence_order_flake_over_shell() {
        let tmp = tempfile::tempdir().unwrap();
        std::fs::write(tmp.path().join("flake.nix"), "{}").unwrap();
        std::fs::write(tmp.path().join("shell.nix"), "{}").unwrap();
        let result = detect_nix(tmp.path(), None).unwrap();
        assert_eq!(result.nix_type, NixType::Flake);
    }
}
