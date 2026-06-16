use serde::{Deserialize, Serialize};

/// Raw JSON from `workset repo ls --json`
#[derive(Debug, Clone, Deserialize)]
pub struct RepoListEntry {
    pub name: String,
    pub local_path: String,
    #[serde(default)]
    pub managed: bool,
    pub repo_dir: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub remote: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub default_branch: Option<String>,
}

/// Enriched repo info sent to the frontend
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RepoInstance {
    pub name: String,
    pub worktree_path: String,
    pub repo_dir: String,
    #[serde(default)]
    pub missing: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub default_branch: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub default_remote: Option<String>,
}
