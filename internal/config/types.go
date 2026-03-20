package config

type Defaults struct {
	Remote               string              `yaml:"remote" json:"remote" mapstructure:"remote"`
	BaseBranch           string              `yaml:"base_branch" json:"base_branch" mapstructure:"base_branch"`
	Thread               string              `yaml:"thread" json:"thread" mapstructure:"thread"`
	WorksetRoot          string              `yaml:"workset_root" json:"workset_root" mapstructure:"workset_root"`
	RepoStoreRoot        string              `yaml:"repo_store_root" json:"repo_store_root" mapstructure:"repo_store_root"`
	Agent                string              `yaml:"agent" json:"agent" mapstructure:"agent"`
	AgentModel           string              `yaml:"agent_model" json:"agent_model" mapstructure:"agent_model"`
	TerminalIdleTimeout  string              `yaml:"terminal_idle_timeout" json:"terminal_idle_timeout" mapstructure:"terminal_idle_timeout"`
	TerminalProtocolLog  string              `yaml:"terminal_protocol_log" json:"terminal_protocol_log" mapstructure:"terminal_protocol_log"`
	TerminalDebugOverlay string              `yaml:"terminal_debug_overlay" json:"terminal_debug_overlay" mapstructure:"terminal_debug_overlay"`
	TerminalFontSize     string              `yaml:"terminal_font_size" json:"terminal_font_size" mapstructure:"terminal_font_size"`
	TerminalCursorBlink  string              `yaml:"terminal_cursor_blink" json:"terminal_cursor_blink" mapstructure:"terminal_cursor_blink"`
	TerminalKeybindings  map[string][]string `yaml:"terminal_keybindings" json:"terminal_keybindings" mapstructure:"terminal_keybindings"`
}

type GitHubConfig struct {
	CLIPath string `yaml:"cli_path,omitempty" json:"cli_path,omitempty" mapstructure:"cli_path"`
}

type AgentConfig struct {
	CLIPath string `yaml:"cli_path,omitempty" json:"cli_path,omitempty" mapstructure:"cli_path"`
}

type RegisteredRepo struct {
	URL           string `yaml:"url,omitempty" json:"url,omitempty" mapstructure:"url"`
	Path          string `yaml:"path,omitempty" json:"path,omitempty" mapstructure:"path"`
	Remote        string `yaml:"remote,omitempty" json:"remote,omitempty" mapstructure:"remote"`
	DefaultBranch string `yaml:"default_branch" json:"default_branch" mapstructure:"default_branch"`
}

type WorkspaceRef struct {
	Path           string   `yaml:"path" json:"path" mapstructure:"path"`
	Workset        string   `yaml:"workset,omitempty" json:"workset,omitempty" mapstructure:"workset"`
	RepoOverrides  []string `yaml:"repo_overrides,omitempty" json:"repo_overrides,omitempty" mapstructure:"repo_overrides"`
	CreatedAt      string   `yaml:"created_at,omitempty" json:"created_at,omitempty" mapstructure:"created_at"`
	LastUsed       string   `yaml:"last_used,omitempty" json:"last_used,omitempty" mapstructure:"last_used"`
	ArchivedAt     string   `yaml:"archived_at,omitempty" json:"archived_at,omitempty" mapstructure:"archived_at"`
	ArchivedReason string   `yaml:"archived_reason,omitempty" json:"archived_reason,omitempty" mapstructure:"archived_reason"`
	Pinned         bool     `yaml:"pinned,omitempty" json:"pinned,omitempty" mapstructure:"pinned"`
	PinOrder       int      `yaml:"pin_order,omitempty" json:"pin_order,omitempty" mapstructure:"pin_order"`
	Color          string   `yaml:"color,omitempty" json:"color,omitempty" mapstructure:"color"`
	Description    string   `yaml:"description,omitempty" json:"description,omitempty" mapstructure:"description"`
	Expanded       bool     `yaml:"expanded,omitempty" json:"expanded,omitempty" mapstructure:"expanded"`
}

type GlobalConfig struct {
	ConfigVersion int                       `yaml:"config_version,omitempty" json:"config_version,omitempty" mapstructure:"config_version"`
	Defaults      Defaults                  `yaml:"defaults" json:"defaults" mapstructure:"defaults"`
	GitHub        GitHubConfig              `yaml:"github,omitempty" json:"github,omitempty" mapstructure:"github"`
	Agent         AgentConfig               `yaml:"agent,omitempty" json:"agent,omitempty" mapstructure:"agent"`
	Hooks         HooksConfig               `yaml:"hooks,omitempty" json:"hooks,omitempty" mapstructure:"hooks"`
	Repos         map[string]RegisteredRepo `yaml:"repos" json:"repos" mapstructure:"repos"`
	Workspaces    map[string]WorkspaceRef   `yaml:"worksets" json:"worksets" mapstructure:"worksets"`
	WorksetRepos  map[string][]string       `yaml:"-" json:"-" mapstructure:"-"`
}

type WorkspaceConfig struct {
	Name  string       `yaml:"name" json:"name" mapstructure:"name"`
	Repos []RepoConfig `yaml:"repos" json:"repos" mapstructure:"repos"`
}

type RepoConfig struct {
	Name      string `yaml:"name" json:"name" mapstructure:"name"`
	LocalPath string `yaml:"local_path" json:"local_path" mapstructure:"local_path"`
	Managed   bool   `yaml:"managed,omitempty" json:"managed,omitempty" mapstructure:"managed"`
	RepoDir   string `yaml:"repo_dir" json:"repo_dir" mapstructure:"repo_dir"`
}

type HooksConfig struct {
	Enabled   bool            `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
	OnError   string          `yaml:"on_error,omitempty" json:"on_error,omitempty" mapstructure:"on_error"`
	RepoHooks RepoHooksConfig `yaml:"repo_hooks,omitempty" json:"repo_hooks,omitempty" mapstructure:"repo_hooks"`
	Items     []HookSpec      `yaml:"items,omitempty" json:"items,omitempty" mapstructure:"items"`
}

type RepoHooksConfig struct {
	TrustedRepos []string `yaml:"trusted_repos,omitempty" json:"trusted_repos,omitempty" mapstructure:"trusted_repos"`
}

type HookSpec struct {
	ID      string            `yaml:"id" json:"id" mapstructure:"id"`
	On      []string          `yaml:"on" json:"on" mapstructure:"on"`
	Run     []string          `yaml:"run" json:"run" mapstructure:"run"`
	Cwd     string            `yaml:"cwd,omitempty" json:"cwd,omitempty" mapstructure:"cwd"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty" mapstructure:"env"`
	OnError string            `yaml:"on_error,omitempty" json:"on_error,omitempty" mapstructure:"on_error"`
}
