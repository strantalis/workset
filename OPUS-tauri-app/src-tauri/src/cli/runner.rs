use crate::types::error::ErrorEnvelope;
use std::process::Command;

pub fn run_workset_command(cli_path: &str, args: &[&str]) -> Result<String, ErrorEnvelope> {
    let output = Command::new(cli_path)
        .args(args)
        .output()
        .map_err(|e| {
            ErrorEnvelope::runtime("cli.run", format!("Failed to execute workset CLI: {e}"))
                .with_details(format!("Path: {cli_path}, Args: {args:?}"))
        })?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr).to_string();
        return Err(
            ErrorEnvelope::new("git", "cli.run", format!("CLI command failed: {stderr}"))
                .with_details(format!("Exit code: {:?}", output.status.code()))
                .retryable(),
        );
    }

    Ok(String::from_utf8_lossy(&output.stdout).to_string())
}

pub fn run_workset_json<T: serde::de::DeserializeOwned>(
    cli_path: &str,
    args: &[&str],
) -> Result<T, ErrorEnvelope> {
    let stdout = run_workset_command(cli_path, args)?;
    serde_json::from_str(&stdout).map_err(|e| {
        ErrorEnvelope::runtime("cli.parse", format!("Failed to parse CLI output: {e}"))
            .with_details(format!("Raw output: {}", &stdout[..stdout.len().min(500)]))
    })
}
