use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;

use crate::types::layout::TerminalLayout;

/// Persists terminal layout per workspace path.
pub struct LayoutStore {
    dir: PathBuf,
    cache: HashMap<String, TerminalLayout>,
}

impl LayoutStore {
    pub fn new() -> Self {
        let dir = dirs::home_dir()
            .unwrap_or_default()
            .join(".workset")
            .join("layouts");
        fs::create_dir_all(&dir).ok();
        Self {
            dir,
            cache: HashMap::new(),
        }
    }

    fn path_for(&self, workspace_name: &str) -> PathBuf {
        let safe_name = workspace_name.replace('/', "__");
        self.dir.join(format!("{}.json", safe_name))
    }

    pub fn get(&mut self, workspace_name: &str) -> Option<TerminalLayout> {
        if let Some(layout) = self.cache.get(workspace_name) {
            return Some(layout.clone());
        }
        let path = self.path_for(workspace_name);
        if path.exists() {
            if let Ok(data) = fs::read_to_string(&path) {
                if let Ok(layout) = serde_json::from_str::<TerminalLayout>(&data) {
                    self.cache.insert(workspace_name.to_string(), layout.clone());
                    return Some(layout);
                }
            }
        }
        None
    }

    pub fn save(&mut self, workspace_name: &str, layout: &TerminalLayout) -> Result<(), String> {
        let path = self.path_for(workspace_name);
        let data = serde_json::to_string_pretty(layout)
            .map_err(|e| format!("Failed to serialize layout: {}", e))?;
        fs::write(&path, data).map_err(|e| format!("Failed to write layout: {}", e))?;
        self.cache.insert(workspace_name.to_string(), layout.clone());
        Ok(())
    }

    pub fn delete(&mut self, workspace_name: &str) {
        let path = self.path_for(workspace_name);
        fs::remove_file(path).ok();
        self.cache.remove(workspace_name);
    }
}

impl Default for LayoutStore {
    fn default() -> Self {
        Self::new()
    }
}
