package worksetapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

// GetConfig loads the global config and its load metadata.
func (s *Service) GetConfig(ctx context.Context) (config.GlobalConfig, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	return cfg, info, err
}

// SetDefault updates a defaults.* key in the global config.
func (s *Service) SetDefault(ctx context.Context, key, value string) (ConfigSetResultJSON, config.GlobalConfigLoadInfo, error) {
	var info config.GlobalConfigLoadInfo
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		return setGlobalDefault(cfg, key, value)
	}); err != nil {
		return ConfigSetResultJSON{}, info, err
	}
	return ConfigSetResultJSON{Status: "ok", Key: key, Value: value}, info, nil
}

func setGlobalDefault(cfg *config.GlobalConfig, key, value string) error {
	switch key {
	case "defaults.remote":
		cfg.Defaults.Remote = value
	case "defaults.base_branch":
		cfg.Defaults.BaseBranch = value
	case "defaults.thread":
		cfg.Defaults.Thread = value
	case "defaults.workset_root":
		cfg.Defaults.WorksetRoot = value
	case "defaults.repo_store_root":
		cfg.Defaults.RepoStoreRoot = value
	case "defaults.agent":
		agent := strings.ToLower(strings.TrimSpace(value))
		switch agent {
		case "codex", "claude":
			cfg.Defaults.Agent = agent
		default:
			return fmt.Errorf("unsupported agent %q; supported agents: codex, claude", strings.TrimSpace(value))
		}
	case "defaults.agent_model":
		cfg.Defaults.AgentModel = value
	case "defaults.terminal_idle_timeout":
		cfg.Defaults.TerminalIdleTimeout = value
	case "defaults.terminal_protocol_log":
		cfg.Defaults.TerminalProtocolLog = value
	case "defaults.terminal_debug_overlay":
		cfg.Defaults.TerminalDebugOverlay = value
	case "defaults.remotes.base", "defaults.remotes.write":
		return fmt.Errorf("%s was removed; set defaults.remote or alias remote instead", key)
	case "defaults.parallelism":
		return fmt.Errorf("%s was removed; parallelism is no longer configurable", key)
	default:
		return fmt.Errorf("unsupported key %q", key)
	}
	return nil
}
