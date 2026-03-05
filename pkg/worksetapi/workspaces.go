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
	template := strings.TrimSpace(input.Template)

	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return WorkspaceCreateResult{}, err
	}
	if input.WorksetOnly {
		return s.createWorksetOnly(ctx, cfg, info, input, name)
	}
	if err := workspaceCreateConflict(cfg, name, ""); err != nil {
		return WorkspaceCreateResult{}, err
	}

	root := strings.TrimSpace(input.Path)
	if root == "" {
		base := strings.TrimSpace(cfg.Defaults.WorksetRoot)
		if base == "" {
			base = strings.TrimSpace(cfg.Defaults.WorkspaceRoot)
			if strings.EqualFold(filepath.Base(base), "workspaces") {
				base = filepath.Dir(base)
			}
		}
		if base == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return WorkspaceCreateResult{}, err
			}
			base = cwd
		}
		if template != "" {
			root = filepath.Join(
				base,
				"worksets",
				workspace.WorkspaceDirName(template),
				workspace.WorkspaceDirName(name),
			)
		} else {
			root = filepath.Join(base, "workspaces", workspace.WorkspaceDirName(name))
		}
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return WorkspaceCreateResult{}, err
	}
	worksetPath := workspace.WorksetFile(root)
	if _, err := os.Stat(worksetPath); err == nil {
		return WorkspaceCreateResult{}, ConflictError{
			Message: fmt.Sprintf("workspace %q already exists at %s", name, worksetPath),
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return WorkspaceCreateResult{}, err
	}

	ws, err := s.workspaces.Init(ctx, root, name, cfg.Defaults)
	if err != nil {
		return WorkspaceCreateResult{}, err
	}

	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		if err := workspaceCreateConflict(*cfg, name, ""); err != nil {
			return err
		}
		registerWorkspace(cfg, name, root, s.clock(), template)
		return nil
	}); err != nil {
		return WorkspaceCreateResult{}, err
	}

	repoPlans, err := buildNewWorkspaceRepoPlans(cfg, input.Groups, input.Repos)
	if err != nil {
		return WorkspaceCreateResult{}, err
	}
	type aliasUpdate struct {
		remote string
		branch string
	}
	aliasUpdates := map[string]aliasUpdate{}
	warnings := []string{}
	pendingHooks := []HookPending{}
	hookRuns := []HookExecutionJSON{}
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
			update := aliasUpdates[plan.Name]
			if alias.Remote == "" && resolvedRemote != "" {
				update.remote = resolvedRemote
				aliasUpdated = true
			}
			if alias.DefaultBranch == "" && plan.DefaultBranch != "" {
				update.branch = plan.DefaultBranch
				aliasUpdated = true
			}
			if aliasUpdated {
				aliasUpdates[plan.Name] = update
			}
		}
		repoDir := plan.Name
		worktreePath := workspace.RepoWorktreePath(ws.Root, ws.State.CurrentBranch, repoDir)
		pending, runs, hookWarnings, err := s.runWorktreeCreatedHooks(ctx, cfg, ws.Root, name, config.RepoConfig{
			Name:    plan.Name,
			RepoDir: repoDir,
		}, worktreePath, ws.State.CurrentBranch, "workspace.create")
		if err != nil {
			return WorkspaceCreateResult{}, err
		}
		if len(runs) > 0 {
			hookRuns = append(hookRuns, runs...)
		}
		if len(hookWarnings) > 0 {
			warnings = append(warnings, hookWarnings...)
		}
		if len(pending.Hooks) > 0 {
			pendingHooks = append(pendingHooks, pending)
		}
	}

	warnings = append(warnings, warnOutsideWorkspaceRoot(root, cfg.Defaults.WorksetRoot)...)

	infoPayload := WorkspaceCreatedJSON{
		Name:    name,
		Path:    root,
		Workset: workspace.WorksetFile(root),
		Branch:  ws.State.CurrentBranch,
		Next:    fmt.Sprintf("workset repo add -w %s <alias|url>", shellArg(name)),
	}

	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		if err := workspaceCreateConflict(*cfg, name, root); err != nil {
			return err
		}
		for aliasName, update := range aliasUpdates {
			alias, ok := cfg.Repos[aliasName]
			if !ok {
				continue
			}
			if alias.Remote == "" && update.remote != "" {
				alias.Remote = update.remote
			}
			if alias.DefaultBranch == "" && update.branch != "" {
				alias.DefaultBranch = update.branch
			}
			cfg.Repos[aliasName] = alias
		}
		registerWorkspace(cfg, name, root, s.clock(), template)
		s.rebuildWorksetRepoModel(ctx, cfg)
		return nil
	}); err != nil {
		return WorkspaceCreateResult{}, err
	}
	return WorkspaceCreateResult{
		Workspace:    infoPayload,
		Warnings:     warnings,
		PendingHooks: pendingHooks,
		HookRuns:     hookRuns,
		Config:       info,
	}, nil
}

func (s *Service) createWorksetOnly(
	ctx context.Context,
	cfg config.GlobalConfig,
	info config.GlobalConfigLoadInfo,
	input WorkspaceCreateInput,
	name string,
) (WorkspaceCreateResult, error) {
	if err := worksetCreateConflict(cfg, name); err != nil {
		return WorkspaceCreateResult{}, err
	}

	repoPlans, err := buildNewWorkspaceRepoPlans(cfg, input.Groups, input.Repos)
	if err != nil {
		return WorkspaceCreateResult{}, err
	}
	worksetRepos := make([]string, 0, len(repoPlans))
	for _, plan := range repoPlans {
		worksetRepos = append(worksetRepos, plan.Name)
	}
	worksetRepos = normalizeRepoNames(worksetRepos)

	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		if err := worksetCreateConflict(*cfg, name); err != nil {
			return err
		}
		if cfg.WorksetRepos == nil {
			cfg.WorksetRepos = map[string][]string{}
		}
		cfg.WorksetRepos[name] = append([]string(nil), worksetRepos...)
		if cfg.Repos == nil {
			cfg.Repos = map[string]config.RegisteredRepo{}
		}
		for _, plan := range repoPlans {
			alias, ok := cfg.Repos[plan.Name]
			if !ok {
				alias = config.RegisteredRepo{}
			}
			if alias.URL == "" && plan.URL != "" {
				alias.URL = plan.URL
			}
			if alias.Path == "" && plan.SourcePath != "" {
				alias.Path = plan.SourcePath
			}
			if alias.Remote == "" && plan.Remote != "" {
				alias.Remote = plan.Remote
			}
			if alias.DefaultBranch == "" && plan.DefaultBranch != "" {
				alias.DefaultBranch = plan.DefaultBranch
			}
			cfg.Repos[plan.Name] = alias
		}
		return nil
	}); err != nil {
		return WorkspaceCreateResult{}, err
	}

	return WorkspaceCreateResult{
		Workspace: WorkspaceCreatedJSON{
			Name:    name,
			Path:    "",
			Workset: name,
			Branch:  "",
			Next:    "workset workspace new <thread> --template " + shellArg(name),
		},
		Config: info,
	}, nil
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
		workspaceRoot := strings.TrimSpace(cfg.Defaults.WorksetRoot)
		if workspaceRoot == "" {
			workspaceRoot = cfg.Defaults.WorkspaceRoot
		}
		if workspaceRoot != "" {
			absRoot, err := filepath.Abs(workspaceRoot)
			if err == nil {
				absRoot = filepath.Clean(absRoot)
				inside := absTarget == absRoot || strings.HasPrefix(absTarget, absRoot+string(os.PathSeparator))
				if !inside && !input.Force {
					return WorkspaceDeleteResult{}, UnsafeOperation{Message: fmt.Sprintf("refusing to delete outside defaults.workset_root (%s); use --force to override", absRoot)}
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
		configChanged = true
	}
	if configChanged {
		if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
			info = loadInfo
			removedRefs := collectWorkspaceRefsForDelete(cfg, name, root)
			if name != "" {
				delete(cfg.Workspaces, name)
			} else {
				removeWorkspaceByPath(cfg, root)
			}
			applyWorksetRepoModelAfterWorkspaceRemoval(cfg, removedRefs)
			if cfg.Defaults.Workspace == name || cfg.Defaults.Workspace == root {
				cfg.Defaults.Workspace = ""
			}
			return nil
		}); err != nil {
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

type removedWorkspaceRef struct {
	name string
	ref  config.WorkspaceRef
}

func collectWorkspaceRefsForDelete(
	cfg *config.GlobalConfig,
	name string,
	root string,
) []removedWorkspaceRef {
	if cfg == nil || len(cfg.Workspaces) == 0 {
		return nil
	}
	removed := make([]removedWorkspaceRef, 0, 1)
	if name != "" {
		ref, ok := cfg.Workspaces[name]
		if ok {
			removed = append(removed, removedWorkspaceRef{name: name, ref: ref})
		}
		return removed
	}

	cleanRoot := filepath.Clean(root)
	if cleanRoot == "" {
		return nil
	}
	for workspaceName, ref := range cfg.Workspaces {
		if filepath.Clean(ref.Path) == cleanRoot {
			removed = append(removed, removedWorkspaceRef{name: workspaceName, ref: ref})
		}
	}
	return removed
}

func applyWorksetRepoModelAfterWorkspaceRemoval(
	cfg *config.GlobalConfig,
	removedRefs []removedWorkspaceRef,
) {
	if cfg == nil || len(removedRefs) == 0 {
		return
	}
	if cfg.WorksetRepos == nil {
		cfg.WorksetRepos = map[string][]string{}
	}
	affected := map[string][]removedWorkspaceRef{}
	for _, removed := range removedRefs {
		worksetName := worksetNameForThread(removed.name, removed.ref)
		if worksetName == "" {
			continue
		}
		affected[worksetName] = append(affected[worksetName], removed)
	}
	for worksetName, removedThreads := range affected {
		threadNames := listThreadsForWorkset(cfg.Workspaces, worksetName)
		if len(threadNames) == 0 {
			preservedRepos := normalizeRepoNames(cfg.WorksetRepos[worksetName])
			originalBase := preservedRepos
			for _, removed := range removedThreads {
				effective := resolveRemovedThreadRepos(removed.ref, originalBase)
				preservedRepos = normalizeRepoNames(append(preservedRepos, effective...))
			}
			cfg.WorksetRepos[worksetName] = preservedRepos
			continue
		}
		recomputeWorksetRepoModelForThreads(cfg, worksetName, threadNames)
	}
}

func worksetNameForThread(threadName string, ref config.WorkspaceRef) string {
	worksetName := strings.TrimSpace(workspaceRefWorkset(ref))
	if worksetName == "" {
		worksetName = strings.TrimSpace(threadName)
	}
	return worksetName
}

func listThreadsForWorkset(
	workspaces map[string]config.WorkspaceRef,
	worksetName string,
) []string {
	normalizedWorkset := strings.TrimSpace(worksetName)
	if normalizedWorkset == "" {
		return nil
	}
	threads := make([]string, 0, 4)
	for threadName, ref := range workspaces {
		if worksetNameForThread(threadName, ref) != normalizedWorkset {
			continue
		}
		threads = append(threads, threadName)
	}
	sort.Strings(threads)
	return threads
}

func recomputeWorksetRepoModelForThreads(
	cfg *config.GlobalConfig,
	worksetName string,
	threadNames []string,
) {
	threadRepos := map[string][]string{}
	for _, threadName := range threadNames {
		ref, ok := cfg.Workspaces[threadName]
		if !ok {
			continue
		}
		repos, hasRepos := loadThreadRepoNamesForRef(ref)
		if !hasRepos {
			continue
		}
		threadRepos[threadName] = repos
	}
	baseRepos, hasBase := intersectThreadRepos(threadNames, threadRepos)
	if !hasBase {
		baseRepos = nil
	}
	cfg.WorksetRepos[worksetName] = baseRepos

	for _, threadName := range threadNames {
		ref, ok := cfg.Workspaces[threadName]
		if !ok {
			continue
		}
		currentOverrides := normalizeRepoNames(ref.RepoOverrides)
		nextOverrides := currentOverrides
		if repos, hasRepos := threadRepos[threadName]; hasRepos {
			nextOverrides = subtractRepoNames(repos, baseRepos)
		} else if len(baseRepos) > 0 {
			nextOverrides = subtractRepoNames(currentOverrides, baseRepos)
		}
		if sameRepoNames(currentOverrides, nextOverrides) {
			continue
		}
		ref.RepoOverrides = nextOverrides
		cfg.Workspaces[threadName] = ref
	}
}

func resolveRemovedThreadRepos(ref config.WorkspaceRef, worksetBase []string) []string {
	if repos, hasRepos := loadThreadRepoNamesForRef(ref); hasRepos {
		return repos
	}
	return normalizeRepoNames(append(worksetBase, normalizeRepoNames(ref.RepoOverrides)...))
}

func loadThreadRepoNamesForRef(ref config.WorkspaceRef) ([]string, bool) {
	root := strings.TrimSpace(ref.Path)
	if root == "" {
		return nil, false
	}
	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		return nil, false
	}
	repos := make([]string, 0, len(wsCfg.Repos))
	for _, repo := range wsCfg.Repos {
		repoName := strings.TrimSpace(repo.Name)
		if repoName == "" {
			continue
		}
		repos = append(repos, repoName)
	}
	normalized := normalizeRepoNames(repos)
	return normalized, len(normalized) > 0
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

	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		registerWorkspace(cfg, wsConfig.Name, wsRoot, s.clock(), "")
		return nil
	}); err != nil {
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
		if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, _ config.GlobalConfigLoadInfo) error {
			if existing, ok := cfg.Workspaces[wsConfig.Name]; ok && existing.Path != "" && existing.Path != root {
				return ConflictError{Message: "workspace name already registered to a different path"}
			}
			registerWorkspace(cfg, wsConfig.Name, root, s.clock(), "")
			s.rebuildWorksetRepoModel(ctx, cfg)
			return nil
		}); err != nil {
			return "", config.WorkspaceConfig{}, err
		}
	}

	return root, wsConfig, nil
}

func warnOutsideWorkspaceRoot(root, worksetRoot string) []string {
	if worksetRoot == "" {
		return nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	absWorkspace, err := filepath.Abs(worksetRoot)
	if err != nil {
		return nil
	}
	absRoot = filepath.Clean(absRoot)
	absWorkspace = filepath.Clean(absWorkspace)
	if absRoot == absWorkspace || strings.HasPrefix(absRoot, absWorkspace+string(os.PathSeparator)) {
		return nil
	}
	return []string{fmt.Sprintf("thread created outside defaults.workset_root (%s)", absWorkspace)}
}

func shellArg(value string) string {
	if value == "" {
		return shellEscape(value)
	}
	if strings.ContainsAny(value, " \t\r\n'\"\\$`!&|;<>()[]{}*?~") {
		return shellEscape(value)
	}
	return value
}

func workspaceCreateConflict(cfg config.GlobalConfig, name, allowPath string) error {
	ref, ok := cfg.Workspaces[name]
	if !ok {
		return nil
	}
	path := strings.TrimSpace(ref.Path)
	if path != "" && allowPath != "" && samePath(path, allowPath) {
		return nil
	}
	if path != "" {
		return ConflictError{Message: fmt.Sprintf("workspace %q already exists at %s", name, path)}
	}
	return ConflictError{Message: fmt.Sprintf("workspace %q already exists", name)}
}

func worksetCreateConflict(cfg config.GlobalConfig, name string) error {
	worksetName := strings.TrimSpace(name)
	if worksetName == "" {
		return nil
	}
	if _, ok := cfg.WorksetRepos[worksetName]; ok {
		return ConflictError{Message: fmt.Sprintf("workset %q already exists", worksetName)}
	}
	for threadName, ref := range cfg.Workspaces {
		if strings.TrimSpace(threadName) == worksetName {
			return ConflictError{Message: fmt.Sprintf("workset %q already exists", worksetName)}
		}
		threadWorkset := strings.TrimSpace(workspaceRefWorkset(ref))
		if threadWorkset == "" {
			threadWorkset = strings.TrimSpace(threadName)
		}
		if threadWorkset == worksetName {
			return ConflictError{Message: fmt.Sprintf("workset %q already exists", worksetName)}
		}
	}
	return nil
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		return filepath.Clean(absA) == filepath.Clean(absB)
	}
	return filepath.Clean(a) == filepath.Clean(b)
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
