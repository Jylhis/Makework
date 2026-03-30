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
