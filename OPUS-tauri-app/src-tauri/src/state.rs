use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use crate::commands::pty::PtySession;
use crate::diff_engine::watcher::DiffWatcherHandle;
use crate::store::layout::LayoutStore;
use crate::store::migrations::MigrationStore;
use crate::store::ui_context::UiContextStore;
use crate::store::workset_profiles::WorksetProfileStore;
use crate::terminal_manager::TerminalManager;

pub struct AppState {
    pub profiles: Mutex<WorksetProfileStore>,
    pub ui_context: Mutex<UiContextStore>,
    pub layout_store: Mutex<LayoutStore>,
    pub migration_store: Mutex<MigrationStore>,
    pub pty_sessions: Mutex<HashMap<String, PtySession>>,
    pub diff_watchers: Mutex<HashMap<String, DiffWatcherHandle>>,
    pub cli_path: Mutex<Option<String>>,
    pub sessiond_path: Mutex<Option<String>>,
    pub terminal_manager: Arc<Mutex<TerminalManager>>,
}

impl AppState {
    pub fn new() -> Self {
        let profiles = WorksetProfileStore::load().unwrap_or_default();
        let ui_context = UiContextStore::load().unwrap_or_default();
        Self {
            profiles: Mutex::new(profiles),
            ui_context: Mutex::new(ui_context),
            layout_store: Mutex::new(LayoutStore::new()),
            migration_store: Mutex::new(MigrationStore::new()),
            pty_sessions: Mutex::new(HashMap::new()),
            diff_watchers: Mutex::new(HashMap::new()),
            cli_path: Mutex::new(None),
            sessiond_path: Mutex::new(None),
            terminal_manager: Arc::new(Mutex::new(TerminalManager::new())),
        }
    }
}
