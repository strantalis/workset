package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	koanfyaml "github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"gopkg.in/yaml.v3"
)

func GlobalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "config.yaml"), nil
}

func legacyGlobalConfigPaths() ([]string, error) {
	paths := make([]string, 0, 3)
	configDir, err := os.UserConfigDir()
	if err == nil && configDir != "" {
		paths = append(paths, filepath.Join(configDir, "workset", "config.yaml"))
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if runtime.GOOS == "darwin" {
			paths = append(paths, filepath.Join(home, "Library", "Application Support", "workset", "config.yaml"))
		}
		paths = append(paths, filepath.Join(home, ".config", "workset", "config.yaml"))
	}
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		unique = append(unique, path)
	}
	return unique, nil
}

func migrateLegacyGlobalConfig(path, legacyPath string) error {
	if legacyPath == "" || legacyPath == path {
		return nil
	}
	if _, err := os.Stat(legacyPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.Rename(legacyPath, path); err == nil {
		return nil
	}
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

type GlobalConfigLoadInfo struct {
	Path       string
	LegacyPath string
	Migrated   bool
	UsedLegacy bool
	Exists     bool
}

func LoadGlobalWithInfo(path string) (GlobalConfig, GlobalConfigLoadInfo, error) {
	cfg, info, err := loadGlobal(path)
	return cfg, info, err
}

func LoadGlobal(path string) (GlobalConfig, error) {
	cfg, _, err := loadGlobal(path)
	return cfg, err
}

func loadGlobal(path string) (GlobalConfig, GlobalConfigLoadInfo, error) {
	info := GlobalConfigLoadInfo{}
	if path == "" {
		var err error
		path, err = GlobalConfigPath()
		if err != nil {
			return GlobalConfig{}, info, err
		}
		legacyPaths, legacyErr := legacyGlobalConfigPaths()
		if legacyErr == nil && len(legacyPaths) > 0 {
			newExists := true
			if _, statErr := os.Stat(path); statErr != nil {
				if errors.Is(statErr, os.ErrNotExist) {
					newExists = false
				} else {
					return GlobalConfig{}, info, statErr
				}
			}
			if !newExists {
				for _, legacyPath := range legacyPaths {
					if err := migrateLegacyGlobalConfig(path, legacyPath); err == nil {
						if _, statErr := os.Stat(path); statErr == nil {
							info.Migrated = true
							info.LegacyPath = legacyPath
							break
						}
					} else {
						path = legacyPath
						info.UsedLegacy = true
						info.LegacyPath = legacyPath
						break
					}
				}
			}
		}
	}
	info.Path = path

	defaults := DefaultConfig()

	k := koanf.New(".")
	if err := k.Load(confmap.Provider(defaultConfigMap(defaults), "."), nil); err != nil {
		return GlobalConfig{}, info, err
	}

	if _, err := os.Stat(path); err == nil {
		info.Exists = true
		if err := k.Load(file.Provider(path), koanfyaml.Parser()); err != nil {
			return GlobalConfig{}, info, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return GlobalConfig{}, info, err
	}

	var cfg GlobalConfig
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return GlobalConfig{}, info, err
	}
	finalizeGlobal(&cfg, defaults)
	return cfg, info, nil
}

func loadGlobalFromBytes(data []byte) (GlobalConfig, error) {
	defaults := DefaultConfig()
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(defaultConfigMap(defaults), "."), nil); err != nil {
		return GlobalConfig{}, err
	}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := k.Load(rawbytes.Provider(data), koanfyaml.Parser()); err != nil {
			return GlobalConfig{}, err
		}
	}
	var cfg GlobalConfig
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return GlobalConfig{}, err
	}
	finalizeGlobal(&cfg, defaults)
	return cfg, nil
}

func SaveGlobal(path string, cfg GlobalConfig) error {
	if path == "" {
		var err error
		path, err = GlobalConfigPath()
		if err != nil {
			return err
		}
	}
	cfg.EnsureMaps()
	cfg = stripLegacyGroupRemotes(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil {
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path+".bak", existing, info.Mode().Perm()); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func stripLegacyGroupRemotes(cfg GlobalConfig) GlobalConfig {
	if cfg.Groups == nil {
		return cfg
	}
	for name, group := range cfg.Groups {
		if len(group.Members) == 0 {
			continue
		}
		updated := false
		for i := range group.Members {
			if group.Members[i].LegacyRemotes != nil {
				group.Members[i].LegacyRemotes = nil
				updated = true
			}
		}
		if updated {
			cfg.Groups[name] = group
		}
	}
	return cfg
}

func defaultConfigMap(defaults GlobalConfig) map[string]any {
	return map[string]any{
		"defaults.remote":                    defaults.Defaults.Remote,
		"defaults.base_branch":               defaults.Defaults.BaseBranch,
		"defaults.workspace":                 defaults.Defaults.Workspace,
		"defaults.workspace_root":            defaults.Defaults.WorkspaceRoot,
		"defaults.repo_store_root":           defaults.Defaults.RepoStoreRoot,
		"defaults.session_backend":           defaults.Defaults.SessionBackend,
		"defaults.session_name_format":       defaults.Defaults.SessionNameFormat,
		"defaults.session_theme":             defaults.Defaults.SessionTheme,
		"defaults.session_tmux_status_style": defaults.Defaults.SessionTmuxStyle,
		"defaults.session_tmux_status_left":  defaults.Defaults.SessionTmuxLeft,
		"defaults.session_tmux_status_right": defaults.Defaults.SessionTmuxRight,
		"defaults.session_screen_hardstatus": defaults.Defaults.SessionScreenHard,
		"defaults.agent":                     defaults.Defaults.Agent,
		"defaults.agent_model":               defaults.Defaults.AgentModel,
		"defaults.terminal_idle_timeout":     defaults.Defaults.TerminalIdleTimeout,
		"defaults.terminal_protocol_log":     defaults.Defaults.TerminalProtocolLog,
		"defaults.terminal_debug_overlay":    defaults.Defaults.TerminalDebugOverlay,
		"defaults.terminal_keybindings":      defaults.Defaults.TerminalKeybindings,
		"github.cli_path":                    defaults.GitHub.CLIPath,
		"hooks.enabled":                      defaults.Hooks.Enabled,
		"hooks.on_error":                     defaults.Hooks.OnError,
		"hooks.repo_hooks.trusted_repos":     defaults.Hooks.RepoHooks.TrustedRepos,
	}
}

func finalizeGlobal(cfg *GlobalConfig, defaults GlobalConfig) {
	cfg.EnsureMaps()
	if cfg.Defaults.Remote == "" {
		cfg.Defaults.Remote = defaults.Defaults.Remote
	}
	if cfg.Defaults.BaseBranch == "" {
		cfg.Defaults = defaults.Defaults
	}
	if cfg.Defaults.Workspace == "" {
		cfg.Defaults.Workspace = defaults.Defaults.Workspace
	}
	if cfg.Defaults.WorkspaceRoot == "" {
		cfg.Defaults.WorkspaceRoot = defaults.Defaults.WorkspaceRoot
	}
	if cfg.Defaults.RepoStoreRoot == "" {
		cfg.Defaults.RepoStoreRoot = defaults.Defaults.RepoStoreRoot
	}
	if cfg.Defaults.SessionBackend == "" {
		cfg.Defaults.SessionBackend = defaults.Defaults.SessionBackend
	}
	if cfg.Defaults.SessionNameFormat == "" {
		cfg.Defaults.SessionNameFormat = defaults.Defaults.SessionNameFormat
	}
	if cfg.Defaults.SessionTheme == "" {
		cfg.Defaults.SessionTheme = defaults.Defaults.SessionTheme
	}
	if cfg.Defaults.SessionTmuxStyle == "" {
		cfg.Defaults.SessionTmuxStyle = defaults.Defaults.SessionTmuxStyle
	}
	if cfg.Defaults.SessionTmuxLeft == "" {
		cfg.Defaults.SessionTmuxLeft = defaults.Defaults.SessionTmuxLeft
	}
	if cfg.Defaults.SessionTmuxRight == "" {
		cfg.Defaults.SessionTmuxRight = defaults.Defaults.SessionTmuxRight
	}
	if cfg.Defaults.SessionScreenHard == "" {
		cfg.Defaults.SessionScreenHard = defaults.Defaults.SessionScreenHard
	}
	if cfg.Defaults.Agent == "" {
		cfg.Defaults.Agent = defaults.Defaults.Agent
	}
	if cfg.Defaults.TerminalIdleTimeout == "" {
		cfg.Defaults.TerminalIdleTimeout = defaults.Defaults.TerminalIdleTimeout
	}
	if cfg.Defaults.TerminalProtocolLog == "" {
		cfg.Defaults.TerminalProtocolLog = defaults.Defaults.TerminalProtocolLog
	}
	if cfg.Defaults.TerminalDebugOverlay == "" {
		cfg.Defaults.TerminalDebugOverlay = defaults.Defaults.TerminalDebugOverlay
	}
	if cfg.Defaults.TerminalKeybindings == nil {
		cfg.Defaults.TerminalKeybindings = defaults.Defaults.TerminalKeybindings
	}
	if cfg.Hooks.OnError == "" {
		cfg.Hooks.OnError = defaults.Hooks.OnError
	}
	if cfg.Hooks.RepoHooks.TrustedRepos == nil {
		cfg.Hooks.RepoHooks.TrustedRepos = defaults.Hooks.RepoHooks.TrustedRepos
	}
	if cfg.Hooks.Items == nil {
		cfg.Hooks.Items = defaults.Hooks.Items
	}
}
