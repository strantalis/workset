use std::collections::HashMap;

use crate::jobs::migration::MigrationJobHandle;
use crate::types::job::MigrationProgress;

/// Tracks active and completed migration jobs.
pub struct MigrationStore {
    pub active_jobs: HashMap<String, MigrationJobHandle>,
    pub history: Vec<MigrationProgress>,
}

impl MigrationStore {
    pub fn new() -> Self {
        Self {
            active_jobs: HashMap::new(),
            history: Vec::new(),
        }
    }
}

impl Default for MigrationStore {
    fn default() -> Self {
        Self::new()
    }
}
