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
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "workset", "config.yaml"), nil
}

func LoadGlobal(path string) (GlobalConfig, error) {
	if path == "" {
		var err error
		path, err = GlobalConfigPath()
		if err != nil {
			return GlobalConfig{}, err
		}
	}

	defaults := DefaultConfig()

	k := koanf.New(".")
	if err := k.Load(confmap.Provider(map[string]interface{}{
		"defaults.base_branch":   defaults.Defaults.BaseBranch,
		"defaults.remotes.base":  defaults.Defaults.Remotes.Base,
		"defaults.remotes.write": defaults.Defaults.Remotes.Write,
		"defaults.parallelism":   defaults.Defaults.Parallelism,
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
	if cfg.Defaults.Remotes.Base == "" {
		cfg.Defaults.Remotes.Base = defaults.Defaults.Remotes.Base
	}
	if cfg.Defaults.Remotes.Write == "" {
		cfg.Defaults.Remotes.Write = defaults.Defaults.Remotes.Write
	}
	if cfg.Defaults.Parallelism == 0 {
		cfg.Defaults.Parallelism = defaults.Defaults.Parallelism
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
