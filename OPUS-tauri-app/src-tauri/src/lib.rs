mod cli;
mod commands;
mod diff_engine;
mod jobs;
mod sessiond;
mod state;
mod store;
mod terminal_manager;
mod types;

pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .manage(state::AppState::new())
        .invoke_handler(tauri::generate_handler![
            // Group 1: Worksets
            commands::worksets::worksets_list,
            commands::worksets::worksets_create,
            commands::worksets::worksets_update,
            commands::worksets::worksets_delete,
            commands::worksets::worksets_repos_add,
            commands::worksets::worksets_repos_remove,
            // Group 2: Context
            commands::context::context_get,
            commands::context::context_set_active_workset,
            commands::context::context_set_active_workspace,
            // Group 3: Workspaces
            commands::workspaces::workspaces_list,
            commands::workspaces::workspaces_create,
            commands::workspaces::workspaces_create_status,
            commands::workspaces::workspaces_delete,
            // Group 5: Repos
            commands::repos::workspace_repos_list,
            // Group 8: PTY
            commands::pty::pty_create,
            commands::pty::pty_start,
            commands::pty::pty_write,
            commands::pty::pty_resize,
            commands::pty::pty_ack,
            commands::pty::pty_bootstrap,
            commands::pty::pty_stop,
            // Layout persistence
            commands::pty::layout_get,
            commands::pty::layout_save,
            // Terminal (new portable-pty based)
            commands::terminal::terminal_spawn,
            commands::terminal::terminal_attach,
            commands::terminal::terminal_detach,
            commands::terminal::terminal_write,
            commands::terminal::terminal_resize,
            commands::terminal::terminal_kill,
            // Group 6-7: Diff
            commands::diff::diff_summary,
            commands::diff::diff_file_patch,
            commands::diff::diff_watch_start,
            commands::diff::diff_watch_stop,
            // Group 4: Migrations
            commands::migrations::migration_start,
            commands::migrations::migration_cancel,
            // Group 9: Diagnostics
            commands::diagnostics::diagnostics_env_snapshot,
            commands::diagnostics::diagnostics_reload_login_env,
            commands::diagnostics::diagnostics_sessiond_status,
            commands::diagnostics::diagnostics_sessiond_restart,
            commands::diagnostics::diagnostics_cli_status,
            // Group 10: GitHub
            commands::github::github_list_repos,
            commands::github::github_auth_status,
            commands::github::github_list_accounts,
            commands::github::github_switch_account,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
