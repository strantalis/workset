use std::path::PathBuf;

pub fn resolve_workset_cli(override_path: Option<&str>) -> Option<PathBuf> {
    if let Some(p) = override_path {
        let path = PathBuf::from(p);
        if path.exists() {
            return Some(path);
        }
    }
    if let Some(found) = which("workset") {
        return Some(found);
    }
    // macOS GUI apps inherit a minimal PATH; check common install locations
    for dir in &["go/bin", ".local/bin", "bin"] {
        if let Some(home) = std::env::var_os("HOME") {
            let candidate = PathBuf::from(home).join(dir).join("workset");
            if candidate.is_file() {
                return Some(candidate);
            }
        }
    }
    None
}

pub fn resolve_sessiond(override_path: Option<&str>) -> Option<PathBuf> {
    if let Some(p) = override_path {
        let path = PathBuf::from(p);
        if path.exists() {
            return Some(path);
        }
    }
    which("workset-sessiond")
}

fn which(binary: &str) -> Option<PathBuf> {
    std::env::var_os("PATH").and_then(|paths| {
        std::env::split_paths(&paths).find_map(|dir| {
            let full = dir.join(binary);
            if full.is_file() {
                Some(full)
            } else {
                None
            }
        })
    })
}
