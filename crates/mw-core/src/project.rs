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
}
