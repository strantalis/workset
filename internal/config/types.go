package config

type Defaults struct {
	Remote               string `yaml:"remote" json:"remote" mapstructure:"remote"`
	BaseBranch           string `yaml:"base_branch" json:"base_branch" mapstructure:"base_branch"`
	Workspace            string `yaml:"workspace" json:"workspace" mapstructure:"workspace"`
	WorkspaceRoot        string `yaml:"workspace_root" json:"workspace_root" mapstructure:"workspace_root"`
	RepoStoreRoot        string `yaml:"repo_store_root" json:"repo_store_root" mapstructure:"repo_store_root"`
	SessionBackend       string `yaml:"session_backend" json:"session_backend" mapstructure:"session_backend"`
	SessionNameFormat    string `yaml:"session_name_format" json:"session_name_format" mapstructure:"session_name_format"`
	SessionTheme         string `yaml:"session_theme" json:"session_theme" mapstructure:"session_theme"`
	SessionTmuxStyle     string `yaml:"session_tmux_status_style" json:"session_tmux_status_style" mapstructure:"session_tmux_status_style"`
	SessionTmuxLeft      string `yaml:"session_tmux_status_left" json:"session_tmux_status_left" mapstructure:"session_tmux_status_left"`
	SessionTmuxRight     string `yaml:"session_tmux_status_right" json:"session_tmux_status_right" mapstructure:"session_tmux_status_right"`
	SessionScreenHard    string `yaml:"session_screen_hardstatus" json:"session_screen_hardstatus" mapstructure:"session_screen_hardstatus"`
	Agent                string `yaml:"agent" json:"agent" mapstructure:"agent"`
	AgentLaunch          string `yaml:"agent_launch" json:"agent_launch" mapstructure:"agent_launch"`
	TerminalRenderer     string `yaml:"terminal_renderer" json:"terminal_renderer" mapstructure:"terminal_renderer"`
	TerminalIdleTimeout  string `yaml:"terminal_idle_timeout" json:"terminal_idle_timeout" mapstructure:"terminal_idle_timeout"`
	TerminalProtocolLog  string `yaml:"terminal_protocol_log" json:"terminal_protocol_log" mapstructure:"terminal_protocol_log"`
	TerminalDebugOverlay string `yaml:"terminal_debug_overlay" json:"terminal_debug_overlay" mapstructure:"terminal_debug_overlay"`
}

type GitHubConfig struct {
	CLIPath string `yaml:"cli_path,omitempty" json:"cli_path,omitempty" mapstructure:"cli_path"`
}

type AgentConfig struct {
	CLIPath string `yaml:"cli_path,omitempty" json:"cli_path,omitempty" mapstructure:"cli_path"`
}

type RepoAlias struct {
	URL           string `yaml:"url,omitempty" json:"url,omitempty" mapstructure:"url"`
	Path          string `yaml:"path,omitempty" json:"path,omitempty" mapstructure:"path"`
	Remote        string `yaml:"remote,omitempty" json:"remote,omitempty" mapstructure:"remote"`
	DefaultBranch string `yaml:"default_branch" json:"default_branch" mapstructure:"default_branch"`
}

type Group struct {
	Description string        `yaml:"description" json:"description" mapstructure:"description"`
	Members     []GroupMember `yaml:"members" json:"members" mapstructure:"members"`
}

type GroupMember struct {
	Repo          string   `yaml:"repo" json:"repo" mapstructure:"repo"`
	LegacyRemotes *Remotes `yaml:"remotes,omitempty" json:"-" mapstructure:"remotes"`
}

type WorkspaceRef struct {
	Path           string `yaml:"path" json:"path" mapstructure:"path"`
	CreatedAt      string `yaml:"created_at,omitempty" json:"created_at,omitempty" mapstructure:"created_at"`
	LastUsed       string `yaml:"last_used,omitempty" json:"last_used,omitempty" mapstructure:"last_used"`
	ArchivedAt     string `yaml:"archived_at,omitempty" json:"archived_at,omitempty" mapstructure:"archived_at"`
	ArchivedReason string `yaml:"archived_reason,omitempty" json:"archived_reason,omitempty" mapstructure:"archived_reason"`
}

type GlobalConfig struct {
	Defaults   Defaults                `yaml:"defaults" json:"defaults" mapstructure:"defaults"`
	GitHub     GitHubConfig            `yaml:"github,omitempty" json:"github,omitempty" mapstructure:"github"`
	Agent      AgentConfig             `yaml:"agent,omitempty" json:"agent,omitempty" mapstructure:"agent"`
	Hooks      HooksConfig             `yaml:"hooks,omitempty" json:"hooks,omitempty" mapstructure:"hooks"`
	Repos      map[string]RepoAlias    `yaml:"repos" json:"repos" mapstructure:"repos"`
	Groups     map[string]Group        `yaml:"groups" json:"groups" mapstructure:"groups"`
	Workspaces map[string]WorkspaceRef `yaml:"workspaces" json:"workspaces" mapstructure:"workspaces"`
}

type WorkspaceConfig struct {
	Name  string       `yaml:"name" json:"name" mapstructure:"name"`
	Repos []RepoConfig `yaml:"repos" json:"repos" mapstructure:"repos"`
}

type RepoConfig struct {
	Name          string   `yaml:"name" json:"name" mapstructure:"name"`
	LocalPath     string   `yaml:"local_path" json:"local_path" mapstructure:"local_path"`
	Managed       bool     `yaml:"managed,omitempty" json:"managed,omitempty" mapstructure:"managed"`
	RepoDir       string   `yaml:"repo_dir" json:"repo_dir" mapstructure:"repo_dir"`
	LegacyRemotes *Remotes `yaml:"remotes,omitempty" json:"-" mapstructure:"remotes"`
}

type Remotes struct {
	Base  RemoteConfig `yaml:"base" json:"base" mapstructure:"base"`
	Write RemoteConfig `yaml:"write" json:"write" mapstructure:"write"`
}

type RemoteConfig struct {
	Name          string `yaml:"name" json:"name" mapstructure:"name"`
	DefaultBranch string `yaml:"default_branch,omitempty" json:"default_branch,omitempty" mapstructure:"default_branch"`
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
