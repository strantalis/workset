use crate::types::error::ErrorEnvelope;
use std::collections::HashMap;
use std::path::PathBuf;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
struct UiContextData {
    #[serde(skip_serializing_if = "Option::is_none")]
    active_workset_id: Option<String>,
    #[serde(default)]
    last_workspace_per_workset: HashMap<String, String>,
}

#[derive(Debug, Default)]
pub struct UiContextStore {
    data: UiContextData,
    path: PathBuf,
}

impl UiContextStore {
    pub fn load() -> Result<Self, ErrorEnvelope> {
        let path = Self::store_path();
        let data = if path.exists() {
            let raw = std::fs::read_to_string(&path).map_err(|e| {
                ErrorEnvelope::config("ui_context.load", format!("Failed to read: {e}"))
            })?;
            serde_json::from_str(&raw).unwrap_or_default()
        } else {
            UiContextData::default()
        };
        Ok(Self { data, path })
    }

    fn store_path() -> PathBuf {
        let base = dirs::home_dir().unwrap_or_else(|| PathBuf::from("."));
        base.join(".workset").join("ui_context.json")
    }

    fn save(&self) -> Result<(), ErrorEnvelope> {
        if let Some(parent) = self.path.parent() {
            std::fs::create_dir_all(parent).map_err(|e| {
                ErrorEnvelope::config("ui_context.save", format!("Failed to create dir: {e}"))
            })?;
        }
        let raw = serde_json::to_string_pretty(&self.data).map_err(|e| {
            ErrorEnvelope::config("ui_context.save", format!("Failed to serialize: {e}"))
        })?;
        std::fs::write(&self.path, raw).map_err(|e| {
            ErrorEnvelope::config("ui_context.save", format!("Failed to write: {e}"))
        })?;
        Ok(())
    }

    pub fn active_workset_id(&self) -> Option<&str> {
        self.data.active_workset_id.as_deref()
    }

    pub fn last_workspace_for(&self, workset_id: &str) -> Option<&str> {
        self.data.last_workspace_per_workset.get(workset_id).map(|s| s.as_str())
    }

    pub fn set_active_workset(&mut self, workset_id: &str) -> Result<(), ErrorEnvelope> {
        self.data.active_workset_id = Some(workset_id.to_string());
        self.save()
    }

    pub fn set_active_workspace(
        &mut self,
        workset_id: &str,
        workspace_name: &str,
    ) -> Result<(), ErrorEnvelope> {
        self.data
            .last_workspace_per_workset
            .insert(workset_id.to_string(), workspace_name.to_string());
        self.save()
    }
}
