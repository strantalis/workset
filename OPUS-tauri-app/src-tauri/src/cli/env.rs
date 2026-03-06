use crate::types::diagnostics::EnvSnapshot;
use crate::types::error::ErrorEnvelope;
use std::collections::HashMap;
use std::process::Command;

pub fn capture_env_snapshot() -> EnvSnapshot {
    EnvSnapshot {
        path: std::env::var("PATH").unwrap_or_default(),
        shell: std::env::var("SHELL").unwrap_or_else(|_| "/bin/zsh".into()),
        home: std::env::var("HOME").unwrap_or_default(),
        ssh_auth_sock: std::env::var("SSH_AUTH_SOCK").ok(),
        git_ssh_command: std::env::var("GIT_SSH_COMMAND").ok(),
        git_askpass: std::env::var("GIT_ASKPASS").ok(),
        gh_config_dir: std::env::var("GH_CONFIG_DIR").ok(),
        gh_auth_summary: None,
    }
}

pub fn reload_login_env() -> Result<HashMap<String, String>, ErrorEnvelope> {
    let shell = std::env::var("SHELL").unwrap_or_else(|_| "/bin/zsh".into());
    let output = Command::new(&shell)
        .args(["-l", "-c", "env"])
        .output()
        .map_err(|e| {
            ErrorEnvelope::runtime("reload_login_env", format!("Failed to run login shell: {e}"))
        })?;

    if !output.status.success() {
        return Err(ErrorEnvelope::runtime(
            "reload_login_env",
            "Login shell exited with non-zero status",
        ));
    }

    let stdout = String::from_utf8_lossy(&output.stdout);
    let mut env_map = HashMap::new();
    for line in stdout.lines() {
        if let Some((key, value)) = line.split_once('=') {
            match key {
                "PATH" | "SSH_AUTH_SOCK" | "GIT_SSH_COMMAND" | "GIT_ASKPASS"
                | "GH_CONFIG_DIR" | "HOME" => {
                    std::env::set_var(key, value);
                    env_map.insert(key.to_string(), value.to_string());
                }
                _ => {}
            }
        }
    }
    Ok(env_map)
}
