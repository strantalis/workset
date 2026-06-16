use tauri::State;
use crate::state::AppState;
use crate::types::context::ActiveContext;
use crate::types::error::ErrorEnvelope;

#[tauri::command]
pub fn context_get(state: State<'_, AppState>) -> Result<ActiveContext, ErrorEnvelope> {
    let ctx = state.ui_context.lock().map_err(|e| {
        ErrorEnvelope::runtime("context.get", format!("Lock error: {e}"))
    })?;
    let active_workset_id = ctx.active_workset_id().map(|s| s.to_string());
    let active_workspace = active_workset_id
        .as_deref()
        .and_then(|wid| ctx.last_workspace_for(wid))
        .map(|s| s.to_string());
    Ok(ActiveContext {
        active_workset_id,
        active_workspace,
    })
}

#[tauri::command]
pub fn context_set_active_workset(
    state: State<'_, AppState>,
    workset_id: String,
) -> Result<(), ErrorEnvelope> {
    let mut ctx = state.ui_context.lock().map_err(|e| {
        ErrorEnvelope::runtime("context.set_active_workset", format!("Lock error: {e}"))
    })?;
    ctx.set_active_workset(&workset_id)
}

#[tauri::command]
pub fn context_set_active_workspace(
    state: State<'_, AppState>,
    workspace_name: String,
) -> Result<(), ErrorEnvelope> {
    let ctx_lock = state.ui_context.lock().map_err(|e| {
        ErrorEnvelope::runtime("context.set_active_workspace", format!("Lock error: {e}"))
    })?;
    let workset_id = ctx_lock
        .active_workset_id()
        .ok_or_else(|| ErrorEnvelope::config("context.set_active_workspace", "No active workset"))?
        .to_string();
    drop(ctx_lock);

    let mut ctx = state.ui_context.lock().map_err(|e| {
        ErrorEnvelope::runtime("context.set_active_workspace", format!("Lock error: {e}"))
    })?;
    ctx.set_active_workspace(&workset_id, &workspace_name)
}
