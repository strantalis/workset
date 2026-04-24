use std::process::Command;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

use tauri::{AppHandle, Emitter};

use crate::types::job::{MigrationProgress, WorkspaceMigrationStatus};

#[derive(Debug, Clone, Default)]
pub struct RemoveOptions {
    pub delete_worktrees: bool,
    pub delete_local: bool,
}

pub struct MigrationJobHandle {
    pub job_id: String,
    pub cancel: Arc<AtomicBool>,
}

/// Run a migration job: add or remove a repo from the given workspaces.
pub fn run_migration(
    app: AppHandle,
    cli_path: String,
    job_id: String,
    _workset_id: String,
    repo_url: String,
    action: String, // "add" or "remove"
    workspace_names: Vec<String>,
    remove_opts: Option<RemoveOptions>,
) -> MigrationJobHandle {
    let cancel = Arc::new(AtomicBool::new(false));
    let cancel_clone = cancel.clone();

    let handle = MigrationJobHandle {
        job_id: job_id.clone(),
        cancel: cancel.clone(),
    };

    std::thread::spawn(move || {
        let mut statuses: Vec<WorkspaceMigrationStatus> = workspace_names
            .iter()
            .map(|name| WorkspaceMigrationStatus {
                workspace_name: name.clone(),
                state: "pending".to_string(),
                error: None,
            })
            .collect();

        emit_progress(&app, &job_id, "running", &statuses);

        for (i, ws_name) in workspace_names.iter().enumerate() {
            if cancel_clone.load(Ordering::Relaxed) {
                for status in statuses.iter_mut().skip(i) {
                    status.state = "failed".to_string();
                }
                emit_progress(&app, &job_id, "canceled", &statuses);
                return;
            }

            statuses[i].state = "running".to_string();
            emit_progress(&app, &job_id, "running", &statuses);

            let result = if action == "add" {
                run_add_repo(&cli_path, ws_name, &repo_url)
            } else {
                run_remove_repo(&cli_path, ws_name, &repo_url, remove_opts.as_ref())
            };

            match result {
                Ok(_) => {
                    statuses[i].state = "success".to_string();
                }
                Err(e) => {
                    statuses[i].state = "failed".to_string();
                    statuses[i].error = Some(
                        crate::types::error::ErrorEnvelope::new("migration", &action, &e),
                    );
                }
            }
            emit_progress(&app, &job_id, "running", &statuses);
        }

        let has_failures = statuses.iter().any(|s| s.state == "failed");
        let final_state = if has_failures { "failed" } else { "done" };
        emit_progress(&app, &job_id, final_state, &statuses);
    });

    handle
}

fn run_add_repo(cli_path: &str, workspace_name: &str, repo_url: &str) -> Result<(), String> {
    let output = Command::new(cli_path)
        .args(["repo", "add", "-w", workspace_name, repo_url])
        .output()
        .map_err(|e| format!("Failed to run workset repo add: {}", e))?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!("repo add failed: {}", stderr));
    }
    Ok(())
}

fn run_remove_repo(
    cli_path: &str,
    workspace_name: &str,
    repo_url: &str,
    opts: Option<&RemoveOptions>,
) -> Result<(), String> {
    let repo_name = derive_repo_name(repo_url);
    let mut args = vec!["repo", "remove", "-w", workspace_name, &repo_name, "--yes"];
    if let Some(o) = opts {
        if o.delete_worktrees {
            args.push("--delete-worktrees");
        }
        if o.delete_local {
            args.push("--delete-local");
        }
    }

    let output = Command::new(cli_path)
        .args(&args)
        .output()
        .map_err(|e| format!("Failed to run workset repo remove: {}", e))?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!("repo remove failed: {}", stderr));
    }
    Ok(())
}

/// Derive the repo name from a URL/source string.
/// Mirrors Go's DeriveRepoNameFromURL: strips .git suffix, trailing slashes,
/// then returns the last path segment.
fn derive_repo_name(url: &str) -> String {
    let trimmed = url.trim_end_matches(".git").trim_end_matches('/');
    if let Some(idx) = trimmed.rfind('/') {
        return trimmed[idx + 1..].to_string();
    }
    if let Some(idx) = trimmed.rfind(':') {
        return trimmed[idx + 1..].to_string();
    }
    trimmed.to_string()
}

fn emit_progress(
    app: &AppHandle,
    job_id: &str,
    state: &str,
    workspaces: &[WorkspaceMigrationStatus],
) {
    let progress = MigrationProgress {
        job_id: job_id.to_string(),
        state: state.to_string(),
        workspaces: workspaces.to_vec(),
    };
    app.emit("migration:progress", &progress).ok();
}
