use std::process::Command;

use crate::types::error::ErrorEnvelope;

#[derive(serde::Serialize)]
pub struct GitHubRepo {
    pub full_name: String,
    pub description: Option<String>,
    pub private: bool,
}

/// Fetch all repos the authenticated user has access to (personal, org member,
/// collaborator) sorted by most recently pushed. Results are cached and
/// filtered client-side for instant autocomplete.
#[tauri::command]
pub async fn github_list_repos() -> Result<Vec<GitHubRepo>, ErrorEnvelope> {
    tokio::task::spawn_blocking(|| {
        let output = Command::new("gh")
            .args([
                "api",
                "--method",
                "GET",
                "/user/repos",
                "--paginate",
                "--jq",
                ".[] | {full_name, description, private}",
                "-f",
                "per_page=100",
                "-f",
                "sort=pushed",
                "-f",
                "type=all",
            ])
            .output()
            .map_err(|e| {
                ErrorEnvelope::new(
                    "auth",
                    "github.list_repos",
                    format!("gh CLI not found: {e}"),
                )
            })?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            if stderr.contains("not logged") || stderr.contains("auth login") {
                return Err(ErrorEnvelope::new(
                    "auth",
                    "github.list_repos",
                    "Not authenticated with GitHub. Run `gh auth login` in a terminal.",
                ));
            }
            return Err(ErrorEnvelope::runtime(
                "github.list_repos",
                format!("gh api failed: {stderr}"),
            ));
        }

        let stdout = String::from_utf8_lossy(&output.stdout);

        let mut repos = Vec::new();
        for line in stdout.lines() {
            let line = line.trim();
            if line.is_empty() {
                continue;
            }
            #[derive(serde::Deserialize)]
            struct Row {
                full_name: String,
                description: Option<String>,
                private: bool,
            }
            if let Ok(row) = serde_json::from_str::<Row>(line) {
                repos.push(GitHubRepo {
                    full_name: row.full_name,
                    description: row.description,
                    private: row.private,
                });
            }
        }

        Ok(repos)
    })
    .await
    .map_err(|e| ErrorEnvelope::runtime("github.list_repos", format!("Task failed: {e}")))?
}

#[tauri::command]
pub async fn github_auth_status() -> Result<serde_json::Value, ErrorEnvelope> {
    tokio::task::spawn_blocking(|| {
        let output = Command::new("gh")
            .args(["auth", "status", "--hostname", "github.com"])
            .output();

        match output {
            Ok(out) => {
                let authenticated = out.status.success();
                let message = if authenticated {
                    String::from_utf8_lossy(&out.stdout).trim().to_string()
                } else {
                    String::from_utf8_lossy(&out.stderr).trim().to_string()
                };

                Ok(serde_json::json!({
                    "available": true,
                    "authenticated": authenticated,
                    "message": message,
                }))
            }
            Err(_) => Ok(serde_json::json!({
                "available": false,
                "authenticated": false,
                "message": "gh CLI not installed",
            })),
        }
    })
    .await
    .map_err(|e| ErrorEnvelope::runtime("github.auth_status", format!("Task failed: {e}")))?
}

#[derive(serde::Serialize)]
pub struct GitHubAccount {
    pub login: String,
    pub active: bool,
}

#[tauri::command]
pub async fn github_list_accounts() -> Result<Vec<GitHubAccount>, ErrorEnvelope> {
    tokio::task::spawn_blocking(|| {
        let output = Command::new("gh")
            .args(["auth", "status", "--json", "hosts"])
            .output()
            .map_err(|e| {
                ErrorEnvelope::new(
                    "auth",
                    "github.list_accounts",
                    format!("gh CLI not found: {e}"),
                )
            })?;

        if !output.status.success() {
            return Ok(Vec::new());
        }

        let stdout = String::from_utf8_lossy(&output.stdout);

        #[derive(serde::Deserialize)]
        struct AccountEntry {
            login: String,
            active: bool,
        }

        #[derive(serde::Deserialize)]
        struct AuthStatus {
            hosts: std::collections::HashMap<String, Vec<AccountEntry>>,
        }

        let status: AuthStatus = serde_json::from_str(&stdout).map_err(|e| {
            ErrorEnvelope::runtime(
                "github.list_accounts",
                format!("Failed to parse gh output: {e}"),
            )
        })?;

        let mut accounts = Vec::new();
        for (_host, entries) in status.hosts {
            for entry in entries {
                accounts.push(GitHubAccount {
                    login: entry.login,
                    active: entry.active,
                });
            }
        }

        accounts.sort_by(|a, b| b.active.cmp(&a.active));

        Ok(accounts)
    })
    .await
    .map_err(|e| {
        ErrorEnvelope::runtime("github.list_accounts", format!("Task failed: {e}"))
    })?
}

#[tauri::command]
pub async fn github_switch_account(user: String) -> Result<(), ErrorEnvelope> {
    tokio::task::spawn_blocking(move || {
        let output = Command::new("gh")
            .args(["auth", "switch", "--user", &user])
            .output()
            .map_err(|e| {
                ErrorEnvelope::new(
                    "auth",
                    "github.switch_account",
                    format!("gh CLI not found: {e}"),
                )
            })?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(ErrorEnvelope::runtime(
                "github.switch_account",
                format!("Failed to switch account: {stderr}"),
            ));
        }

        Ok(())
    })
    .await
    .map_err(|e| {
        ErrorEnvelope::runtime("github.switch_account", format!("Task failed: {e}"))
    })?
}
