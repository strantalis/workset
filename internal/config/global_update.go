package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/rogpeppe/go-internal/lockedfile"
	"gopkg.in/yaml.v3"
)

// UpdateGlobal loads the latest global config under a lock, applies fn, and writes atomically.
func UpdateGlobal(path string, fn func(cfg *GlobalConfig, info GlobalConfigLoadInfo) error) (GlobalConfigLoadInfo, error) {
	info, err := resolveGlobalPathForUpdate(path)
	if err != nil {
		return info, err
	}
	if info.Path == "" {
		return info, errors.New("global config path missing")
	}
	if err := os.MkdirAll(filepath.Dir(info.Path), 0o755); err != nil {
		return info, err
	}
	perm := os.FileMode(0o644)
	if stat, err := os.Stat(info.Path); err == nil {
		info.Exists = true
		perm = stat.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return info, err
	}

	err = lockedfile.Transform(info.Path, func(old []byte) ([]byte, error) {
		info.Exists = len(bytes.TrimSpace(old)) > 0
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
	_ = os.Chmod(info.Path, perm)
	return info, nil
}

func resolveGlobalPathForUpdate(path string) (GlobalConfigLoadInfo, error) {
	info := GlobalConfigLoadInfo{}
	if path != "" {
		info.Path = path
		return info, nil
	}

	globalPath, err := GlobalConfigPath()
	if err != nil {
		return info, err
	}
	info.Path = globalPath

	legacyPaths, legacyErr := legacyGlobalConfigPaths()
	if legacyErr != nil || len(legacyPaths) == 0 {
		return info, nil
	}

	newExists := true
	if _, statErr := os.Stat(globalPath); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			newExists = false
		} else {
			return info, statErr
		}
	}
	if newExists {
		return info, nil
	}

	for _, legacyPath := range legacyPaths {
		if err := migrateLegacyGlobalConfig(globalPath, legacyPath); err == nil {
			if _, statErr := os.Stat(globalPath); statErr == nil {
				info.Migrated = true
				info.LegacyPath = legacyPath
				break
			}
			continue
		} else {
			info.Path = legacyPath
			info.UsedLegacy = true
			info.LegacyPath = legacyPath
			break
		}
	}

	return info, nil
}
