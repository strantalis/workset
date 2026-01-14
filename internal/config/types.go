package config

type Defaults struct {
	BaseBranch  string      `yaml:"base_branch" mapstructure:"base_branch"`
	Remotes     RemoteNames `yaml:"remotes" mapstructure:"remotes"`
	Parallelism int         `yaml:"parallelism" mapstructure:"parallelism"`
}

type RemoteNames struct {
	Base  string `yaml:"base" mapstructure:"base"`
	Write string `yaml:"write" mapstructure:"write"`
}

type RepoAlias struct {
	URL           string `yaml:"url" mapstructure:"url"`
	DefaultBranch string `yaml:"default_branch" mapstructure:"default_branch"`
}

type Group struct {
	Description string        `yaml:"description" mapstructure:"description"`
	Members     []GroupMember `yaml:"members" mapstructure:"members"`
}

type GroupMember struct {
	Repo     string  `yaml:"repo" mapstructure:"repo"`
	Editable bool    `yaml:"editable" mapstructure:"editable"`
	Remotes  Remotes `yaml:"remotes" mapstructure:"remotes"`
}

type WorkspaceRef struct {
	Path      string `yaml:"path" mapstructure:"path"`
	CreatedAt string `yaml:"created_at,omitempty" mapstructure:"created_at"`
	LastUsed  string `yaml:"last_used,omitempty" mapstructure:"last_used"`
}

type GlobalConfig struct {
	Defaults   Defaults                `yaml:"defaults" mapstructure:"defaults"`
	Repos      map[string]RepoAlias    `yaml:"repos" mapstructure:"repos"`
	Groups     map[string]Group        `yaml:"groups" mapstructure:"groups"`
	Workspaces map[string]WorkspaceRef `yaml:"workspaces" mapstructure:"workspaces"`
}

type WorkspaceConfig struct {
	Name  string       `yaml:"name" mapstructure:"name"`
	Repos []RepoConfig `yaml:"repos" mapstructure:"repos"`
}

type RepoConfig struct {
	Name     string  `yaml:"name" mapstructure:"name"`
	RepoDir  string  `yaml:"repo_dir" mapstructure:"repo_dir"`
	Editable bool    `yaml:"editable" mapstructure:"editable"`
	Remotes  Remotes `yaml:"remotes" mapstructure:"remotes"`
}

type Remotes struct {
	Base  RemoteConfig `yaml:"base" mapstructure:"base"`
	Write RemoteConfig `yaml:"write" mapstructure:"write"`
}

type RemoteConfig struct {
	Name          string `yaml:"name" mapstructure:"name"`
	DefaultBranch string `yaml:"default_branch,omitempty" mapstructure:"default_branch"`
}
