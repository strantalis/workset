package worksetapi

import (
	"context"
	"path/filepath"
	"strings"
)

// GetAgentCLIStatus reports whether the configured agent command is available.
func (s *Service) GetAgentCLIStatus(ctx context.Context, agent string) (AgentCLIStatusJSON, error) {
	fields := strings.Fields(strings.TrimSpace(agent))
	if len(fields) == 0 {
		return AgentCLIStatusJSON{}, ValidationError{Message: "agent command required"}
	}
	command := fields[0]
	configuredPath, err := s.agentCLIPathFromConfig(ctx)
	if err != nil {
		return AgentCLIStatusJSON{}, err
	}
	configuredPath = normalizeCLIPath(configuredPath)
	status := AgentCLIStatusJSON{
		Command:        command,
		ConfiguredPath: configuredPath,
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
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return AgentCLIStatusJSON{}, err
	}
	cfg.Agent.CLIPath = path
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return AgentCLIStatusJSON{}, err
	}
	if strings.TrimSpace(agent) == "" {
		agent = cfg.Defaults.Agent
	}
	return s.GetAgentCLIStatus(ctx, agent)
}

func (s *Service) agentCLIPathFromConfig(ctx context.Context) (string, error) {
	cfg, _, err := s.loadGlobal(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cfg.Agent.CLIPath), nil
}
