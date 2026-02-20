use std::process::Command;

use tauri::State;

use crate::cli::env::{capture_env_snapshot, reload_login_env};
use crate::cli::paths::{resolve_sessiond, resolve_workset_cli};
use crate::sessiond::client::SessiondClient;
use crate::sessiond::lifecycle::is_sessiond_running;
use crate::state::AppState;
use crate::types::diagnostics::{EnvSnapshot, SessiondStatus};
use crate::types::error::ErrorEnvelope;

#[tauri::command]
pub fn diagnostics_env_snapshot(
    _state: State<'_, AppState>,
) -> Result<EnvSnapshot, ErrorEnvelope> {
    Ok(capture_env_snapshot())
}

#[tauri::command]
pub fn diagnostics_reload_login_env(
    state: State<'_, AppState>,
) -> Result<EnvSnapshot, ErrorEnvelope> {
    // reload_login_env updates env vars and returns a HashMap
    let _env_map = reload_login_env()?;

    // Update CLI path if it changed
    if let Some(cli) = resolve_workset_cli(None) {
        let mut cli_path = state.cli_path.lock().unwrap();
        *cli_path = Some(cli.to_string_lossy().to_string());
    }

    if let Some(sessiond) = resolve_sessiond(None) {
        let mut sessiond_path = state.sessiond_path.lock().unwrap();
        *sessiond_path = Some(sessiond.to_string_lossy().to_string());
    }

    // Re-capture snapshot after reload
    Ok(capture_env_snapshot())
}

#[tauri::command]
pub fn diagnostics_sessiond_status(
    _state: State<'_, AppState>,
) -> Result<SessiondStatus, ErrorEnvelope> {
    let socket_path = SessiondClient::default_socket_path();
    let running = is_sessiond_running(&socket_path);

    Ok(SessiondStatus {
        running,
        version: None,
        socket_path: Some(socket_path),
        last_error: None,
        last_restart: None,
    })
}

#[tauri::command]
pub fn diagnostics_sessiond_restart(
    state: State<'_, AppState>,
) -> Result<SessiondStatus, ErrorEnvelope> {
    let sessiond_path = {
        state
            .sessiond_path
            .lock()
            .unwrap()
            .clone()
    };

    if let Some(path) = sessiond_path {
        crate::sessiond::lifecycle::start_sessiond(&path).map_err(|e| {
            ErrorEnvelope::new("diagnostics", "sessiond_restart", &e)
        })?;
    }

    let socket_path = SessiondClient::default_socket_path();
    let running = is_sessiond_running(&socket_path);

    Ok(SessiondStatus {
        running,
        version: None,
        socket_path: Some(socket_path),
        last_error: None,
        last_restart: Some(chrono::Utc::now().to_rfc3339()),
    })
}

#[tauri::command]
pub fn diagnostics_cli_status(
    state: State<'_, AppState>,
) -> Result<serde_json::Value, ErrorEnvelope> {
    match resolve_workset_cli(None) {
        Some(path) => {
            let path_str = path.to_string_lossy().to_string();

            // Get version
            let version = Command::new(&path)
                .arg("version")
                .output()
                .ok()
                .and_then(|out| String::from_utf8(out.stdout).ok())
                .map(|s| s.trim().to_string());

            let mut cli_path = state.cli_path.lock().unwrap();
            *cli_path = Some(path_str.clone());

            Ok(serde_json::json!({
                "available": true,
                "path": path_str,
                "version": version,
            }))
        }
        None => Ok(serde_json::json!({
            "available": false,
            "error": "workset CLI not found in PATH",
        })),
    }
}
