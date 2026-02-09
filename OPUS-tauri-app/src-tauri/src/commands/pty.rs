use tauri::{AppHandle, Emitter, State};

use crate::sessiond::client::{read_stream_message, SessiondClient};
use crate::state::AppState;
use crate::types::error::ErrorEnvelope;
use crate::types::layout::TerminalLayout;
use crate::types::pty::{BootstrapPayload, PtyCreateResult};

/// Active PTY session handle
pub struct PtySession {
    pub session_id: String,
    pub workspace_name: String,
    pub terminal_id: String,
    pub stream_id: String,
    pub cancel: std::sync::Arc<std::sync::atomic::AtomicBool>,
}

fn make_session_id(workspace_name: &str, terminal_id: &str) -> String {
    format!("{}:{}", workspace_name, terminal_id)
}

fn get_socket_path(state: &AppState) -> String {
    state
        .sessiond_path
        .lock()
        .ok()
        .and_then(|p| p.clone())
        .unwrap_or_else(SessiondClient::default_socket_path)
}

#[tauri::command]
pub fn pty_create(
    _state: State<'_, AppState>,
) -> Result<PtyCreateResult, ErrorEnvelope> {
    let terminal_id = uuid::Uuid::new_v4().to_string();
    Ok(PtyCreateResult { terminal_id })
}

#[tauri::command]
pub fn pty_start(
    app: AppHandle,
    state: State<'_, AppState>,
    workspace_name: String,
    terminal_id: String,
    _kind: String,
    cwd: String,
) -> Result<(), ErrorEnvelope> {
    let session_id = make_session_id(&workspace_name, &terminal_id);
    let socket_path = get_socket_path(&state);
    let client = SessiondClient::new(&socket_path);

    // Create session
    let _resp = client.create(&session_id, &cwd).map_err(|e| {
        ErrorEnvelope::new("sessiond", "pty.start", &e)
    })?;

    let stream_id = uuid::Uuid::new_v4().to_string();
    let cancel = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
    let cancel_clone = cancel.clone();

    // Store session, cancelling any existing streaming thread first
    {
        let mut sessions = state.pty_sessions.lock().unwrap();
        if let Some(old) = sessions.remove(&session_id) {
            old.cancel
                .store(true, std::sync::atomic::Ordering::Relaxed);
        }
        sessions.insert(
            session_id.clone(),
            PtySession {
                session_id: session_id.clone(),
                workspace_name: workspace_name.clone(),
                terminal_id: terminal_id.clone(),
                stream_id: stream_id.clone(),
                cancel,
            },
        );
    }

    // Spawn streaming thread
    let ws_name = workspace_name.clone();
    let term_id = terminal_id.clone();
    let sid = session_id.clone();
    let strid = stream_id.clone();

    std::thread::spawn(move || {
        let client = SessiondClient::new(&socket_path);
        let attach_result = client.attach(&sid, &strid, 0, false);

        let (mut reader, first) = match attach_result {
            Ok(r) => r,
            Err(e) => {
                app.emit("pty:lifecycle", serde_json::json!({
                    "workspace_name": ws_name,
                    "terminal_id": term_id,
                    "status": "error",
                    "message": e,
                })).ok();
                return;
            }
        };

        // Handle first bootstrap message
        emit_stream_message(&app, &ws_name, &term_id, &first);

        // Emit lifecycle started
        app.emit("pty:lifecycle", serde_json::json!({
            "workspace_name": ws_name,
            "terminal_id": term_id,
            "status": "started",
        })).ok();

        // Stream loop
        loop {
            if cancel_clone.load(std::sync::atomic::Ordering::Relaxed) {
                break;
            }
            match read_stream_message(&mut reader) {
                Ok(msg) => {
                    // Re-check cancel after blocking read to avoid emitting
                    // stale data from cancelled threads that were blocked on read
                    if cancel_clone.load(std::sync::atomic::Ordering::Relaxed) {
                        break;
                    }
                    emit_stream_message(&app, &ws_name, &term_id, &msg);
                }
                Err(_) => {
                    break;
                }
            }
        }

        // Emit lifecycle closed
        app.emit("pty:lifecycle", serde_json::json!({
            "workspace_name": ws_name,
            "terminal_id": term_id,
            "status": "closed",
        })).ok();
    });

    Ok(())
}

fn emit_stream_message(
    app: &AppHandle,
    workspace_name: &str,
    terminal_id: &str,
    msg: &crate::sessiond::protocol::StreamMessage,
) {
    match msg.msg_type.as_str() {
        "bootstrap" => {
            app.emit("pty:bootstrap", serde_json::json!({
                "workspace_name": workspace_name,
                "terminal_id": terminal_id,
                "snapshot": msg.data,
                "alt_screen": msg.alt_screen,
                "mouse": msg.mouse,
                "mouse_sgr": msg.mouse_sgr,
                "safe_to_replay": msg.safe_to_replay,
                "initial_credit": msg.initial_credit,
                "next_offset": msg.next_offset,
            })).ok();
        }
        "data" => {
            let bytes = msg.data.as_ref().map(|d| d.len() as i64).unwrap_or(0);
            app.emit("pty:data", serde_json::json!({
                "workspace_name": workspace_name,
                "terminal_id": terminal_id,
                "data": msg.data,
                "bytes": bytes,
            })).ok();
        }
        "modes" => {
            app.emit("pty:modes", serde_json::json!({
                "workspace_name": workspace_name,
                "terminal_id": terminal_id,
                "alt_screen": msg.alt_screen,
                "mouse": msg.mouse,
                "mouse_sgr": msg.mouse_sgr,
                "mouse_encoding": msg.mouse_encoding,
            })).ok();
        }
        "error" => {
            app.emit("pty:lifecycle", serde_json::json!({
                "workspace_name": workspace_name,
                "terminal_id": terminal_id,
                "status": "error",
                "message": msg.error,
            })).ok();
        }
        _ => {}
    }
}

#[tauri::command]
pub fn pty_write(
    state: State<'_, AppState>,
    workspace_name: String,
    terminal_id: String,
    data: String,
) -> Result<(), ErrorEnvelope> {
    let session_id = make_session_id(&workspace_name, &terminal_id);
    let socket_path = get_socket_path(&state);
    let client = SessiondClient::new(&socket_path);

    client.send_input(&session_id, &data).map_err(|e| {
        ErrorEnvelope::new("sessiond", "pty.write", &e)
    })
}

#[tauri::command]
pub fn pty_resize(
    state: State<'_, AppState>,
    workspace_name: String,
    terminal_id: String,
    cols: u32,
    rows: u32,
) -> Result<(), ErrorEnvelope> {
    let session_id = make_session_id(&workspace_name, &terminal_id);
    let socket_path = get_socket_path(&state);
    let client = SessiondClient::new(&socket_path);

    client.resize(&session_id, cols, rows).map_err(|e| {
        ErrorEnvelope::new("sessiond", "pty.resize", &e)
    })
}

#[tauri::command]
pub fn pty_ack(
    state: State<'_, AppState>,
    workspace_name: String,
    terminal_id: String,
    bytes: i64,
) -> Result<(), ErrorEnvelope> {
    let session_id = make_session_id(&workspace_name, &terminal_id);
    let socket_path = get_socket_path(&state);

    // Look up stream_id from active sessions
    let stream_id = {
        let sessions = state.pty_sessions.lock().unwrap();
        sessions
            .get(&session_id)
            .map(|s| s.stream_id.clone())
            .unwrap_or_default()
    };

    if stream_id.is_empty() {
        return Err(ErrorEnvelope::new("sessiond", "pty.ack", "No active session"));
    }

    let client = SessiondClient::new(&socket_path);
    client.ack(&session_id, &stream_id, bytes).map_err(|e| {
        ErrorEnvelope::new("sessiond", "pty.ack", &e)
    })
}

#[tauri::command]
pub fn pty_bootstrap(
    state: State<'_, AppState>,
    workspace_name: String,
    terminal_id: String,
) -> Result<BootstrapPayload, ErrorEnvelope> {
    let session_id = make_session_id(&workspace_name, &terminal_id);
    let socket_path = get_socket_path(&state);
    let client = SessiondClient::new(&socket_path);

    let resp = client.bootstrap(&session_id).map_err(|e| {
        ErrorEnvelope::new("sessiond", "pty.bootstrap", &e)
    })?;

    Ok(BootstrapPayload {
        workspace_name,
        terminal_id,
        snapshot: resp.snapshot,
        backlog: resp.backlog,
        backlog_truncated: Some(resp.backlog_truncated),
        next_offset: resp.next_offset.map(|n| n as u64),
        alt_screen: Some(resp.alt_screen),
        mouse: Some(resp.mouse),
        mouse_sgr: Some(resp.mouse_sgr),
        safe_to_replay: Some(resp.safe_to_replay),
        initial_credit: resp.initial_credit.map(|n| n as u64),
    })
}

#[tauri::command]
pub fn pty_stop(
    state: State<'_, AppState>,
    workspace_name: String,
    terminal_id: String,
) -> Result<(), ErrorEnvelope> {
    let session_id = make_session_id(&workspace_name, &terminal_id);

    // Cancel the streaming thread
    {
        let mut sessions = state.pty_sessions.lock().unwrap();
        if let Some(session) = sessions.remove(&session_id) {
            session
                .cancel
                .store(true, std::sync::atomic::Ordering::Relaxed);
        }
    }

    let socket_path = get_socket_path(&state);
    let client = SessiondClient::new(&socket_path);
    client.stop(&session_id).map_err(|e| {
        ErrorEnvelope::new("sessiond", "pty.stop", &e)
    })
}

// ---------- Layout persistence commands ----------

#[tauri::command]
pub fn layout_get(
    state: State<'_, AppState>,
    workspace_name: String,
) -> Result<Option<TerminalLayout>, ErrorEnvelope> {
    let mut store = state.layout_store.lock().unwrap();
    Ok(store.get(&workspace_name))
}

#[tauri::command]
pub fn layout_save(
    state: State<'_, AppState>,
    workspace_name: String,
    layout: TerminalLayout,
) -> Result<(), ErrorEnvelope> {
    let mut store = state.layout_store.lock().unwrap();
    store.save(&workspace_name, &layout).map_err(|e| {
        ErrorEnvelope::new("persistence", "layout.save", &e)
    })
}
