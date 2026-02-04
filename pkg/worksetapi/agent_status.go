package worksetapi

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

// GetAgentCLIStatus reports whether the configured agent command is available.
func (s *Service) GetAgentCLIStatus(ctx context.Context, agent string) (AgentCLIStatusJSON, error) {
	fields := strings.Fields(strings.TrimSpace(agent))
	if len(fields) == 0 {
		return AgentCLIStatusJSON{}, ValidationError{Message: "agent command required"}
	}
	command := fields[0]
	cfg, _, err := s.loadGlobal(ctx)
	if err != nil {
		return AgentCLIStatusJSON{}, err
	}
	configuredPath := normalizeCLIPath(cfg.Agent.CLIPath)
	status := AgentCLIStatusJSON{
		Command:        command,
		ConfiguredPath: configuredPath,
	}

	launch := normalizeAgentLaunchMode(cfg.Defaults.AgentLaunch)
	if launch == agentLaunchStrict {
		if configuredPath != "" {
			if isExecutableCandidate(configuredPath) {
				if filepath.Base(configuredPath) == filepath.Base(command) {
					status.Installed = true
					status.Path = configuredPath
					return status, nil
				}
			} else {
				status.Error = "Configured agent path is not executable"
			}
		}
		if hasPathSeparator(command) {
			normalizedCommand := normalizeCLIPath(command)
			if isExecutableCandidate(normalizedCommand) {
				status.Installed = true
				status.Path = filepath.Clean(normalizedCommand)
				return status, nil
			}
			status.Error = "agent command is not executable: " + normalizedCommand
			return status, nil
		}
		if status.Error == "" {
			status.Error = "strict agent launch requires a path with directory separators or agent CLI path"
		}
		return status, nil
	}

	if configuredPath != "" {
		if isExecutableCandidate(configuredPath) {
			if filepath.Base(configuredPath) == filepath.Base(command) {
				status.Installed = true
				status.Path = configuredPath
				return status, nil
			}
		} else {
			status.Error = "Configured agent path is not executable"
		}
	}
	path := resolveCLIPath(command)
	if path == "" {
		if status.Error == "" {
			status.Error = "agent command not found: " + command
		}
		return status, nil
	}
	status.Installed = true
	status.Path = path
	return status, nil
}

// SetAgentCLIPath stores an explicit path to the agent CLI binary.
func (s *Service) SetAgentCLIPath(ctx context.Context, agent, path string) (AgentCLIStatusJSON, error) {
	path = normalizeCLIPath(path)
	if path != "" && !isExecutableCandidate(path) {
		return AgentCLIStatusJSON{}, ValidationError{Message: "Agent CLI path is not executable"}
	}
	var defaultsAgent string
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, _ config.GlobalConfigLoadInfo) error {
		cfg.Agent.CLIPath = path
		defaultsAgent = cfg.Defaults.Agent
		return nil
	}); err != nil {
		return AgentCLIStatusJSON{}, err
	}
	if strings.TrimSpace(agent) == "" {
		agent = defaultsAgent
	}
	return s.GetAgentCLIStatus(ctx, agent)
}
