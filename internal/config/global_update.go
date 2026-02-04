package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/rogpeppe/go-internal/lockedfile"
	"gopkg.in/yaml.v3"
)

// UpdateGlobal loads the latest global config under a lock, applies fn, and writes atomically.
func UpdateGlobal(path string, fn func(cfg *GlobalConfig, info GlobalConfigLoadInfo) error) (GlobalConfigLoadInfo, error) {
	_, info, err := loadGlobal(path)
	if err != nil {
		return info, err
	}
	if info.Path == "" {
		return info, errors.New("global config path missing")
	}
	if err := os.MkdirAll(filepath.Dir(info.Path), 0o755); err != nil {
		return info, err
	}
	if _, err := os.Stat(info.Path); err == nil {
		info.Exists = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return info, err
	}

	err = lockedfile.Transform(info.Path, func(old []byte) ([]byte, error) {
		cfg, err := loadGlobalFromBytes(old)
		if err != nil {
			return nil, err
		}
		cfg.EnsureMaps()
		if err := fn(&cfg, info); err != nil {
			return nil, err
		}
		cfg.EnsureMaps()
		cfg = stripLegacyGroupRemotes(cfg)
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return nil, err
		}
		if info.Exists {
			perm := os.FileMode(0o644)
			if stat, err := os.Stat(info.Path); err == nil {
				perm = stat.Mode().Perm()
			}
			if err := os.WriteFile(info.Path+".bak", old, perm); err != nil {
				return nil, err
			}
		}
		return data, nil
	})
	if err != nil {
		return info, err
	}
	_ = os.Chmod(info.Path, 0o644)
	return info, nil
}
