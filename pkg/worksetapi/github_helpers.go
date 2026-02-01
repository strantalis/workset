package worksetapi

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/ops"
	"github.com/strantalis/workset/internal/workspace"
)

type repoResolution struct {
	ConfigInfo    config.GlobalConfigLoadInfo
	WorkspaceName string
	WorkspaceRoot string
	Workspace     config.WorkspaceConfig
	Repo          config.RepoConfig
	RepoPath      string
	Branch        string
	Defaults      config.Defaults
	RepoDefaults  ops.RepoDefaults
}

func (s *Service) resolveRepo(ctx context.Context, input RepoSelectionInput) (repoResolution, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return repoResolution{}, err
	}
	wsRoot, wsConfig, err := s.resolveWorkspace(ctx, &cfg, info.Path, input.Workspace)
	if err != nil {
		return repoResolution{}, err
	}
	ws, err := s.workspaces.Load(ctx, wsRoot, cfg.Defaults)
	if err != nil {
		return repoResolution{}, err
	}
	branch := ws.State.CurrentBranch
	if branch == "" {
		branch = cfg.Defaults.BaseBranch
	}

	repoName := strings.TrimSpace(input.Repo)
	if repoName == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return repoResolution{}, err
		}
		repoName = repoNameFromCWD(cwd, wsRoot, branch, cfg.Defaults, wsConfig.Repos)
	}
	if repoName == "" {
		if len(wsConfig.Repos) == 1 {
			repoName = wsConfig.Repos[0].Name
		} else {
			names := make([]string, 0, len(wsConfig.Repos))
			for _, repo := range wsConfig.Repos {
				names = append(names, repo.Name)
			}
			return repoResolution{}, ValidationError{Message: "repo required; available: " + strings.Join(names, ", ")}
		}
	}

	for _, repo := range wsConfig.Repos {
		if repo.Name != repoName {
			continue
		}
		config.ApplyRepoDefaults(&repo, cfg.Defaults)
		repoPath := resolveRepoPath(wsRoot, branch, repo)
		if repoPath == "" {
			return repoResolution{}, ValidationError{Message: fmt.Sprintf("repo path unavailable for %q", repo.Name)}
		}
		repoDefaults := resolveRepoDefaults(cfg, repo.Name)
		return repoResolution{
			ConfigInfo:    info,
			WorkspaceName: wsConfig.Name,
			WorkspaceRoot: wsRoot,
			Workspace:     wsConfig,
			Repo:          repo,
			RepoPath:      repoPath,
			Branch:        branch,
			Defaults:      cfg.Defaults,
			RepoDefaults:  repoDefaults,
		}, nil
	}
	return repoResolution{}, NotFoundError{Message: fmt.Sprintf("repo %q not found in workspace", repoName)}
}

func repoNameFromCWD(cwd, wsRoot, branch string, defaults config.Defaults, repos []config.RepoConfig) string {
	if cwd == "" {
		return ""
	}
	cwd = filepath.Clean(cwd)
	bestMatch := ""
	bestLen := 0
	for _, repo := range repos {
		config.ApplyRepoDefaults(&repo, defaults)
		path := resolveRepoPath(wsRoot, branch, repo)
		if path == "" {
			continue
		}
		clean := filepath.Clean(path)
		if !pathContains(cwd, clean) {
			continue
		}
		if len(clean) > bestLen {
			bestLen = len(clean)
			bestMatch = repo.Name
		}
	}
	return bestMatch
}

func resolveRepoPath(workspaceRoot, branch string, repo config.RepoConfig) string {
	if repo.RepoDir != "" && workspaceRoot != "" {
		worktreePath := workspace.RepoWorktreePath(workspaceRoot, branch, repo.RepoDir)
		if stat, err := os.Stat(worktreePath); err == nil && stat.IsDir() {
			return worktreePath
		}
	}
	if repo.LocalPath != "" {
		return repo.LocalPath
	}
	if repo.RepoDir != "" && workspaceRoot != "" {
		return workspace.RepoWorktreePath(workspaceRoot, branch, repo.RepoDir)
	}
	return ""
}

func pathContains(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

type remoteInfo struct {
	Remote string
	Host   string
	Owner  string
	Repo   string
	URL    string
}

func parseGitHubRemoteURL(raw string) (remoteInfo, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return remoteInfo{}, errors.New("remote URL required")
	}
	if strings.Contains(raw, "://") {
		return parseURLRemote(raw)
	}
	// Handle scp-like syntax: git@github.com:owner/repo.git
	if strings.Contains(raw, ":") && strings.Contains(raw, "@") {
		return parseSCPRemote(raw)
	}
	return parseURLRemote("https://" + raw)
}

func parseSCPRemote(raw string) (remoteInfo, error) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return remoteInfo{}, fmt.Errorf("invalid remote URL: %s", raw)
	}
	hostPart := parts[0]
	pathPart := parts[1]
	if at := strings.LastIndex(hostPart, "@"); at != -1 {
		hostPart = hostPart[at+1:]
	}
	return parseHostPath(hostPart, pathPart)
}

func parseURLRemote(raw string) (remoteInfo, error) {
	parsed, err := parseURL(raw)
	if err != nil {
		return remoteInfo{}, err
	}
	host := parsed.Hostname()
	path := strings.TrimPrefix(parsed.Path, "/")
	return parseHostPath(host, path)
}

func parseHostPath(host, path string) (remoteInfo, error) {
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return remoteInfo{}, fmt.Errorf("invalid repo path: %s", path)
	}
	return remoteInfo{
		Host:  host,
		Owner: parts[0],
		Repo:  parts[1],
	}, nil
}

func parseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid URL: %s", raw)
	}
	return parsed, nil
}
