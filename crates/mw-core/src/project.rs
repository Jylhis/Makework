use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum NixType {
    Flake,
    Classic,
    Devenv,
    Custom,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct NixConfig {
    #[serde(rename = "type", skip_serializing_if = "Option::is_none")]
    pub nix_type: Option<NixType>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub devshell: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub path: Option<String>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Project {
    pub name: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub tags: Vec<String>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub subprojects: BTreeMap<String, Subproject>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Subproject {
    pub name: String,
    pub subproject_path: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub docs: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub nix: Option<NixConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sparse_paths: Option<Vec<String>>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn nix_type_round_trip_via_config() {
        // TOML requires enum values to be embedded in a table
        let config = NixConfig {
            nix_type: Some(NixType::Flake),
            devshell: None,
            path: None,
        };
        let serialized = toml::to_string_pretty(&config).unwrap();
        assert!(serialized.contains("\"flake\""));
        let deserialized: NixConfig = toml::from_str(&serialized).unwrap();
        assert_eq!(deserialized.nix_type, Some(NixType::Flake));

        for variant in [NixType::Classic, NixType::Devenv, NixType::Custom] {
            let cfg = NixConfig {
                nix_type: Some(variant.clone()),
                devshell: None,
                path: None,
            };
            let s = toml::to_string_pretty(&cfg).unwrap();
            let d: NixConfig = toml::from_str(&s).unwrap();
            assert_eq!(d.nix_type, Some(variant));
        }
    }

    #[test]
    fn nix_config_round_trip() {
        let config = NixConfig {
            nix_type: Some(NixType::Flake),
            devshell: Some("myshell".to_string()),
            path: Some("nix/".to_string()),
        };
        let serialized = toml::to_string_pretty(&config).unwrap();
        let deserialized: NixConfig = toml::from_str(&serialized).unwrap();
        assert_eq!(deserialized.nix_type, Some(NixType::Flake));
        assert_eq!(deserialized.devshell, Some("myshell".to_string()));
        assert_eq!(deserialized.path, Some("nix/".to_string()));
    }

    #[test]
    fn nix_config_omits_none_fields() {
        let config = NixConfig {
            nix_type: None,
            devshell: None,
            path: None,
        };
        let serialized = toml::to_string_pretty(&config).unwrap();
        assert!(
            !serialized.contains("type"),
            "None nix_type should be omitted"
        );
        assert!(
            !serialized.contains("devshell"),
            "None devshell should be omitted"
        );
        assert!(!serialized.contains("path"), "None path should be omitted");
    }

    #[test]
    fn project_with_tags_round_trip() {
        let project = Project {
            name: "myproject".to_string(),
            tags: vec!["rust".to_string(), "cli".to_string()],
            subprojects: BTreeMap::new(),
        };
        let serialized = toml::to_string_pretty(&project).unwrap();
        let deserialized: Project = toml::from_str(&serialized).unwrap();
        assert_eq!(deserialized.tags, vec!["rust", "cli"]);
    }

    #[test]
    fn project_empty_tags_omitted() {
        let project = Project {
            name: "myproject".to_string(),
            tags: Vec::new(),
            subprojects: BTreeMap::new(),
        };
        let serialized = toml::to_string_pretty(&project).unwrap();
        assert!(!serialized.contains("tags"), "empty tags should be omitted");
    }

    #[test]
    fn subproject_with_nix_config() {
        let sub = Subproject {
            name: "api".to_string(),
            subproject_path: "services/api".to_string(),
            docs: Some("https://docs.example.com".to_string()),
            nix: Some(NixConfig {
                nix_type: Some(NixType::Devenv),
                devshell: None,
                path: Some("services/api".to_string()),
            }),
            sparse_paths: None,
        };
        let serialized = toml::to_string_pretty(&sub).unwrap();
        let deserialized: Subproject = toml::from_str(&serialized).unwrap();
        assert_eq!(deserialized.docs, Some("https://docs.example.com".to_string()));
        assert!(deserialized.nix.is_some());
        assert_eq!(deserialized.nix.unwrap().nix_type, Some(NixType::Devenv));
    }

    #[test]
    fn subproject_sparse_paths_roundtrip() {
        let sub = Subproject {
            name: "api".to_string(),
            subproject_path: "services/api".to_string(),
            sparse_paths: Some(vec!["src/api".to_string(), "libs/shared".to_string()]),
            ..Default::default()
        };

        let toml_str = toml::to_string_pretty(&sub).unwrap();
        let deserialized: Subproject = toml::from_str(&toml_str).unwrap();
        assert_eq!(
            deserialized.sparse_paths,
            Some(vec!["src/api".to_string(), "libs/shared".to_string()])
        );

        // With sparse_paths: None, the field should not appear in TOML
        let sub_none = Subproject {
            name: "web".to_string(),
            subproject_path: "services/web".to_string(),
            sparse_paths: None,
            ..Default::default()
        };
        let toml_none = toml::to_string_pretty(&sub_none).unwrap();
        assert!(
            !toml_none.contains("sparse_paths"),
            "sparse_paths should be absent when None"
        );
    }
}
