use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TerminalLayout {
    pub version: u32,
    pub root: LayoutNode,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub focused_pane_id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "kind")]
pub enum LayoutNode {
    #[serde(rename = "pane")]
    Pane {
        id: String,
        tabs: Vec<LayoutTab>,
        #[serde(skip_serializing_if = "Option::is_none")]
        active_tab_id: Option<String>,
    },
    #[serde(rename = "split")]
    Split {
        id: String,
        direction: String,
        ratio: f64,
        first: Box<LayoutNode>,
        second: Box<LayoutNode>,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LayoutTab {
    pub id: String,
    pub terminal_id: String,
    pub title: String,
    pub kind: String,
}
