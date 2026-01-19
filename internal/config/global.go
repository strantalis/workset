package config

import (
	"errors"
	"os"
	"path/filepath"

	koanfyaml "github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
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

func legacyGlobalConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "workset", "config.yaml"), nil
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

func LoadGlobal(path string) (GlobalConfig, error) {
	if path == "" {
		var err error
		path, err = GlobalConfigPath()
		if err != nil {
			return GlobalConfig{}, err
		}
		legacyPath, legacyErr := legacyGlobalConfigPath()
		if legacyErr == nil && legacyPath != "" {
			newExists := true
			if _, statErr := os.Stat(path); statErr != nil {
				if errors.Is(statErr, os.ErrNotExist) {
					newExists = false
				} else {
					return GlobalConfig{}, statErr
				}
			}
			if !newExists {
				if err := migrateLegacyGlobalConfig(path, legacyPath); err != nil {
					path = legacyPath
				} else if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
					path = legacyPath
				}
			}
		}
	}

	defaults := DefaultConfig()

	k := koanf.New(".")
	if err := k.Load(confmap.Provider(map[string]interface{}{
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
	}, "."), nil); err != nil {
		return GlobalConfig{}, err
	}

	if _, err := os.Stat(path); err == nil {
		if err := k.Load(file.Provider(path), koanfyaml.Parser()); err != nil {
			return GlobalConfig{}, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return GlobalConfig{}, err
	}

	var cfg GlobalConfig
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return GlobalConfig{}, err
	}
	cfg.EnsureMaps()
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
