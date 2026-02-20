use std::process::Command;

use crate::types::diff::FilePatch;

const MAX_PATCH_BYTES: u64 = 2 * 1024 * 1024; // 2MB
const MAX_PATCH_LINES: u32 = 20_000;

/// Extract a unified diff patch for a single file.
pub fn compute_file_patch(
    repo_path: &str,
    path: &str,
    _prev_path: Option<&str>,
    status: &str,
) -> Result<FilePatch, String> {
    let mut patch = String::new();

    if status == "A" || status == "untracked" {
        // Untracked file: use no-index diff
        let output = Command::new("git")
            .current_dir(repo_path)
            .args(["diff", "--no-index", "--unified=3", "--", "/dev/null", path])
            .output()
            .map_err(|e| format!("git diff --no-index failed: {}", e))?;
        // git diff --no-index returns exit code 1 for differences, which is expected
        patch = String::from_utf8_lossy(&output.stdout).to_string();
    } else {
        // Staged changes
        let staged = Command::new("git")
            .current_dir(repo_path)
            .args(["diff", "--cached", "--unified=3", "--", path])
            .output()
            .map_err(|e| format!("git diff --cached failed: {}", e))?;
        let staged_out = String::from_utf8_lossy(&staged.stdout);

        // Unstaged changes
        let unstaged = Command::new("git")
            .current_dir(repo_path)
            .args(["diff", "--unified=3", "--", path])
            .output()
            .map_err(|e| format!("git diff failed: {}", e))?;
        let unstaged_out = String::from_utf8_lossy(&unstaged.stdout);

        if !staged_out.is_empty() {
            patch.push_str(&staged_out);
        }
        if !unstaged_out.is_empty() {
            if !patch.is_empty() {
                patch.push('\n');
            }
            patch.push_str(&unstaged_out);
        }
    }

    let total_bytes = patch.len() as u64;
    let total_lines = patch.lines().count() as u32;

    // Check for binary
    let is_binary = patch.contains("Binary files") || patch.contains("GIT binary patch");

    let truncated = total_bytes > MAX_PATCH_BYTES || total_lines > MAX_PATCH_LINES;
    let final_patch = if truncated {
        let limit = MAX_PATCH_BYTES.min(total_bytes) as usize;
        patch[..limit].to_string()
    } else {
        patch
    };

    Ok(FilePatch {
        patch: final_patch,
        truncated,
        total_bytes,
        total_lines,
        binary: if is_binary { Some(true) } else { None },
    })
}
