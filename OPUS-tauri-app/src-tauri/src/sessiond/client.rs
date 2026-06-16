use std::io::{BufRead, BufReader, Write};
use std::os::unix::net::UnixStream;
use std::path::Path;
use std::time::Duration;

use crate::sessiond::protocol::*;

pub struct SessiondClient {
    socket_path: String,
}

impl SessiondClient {
    pub fn new(socket_path: &str) -> Self {
        Self {
            socket_path: socket_path.to_string(),
        }
    }

    pub fn default_socket_path() -> String {
        let home = dirs::home_dir().unwrap_or_default();
        let base = home.join(".workset");
        // Prefer dev socket if it exists (used during development)
        let dev = base.join("sessiond-dev.sock");
        if dev.exists() {
            return dev.to_string_lossy().to_string();
        }
        base.join("sessiond.sock")
            .to_string_lossy()
            .to_string()
    }

    fn connect(&self) -> Result<UnixStream, String> {
        let path = Path::new(&self.socket_path);
        if !path.exists() {
            return Err(format!("sessiond socket not found at {}", self.socket_path));
        }
        let stream = UnixStream::connect(path)
            .map_err(|e| format!("Failed to connect to sessiond: {}", e))?;
        stream
            .set_read_timeout(Some(Duration::from_secs(10)))
            .ok();
        stream
            .set_write_timeout(Some(Duration::from_secs(5)))
            .ok();
        Ok(stream)
    }

    fn control_call(
        &self,
        method: &str,
        params: Option<serde_json::Value>,
    ) -> Result<serde_json::Value, String> {
        let mut stream = self.connect()?;

        let req = ControlRequest {
            protocol_version: PROTOCOL_VERSION,
            method: method.to_string(),
            params,
        };
        let mut payload = serde_json::to_string(&req)
            .map_err(|e| format!("Failed to serialize request: {}", e))?;
        payload.push('\n');

        stream
            .write_all(payload.as_bytes())
            .map_err(|e| format!("Failed to send request: {}", e))?;
        stream
            .flush()
            .map_err(|e| format!("Failed to flush: {}", e))?;

        let mut reader = BufReader::new(stream);
        let mut line = String::new();
        reader
            .read_line(&mut line)
            .map_err(|e| format!("Failed to read response: {}", e))?;

        let resp: ControlResponse = serde_json::from_str(line.trim())
            .map_err(|e| format!("Failed to parse response: {}", e))?;

        if !resp.ok {
            return Err(resp.error.unwrap_or_else(|| "Unknown error".to_string()));
        }
        Ok(resp.result)
    }

    pub fn create(&self, session_id: &str, cwd: &str) -> Result<CreateResponse, String> {
        let params = serde_json::to_value(CreateRequest {
            session_id: session_id.to_string(),
            cwd: cwd.to_string(),
        })
        .map_err(|e| e.to_string())?;

        let result = self.control_call("create", Some(params))?;
        serde_json::from_value(result).map_err(|e| format!("Failed to parse create response: {}", e))
    }

    pub fn send_input(&self, session_id: &str, data: &str) -> Result<(), String> {
        let params = serde_json::to_value(SendRequest {
            session_id: session_id.to_string(),
            data: data.to_string(),
        })
        .map_err(|e| e.to_string())?;

        self.control_call("send", Some(params))?;
        Ok(())
    }

    pub fn resize(&self, session_id: &str, cols: u32, rows: u32) -> Result<(), String> {
        let params = serde_json::to_value(ResizeRequest {
            session_id: session_id.to_string(),
            cols,
            rows,
        })
        .map_err(|e| e.to_string())?;

        self.control_call("resize", Some(params))?;
        Ok(())
    }

    pub fn ack(
        &self,
        session_id: &str,
        stream_id: &str,
        bytes: i64,
    ) -> Result<(), String> {
        let params = serde_json::to_value(AckRequest {
            session_id: session_id.to_string(),
            stream_id: stream_id.to_string(),
            bytes,
        })
        .map_err(|e| e.to_string())?;

        self.control_call("ack", Some(params))?;
        Ok(())
    }

    pub fn bootstrap(&self, session_id: &str) -> Result<BootstrapResponse, String> {
        let params = serde_json::to_value(BootstrapRequest {
            session_id: session_id.to_string(),
        })
        .map_err(|e| e.to_string())?;

        let result = self.control_call("bootstrap", Some(params))?;
        serde_json::from_value(result)
            .map_err(|e| format!("Failed to parse bootstrap response: {}", e))
    }

    pub fn stop(&self, session_id: &str) -> Result<(), String> {
        let params = serde_json::to_value(StopRequest {
            session_id: session_id.to_string(),
        })
        .map_err(|e| e.to_string())?;

        self.control_call("stop", Some(params))?;
        Ok(())
    }

    /// Opens a long-lived attach stream. Returns the stream and the first bootstrap message.
    /// The caller should spawn a task to read from the stream in a loop.
    pub fn attach(
        &self,
        session_id: &str,
        stream_id: &str,
        since: i64,
        with_buffer: bool,
    ) -> Result<(BufReader<UnixStream>, StreamMessage), String> {
        let mut stream = self.connect()?;

        // Attach uses a longer read timeout since it's a long-lived stream
        stream
            .set_read_timeout(Some(Duration::from_secs(300)))
            .ok();

        let req = AttachRequest {
            protocol_version: PROTOCOL_VERSION,
            msg_type: "attach".to_string(),
            session_id: session_id.to_string(),
            stream_id: Some(stream_id.to_string()),
            since,
            with_buffer,
        };

        let mut payload = serde_json::to_string(&req)
            .map_err(|e| format!("Failed to serialize attach request: {}", e))?;
        payload.push('\n');

        stream
            .write_all(payload.as_bytes())
            .map_err(|e| format!("Failed to send attach request: {}", e))?;
        stream
            .flush()
            .map_err(|e| format!("Failed to flush: {}", e))?;

        let mut reader = BufReader::new(stream);
        let mut line = String::new();
        reader
            .read_line(&mut line)
            .map_err(|e| format!("Failed to read initial message: {}", e))?;

        let first: StreamMessage = serde_json::from_str(line.trim())
            .map_err(|e| format!("Failed to parse initial stream message: {}", e))?;

        Ok((reader, first))
    }
}

/// Read the next stream message from an attached stream.
pub fn read_stream_message(
    reader: &mut BufReader<UnixStream>,
) -> Result<StreamMessage, String> {
    let mut line = String::new();
    reader
        .read_line(&mut line)
        .map_err(|e| format!("Stream read error: {}", e))?;
    if line.is_empty() {
        return Err("Stream closed".to_string());
    }
    serde_json::from_str(line.trim())
        .map_err(|e| format!("Failed to parse stream message: {}", e))
}
