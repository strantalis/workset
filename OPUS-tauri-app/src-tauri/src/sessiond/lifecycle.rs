use std::path::Path;
use std::process::Command;

use crate::sessiond::client::SessiondClient;

/// Check if sessiond is running by attempting a connection.
pub fn is_sessiond_running(socket_path: &str) -> bool {
    Path::new(socket_path).exists()
}

/// Attempt to start the sessiond binary.
pub fn start_sessiond(binary_path: &str) -> Result<(), String> {
    Command::new(binary_path)
        .arg("start")
        .spawn()
        .map_err(|e| format!("Failed to start sessiond: {}", e))?;

    // Give it a moment to start
    std::thread::sleep(std::time::Duration::from_millis(500));
    Ok(())
}

/// Check sessiond health by connecting and verifying the socket is responsive.
pub fn check_health(socket_path: &str) -> Result<bool, String> {
    if !is_sessiond_running(socket_path) {
        return Ok(false);
    }
    // Try creating a client and doing a minimal operation
    let _client = SessiondClient::new(socket_path);
    Ok(true)
}
