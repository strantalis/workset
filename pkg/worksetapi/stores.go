package worksetapi

import (
	"context"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
)

// ConfigStore abstracts global config persistence for the Service.
type ConfigStore interface {
	Load(ctx context.Context, path string) (config.GlobalConfig, config.GlobalConfigLoadInfo, error)
	Save(ctx context.Context, path string, cfg config.GlobalConfig) error
}

// ConfigUpdater optionally provides atomic update support for global config.
type ConfigUpdater interface {
	Update(ctx context.Context, path string, fn func(cfg *config.GlobalConfig, info config.GlobalConfigLoadInfo) error) (config.GlobalConfigLoadInfo, error)
}

// WorkspaceStore abstracts workspace config/state persistence for the Service.
type WorkspaceStore interface {
	Init(ctx context.Context, root, name string, defaults config.Defaults) (workspace.Workspace, error)
	Load(ctx context.Context, root string, defaults config.Defaults) (workspace.Workspace, error)
	LoadConfig(ctx context.Context, root string) (config.WorkspaceConfig, error)
	SaveConfig(ctx context.Context, root string, cfg config.WorkspaceConfig) error
	LoadState(ctx context.Context, root string) (workspace.State, error)
	SaveState(ctx context.Context, root string, state workspace.State) error
}

// FileConfigStore implements ConfigStore using filesystem-backed config.
type FileConfigStore struct{}

// FileWorkspaceStore implements WorkspaceStore using filesystem-backed config/state.
type FileWorkspaceStore struct{}

func (FileConfigStore) Load(_ context.Context, path string) (config.GlobalConfig, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := config.LoadGlobalWithInfo(path)
	return cfg, info, err
}

func (FileConfigStore) Save(_ context.Context, path string, cfg config.GlobalConfig) error {
	return config.SaveGlobal(path, cfg)
}

func (FileConfigStore) Update(_ context.Context, path string, fn func(cfg *config.GlobalConfig, info config.GlobalConfigLoadInfo) error) (config.GlobalConfigLoadInfo, error) {
	return config.UpdateGlobal(path, fn)
}

func (FileWorkspaceStore) Init(_ context.Context, root, name string, defaults config.Defaults) (workspace.Workspace, error) {
	return workspace.Init(root, name, defaults)
}

func (FileWorkspaceStore) Load(_ context.Context, root string, defaults config.Defaults) (workspace.Workspace, error) {
	return workspace.Load(root, defaults)
}

func (FileWorkspaceStore) LoadConfig(_ context.Context, root string) (config.WorkspaceConfig, error) {
	return config.LoadWorkspace(workspace.WorksetFile(root))
}

func (FileWorkspaceStore) SaveConfig(_ context.Context, root string, cfg config.WorkspaceConfig) error {
	return config.SaveWorkspace(workspace.WorksetFile(root), cfg)
}

func (FileWorkspaceStore) LoadState(_ context.Context, root string) (workspace.State, error) {
	return workspace.LoadState(root)
}

func (FileWorkspaceStore) SaveState(_ context.Context, root string, state workspace.State) error {
	return workspace.SaveState(root, state)
}
