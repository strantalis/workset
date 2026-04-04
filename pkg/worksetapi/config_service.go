package worksetapi

import (
	"context"
	"fmt"
	"strconv"
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
	case "defaults.terminal_debug_log":
		normalized, err := normalizeOnOff(value, key)
		if err != nil {
			return err
		}
		cfg.Defaults.TerminalDebugLog = normalized
	case "defaults.terminal_protocol_log":
		normalized, err := normalizeOnOff(value, key)
		if err != nil {
			return err
		}
		cfg.Defaults.TerminalProtocolLog = normalized
	case "defaults.terminal_debug_overlay":
		normalized, err := normalizeOnOff(value, key)
		if err != nil {
			return err
		}
		cfg.Defaults.TerminalDebugOverlay = normalized
	case "defaults.terminal_font_size":
		normalized, err := normalizeTerminalFontSize(value)
		if err != nil {
			return err
		}
		cfg.Defaults.TerminalFontSize = normalized
	case "defaults.terminal_cursor_blink":
		normalized, err := normalizeOnOff(value, key)
		if err != nil {
			return err
		}
		cfg.Defaults.TerminalCursorBlink = normalized
	case "defaults.remotes.base", "defaults.remotes.write":
		return fmt.Errorf("%s was removed; set defaults.remote or alias remote instead", key)
	case "defaults.parallelism":
		return fmt.Errorf("%s was removed; parallelism is no longer configurable", key)
	default:
		return fmt.Errorf("unsupported key %q", key)
	}
	return nil
}

const (
	minTerminalFontSize = 8
	maxTerminalFontSize = 28
)

func normalizeTerminalFontSize(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("defaults.terminal_font_size must be an integer between %d and %d", minTerminalFontSize, maxTerminalFontSize)
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed < minTerminalFontSize || parsed > maxTerminalFontSize {
		return "", fmt.Errorf("defaults.terminal_font_size must be an integer between %d and %d", minTerminalFontSize, maxTerminalFontSize)
	}
	return strconv.Itoa(parsed), nil
}

func normalizeOnOff(value, key string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "on", "off":
		return normalized, nil
	default:
		return "", fmt.Errorf("%s must be 'on' or 'off'", key)
	}
}
