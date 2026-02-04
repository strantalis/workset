package worksetapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/ops"
	"github.com/strantalis/workset/internal/workspace"
)

// ListWorkspaces returns registered workspaces from global config.
func (s *Service) ListWorkspaces(ctx context.Context) (WorkspaceListResult, error) {
	return s.ListWorkspacesWithOptions(ctx, WorkspaceListOptions{IncludeArchived: true})
}

// ListWorkspacesWithOptions returns registered workspaces with optional filters.
func (s *Service) ListWorkspacesWithOptions(ctx context.Context, opts WorkspaceListOptions) (WorkspaceListResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return WorkspaceListResult{}, err
	}
	if len(cfg.Workspaces) == 0 {
		return WorkspaceListResult{Workspaces: []WorkspaceRefJSON{}, Config: info}, nil
	}
	names := make([]string, 0, len(cfg.Workspaces))
	for name := range cfg.Workspaces {
		names = append(names, name)
	}
	sort.Strings(names)
	rows := make([]WorkspaceRefJSON, 0, len(names))
	for _, name := range names {
		ref := cfg.Workspaces[name]
		if !opts.IncludeArchived && ref.ArchivedAt != "" {
			continue
		}
		rows = append(rows, workspaceRefJSON(name, ref))
	}
	return WorkspaceListResult{Workspaces: rows, Config: info}, nil
}

// CreateWorkspace creates a new workspace and optionally adds repos/groups.
func (s *Service) CreateWorkspace(ctx context.Context, input WorkspaceCreateInput) (WorkspaceCreateResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return WorkspaceCreateResult{}, ValidationError{Message: "workspace name required"}
	}

	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return WorkspaceCreateResult{}, err
	}

	root := strings.TrimSpace(input.Path)
	if root == "" {
		base := cfg.Defaults.WorkspaceRoot
		if base == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return WorkspaceCreateResult{}, err
			}
			base = cwd
		}
		root = filepath.Join(base, workspace.WorkspaceDirName(name))
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return WorkspaceCreateResult{}, err
	}

	ws, err := s.workspaces.Init(ctx, root, name, cfg.Defaults)
	if err != nil {
		return WorkspaceCreateResult{}, err
	}

	registerWorkspace(&cfg, name, root, s.clock())
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return WorkspaceCreateResult{}, err
	}

	repoPlans, err := buildNewWorkspaceRepoPlans(cfg, input.Groups, input.Repos)
	if err != nil {
		return WorkspaceCreateResult{}, err
	}
	warnings := []string{}
	pendingHooks := []HookPending{}
	for _, plan := range repoPlans {
		_, resolvedRemote, repoWarnings, err := ops.AddRepo(ctx, ops.AddRepoInput{
			WorkspaceRoot: ws.Root,
			Name:          plan.Name,
			URL:           plan.URL,
			SourcePath:    plan.SourcePath,
			Defaults:      cfg.Defaults,
			Remote:        plan.Remote,
			DefaultBranch: plan.DefaultBranch,
			AllowFallback: false,
			Git:           s.git,
		})
		if err != nil {
			return WorkspaceCreateResult{}, err
		}
		if len(repoWarnings) > 0 {
			warnings = append(warnings, repoWarnings...)
		}
		if alias, ok := cfg.Repos[plan.Name]; ok {
			aliasUpdated := false
			if alias.Remote == "" && resolvedRemote != "" {
				alias.Remote = resolvedRemote
				aliasUpdated = true
			}
			if alias.DefaultBranch == "" && plan.DefaultBranch != "" {
				alias.DefaultBranch = plan.DefaultBranch
				aliasUpdated = true
			}
			if aliasUpdated {
				cfg.Repos[plan.Name] = alias
			}
		}
		repoDir := plan.Name
		worktreePath := workspace.RepoWorktreePath(ws.Root, ws.State.CurrentBranch, repoDir)
		pending, hookWarnings, err := s.runWorktreeCreatedHooks(ctx, cfg, ws.Root, name, config.RepoConfig{
			Name:    plan.Name,
			RepoDir: repoDir,
		}, worktreePath, ws.State.CurrentBranch, "workspace.create")
		if err != nil {
			return WorkspaceCreateResult{}, err
		}
		if len(hookWarnings) > 0 {
			warnings = append(warnings, hookWarnings...)
		}
		if len(pending.Hooks) > 0 {
			pendingHooks = append(pendingHooks, pending)
		}
	}

	warnings = append(warnings, warnOutsideWorkspaceRoot(root, cfg.Defaults.WorkspaceRoot)...)

	infoPayload := WorkspaceCreatedJSON{
		Name:    name,
		Path:    root,
		Workset: workspace.WorksetFile(root),
		Branch:  ws.State.CurrentBranch,
		Next:    fmt.Sprintf("workset repo add -w %s <alias|url>", name),
	}

	registerWorkspace(&cfg, name, root, s.clock())
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return WorkspaceCreateResult{}, err
	}
	return WorkspaceCreateResult{Workspace: infoPayload, Warnings: warnings, PendingHooks: pendingHooks, Config: info}, nil
}

// DeleteWorkspace removes a workspace registration or deletes files when requested.
func (s *Service) DeleteWorkspace(ctx context.Context, input WorkspaceDeleteInput) (WorkspaceDeleteResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return WorkspaceDeleteResult{}, err
	}
	name, root, err := resolveWorkspaceSelector(&cfg, input.Selector)
	if err != nil {
		return WorkspaceDeleteResult{}, err
	}
	wsConfig := config.WorkspaceConfig{}
	if input.DeleteFiles {
		if loaded, err := s.workspaces.LoadConfig(ctx, root); err == nil {
			wsConfig = loaded
		}
	}

	if input.DeleteFiles {
		absTarget, err := filepath.Abs(root)
		if err != nil {
			return WorkspaceDeleteResult{}, err
		}
		absTarget = filepath.Clean(absTarget)
		if info.Exists && info.Path != "" {
			absConfig, err := filepath.Abs(info.Path)
			if err != nil {
				return WorkspaceDeleteResult{}, err
			}
			absConfig = filepath.Clean(absConfig)
			if absConfig == absTarget || strings.HasPrefix(absConfig, absTarget+string(os.PathSeparator)) {
				return WorkspaceDeleteResult{}, UnsafeOperation{
					Message: fmt.Sprintf("refusing to delete %s: contains global config %s", absTarget, absConfig),
				}
			}
		}
		workspaceRoot := cfg.Defaults.WorkspaceRoot
		if workspaceRoot != "" {
			absRoot, err := filepath.Abs(workspaceRoot)
			if err == nil {
				absRoot = filepath.Clean(absRoot)
				inside := absTarget == absRoot || strings.HasPrefix(absTarget, absRoot+string(os.PathSeparator))
				if !inside && !input.Force {
					return WorkspaceDeleteResult{}, UnsafeOperation{Message: fmt.Sprintf("refusing to delete outside defaults.workspace_root (%s); use --force to override", absRoot)}
				}
			}
		}
		contained, err := workspacesWithin(cfg, name, absTarget)
		if err != nil {
			return WorkspaceDeleteResult{}, err
		}
		if len(contained) > 0 {
			return WorkspaceDeleteResult{}, UnsafeOperation{
				Message: fmt.Sprintf("refusing to delete %s: contains other workspaces: %s", absTarget, strings.Join(contained, ", ")),
			}
		}
	}

	var report ops.WorkspaceSafetyReport
	var warnings []string
	var unpushed []string
	if input.DeleteFiles {
		report, err = ops.CheckWorkspaceSafety(ctx, ops.WorkspaceSafetyInput{
			WorkspaceRoot: root,
			Defaults:      cfg.Defaults,
			RepoDefaults:  repoDefaultsMap(wsConfig, cfg),
			Git:           s.git,
			FetchRemotes:  input.FetchRemotes,
		})
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Missing workset.yaml, skip safety checks.
			} else if !input.Force {
				return WorkspaceDeleteResult{}, err
			}
		}

		var dirty []string
		var unmerged []string
		dirty, unmerged, unpushed, warnings = summarizeWorkspaceSafety(report)
		if !input.Force {
			if len(dirty) > 0 {
				return WorkspaceDeleteResult{}, UnsafeOperation{
					Message:  fmt.Sprintf("refusing to delete: dirty worktrees: %s (use --force)", strings.Join(dirty, ", ")),
					Dirty:    dirty,
					Warnings: warnings,
				}
			}
			if len(unmerged) > 0 {
				warnings = append(warnings, unmergedWorkspaceDetails(report)...)
				return WorkspaceDeleteResult{}, UnsafeOperation{
					Message:  fmt.Sprintf("refusing to delete: unmerged branches: %s (use --force)", strings.Join(unmerged, ", ")),
					Unmerged: unmerged,
					Warnings: warnings,
				}
			}
		}
	}

	if input.DeleteFiles && !input.Confirmed {
		return WorkspaceDeleteResult{}, ConfirmationRequired{Message: fmt.Sprintf("delete workspace %s?", root)}
	}

	if input.DeleteFiles {
		if err := s.stopWorkspaceSessions(ctx, root, input.Force); err != nil {
			return WorkspaceDeleteResult{}, err
		}
		if err := s.removeWorkspaceRepoWorktrees(ctx, root, cfg.Defaults, input.Force); err != nil {
			return WorkspaceDeleteResult{}, err
		}
		if err := os.RemoveAll(root); err != nil {
			return WorkspaceDeleteResult{}, err
		}
	}

	configChanged := false
	if name != "" {
		if _, ok := cfg.Workspaces[name]; ok {
			delete(cfg.Workspaces, name)
			configChanged = true
		}
	} else {
		before := len(cfg.Workspaces)
		removeWorkspaceByPath(&cfg, root)
		if len(cfg.Workspaces) != before {
			configChanged = true
		}
	}
	if cfg.Defaults.Workspace == name || cfg.Defaults.Workspace == root {
		cfg.Defaults.Workspace = ""
		configChanged = true
	}
	if configChanged {
		if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
			return WorkspaceDeleteResult{}, err
		}
	}

	payload := WorkspaceDeleteResultJSON{
		Status:       "ok",
		Name:         name,
		Path:         root,
		DeletedFiles: input.DeleteFiles,
	}
	return WorkspaceDeleteResult{Payload: payload, Warnings: warnings, Unpushed: unpushed, Safety: report, Config: info}, nil
}

// StatusWorkspace reports per-repo status for a workspace.
func (s *Service) StatusWorkspace(ctx context.Context, selector WorkspaceSelector) (WorkspaceStatusResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return WorkspaceStatusResult{}, err
	}
	wsRoot, wsConfig, err := s.resolveWorkspace(ctx, &cfg, info.Path, selector)
	if err != nil {
		return WorkspaceStatusResult{}, err
	}

	statuses, err := ops.Status(ctx, ops.StatusInput{
		WorkspaceRoot:       wsRoot,
		Defaults:            cfg.Defaults,
		RepoDefaultBranches: repoDefaultBranches(wsConfig, cfg),
		Git:                 s.git,
	})
	if err != nil {
		return WorkspaceStatusResult{}, err
	}
	payload := make([]RepoStatusJSON, 0, len(statuses))
	for _, repo := range statuses {
		state := "clean"
		switch {
		case repo.Missing:
			state = "missing"
		case repo.Dirty:
			state = "dirty"
		case repo.Err != nil:
			state = "error"
		}
		entry := RepoStatusJSON{
			Name:    repo.Name,
			Path:    repo.Path,
			State:   state,
			Dirty:   repo.Dirty,
			Missing: repo.Missing,
		}
		if repo.Err != nil {
			entry.Error = repo.Err.Error()
		}
		payload = append(payload, entry)
	}

	registerWorkspace(&cfg, wsConfig.Name, wsRoot, s.clock())
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return WorkspaceStatusResult{}, err
	}

	return WorkspaceStatusResult{Statuses: payload, Config: info}, nil
}

func (s *Service) resolveWorkspace(ctx context.Context, cfg *config.GlobalConfig, configPath string, selector WorkspaceSelector) (string, config.WorkspaceConfig, error) {
	arg := strings.TrimSpace(selector.Value)
	if arg == "" {
		arg = strings.TrimSpace(cfg.Defaults.Workspace)
	}
	if arg == "" {
		return "", config.WorkspaceConfig{}, ValidationError{Message: "workspace required"}
	}

	var root string
	if ref, ok := cfg.Workspaces[arg]; ok {
		root = ref.Path
	} else if cfg.Defaults.WorkspaceRoot != "" {
		candidates := []string{filepath.Join(cfg.Defaults.WorkspaceRoot, arg)}
		if sanitized := workspace.WorkspaceDirName(arg); sanitized != "" && sanitized != arg {
			candidates = append(candidates, filepath.Join(cfg.Defaults.WorkspaceRoot, sanitized))
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				root = candidate
				break
			}
		}
	}
	if root == "" {
		if filepath.IsAbs(arg) {
			root = arg
		} else {
			return "", config.WorkspaceConfig{}, NotFoundError{Message: fmt.Sprintf("workspace not found: %q", arg)}
		}
	}

	wsConfig, err := s.workspaces.LoadConfig(ctx, root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", config.WorkspaceConfig{}, NotFoundError{Message: "workset.yaml not found at " + worksetFilePath(root)}
		}
		return "", config.WorkspaceConfig{}, err
	}
	if err := s.migrateLegacyWorkspaceRemotes(ctx, cfg, configPath, root, &wsConfig); err != nil {
		return "", config.WorkspaceConfig{}, err
	}

	if cfg.Workspaces == nil {
		cfg.Workspaces = map[string]config.WorkspaceRef{}
	}
	ref, exists := cfg.Workspaces[wsConfig.Name]
	if exists && ref.Path != "" && ref.Path != root {
		return "", config.WorkspaceConfig{}, ConflictError{Message: "workspace name already registered to a different path"}
	}
	if !exists {
		registerWorkspace(cfg, wsConfig.Name, root, s.clock())
		if err := s.configs.Save(ctx, configPath, *cfg); err != nil {
			return "", config.WorkspaceConfig{}, err
		}
	}

	return root, wsConfig, nil
}

func warnOutsideWorkspaceRoot(root, workspaceRoot string) []string {
	if workspaceRoot == "" {
		return nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	absWorkspace, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil
	}
	absRoot = filepath.Clean(absRoot)
	absWorkspace = filepath.Clean(absWorkspace)
	if absRoot == absWorkspace || strings.HasPrefix(absRoot, absWorkspace+string(os.PathSeparator)) {
		return nil
	}
	return []string{fmt.Sprintf("workspace created outside defaults.workspace_root (%s)", absWorkspace)}
}

func workspacesWithin(cfg config.GlobalConfig, targetName, absTarget string) ([]string, error) {
	if absTarget == "" {
		return nil, nil
	}
	contained := []string{}
	for name, ref := range cfg.Workspaces {
		path := strings.TrimSpace(ref.Path)
		if path == "" {
			continue
		}
		absOther, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		absOther = filepath.Clean(absOther)
		if name == targetName && absOther == absTarget {
			continue
		}
		if absOther == absTarget || strings.HasPrefix(absOther, absTarget+string(os.PathSeparator)) {
			contained = append(contained, name)
		}
	}
	sort.Strings(contained)
	return contained, nil
}
