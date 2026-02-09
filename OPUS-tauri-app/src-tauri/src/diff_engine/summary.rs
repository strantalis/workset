use std::collections::HashMap;
use std::process::Command;

use crate::types::diff::{DiffFileSummary, DiffSummary};

/// Compute a diff summary for a repo worktree, combining staged + unstaged + untracked.
pub fn compute_diff_summary(repo_path: &str) -> Result<DiffSummary, String> {
    let mut file_map: HashMap<String, DiffFileSummary> = HashMap::new();

    // 1. Staged name-status
    parse_name_status(repo_path, true, &mut file_map)?;
    // 2. Unstaged name-status
    parse_name_status(repo_path, false, &mut file_map)?;
    // 3. Staged numstat
    parse_numstat(repo_path, true, &mut file_map)?;
    // 4. Unstaged numstat
    parse_numstat(repo_path, false, &mut file_map)?;
    // 5. Untracked files
    add_untracked(repo_path, &mut file_map)?;

    let mut files: Vec<DiffFileSummary> = file_map.into_values().collect();
    files.sort_by(|a, b| a.path.cmp(&b.path));

    let total_added: u32 = files.iter().map(|f| f.added).sum();
    let total_removed: u32 = files.iter().map(|f| f.removed).sum();

    Ok(DiffSummary {
        files,
        total_added,
        total_removed,
    })
}

fn parse_name_status(
    repo_path: &str,
    cached: bool,
    map: &mut HashMap<String, DiffFileSummary>,
) -> Result<(), String> {
    let mut cmd = Command::new("git");
    cmd.current_dir(repo_path)
        .args(["diff", "--name-status", "--find-renames", "-z"]);
    if cached {
        cmd.arg("--cached");
    }

    let output = cmd.output().map_err(|e| format!("git diff failed: {}", e))?;
    let stdout = String::from_utf8_lossy(&output.stdout);

    let parts: Vec<&str> = stdout.split('\0').collect();
    let mut i = 0;
    while i < parts.len() {
        let status_str = parts[i].trim();
        if status_str.is_empty() {
            i += 1;
            continue;
        }
        let status_char = status_str.chars().next().unwrap_or('M');
        let status = match status_char {
            'A' => "A",
            'D' => "D",
            'R' | 'C' => "R",
            _ => "M",
        };

        if status_char == 'R' || status_char == 'C' {
            // Rename/copy: next two entries are old_path and new_path
            if i + 2 < parts.len() {
                let prev_path = parts[i + 1].to_string();
                let path = parts[i + 2].to_string();
                map.entry(path.clone()).or_insert(DiffFileSummary {
                    path,
                    prev_path: Some(prev_path),
                    added: 0,
                    removed: 0,
                    status: status.to_string(),
                    binary: None,
                });
                i += 3;
            } else {
                break;
            }
        } else {
            if i + 1 < parts.len() {
                let path = parts[i + 1].to_string();
                map.entry(path.clone()).or_insert(DiffFileSummary {
                    path,
                    prev_path: None,
                    added: 0,
                    removed: 0,
                    status: status.to_string(),
                    binary: None,
                });
                i += 2;
            } else {
                break;
            }
        }
    }
    Ok(())
}

fn parse_numstat(
    repo_path: &str,
    cached: bool,
    map: &mut HashMap<String, DiffFileSummary>,
) -> Result<(), String> {
    let mut cmd = Command::new("git");
    cmd.current_dir(repo_path)
        .args(["diff", "--numstat", "--find-renames", "-z"]);
    if cached {
        cmd.arg("--cached");
    }

    let output = cmd.output().map_err(|e| format!("git numstat failed: {}", e))?;
    let stdout = String::from_utf8_lossy(&output.stdout);

    for line in stdout.lines() {
        let parts: Vec<&str> = line.split('\t').collect();
        if parts.len() >= 3 {
            let added: u32 = parts[0].parse().unwrap_or(0);
            let removed: u32 = parts[1].parse().unwrap_or(0);
            let path = parts[2].trim_matches('\0').to_string();

            if parts[0] == "-" && parts[1] == "-" {
                // Binary file
                if let Some(entry) = map.get_mut(&path) {
                    entry.binary = Some(true);
                }
            } else if let Some(entry) = map.get_mut(&path) {
                entry.added += added;
                entry.removed += removed;
            }
        }
    }
    Ok(())
}

fn add_untracked(
    repo_path: &str,
    map: &mut HashMap<String, DiffFileSummary>,
) -> Result<(), String> {
    let output = Command::new("git")
        .current_dir(repo_path)
        .args(["ls-files", "--others", "--exclude-standard", "-z"])
        .output()
        .map_err(|e| format!("git ls-files failed: {}", e))?;

    let stdout = String::from_utf8_lossy(&output.stdout);
    let untracked: Vec<&str> = stdout.split('\0').filter(|s| !s.is_empty()).collect();

    // Batch numstat for untracked files
    if !untracked.is_empty() {
        for chunk in untracked.chunks(200) {
            let mut cmd = Command::new("git");
            cmd.current_dir(repo_path)
                .args(["diff", "--no-index", "--numstat", "-z", "--"]);
            for path in chunk {
                cmd.arg("/dev/null").arg(path);
            }
            let result = cmd.output();
            match result {
                Ok(out) => {
                    let s = String::from_utf8_lossy(&out.stdout);
                    for line in s.lines() {
                        let parts: Vec<&str> = line.split('\t').collect();
                        if parts.len() >= 3 {
                            let added: u32 = parts[0].parse().unwrap_or(0);
                            let path = parts[2]
                                .trim_matches('\0')
                                .trim_start_matches("./")
                                .to_string();
                            map.entry(path.clone()).or_insert(DiffFileSummary {
                                path,
                                prev_path: None,
                                added,
                                removed: 0,
                                status: "A".to_string(),
                                binary: None,
                            });
                        }
                    }
                }
                Err(_) => {
                    // Fallback: just list them without stats
                    for path in chunk {
                        let p = path.to_string();
                        map.entry(p.clone()).or_insert(DiffFileSummary {
                            path: p,
                            prev_path: None,
                            added: 0,
                            removed: 0,
                            status: "A".to_string(),
                            binary: None,
                        });
                    }
                }
            }
        }
    }
    Ok(())
}
