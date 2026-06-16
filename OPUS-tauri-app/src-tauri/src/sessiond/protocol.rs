use serde::{Deserialize, Serialize};

pub const PROTOCOL_VERSION: u32 = 2;

// ---------- Control request/response ----------

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlRequest {
    pub protocol_version: u32,
    pub method: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub params: Option<serde_json::Value>,
}

#[derive(Debug, Deserialize)]
pub struct ControlResponse {
    pub ok: bool,
    #[serde(default)]
    pub result: serde_json::Value,
    #[serde(default)]
    pub error: Option<String>,
}

// ---------- Create ----------

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CreateRequest {
    pub session_id: String,
    pub cwd: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CreateResponse {
    pub session_id: String,
    #[serde(default)]
    pub existing: bool,
}

// ---------- Attach ----------

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct AttachRequest {
    pub protocol_version: u32,
    #[serde(rename = "type")]
    pub msg_type: String,
    pub session_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stream_id: Option<String>,
    pub since: i64,
    pub with_buffer: bool,
}

// ---------- Stream message (received from sessiond) ----------

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct StreamMessage {
    #[serde(rename = "type")]
    pub msg_type: String,
    #[serde(default)]
    pub session_id: String,
    #[serde(default)]
    pub stream_id: String,
    #[serde(default)]
    pub data: Option<String>,
    #[serde(default)]
    pub len: Option<usize>,
    #[serde(default)]
    pub next_offset: Option<i64>,
    #[serde(default)]
    pub truncated: bool,
    #[serde(default)]
    pub source: Option<String>,
    #[serde(default)]
    pub snapshot_source: Option<String>,
    #[serde(default)]
    pub backlog_source: Option<String>,
    #[serde(default)]
    pub backlog_truncated: bool,
    #[serde(default)]
    pub alt_screen: bool,
    #[serde(default)]
    pub mouse_mask: u8,
    #[serde(default)]
    pub mouse: bool,
    #[serde(default)]
    pub mouse_sgr: bool,
    #[serde(default)]
    pub mouse_encoding: Option<String>,
    #[serde(default)]
    pub safe_to_replay: bool,
    #[serde(default)]
    pub initial_credit: Option<i64>,
    #[serde(default)]
    pub error: Option<String>,
}

// ---------- Send / Resize / Stop / ACK ----------

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SendRequest {
    pub session_id: String,
    pub data: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ResizeRequest {
    pub session_id: String,
    pub cols: u32,
    pub rows: u32,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct StopRequest {
    pub session_id: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct AckRequest {
    pub session_id: String,
    pub stream_id: String,
    pub bytes: i64,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BootstrapRequest {
    pub session_id: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BootstrapResponse {
    pub session_id: String,
    #[serde(default)]
    pub snapshot: Option<String>,
    #[serde(default)]
    pub backlog: Option<String>,
    #[serde(default)]
    pub next_offset: Option<i64>,
    #[serde(default)]
    pub backlog_truncated: bool,
    #[serde(default)]
    pub alt_screen: bool,
    #[serde(default)]
    pub mouse: bool,
    #[serde(default)]
    pub mouse_sgr: bool,
    #[serde(default)]
    pub mouse_encoding: Option<String>,
    #[serde(default)]
    pub safe_to_replay: bool,
    #[serde(default)]
    pub initial_credit: Option<i64>,
}
