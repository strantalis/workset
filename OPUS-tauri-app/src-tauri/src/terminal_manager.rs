use std::collections::{HashMap, VecDeque};
use std::io::{Read, Write};
use std::sync::{Arc, Mutex};

use portable_pty::{native_pty_system, CommandBuilder, MasterPty, PtySize};
use serde::Serialize;
use tauri::ipc::Channel;

const RING_BUFFER_MAX_BYTES: usize = 64 * 1024;

#[derive(Clone, Serialize)]
#[serde(tag = "type")]
pub enum PtyEvent {
    Data { data: String },
    Closed { exit_code: Option<i32> },
    Error { message: String },
}

struct ManagedTerminal {
    master: Box<dyn MasterPty + Send>,
    writer: Box<dyn Write + Send>,
    subscribers: Vec<Channel<PtyEvent>>,
    ring_buffer: VecDeque<u8>,
    alive: bool,
    cancel: Arc<std::sync::atomic::AtomicBool>,
}

pub struct TerminalManager {
    sessions: HashMap<String, ManagedTerminal>,
}

impl TerminalManager {
    pub fn new() -> Self {
        Self {
            sessions: HashMap::new(),
        }
    }

    /// Spawn a new PTY process with the given terminal_id.
    /// The `manager_ref` is needed so the reader thread can broadcast output.
    pub fn spawn(
        &mut self,
        terminal_id: &str,
        cwd: &str,
        channel: Channel<PtyEvent>,
        manager_ref: Arc<Mutex<TerminalManager>>,
    ) -> Result<(), String> {
        let pty_system = native_pty_system();
        let pair = pty_system
            .openpty(PtySize {
                rows: 24,
                cols: 80,
                pixel_width: 0,
                pixel_height: 0,
            })
            .map_err(|e| format!("Failed to open PTY: {}", e))?;

        let mut cmd = CommandBuilder::new_default_prog();
        cmd.cwd(cwd);

        let child = pair
            .slave
            .spawn_command(cmd)
            .map_err(|e| format!("Failed to spawn shell: {}", e))?;

        drop(pair.slave);

        let writer = pair
            .master
            .take_writer()
            .map_err(|e| format!("Failed to get PTY writer: {}", e))?;

        let reader = pair
            .master
            .try_clone_reader()
            .map_err(|e| format!("Failed to get PTY reader: {}", e))?;

        let cancel = Arc::new(std::sync::atomic::AtomicBool::new(false));

        let managed = ManagedTerminal {
            master: pair.master,
            writer,
            subscribers: vec![channel],
            ring_buffer: VecDeque::with_capacity(RING_BUFFER_MAX_BYTES),
            alive: true,
            cancel: cancel.clone(),
        };

        let tid = terminal_id.to_string();
        self.sessions.insert(tid.clone(), managed);

        std::thread::spawn(move || {
            reader_loop(tid, reader, child, cancel, manager_ref);
        });

        Ok(())
    }

    /// Attach a new Channel subscriber to an existing terminal.
    /// Replays the ring buffer so the UI gets immediate screen content.
    pub fn attach(
        &mut self,
        terminal_id: &str,
        channel: Channel<PtyEvent>,
    ) -> Result<(), String> {
        let session = self
            .sessions
            .get_mut(terminal_id)
            .ok_or_else(|| format!("No terminal with id: {}", terminal_id))?;

        // Replay ring buffer
        if !session.ring_buffer.is_empty() {
            let (front, back) = session.ring_buffer.as_slices();
            let mut combined = Vec::with_capacity(front.len() + back.len());
            combined.extend_from_slice(front);
            combined.extend_from_slice(back);
            if let Ok(data) = String::from_utf8(combined) {
                channel.send(PtyEvent::Data { data }).ok();
            }
        }

        if !session.alive {
            channel.send(PtyEvent::Closed { exit_code: None }).ok();
        }

        session.subscribers.push(channel);
        Ok(())
    }

    /// Remove the most recently added subscriber (the one being detached).
    /// The PTY keeps running.
    pub fn detach(&mut self, terminal_id: &str) -> Result<(), String> {
        let session = self
            .sessions
            .get_mut(terminal_id)
            .ok_or_else(|| format!("No terminal with id: {}", terminal_id))?;

        if !session.subscribers.is_empty() {
            session.subscribers.pop();
        }
        Ok(())
    }

    /// Write input data to the PTY.
    pub fn write(&mut self, terminal_id: &str, data: &str) -> Result<(), String> {
        let session = self
            .sessions
            .get_mut(terminal_id)
            .ok_or_else(|| format!("No terminal with id: {}", terminal_id))?;

        session
            .writer
            .write_all(data.as_bytes())
            .map_err(|e| format!("Write failed: {}", e))?;
        session
            .writer
            .flush()
            .map_err(|e| format!("Flush failed: {}", e))?;
        Ok(())
    }

    /// Resize the PTY.
    pub fn resize(&mut self, terminal_id: &str, cols: u32, rows: u32) -> Result<(), String> {
        let session = self
            .sessions
            .get_mut(terminal_id)
            .ok_or_else(|| format!("No terminal with id: {}", terminal_id))?;

        session
            .master
            .resize(PtySize {
                rows: rows as u16,
                cols: cols as u16,
                pixel_width: 0,
                pixel_height: 0,
            })
            .map_err(|e| format!("Resize failed: {}", e))?;
        Ok(())
    }

    /// Kill the PTY process and remove the session.
    pub fn kill(&mut self, terminal_id: &str) -> Result<(), String> {
        if let Some(session) = self.sessions.remove(terminal_id) {
            session
                .cancel
                .store(true, std::sync::atomic::Ordering::Relaxed);
            // Dropping master/writer closes the PTY fd, causing the child to exit
        }
        Ok(())
    }

    pub fn is_alive(&self, terminal_id: &str) -> bool {
        self.sessions.get(terminal_id).map(|s| s.alive).unwrap_or(false)
    }

    pub fn exists(&self, terminal_id: &str) -> bool {
        self.sessions.contains_key(terminal_id)
    }

    fn push_to_ring_buffer(session: &mut ManagedTerminal, data: &[u8]) {
        for &byte in data {
            if session.ring_buffer.len() >= RING_BUFFER_MAX_BYTES {
                session.ring_buffer.pop_front();
            }
            session.ring_buffer.push_back(byte);
        }
    }

    fn broadcast(session: &ManagedTerminal, event: &PtyEvent) {
        for sub in &session.subscribers {
            sub.send(event.clone()).ok();
        }
    }
}

/// Background thread that reads PTY output and broadcasts to subscribers.
fn reader_loop(
    terminal_id: String,
    mut reader: Box<dyn Read + Send>,
    mut child: Box<dyn portable_pty::Child + Send>,
    cancel: Arc<std::sync::atomic::AtomicBool>,
    manager: Arc<Mutex<TerminalManager>>,
) {
    let mut buf = [0u8; 4096];

    loop {
        if cancel.load(std::sync::atomic::Ordering::Relaxed) {
            break;
        }

        match reader.read(&mut buf) {
            Ok(0) => break,
            Ok(n) => {
                let chunk = &buf[..n];
                let data = String::from_utf8_lossy(chunk).to_string();
                let event = PtyEvent::Data { data };

                if let Ok(mut mgr) = manager.lock() {
                    if let Some(session) = mgr.sessions.get_mut(&terminal_id) {
                        TerminalManager::push_to_ring_buffer(session, chunk);
                        TerminalManager::broadcast(session, &event);
                    }
                }
            }
            Err(_) => break,
        }
    }

    let exit_code = child.wait().ok().map(|status| if status.success() { 0 } else { 1 });
    let event = PtyEvent::Closed { exit_code };

    if let Ok(mut mgr) = manager.lock() {
        if let Some(session) = mgr.sessions.get_mut(&terminal_id) {
            session.alive = false;
            TerminalManager::broadcast(session, &event);
        }
    }
}
