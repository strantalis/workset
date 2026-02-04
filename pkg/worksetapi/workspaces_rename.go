package worksetapi

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/config"
)

// RenameWorkspace updates the workspace name in global config and workset.yaml.
func (s *Service) RenameWorkspace(ctx context.Context, input WorkspaceRenameInput) (WorkspaceRefJSON, error) {
	var (
		ref     config.WorkspaceRef
		outName string
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, _ config.GlobalConfigLoadInfo) error {
		currentName, root, err := resolveWorkspaceSelector(cfg, input.Selector)
		if err != nil {
			return err
		}
		newName := strings.TrimSpace(input.NewName)
		if newName == "" {
			return ValidationError{Message: "new workspace name required"}
		}

		wsConfig, err := s.workspaces.LoadConfig(ctx, root)
		if err != nil {
			return err
		}
		if currentName == "" {
			currentName = strings.TrimSpace(wsConfig.Name)
		}
		if currentName == "" {
			return ValidationError{Message: "workspace name required"}
		}
		if currentName == newName {
			ref = cfg.Workspaces[currentName]
			if ref.Path == "" {
				ref.Path = root
			}
			outName = currentName
			return nil
		}

		if existing, ok := cfg.Workspaces[newName]; ok {
			if filepath.Clean(existing.Path) != filepath.Clean(root) && existing.Path != "" {
				return ConflictError{Message: "workspace name already registered to a different path"}
			}
		}

		wsConfig.Name = newName
		if err := s.workspaces.SaveConfig(ctx, root, wsConfig); err != nil {
			return err
		}

		ref = cfg.Workspaces[currentName]
		if ref.Path == "" {
			ref.Path = root
		}
		ref.LastUsed = s.clock().Format(time.RFC3339)
		delete(cfg.Workspaces, currentName)
		cfg.Workspaces[newName] = ref
		outName = newName
		return nil
	}); err != nil {
		return WorkspaceRefJSON{}, err
	}

	return workspaceRefJSON(outName, ref), nil
}
