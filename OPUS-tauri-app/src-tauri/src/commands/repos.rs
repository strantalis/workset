use tauri::State;
use crate::cli::{paths, runner};
use crate::state::AppState;
use crate::types::error::ErrorEnvelope;
use crate::types::repo::{RepoInstance, RepoListEntry};
use crate::types::workspace::WorkspaceSummary;

#[tauri::command]
pub fn workspace_repos_list(
    state: State<'_, AppState>,
    workspace_name: String,
) -> Result<Vec<RepoInstance>, ErrorEnvelope> {
    let cli_path = resolve_cli(&state)?;

    // Get workspace path from `workset ls --json`
    let all_ws: Vec<WorkspaceSummary> = runner::run_workset_json(
        &cli_path,
        &["ls", "--json"],
    )
    .unwrap_or_default();
    let ws_path = all_ws
        .iter()
        .find(|w| w.name == workspace_name)
        .map(|w| w.path.clone())
        .unwrap_or_default();

    // Get repo list from `workset repo ls -w <name> --json`
    let entries: Vec<RepoListEntry> = runner::run_workset_json(
        &cli_path,
        &["repo", "ls", "-w", &workspace_name, "--json"],
    )
    .unwrap_or_default();

    // Convert to RepoInstance with computed worktree paths
    let repos = entries
        .into_iter()
        .map(|e| {
            let worktree_path = if ws_path.is_empty() {
                e.local_path.clone()
            } else {
                format!("{}/{}", ws_path, e.repo_dir)
            };
            let missing = !std::path::Path::new(&worktree_path).exists();
            RepoInstance {
                name: e.name,
                worktree_path,
                repo_dir: e.repo_dir,
                missing,
                default_branch: e.default_branch,
                default_remote: e.remote,
            }
        })
        .collect();

    Ok(repos)
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
