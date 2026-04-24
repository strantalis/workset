use tauri::ipc::Channel;
use tauri::State;

use crate::state::AppState;
use crate::terminal_manager::PtyEvent;
use crate::types::error::ErrorEnvelope;

#[tauri::command]
pub fn terminal_spawn(
    state: State<'_, AppState>,
    terminal_id: String,
    cwd: String,
    channel: Channel<PtyEvent>,
) -> Result<(), ErrorEnvelope> {
    let manager_ref = state.terminal_manager.clone();
    let mut mgr = state.terminal_manager.lock().unwrap();
    mgr.spawn(&terminal_id, &cwd, channel, manager_ref)
        .map_err(|e| ErrorEnvelope::new("terminal", "terminal.spawn", &e))
}

#[tauri::command]
pub fn terminal_attach(
    state: State<'_, AppState>,
    terminal_id: String,
    channel: Channel<PtyEvent>,
) -> Result<(), ErrorEnvelope> {
    let mut mgr = state.terminal_manager.lock().unwrap();
    mgr.attach(&terminal_id, channel)
        .map_err(|e| ErrorEnvelope::new("terminal", "terminal.attach", &e))
}

#[tauri::command]
pub fn terminal_detach(
    state: State<'_, AppState>,
    terminal_id: String,
) -> Result<(), ErrorEnvelope> {
    let mut mgr = state.terminal_manager.lock().unwrap();
    mgr.detach(&terminal_id)
        .map_err(|e| ErrorEnvelope::new("terminal", "terminal.detach", &e))
}

#[tauri::command]
pub fn terminal_write(
    state: State<'_, AppState>,
    terminal_id: String,
    data: String,
) -> Result<(), ErrorEnvelope> {
    let mut mgr = state.terminal_manager.lock().unwrap();
    mgr.write(&terminal_id, &data)
        .map_err(|e| ErrorEnvelope::new("terminal", "terminal.write", &e))
}

#[tauri::command]
pub fn terminal_resize(
    state: State<'_, AppState>,
    terminal_id: String,
    cols: u32,
    rows: u32,
) -> Result<(), ErrorEnvelope> {
    let mut mgr = state.terminal_manager.lock().unwrap();
    mgr.resize(&terminal_id, cols, rows)
        .map_err(|e| ErrorEnvelope::new("terminal", "terminal.resize", &e))
}

#[tauri::command]
pub fn terminal_kill(
    state: State<'_, AppState>,
    terminal_id: String,
) -> Result<(), ErrorEnvelope> {
    let mut mgr = state.terminal_manager.lock().unwrap();
    mgr.kill(&terminal_id)
        .map_err(|e| ErrorEnvelope::new("terminal", "terminal.kill", &e))
}
