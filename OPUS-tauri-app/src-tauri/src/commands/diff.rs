use tauri::{AppHandle, State};

use crate::diff_engine::patch::compute_file_patch;
use crate::diff_engine::summary::compute_diff_summary;
use crate::diff_engine::watcher::start_watcher;
use crate::state::AppState;
use crate::types::diff::{DiffSummary, FilePatch};
use crate::types::error::ErrorEnvelope;

#[tauri::command]
pub fn diff_summary(
    _state: State<'_, AppState>,
    _workspace_name: String,
    _repo: String,
    repo_path: String,
) -> Result<DiffSummary, ErrorEnvelope> {
    compute_diff_summary(&repo_path).map_err(|e| {
        ErrorEnvelope::new("diff", "diff.summary", &e)
    })
}

#[tauri::command]
pub fn diff_file_patch(
    _state: State<'_, AppState>,
    repo_path: String,
    path: String,
    prev_path: Option<String>,
    status: String,
) -> Result<FilePatch, ErrorEnvelope> {
    compute_file_patch(&repo_path, &path, prev_path.as_deref(), &status).map_err(|e| {
        ErrorEnvelope::new("diff", "diff.file_patch", &e)
    })
}

#[tauri::command]
pub fn diff_watch_start(
    app: AppHandle,
    state: State<'_, AppState>,
    workspace_name: String,
    repo: String,
    repo_path: String,
) -> Result<(), ErrorEnvelope> {
    let key = format!("{}:{}", workspace_name, repo);

    let mut watchers = state.diff_watchers.lock().unwrap();
    if watchers.contains_key(&key) {
        return Ok(()); // Already watching
    }

    let handle = start_watcher(app, workspace_name, repo, repo_path);
    watchers.insert(key, handle);
    Ok(())
}

#[tauri::command]
pub fn diff_watch_stop(
    state: State<'_, AppState>,
    workspace_name: String,
    repo: String,
) -> Result<(), ErrorEnvelope> {
    let key = format!("{}:{}", workspace_name, repo);

    let mut watchers = state.diff_watchers.lock().unwrap();
    if let Some(handle) = watchers.remove(&key) {
        handle
            .cancel
            .store(true, std::sync::atomic::Ordering::Relaxed);
    }
    Ok(())
}
