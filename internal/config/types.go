package config

type Defaults struct {
	BaseBranch    string      `yaml:"base_branch" json:"base_branch" mapstructure:"base_branch"`
	Workspace     string      `yaml:"workspace" json:"workspace" mapstructure:"workspace"`
	WorkspaceRoot string      `yaml:"workspace_root" json:"workspace_root" mapstructure:"workspace_root"`
	RepoStoreRoot string      `yaml:"repo_store_root" json:"repo_store_root" mapstructure:"repo_store_root"`
	Remotes       RemoteNames `yaml:"remotes" json:"remotes" mapstructure:"remotes"`
	Parallelism   int         `yaml:"parallelism" json:"parallelism" mapstructure:"parallelism"`
}

type RemoteNames struct {
	Base  string `yaml:"base" json:"base" mapstructure:"base"`
	Write string `yaml:"write" json:"write" mapstructure:"write"`
}

type RepoAlias struct {
	URL           string `yaml:"url,omitempty" json:"url,omitempty" mapstructure:"url"`
	Path          string `yaml:"path,omitempty" json:"path,omitempty" mapstructure:"path"`
	DefaultBranch string `yaml:"default_branch" json:"default_branch" mapstructure:"default_branch"`
}

type Group struct {
	Description string        `yaml:"description" json:"description" mapstructure:"description"`
	Members     []GroupMember `yaml:"members" json:"members" mapstructure:"members"`
}

type GroupMember struct {
	Repo    string  `yaml:"repo" json:"repo" mapstructure:"repo"`
	Remotes Remotes `yaml:"remotes" json:"remotes" mapstructure:"remotes"`
}

type WorkspaceRef struct {
	Path      string `yaml:"path" json:"path" mapstructure:"path"`
	CreatedAt string `yaml:"created_at,omitempty" json:"created_at,omitempty" mapstructure:"created_at"`
	LastUsed  string `yaml:"last_used,omitempty" json:"last_used,omitempty" mapstructure:"last_used"`
}

type GlobalConfig struct {
	Defaults   Defaults                `yaml:"defaults" json:"defaults" mapstructure:"defaults"`
	Repos      map[string]RepoAlias    `yaml:"repos" json:"repos" mapstructure:"repos"`
	Groups     map[string]Group        `yaml:"groups" json:"groups" mapstructure:"groups"`
	Workspaces map[string]WorkspaceRef `yaml:"workspaces" json:"workspaces" mapstructure:"workspaces"`
}

type WorkspaceConfig struct {
	Name  string       `yaml:"name" json:"name" mapstructure:"name"`
	Repos []RepoConfig `yaml:"repos" json:"repos" mapstructure:"repos"`
}

type RepoConfig struct {
	Name      string  `yaml:"name" json:"name" mapstructure:"name"`
	LocalPath string  `yaml:"local_path" json:"local_path" mapstructure:"local_path"`
	Managed   bool    `yaml:"managed,omitempty" json:"managed,omitempty" mapstructure:"managed"`
	RepoDir   string  `yaml:"repo_dir" json:"repo_dir" mapstructure:"repo_dir"`
	Remotes   Remotes `yaml:"remotes" json:"remotes" mapstructure:"remotes"`
}

type Remotes struct {
	Base  RemoteConfig `yaml:"base" json:"base" mapstructure:"base"`
	Write RemoteConfig `yaml:"write" json:"write" mapstructure:"write"`
}

type RemoteConfig struct {
	Name          string `yaml:"name" json:"name" mapstructure:"name"`
	DefaultBranch string `yaml:"default_branch,omitempty" json:"default_branch,omitempty" mapstructure:"default_branch"`
}
