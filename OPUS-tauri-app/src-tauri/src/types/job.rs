use serde::Serialize;
use super::error::ErrorEnvelope;

#[derive(Debug, Clone, Serialize)]
pub struct MigrationJobRef {
    pub job_id: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct MigrationProgress {
    pub job_id: String,
    pub state: String,
    pub workspaces: Vec<WorkspaceMigrationStatus>,
}

#[derive(Debug, Clone, Serialize)]
pub struct WorkspaceMigrationStatus {
    pub workspace_name: String,
    pub state: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<ErrorEnvelope>,
}
