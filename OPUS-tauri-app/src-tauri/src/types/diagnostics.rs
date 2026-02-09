use serde::Serialize;

#[derive(Debug, Clone, Serialize)]
pub struct EnvSnapshot {
    pub path: String,
    pub shell: String,
    pub home: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ssh_auth_sock: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub git_ssh_command: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub git_askpass: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gh_config_dir: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gh_auth_summary: Option<String>,
}

#[derive(Debug, Clone, Serialize)]
pub struct SessiondStatus {
    pub running: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub socket_path: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_restart: Option<String>,
}
