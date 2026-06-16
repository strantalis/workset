use serde::{Deserialize, Serialize};
use super::error::ErrorEnvelope;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkspaceSummary {
    pub name: String,
    pub path: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub created_at: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_used: Option<String>,
    #[serde(default)]
    pub archived: bool,
    #[serde(default)]
    pub pinned: bool,
    #[serde(default)]
    pub pin_order: i32,
    #[serde(default)]
    pub expanded: bool,
}

#[derive(Debug, Clone, Serialize)]
pub struct WorkspaceCreateJobRef {
    pub job_id: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct WorkspaceCreateProgress {
    pub job_id: String,
    pub state: String,
    pub repos: Vec<RepoProvisionStatus>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub workspace_name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub workspace_path: Option<String>,
}

#[derive(Debug, Clone, Serialize)]
pub struct RepoProvisionStatus {
    pub name: String,
    pub state: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub step: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<ErrorEnvelope>,
}
