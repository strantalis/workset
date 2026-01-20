package worksetapi

import (
	"context"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/config"
)

// ArchiveWorkspace marks a workspace as archived in the global config.
func (s *Service) ArchiveWorkspace(ctx context.Context, selector WorkspaceSelector, reason string) (WorkspaceRefJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return WorkspaceRefJSON{}, info, err
	}
	name, _, err := resolveWorkspaceSelector(&cfg, selector)
	if err != nil {
		return WorkspaceRefJSON{}, info, err
	}
	ref, ok := cfg.Workspaces[name]
	if !ok {
		return WorkspaceRefJSON{}, info, NotFoundError{Message: "workspace not found"}
	}

	ref.ArchivedAt = s.clock().Format(time.RFC3339)
	ref.ArchivedReason = strings.TrimSpace(reason)
	cfg.Workspaces[name] = ref

	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return WorkspaceRefJSON{}, info, err
	}
	return workspaceRefJSON(name, ref), info, nil
}

// UnarchiveWorkspace removes archived flags for a workspace.
func (s *Service) UnarchiveWorkspace(ctx context.Context, selector WorkspaceSelector) (WorkspaceRefJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return WorkspaceRefJSON{}, info, err
	}
	name, _, err := resolveWorkspaceSelector(&cfg, selector)
	if err != nil {
		return WorkspaceRefJSON{}, info, err
	}
	ref, ok := cfg.Workspaces[name]
	if !ok {
		return WorkspaceRefJSON{}, info, NotFoundError{Message: "workspace not found"}
	}

	ref.ArchivedAt = ""
	ref.ArchivedReason = ""
	ref.LastUsed = s.clock().Format(time.RFC3339)
	cfg.Workspaces[name] = ref

	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return WorkspaceRefJSON{}, info, err
	}
	return workspaceRefJSON(name, ref), info, nil
}

func workspaceRefJSON(name string, ref config.WorkspaceRef) WorkspaceRefJSON {
	return WorkspaceRefJSON{
		Name:           name,
		Path:           ref.Path,
		CreatedAt:      ref.CreatedAt,
		LastUsed:       ref.LastUsed,
		ArchivedAt:     ref.ArchivedAt,
		ArchivedReason: ref.ArchivedReason,
		Archived:       ref.ArchivedAt != "",
	}
}
