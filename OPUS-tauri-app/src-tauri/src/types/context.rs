use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ActiveContext {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub active_workset_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub active_workspace: Option<String>,
}
