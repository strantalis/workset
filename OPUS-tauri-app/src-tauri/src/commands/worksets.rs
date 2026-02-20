use tauri::State;
use crate::state::AppState;
use crate::types::error::ErrorEnvelope;
use crate::types::workset::{WorksetDefaults, WorksetProfile};

#[tauri::command]
pub fn worksets_list(state: State<'_, AppState>) -> Result<Vec<WorksetProfile>, ErrorEnvelope> {
    let store = state.profiles.lock().map_err(|e| {
        ErrorEnvelope::runtime("worksets.list", format!("Lock error: {e}"))
    })?;
    Ok(store.list())
}

#[tauri::command]
pub fn worksets_create(
    state: State<'_, AppState>,
    name: String,
    defaults: Option<WorksetDefaults>,
) -> Result<WorksetProfile, ErrorEnvelope> {
    let mut store = state.profiles.lock().map_err(|e| {
        ErrorEnvelope::runtime("worksets.create", format!("Lock error: {e}"))
    })?;
    store.create(&name, defaults)
}

#[tauri::command]
pub fn worksets_update(
    state: State<'_, AppState>,
    id: String,
    name: Option<String>,
    defaults: Option<WorksetDefaults>,
) -> Result<WorksetProfile, ErrorEnvelope> {
    let mut store = state.profiles.lock().map_err(|e| {
        ErrorEnvelope::runtime("worksets.update", format!("Lock error: {e}"))
    })?;
    store.update(&id, name.as_deref(), defaults)
}

#[tauri::command]
pub fn worksets_delete(state: State<'_, AppState>, id: String) -> Result<(), ErrorEnvelope> {
    let mut store = state.profiles.lock().map_err(|e| {
        ErrorEnvelope::runtime("worksets.delete", format!("Lock error: {e}"))
    })?;
    store.delete(&id)
}

#[tauri::command]
pub fn worksets_repos_add(
    state: State<'_, AppState>,
    workset_id: String,
    source: String,
) -> Result<WorksetProfile, ErrorEnvelope> {
    let mut store = state.profiles.lock().map_err(|e| {
        ErrorEnvelope::runtime("worksets.repos.add", format!("Lock error: {e}"))
    })?;
    store.add_repo(&workset_id, &source)
}

#[tauri::command]
pub fn worksets_repos_remove(
    state: State<'_, AppState>,
    workset_id: String,
    source: String,
) -> Result<WorksetProfile, ErrorEnvelope> {
    let mut store = state.profiles.lock().map_err(|e| {
        ErrorEnvelope::runtime("worksets.repos.remove", format!("Lock error: {e}"))
    })?;
    store.remove_repo(&workset_id, &source)
}
