package worksetapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
)

func (s *Service) loadGlobal(ctx context.Context) (config.GlobalConfig, config.GlobalConfigLoadInfo, error) {
	return s.configs.Load(ctx, s.configPath)
}

func (s *Service) updateGlobal(ctx context.Context, fn func(cfg *config.GlobalConfig, info config.GlobalConfigLoadInfo) error) (config.GlobalConfigLoadInfo, error) {
	if updater, ok := s.configs.(ConfigUpdater); ok {
		return updater.Update(ctx, s.configPath, fn)
	}
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return info, err
	}
	if err := fn(&cfg, info); err != nil {
		return info, err
	}
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return info, err
	}
	return info, nil
}

func registerWorkspace(cfg *config.GlobalConfig, name, path string, now time.Time, workset string) {
	if cfg.Workspaces == nil {
		cfg.Workspaces = map[string]config.WorkspaceRef{}
	}
	ref := cfg.Workspaces[name]
	if ref.Path == "" {
		ref.Path = path
		if ref.CreatedAt == "" {
			ref.CreatedAt = now.Format(time.RFC3339)
		}
	}
	if ref.Workset == "" {
		if workset != "" {
			ref.Workset = workset
		} else {
			ref.Workset = name
		}
	}
	ref.RepoOverrides = normalizeRepoNames(ref.RepoOverrides)
	ref.LastUsed = now.Format(time.RFC3339)
	cfg.Workspaces[name] = ref
	if cfg.WorksetRepos == nil {
		cfg.WorksetRepos = map[string][]string{}
	}
	worksetName := strings.TrimSpace(ref.Workset)
	if worksetName != "" {
		if _, ok := cfg.WorksetRepos[worksetName]; !ok {
			cfg.WorksetRepos[worksetName] = nil
		}
	}
}

func workspaceRefWorkset(ref config.WorkspaceRef) string {
	return strings.TrimSpace(ref.Workset)
}

func resolveThreadTarget(arg string, cfg *config.GlobalConfig) (string, string, error) {
	target := strings.TrimSpace(arg)
	if target == "" {
		target = strings.TrimSpace(cfg.Defaults.Thread)
	}
	if target == "" {
		return "", "", ValidationError{Message: "thread required"}
	}
	if ref, ok := cfg.Workspaces[target]; ok {
		return target, ref.Path, nil
	}
	if filepath.IsAbs(target) {
		name := threadNameByPath(cfg, target)
		return name, target, nil
	}
	return "", "", NotFoundError{Message: fmt.Sprintf("thread not found: %q", target)}
}

func resolveWorkspaceSelector(cfg *config.GlobalConfig, selector WorkspaceSelector) (string, string, error) {
	if selector.Value == "" && !selector.Require {
		return resolveThreadTarget("", cfg)
	}
	return resolveThreadTarget(selector.Value, cfg)
}

func threadNameByPath(cfg *config.GlobalConfig, path string) string {
	clean := filepath.Clean(path)
	for name, ref := range cfg.Workspaces {
		if filepath.Clean(ref.Path) == clean {
			return name
		}
	}
	return ""
}

func removeWorkspaceByPath(cfg *config.GlobalConfig, path string) {
	clean := filepath.Clean(path)
	for name, ref := range cfg.Workspaces {
		if filepath.Clean(ref.Path) == clean {
			delete(cfg.Workspaces, name)
		}
	}
}

func looksLikeURL(value string) bool {
	if strings.Contains(value, "://") {
		return true
	}
	if strings.Contains(value, "@") && strings.Contains(value, ":") {
		return true
	}
	return false
}

func looksLikeLocalPath(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "~") || strings.HasPrefix(value, ".") {
		return true
	}
	return filepath.IsAbs(value)
}

func resolveLocalPathInput(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", ValidationError{Message: "local path required"}
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func worksetFilePath(root string) string {
	return workspace.WorksetFile(root)
}
