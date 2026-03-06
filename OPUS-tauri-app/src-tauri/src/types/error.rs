use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ErrorEnvelope {
    pub category: String,
    pub operation: String,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<String>,
    pub retryable: bool,
    #[serde(default)]
    pub suggested_actions: Vec<String>,
}

impl ErrorEnvelope {
    pub fn new(category: &str, operation: &str, message: impl Into<String>) -> Self {
        Self {
            category: category.to_string(),
            operation: operation.to_string(),
            message: message.into(),
            details: None,
            retryable: false,
            suggested_actions: Vec::new(),
        }
    }

    pub fn with_details(mut self, details: impl Into<String>) -> Self {
        self.details = Some(details.into());
        self
    }

    pub fn retryable(mut self) -> Self {
        self.retryable = true;
        self
    }

    pub fn config(operation: &str, message: impl Into<String>) -> Self {
        Self::new("config", operation, message)
    }

    pub fn runtime(operation: &str, message: impl Into<String>) -> Self {
        Self::new("runtime", operation, message)
    }

    pub fn unknown(operation: &str, message: impl Into<String>) -> Self {
        Self::new("unknown", operation, message)
    }
}

impl std::fmt::Display for ErrorEnvelope {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "[{}] {}: {}", self.category, self.operation, self.message)
    }
}

impl std::error::Error for ErrorEnvelope {}
