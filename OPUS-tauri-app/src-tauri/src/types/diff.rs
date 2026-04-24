use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiffSummary {
    pub files: Vec<DiffFileSummary>,
    pub total_added: u32,
    pub total_removed: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiffFileSummary {
    pub path: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prev_path: Option<String>,
    pub added: u32,
    pub removed: u32,
    pub status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub binary: Option<bool>,
}

#[derive(Debug, Clone, Serialize)]
pub struct FilePatch {
    pub patch: String,
    pub truncated: bool,
    pub total_bytes: u64,
    pub total_lines: u32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub binary: Option<bool>,
}
