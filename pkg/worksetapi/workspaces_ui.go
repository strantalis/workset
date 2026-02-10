package worksetapi

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/config"
)

// PinWorkspace pins or unpins a workspace.
func (s *Service) PinWorkspace(ctx context.Context, selector WorkspaceSelector, pin bool) (WorkspaceRefJSON, config.GlobalConfigLoadInfo, error) {
	var (
		info config.GlobalConfigLoadInfo
		name string
		ref  config.WorkspaceRef
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		var err error
		name, _, err = resolveWorkspaceSelector(cfg, selector)
		if err != nil {
			return err
		}
		var ok bool
		ref, ok = cfg.Workspaces[name]
		if !ok {
			return NotFoundError{Message: "workspace not found"}
		}
		ref.Pinned = pin
		if pin {
			// Assign next pin order
			maxOrder := -1
			for _, r := range cfg.Workspaces {
				if r.Pinned && r.PinOrder > maxOrder {
					maxOrder = r.PinOrder
				}
			}
			ref.PinOrder = maxOrder + 1
		} else {
			ref.PinOrder = 0
		}
		ref.LastUsed = s.clock().Format(time.RFC3339)
		cfg.Workspaces[name] = ref
		return nil
	}); err != nil {
		return WorkspaceRefJSON{}, info, err
	}
	return workspaceRefJSON(name, ref), info, nil
}

// SetWorkspaceColor sets the color for a workspace.
func (s *Service) SetWorkspaceColor(ctx context.Context, selector WorkspaceSelector, color string) (WorkspaceRefJSON, config.GlobalConfigLoadInfo, error) {
	var (
		info config.GlobalConfigLoadInfo
		name string
		ref  config.WorkspaceRef
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		var err error
		name, _, err = resolveWorkspaceSelector(cfg, selector)
		if err != nil {
			return err
		}
		var ok bool
		ref, ok = cfg.Workspaces[name]
		if !ok {
			return NotFoundError{Message: "workspace not found"}
		}
		ref.Color = color
		cfg.Workspaces[name] = ref
		return nil
	}); err != nil {
		return WorkspaceRefJSON{}, info, err
	}
	return workspaceRefJSON(name, ref), info, nil
}

// SetWorkspaceDescription sets the description for a workspace.
func (s *Service) SetWorkspaceDescription(ctx context.Context, selector WorkspaceSelector, description string) (WorkspaceRefJSON, config.GlobalConfigLoadInfo, error) {
	var (
		info config.GlobalConfigLoadInfo
		name string
		ref  config.WorkspaceRef
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		var err error
		name, _, err = resolveWorkspaceSelector(cfg, selector)
		if err != nil {
			return err
		}
		var ok bool
		ref, ok = cfg.Workspaces[name]
		if !ok {
			return NotFoundError{Message: "workspace not found"}
		}
		ref.Description = description
		cfg.Workspaces[name] = ref
		return nil
	}); err != nil {
		return WorkspaceRefJSON{}, info, err
	}
	return workspaceRefJSON(name, ref), info, nil
}

// SetWorkspaceExpanded sets the expanded state for a workspace.
func (s *Service) SetWorkspaceExpanded(ctx context.Context, selector WorkspaceSelector, expanded bool) (WorkspaceRefJSON, config.GlobalConfigLoadInfo, error) {
	var (
		info config.GlobalConfigLoadInfo
		name string
		ref  config.WorkspaceRef
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		var err error
		name, _, err = resolveWorkspaceSelector(cfg, selector)
		if err != nil {
			return err
		}
		var ok bool
		ref, ok = cfg.Workspaces[name]
		if !ok {
			return NotFoundError{Message: "workspace not found"}
		}
		ref.Expanded = expanded
		cfg.Workspaces[name] = ref
		return nil
	}); err != nil {
		return WorkspaceRefJSON{}, info, err
	}
	return workspaceRefJSON(name, ref), info, nil
}

// ReorderWorkspaces updates the pin order for multiple workspaces.
// orders is a map of workspace name to pin order.
func (s *Service) ReorderWorkspaces(ctx context.Context, orders map[string]int) ([]WorkspaceRefJSON, config.GlobalConfigLoadInfo, error) {
	var (
		info config.GlobalConfigLoadInfo
		refs []WorkspaceRefJSON
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		names := make([]string, 0, len(orders))
		for name := range orders {
			names = append(names, name)
		}
		sort.Strings(names)
		missing := make([]string, 0)
		for _, name := range names {
			order := orders[name]
			ref, ok := cfg.Workspaces[name]
			if !ok {
				missing = append(missing, name)
				continue
			}
			ref.PinOrder = order
			cfg.Workspaces[name] = ref
			refs = append(refs, workspaceRefJSON(name, ref))
		}
		if len(missing) > 0 {
			return NotFoundError{Message: "workspace(s) not found: " + strings.Join(missing, ", ")}
		}
		return nil
	}); err != nil {
		return nil, info, err
	}
	return refs, info, nil
}

// UpdateWorkspaceLastUsed updates the last used timestamp for a workspace.
func (s *Service) UpdateWorkspaceLastUsed(ctx context.Context, selector WorkspaceSelector) (WorkspaceRefJSON, config.GlobalConfigLoadInfo, error) {
	var (
		info config.GlobalConfigLoadInfo
		name string
		ref  config.WorkspaceRef
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		var err error
		name, _, err = resolveWorkspaceSelector(cfg, selector)
		if err != nil {
			return err
		}
		var ok bool
		ref, ok = cfg.Workspaces[name]
		if !ok {
			return NotFoundError{Message: "workspace not found"}
		}
		ref.LastUsed = s.clock().Format(time.RFC3339)
		cfg.Workspaces[name] = ref
		return nil
	}); err != nil {
		return WorkspaceRefJSON{}, info, err
	}
	return workspaceRefJSON(name, ref), info, nil
}
