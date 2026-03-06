use crate::types::error::ErrorEnvelope;
use crate::types::workset::{WorksetDefaults, WorksetProfile};
use std::path::PathBuf;

#[derive(Debug, Default)]
pub struct WorksetProfileStore {
    profiles: Vec<WorksetProfile>,
    path: PathBuf,
}

impl WorksetProfileStore {
    pub fn load() -> Result<Self, ErrorEnvelope> {
        let path = Self::store_path();
        let profiles = if path.exists() {
            let data = std::fs::read_to_string(&path).map_err(|e| {
                ErrorEnvelope::config("workset_profiles.load", format!("Failed to read store: {e}"))
            })?;
            serde_json::from_str(&data).unwrap_or_default()
        } else {
            Vec::new()
        };
        Ok(Self { profiles, path })
    }

    fn store_path() -> PathBuf {
        let base = dirs::home_dir().unwrap_or_else(|| PathBuf::from("."));
        base.join(".workset").join("ui_worksets.json")
    }

    fn save(&self) -> Result<(), ErrorEnvelope> {
        if let Some(parent) = self.path.parent() {
            std::fs::create_dir_all(parent).map_err(|e| {
                ErrorEnvelope::config("workset_profiles.save", format!("Failed to create dir: {e}"))
            })?;
        }
        let data = serde_json::to_string_pretty(&self.profiles).map_err(|e| {
            ErrorEnvelope::config("workset_profiles.save", format!("Failed to serialize: {e}"))
        })?;
        std::fs::write(&self.path, data).map_err(|e| {
            ErrorEnvelope::config("workset_profiles.save", format!("Failed to write: {e}"))
        })?;
        Ok(())
    }

    pub fn list(&self) -> Vec<WorksetProfile> {
        self.profiles.clone()
    }

    pub fn create(
        &mut self,
        name: &str,
        defaults: Option<WorksetDefaults>,
    ) -> Result<WorksetProfile, ErrorEnvelope> {
        let id = self.slugify_unique(name);
        let now = chrono::Utc::now().to_rfc3339();
        let profile = WorksetProfile {
            id,
            name: name.to_string(),
            repos: Vec::new(),
            workspace_ids: Vec::new(),
            defaults,
            created_at: now.clone(),
            updated_at: now,
        };
        self.profiles.push(profile.clone());
        self.save()?;
        Ok(profile)
    }

    pub fn update(
        &mut self,
        id: &str,
        name: Option<&str>,
        defaults: Option<WorksetDefaults>,
    ) -> Result<WorksetProfile, ErrorEnvelope> {
        let profile = self.profiles.iter_mut().find(|p| p.id == id).ok_or_else(|| {
            ErrorEnvelope::config("workset_profiles.update", format!("Workset '{id}' not found"))
        })?;
        if let Some(n) = name {
            profile.name = n.to_string();
        }
        if let Some(d) = defaults {
            profile.defaults = Some(d);
        }
        profile.updated_at = chrono::Utc::now().to_rfc3339();
        let result = profile.clone();
        self.save()?;
        Ok(result)
    }

    pub fn delete(&mut self, id: &str) -> Result<(), ErrorEnvelope> {
        let before = self.profiles.len();
        self.profiles.retain(|p| p.id != id);
        if self.profiles.len() == before {
            return Err(ErrorEnvelope::config(
                "workset_profiles.delete",
                format!("Workset '{id}' not found"),
            ));
        }
        self.save()?;
        Ok(())
    }

    pub fn add_repo(&mut self, workset_id: &str, source: &str) -> Result<WorksetProfile, ErrorEnvelope> {
        let profile = self.profiles.iter_mut().find(|p| p.id == workset_id).ok_or_else(|| {
            ErrorEnvelope::config("workset_profiles.add_repo", format!("Workset '{workset_id}' not found"))
        })?;
        if !profile.repos.contains(&source.to_string()) {
            profile.repos.push(source.to_string());
            profile.updated_at = chrono::Utc::now().to_rfc3339();
        }
        let result = profile.clone();
        self.save()?;
        Ok(result)
    }

    pub fn remove_repo(&mut self, workset_id: &str, source: &str) -> Result<WorksetProfile, ErrorEnvelope> {
        let profile = self.profiles.iter_mut().find(|p| p.id == workset_id).ok_or_else(|| {
            ErrorEnvelope::config("workset_profiles.remove_repo", format!("Workset '{workset_id}' not found"))
        })?;
        profile.repos.retain(|r| r != source);
        profile.updated_at = chrono::Utc::now().to_rfc3339();
        let result = profile.clone();
        self.save()?;
        Ok(result)
    }

    pub fn add_workspace(&mut self, workset_id: &str, workspace_name: &str) -> Result<(), ErrorEnvelope> {
        let profile = self.profiles.iter_mut().find(|p| p.id == workset_id).ok_or_else(|| {
            ErrorEnvelope::config("workset_profiles.add_workspace", format!("Workset '{workset_id}' not found"))
        })?;
        if !profile.workspace_ids.contains(&workspace_name.to_string()) {
            profile.workspace_ids.push(workspace_name.to_string());
            profile.updated_at = chrono::Utc::now().to_rfc3339();
        }
        self.save()
    }

    pub fn remove_workspace(&mut self, workset_id: &str, workspace_name: &str) -> Result<(), ErrorEnvelope> {
        let profile = self.profiles.iter_mut().find(|p| p.id == workset_id).ok_or_else(|| {
            ErrorEnvelope::config("workset_profiles.remove_workspace", format!("Workset '{workset_id}' not found"))
        })?;
        profile.workspace_ids.retain(|w| w != workspace_name);
        profile.updated_at = chrono::Utc::now().to_rfc3339();
        self.save()
    }

    fn slugify_unique(&self, name: &str) -> String {
        let base: String = name
            .to_lowercase()
            .chars()
            .map(|c| if c.is_alphanumeric() { c } else { '-' })
            .collect::<String>()
            .trim_matches('-')
            .to_string();
        let base = if base.is_empty() {
            "workset".to_string()
        } else {
            base
        };
        if !self.profiles.iter().any(|p| p.id == base) {
            return base;
        }
        for i in 2.. {
            let candidate = format!("{base}-{i}");
            if !self.profiles.iter().any(|p| p.id == candidate) {
                return candidate;
            }
        }
        unreachable!()
    }
}
