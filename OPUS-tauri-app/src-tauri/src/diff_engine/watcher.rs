use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::Duration;

use tauri::{AppHandle, Emitter};

use crate::diff_engine::summary::compute_diff_summary;

const POLL_INTERVAL_SECS: u64 = 5;

/// Handle for a running diff watcher.
pub struct DiffWatcherHandle {
    pub cancel: Arc<AtomicBool>,
}

/// Start a background diff watcher for a repo worktree. Emits `diff:summary` events.
pub fn start_watcher(
    app: AppHandle,
    workspace_name: String,
    repo_name: String,
    repo_path: String,
) -> DiffWatcherHandle {
    let cancel = Arc::new(AtomicBool::new(false));
    let cancel_clone = cancel.clone();

    std::thread::spawn(move || {
        let mut last_hash: Option<u64> = None;

        loop {
            if cancel_clone.load(Ordering::Relaxed) {
                break;
            }

            match compute_diff_summary(&repo_path) {
                Ok(summary) => {
                    // Simple hash to deduplicate
                    let hash = {
                        use std::hash::{Hash, Hasher};
                        let mut hasher = std::collections::hash_map::DefaultHasher::new();
                        format!("{:?}", summary.files).hash(&mut hasher);
                        hasher.finish()
                    };

                    if last_hash != Some(hash) {
                        last_hash = Some(hash);
                        app.emit("diff:summary", serde_json::json!({
                            "workspace_name": workspace_name,
                            "repo": repo_name,
                            "summary": summary,
                        })).ok();
                    }
                }
                Err(e) => {
                    app.emit("diff:status", serde_json::json!({
                        "workspace_name": workspace_name,
                        "repo": repo_name,
                        "status": "error",
                        "message": e,
                    })).ok();
                }
            }

            // Sleep with cancellation check
            for _ in 0..(POLL_INTERVAL_SECS * 1000 / 500) {
                if cancel_clone.load(Ordering::Relaxed) {
                    return;
                }
                std::thread::sleep(Duration::from_millis(500));
            }
        }
    });

    DiffWatcherHandle { cancel }
}
