use tauri::{AppHandle, State};

use crate::jobs::migration::{run_migration, RemoveOptions};
use crate::state::AppState;
use crate::types::error::ErrorEnvelope;
use crate::types::job::MigrationJobRef;

#[tauri::command]
pub fn migration_start(
    app: AppHandle,
    state: State<'_, AppState>,
    workset_id: String,
    repo_url: String,
    action: String,
    workspace_names: Vec<String>,
    delete_worktrees: Option<bool>,
    delete_local: Option<bool>,
) -> Result<MigrationJobRef, ErrorEnvelope> {
    let cli_path = resolve_cli(&state).map_err(|e| {
        ErrorEnvelope::new("cli", "migration.start", &e)
    })?;

    let remove_opts = if action == "remove" {
        Some(RemoveOptions {
            delete_worktrees: delete_worktrees.unwrap_or(true),
            delete_local: delete_local.unwrap_or(false),
        })
    } else {
        None
    };

    let job_id = uuid::Uuid::new_v4().to_string();

    let handle = run_migration(
        app,
        cli_path,
        job_id.clone(),
        workset_id,
        repo_url,
        action,
        workspace_names,
        remove_opts,
    );

    let mut migrations = state.migration_store.lock().unwrap();
    migrations.active_jobs.insert(job_id.clone(), handle);

    Ok(MigrationJobRef { job_id })
}

#[tauri::command]
pub fn migration_cancel(
    state: State<'_, AppState>,
    job_id: String,
) -> Result<(), ErrorEnvelope> {
    let mut migrations = state.migration_store.lock().unwrap();
    if let Some(handle) = migrations.active_jobs.remove(&job_id) {
        handle
            .cancel
            .store(true, std::sync::atomic::Ordering::Relaxed);
    }
    Ok(())
}

fn resolve_cli(state: &AppState) -> Result<String, String> {
    let cli = state.cli_path.lock().unwrap();
    if let Some(ref path) = *cli {
        return Ok(path.clone());
    }
    if let Some(found) = crate::cli::paths::resolve_workset_cli(None) {
        return Ok(found.to_string_lossy().to_string());
    }
    Err("workset CLI not found".to_string())
}
