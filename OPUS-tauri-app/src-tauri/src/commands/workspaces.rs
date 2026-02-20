use tauri::State;
use crate::cli::paths;
use crate::cli::runner;
use crate::state::AppState;
use crate::types::error::ErrorEnvelope;
use crate::types::workspace::{WorkspaceCreateJobRef, WorkspaceCreateProgress, WorkspaceSummary};

#[tauri::command]
pub fn workspaces_list(
    state: State<'_, AppState>,
    workset_id: String,
) -> Result<Vec<WorkspaceSummary>, ErrorEnvelope> {
    let cli_path = resolve_cli(&state)?;

    let all: Vec<WorkspaceSummary> = runner::run_workset_json(&cli_path, &["ls", "--json"])
        .unwrap_or_default();

    // Filter to workspaces belonging to this workset
    let profiles = state.profiles.lock().map_err(|e| {
        ErrorEnvelope::runtime("workspaces.list", format!("Lock error: {e}"))
    })?;
    let workset = profiles.list().into_iter().find(|w| w.id == workset_id);
    let workspace_ids: Vec<String> = workset
        .map(|w| w.workspace_ids)
        .unwrap_or_default();

    let filtered: Vec<WorkspaceSummary> = all
        .into_iter()
        .filter(|ws| workspace_ids.contains(&ws.name))
        .collect();
    Ok(filtered)
}

#[tauri::command]
pub fn workspaces_create(
    state: State<'_, AppState>,
    workset_id: String,
    name: String,
    _path: Option<String>,
) -> Result<WorkspaceCreateJobRef, ErrorEnvelope> {
    let cli_path = resolve_cli(&state)?;

    // Get repos from the workset profile
    let profiles = state.profiles.lock().map_err(|e| {
        ErrorEnvelope::runtime("workspaces.create", format!("Lock error: {e}"))
    })?;
    let workset = profiles
        .list()
        .into_iter()
        .find(|w| w.id == workset_id)
        .ok_or_else(|| {
            ErrorEnvelope::config("workspaces.create", format!("Workset '{workset_id}' not found"))
        })?;
    let repos = workset.repos.clone();
    drop(profiles);

    // Build CLI args: workset new <name> --repo <url> ...
    let mut args: Vec<String> = vec!["new".into(), name.clone()];
    for repo in &repos {
        args.push("--repo".into());
        args.push(repo.clone());
    }
    let arg_refs: Vec<&str> = args.iter().map(|s| s.as_str()).collect();

    runner::run_workset_command(&cli_path, &arg_refs)?;

    // Register the workspace in the workset profile
    let mut profiles = state.profiles.lock().map_err(|e| {
        ErrorEnvelope::runtime("workspaces.create", format!("Lock error: {e}"))
    })?;
    profiles.add_workspace(&workset_id, &name)?;

    let job_id = uuid::Uuid::new_v4().to_string();
    Ok(WorkspaceCreateJobRef { job_id })
}

#[tauri::command]
pub fn workspaces_create_status(
    _state: State<'_, AppState>,
    job_id: String,
) -> Result<WorkspaceCreateProgress, ErrorEnvelope> {
    Ok(WorkspaceCreateProgress {
        job_id,
        state: "succeeded".into(),
        repos: Vec::new(),
        workspace_name: None,
        workspace_path: None,
    })
}

#[tauri::command]
pub fn workspaces_delete(
    state: State<'_, AppState>,
    workset_id: String,
    workspace_name: String,
    delete: Option<bool>,
) -> Result<(), ErrorEnvelope> {
    let cli_path = resolve_cli(&state)?;
    let mut args = vec!["rm", &workspace_name];
    if delete.unwrap_or(false) {
        args.push("--delete");
    }
    runner::run_workset_command(&cli_path, &args)?;

    // Remove the workspace from the workset profile
    let mut profiles = state.profiles.lock().map_err(|e| {
        ErrorEnvelope::runtime("workspaces.delete", format!("Lock error: {e}"))
    })?;
    profiles.remove_workspace(&workset_id, &workspace_name)?;

    Ok(())
}

fn resolve_cli(state: &State<'_, AppState>) -> Result<String, ErrorEnvelope> {
    let cli = state.cli_path.lock().map_err(|e| {
        ErrorEnvelope::runtime("resolve_cli", format!("Lock error: {e}"))
    })?;

    if let Some(ref path) = *cli {
        return Ok(path.clone());
    }

    if let Some(found) = paths::resolve_workset_cli(None) {
        return Ok(found.to_string_lossy().to_string());
    }

    Err(ErrorEnvelope::config(
        "resolve_cli",
        "workset CLI not found. Install it with: go install ./cmd/workset",
    ))
}
