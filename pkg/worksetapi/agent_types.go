package worksetapi

// AgentCLIStatusJSON describes local availability for an agent command.
type AgentCLIStatusJSON struct {
	Installed      bool   `json:"installed"`
	Path           string `json:"path,omitempty"`
	ConfiguredPath string `json:"configuredPath,omitempty"`
	Command        string `json:"command"`
	Error          string `json:"error,omitempty"`
}
