package worksetapi

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/hooks"
	"github.com/strantalis/workset/internal/workspace"
)

func (s *Service) RunHooks(ctx context.Context, input HooksRunInput) (HooksRunResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return HooksRunResult{}, err
	}
	wsRoot, wsConfig, err := s.resolveWorkspace(ctx, &cfg, info.Path, input.Workspace)
	if err != nil {
		return HooksRunResult{}, err
	}

	repoName := strings.TrimSpace(input.Repo)
	if repoName == "" {
		return HooksRunResult{}, ValidationError{Message: "repo name required"}
	}

	repo, ok := findWorkspaceRepo(wsConfig, repoName)
	if !ok {
		return HooksRunResult{}, NotFoundError{Message: fmt.Sprintf("repo %q not found in workspace", repoName)}
	}

	state, err := s.workspaces.LoadState(ctx, wsRoot)
	if err != nil {
		return HooksRunResult{}, err
	}
	branch := state.CurrentBranch
	if branch == "" {
		branch = cfg.Defaults.BaseBranch
	}
	worktreePath := workspace.RepoWorktreePath(wsRoot, branch, repo.RepoDir)

	hookFile, exists, err := hooks.LoadRepoHooks(worktreePath)
	if err != nil {
		return HooksRunResult{}, err
	}
	if !exists || len(hookFile.Hooks) == 0 {
		return HooksRunResult{}, NotFoundError{Message: "no repo hooks found"}
	}

	event := hooks.Event(strings.TrimSpace(input.Event))
	if event == "" {
		event = hooks.EventWorktreeCreated
	}

	wsName := wsConfig.Name
	if wsName == "" {
		wsName = workspaceNameByPath(&cfg, wsRoot)
	}
	if wsName == "" {
		wsName = filepath.Base(wsRoot)
	}

	ctxPayload := hooks.Context{
		WorkspaceRoot:   wsRoot,
		WorkspaceName:   wsName,
		WorkspaceConfig: workspace.WorksetFile(wsRoot),
		RepoName:        repo.Name,
		RepoDir:         repo.RepoDir,
		RepoPath:        worktreePath,
		WorktreePath:    worktreePath,
		Branch:          branch,
		Event:           event,
		Reason:          input.Reason,
	}

	engine := hooks.Engine{Runner: s.hookRunner, Clock: s.clock}
	report, err := engine.Run(ctx, hooks.RunInput{
		Event:          event,
		Hooks:          hookFile.Hooks,
		DefaultOnError: cfg.Hooks.OnError,
		LogRoot:        hooksLogRoot(wsRoot),
		Context:        ctxPayload,
	})
	if err != nil {
		return HooksRunResult{}, err
	}
	if len(report.Results) == 0 {
		return HooksRunResult{}, NotFoundError{Message: fmt.Sprintf("no hooks matched event %q", event)}
	}

	if input.TrustRepo {
		if addTrustedRepo(&cfg, repo.Name) {
			if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
				return HooksRunResult{}, err
			}
		}
	}

	results := make([]HookRunJSON, 0, len(report.Results))
	for _, res := range report.Results {
		status := HookRunStatus(res.Status)
		if status == "" {
			status = HookRunStatusOK
		}
		results = append(results, HookRunJSON{
			ID:      res.HookID,
			Status:  status,
			LogPath: res.LogPath,
		})
	}

	return HooksRunResult{
		Event:   string(event),
		Repo:    repo.Name,
		Results: results,
		Config:  info,
	}, nil
}

func (s *Service) TrustRepoHooks(ctx context.Context, repoName string) (config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return info, err
	}
	repoName = strings.TrimSpace(repoName)
	if repoName == "" {
		return info, ValidationError{Message: "repo name required"}
	}
	if addTrustedRepo(&cfg, repoName) {
		if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
			return info, err
		}
	}
	return info, nil
}

func (s *Service) runWorktreeCreatedHooks(ctx context.Context, cfg config.GlobalConfig, wsRoot, wsName string, repo config.RepoConfig, worktreePath, branch, reason string) (HookPending, []string, error) {
	hookFile, exists, err := hooks.LoadRepoHooks(worktreePath)
	if err != nil {
		return HookPending{}, nil, err
	}
	if !exists || len(hookFile.Hooks) == 0 {
		return HookPending{}, nil, nil
	}

	event := hooks.EventWorktreeCreated
	candidateIDs := hookIDsForEvent(hookFile.Hooks, event)
	if len(candidateIDs) == 0 {
		return HookPending{}, nil, nil
	}

	if !cfg.Hooks.Enabled {
		warn := fmt.Sprintf("repo hooks found for %s but hooks are disabled (set hooks.enabled to true)", repo.Name)
		return HookPending{
			Event:  string(event),
			Repo:   repo.Name,
			Hooks:  candidateIDs,
			Status: HookRunStatusSkipped,
			Reason: "disabled",
		}, []string{warn}, nil
	}

	if !isTrustedRepo(&cfg, repo.Name) {
		pending := HookPending{
			Event:  string(event),
			Repo:   repo.Name,
			Hooks:  candidateIDs,
			Status: HookRunStatusSkipped,
			Reason: "untrusted",
		}
		warn := fmt.Sprintf("repo %s defines hooks; run `workset hooks run -w %s %s` to execute or trust", repo.Name, wsName, repo.Name)
		return pending, []string{warn}, nil
	}

	ctxPayload := hooks.Context{
		WorkspaceRoot:   wsRoot,
		WorkspaceName:   wsName,
		WorkspaceConfig: workspace.WorksetFile(wsRoot),
		RepoName:        repo.Name,
		RepoDir:         repo.RepoDir,
		RepoPath:        worktreePath,
		WorktreePath:    worktreePath,
		Branch:          branch,
		Event:           event,
		Reason:          reason,
	}
	engine := hooks.Engine{Runner: s.hookRunner, Clock: s.clock}
	if _, err := engine.Run(ctx, hooks.RunInput{
		Event:          event,
		Hooks:          hookFile.Hooks,
		DefaultOnError: cfg.Hooks.OnError,
		LogRoot:        hooksLogRoot(wsRoot),
		Context:        ctxPayload,
	}); err != nil {
		return HookPending{}, nil, err
	}
	return HookPending{}, nil, nil
}

func hooksLogRoot(workspaceRoot string) string {
	return filepath.Join(workspace.LogsPath(workspaceRoot), "hooks")
}

func addTrustedRepo(cfg *config.GlobalConfig, repoName string) bool {
	if cfg == nil {
		return false
	}
	if repoName == "" {
		return false
	}
	if slices.Contains(cfg.Hooks.RepoHooks.TrustedRepos, repoName) {
		return false
	}
	cfg.Hooks.RepoHooks.TrustedRepos = append(cfg.Hooks.RepoHooks.TrustedRepos, repoName)
	sort.Strings(cfg.Hooks.RepoHooks.TrustedRepos)
	return true
}

func isTrustedRepo(cfg *config.GlobalConfig, repoName string) bool {
	if cfg == nil || repoName == "" {
		return false
	}
	return slices.Contains(cfg.Hooks.RepoHooks.TrustedRepos, repoName)
}

func hookIDsForEvent(hooksList []hooks.Hook, event hooks.Event) []string {
	ids := make([]string, 0, len(hooksList))
	for _, hook := range hooksList {
		if slices.Contains(hook.On, event) {
			if hook.ID != "" {
				ids = append(ids, hook.ID)
			}
		}
	}
	return ids
}

func findWorkspaceRepo(wsConfig config.WorkspaceConfig, name string) (config.RepoConfig, bool) {
	for _, repo := range wsConfig.Repos {
		if repo.Name == name {
			return repo, true
		}
	}
	return config.RepoConfig{}, false
}
