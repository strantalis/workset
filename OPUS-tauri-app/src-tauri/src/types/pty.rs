use serde::Serialize;

#[derive(Debug, Clone, Serialize)]
pub struct PtyCreateResult {
    pub terminal_id: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct BootstrapPayload {
    pub workspace_name: String,
    pub terminal_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub snapshot: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub backlog: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub backlog_truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub next_offset: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub alt_screen: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mouse: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mouse_sgr: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub safe_to_replay: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub initial_credit: Option<u64>,
}
