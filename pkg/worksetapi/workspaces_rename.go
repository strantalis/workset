package worksetapi

import (
	"context"
	"path/filepath"
	"strings"
	"time"
)

// RenameWorkspace updates the workspace name in global config and workset.yaml.
func (s *Service) RenameWorkspace(ctx context.Context, input WorkspaceRenameInput) (WorkspaceRefJSON, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return WorkspaceRefJSON{}, err
	}
	currentName, root, err := resolveWorkspaceSelector(&cfg, input.Selector)
	if err != nil {
		return WorkspaceRefJSON{}, err
	}
	newName := strings.TrimSpace(input.NewName)
	if newName == "" {
		return WorkspaceRefJSON{}, ValidationError{Message: "new workspace name required"}
	}

	wsConfig, err := s.workspaces.LoadConfig(ctx, root)
	if err != nil {
		return WorkspaceRefJSON{}, err
	}
	if currentName == "" {
		currentName = strings.TrimSpace(wsConfig.Name)
	}
	if currentName == "" {
		return WorkspaceRefJSON{}, ValidationError{Message: "workspace name required"}
	}
	if currentName == newName {
		ref := cfg.Workspaces[currentName]
		if ref.Path == "" {
			ref.Path = root
		}
		return workspaceRefJSON(currentName, ref), nil
	}

	if existing, ok := cfg.Workspaces[newName]; ok {
		if filepath.Clean(existing.Path) != filepath.Clean(root) && existing.Path != "" {
			return WorkspaceRefJSON{}, ConflictError{Message: "workspace name already registered to a different path"}
		}
	}

	wsConfig.Name = newName
	if err := s.workspaces.SaveConfig(ctx, root, wsConfig); err != nil {
		return WorkspaceRefJSON{}, err
	}

	ref := cfg.Workspaces[currentName]
	if ref.Path == "" {
		ref.Path = root
	}
	ref.LastUsed = s.clock().Format(time.RFC3339)
	delete(cfg.Workspaces, currentName)
	cfg.Workspaces[newName] = ref

	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return WorkspaceRefJSON{}, err
	}

	return workspaceRefJSON(newName, ref), nil
}
