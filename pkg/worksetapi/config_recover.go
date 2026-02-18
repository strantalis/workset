package worksetapi

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
)

// ConfigRecoverInput controls config recovery behavior.
type ConfigRecoverInput struct {
	WorkspaceRoot string
	RebuildRepos  bool
	DryRun        bool
}

// ConfigRecoverResultJSON is the JSON payload for config recovery.
type ConfigRecoverResultJSON struct {
	Status              string   `json:"status"`
	WorkspaceRoot       string   `json:"workspace_root"`
	WorkspacesRecovered []string `json:"workspaces_recovered,omitempty"`
	ReposRecovered      []string `json:"repos_recovered,omitempty"`
	Conflicts           []string `json:"conflicts,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
	DryRun              bool     `json:"dry_run"`
}

// ConfigRecoverResult wraps the recovery payload with config metadata.
type ConfigRecoverResult struct {
	Payload ConfigRecoverResultJSON
	Config  config.GlobalConfigLoadInfo
}

type recoverCandidate struct {
	name   string
	root   string
	config config.WorkspaceConfig
}

type recoverApplyResult struct {
	recovered      []string
	reposRecovered map[string]struct{}
	conflicts      []string
	warnings       []string
	configChanged  bool
}

// RecoverConfig rebuilds workspace registrations (and optionally repo aliases) from workset.yaml files.
func (s *Service) RecoverConfig(ctx context.Context, input ConfigRecoverInput) (ConfigRecoverResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return ConfigRecoverResult{}, err
	}

	root := strings.TrimSpace(input.WorkspaceRoot)
	if root == "" {
		root = strings.TrimSpace(cfg.Defaults.WorkspaceRoot)
	}
	if root == "" {
		root = config.DefaultConfig().Defaults.WorkspaceRoot
	}
	if root == "" {
		return ConfigRecoverResult{}, ValidationError{Message: "workspace root required"}
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ConfigRecoverResult{}, err
	}
	absRoot = filepath.Clean(absRoot)
	if _, err := os.Stat(absRoot); err != nil {
		if os.IsNotExist(err) {
			return ConfigRecoverResult{}, NotFoundError{Message: "workspace root not found: " + absRoot}
		}
		return ConfigRecoverResult{}, err
	}

	preWarnings := []string{}

	worksetFiles, err := findWorksetFiles(absRoot)
	if err != nil {
		return ConfigRecoverResult{}, err
	}
	if len(worksetFiles) == 0 {
		preWarnings = append(preWarnings, "no workset.yaml files found under "+absRoot)
	}

	candidates := []recoverCandidate{}
	for _, worksetFile := range worksetFiles {
		wsRoot := filepath.Dir(worksetFile)
		wsConfig, err := config.LoadWorkspace(worksetFile)
		if err != nil {
			preWarnings = append(preWarnings, fmt.Sprintf("failed to load %s: %v", worksetFile, err))
			continue
		}
		name := strings.TrimSpace(wsConfig.Name)
		if name == "" {
			name = filepath.Base(wsRoot)
		}
		if name == "" {
			preWarnings = append(preWarnings, fmt.Sprintf("skipping %s: workspace name missing", wsRoot))
			continue
		}
		candidates = append(candidates, recoverCandidate{
			name:   name,
			root:   wsRoot,
			config: wsConfig,
		})
	}

	now := s.clock()
	applyResult := s.applyRecoverCandidates(&cfg, candidates, input.RebuildRepos, now)
	if !input.DryRun {
		if applyResult.configChanged {
			_, err := s.updateGlobal(ctx, func(target *config.GlobalConfig, info config.GlobalConfigLoadInfo) error {
				applyResult = s.applyRecoverCandidates(target, candidates, input.RebuildRepos, now)
				return nil
			})
			if err != nil {
				return ConfigRecoverResult{}, err
			}
		}
	}

	recovered := applyResult.recovered
	reposRecovered := applyResult.reposRecovered
	conflicts := applyResult.conflicts
	warnings := append([]string{}, preWarnings...)
	warnings = append(warnings, applyResult.warnings...)

	sort.Strings(recovered)
	recoveredRepos := make([]string, 0, len(reposRecovered))
	for repo := range reposRecovered {
		recoveredRepos = append(recoveredRepos, repo)
	}
	sort.Strings(recoveredRepos)
	sort.Strings(conflicts)
	sort.Strings(warnings)

	payload := ConfigRecoverResultJSON{
		Status:              "ok",
		WorkspaceRoot:       absRoot,
		WorkspacesRecovered: recovered,
		ReposRecovered:      recoveredRepos,
		Conflicts:           conflicts,
		Warnings:            warnings,
		DryRun:              input.DryRun,
	}
	return ConfigRecoverResult{Payload: payload, Config: info}, nil
}

func (s *Service) applyRecoverCandidates(cfg *config.GlobalConfig, candidates []recoverCandidate, rebuildRepos bool, now time.Time) recoverApplyResult {
	cfg.EnsureMaps()
	result := recoverApplyResult{
		reposRecovered: map[string]struct{}{},
	}
	for _, candidate := range candidates {
		ref, ok := cfg.Workspaces[candidate.name]
		existingPath := strings.TrimSpace(ref.Path)
		existing := ""
		if existingPath != "" {
			existing = filepath.Clean(existingPath)
		}
		target := filepath.Clean(candidate.root)
		if ok && existing != "" && existing != target {
			result.conflicts = append(result.conflicts, fmt.Sprintf("%s (existing %s, found %s)", candidate.name, existing, candidate.root))
			continue
		}
		if ok && existing == target {
			if rebuildRepos {
				if repos := recoverRepoAliases(cfg, candidate.config, s.git, cfg.Defaults, &result.warnings); len(repos) > 0 {
					for _, repo := range repos {
						result.reposRecovered[repo] = struct{}{}
					}
					result.configChanged = true
				}
			}
			continue
		}
		registerWorkspace(cfg, candidate.name, candidate.root, now, "")
		result.recovered = append(result.recovered, candidate.name)
		result.configChanged = true
		if rebuildRepos {
			if repos := recoverRepoAliases(cfg, candidate.config, s.git, cfg.Defaults, &result.warnings); len(repos) > 0 {
				for _, repo := range repos {
					result.reposRecovered[repo] = struct{}{}
				}
				result.configChanged = true
			}
		}
	}
	return result
}

func findWorksetFiles(root string) ([]string, error) {
	root = filepath.Clean(root)
	paths := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".workset":
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == "workset.yaml" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func recoverRepoAliases(cfg *config.GlobalConfig, wsConfig config.WorkspaceConfig, gitClient git.Client, defaults config.Defaults, warnings *[]string) []string {
	if cfg.Repos == nil {
		cfg.Repos = map[string]config.RegisteredRepo{}
	}
	recovered := []string{}
	for _, repo := range wsConfig.Repos {
		if repo.Name == "" {
			continue
		}
		repoPath := strings.TrimSpace(repo.LocalPath)
		if repoPath == "" && defaults.RepoStoreRoot != "" {
			candidate := filepath.Join(defaults.RepoStoreRoot, repo.Name)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				repoPath = candidate
			}
		}
		if repoPath != "" {
			if _, err := os.Stat(repoPath); err != nil && warnings != nil {
				*warnings = append(*warnings, fmt.Sprintf("repo %s path missing at %s", repo.Name, repoPath))
			}
		}
		alias := cfg.Repos[repo.Name]
		updated := false
		if alias.Path == "" && repoPath != "" {
			alias.Path = repoPath
			updated = true
		}
		if alias.Remote == "" && gitClient != nil && repoPath != "" {
			if remote := recoverRemoteName(repoPath, gitClient, defaults.Remote, warnings); remote != "" {
				alias.Remote = remote
				updated = true
			}
		}
		if alias.URL == "" && gitClient != nil && repoPath != "" {
			remote := alias.Remote
			if url := recoverRemoteURL(repoPath, gitClient, remote, warnings); url != "" {
				alias.URL = url
				updated = true
			}
		}
		if alias.DefaultBranch == "" && gitClient != nil && repoPath != "" {
			branch, ok, err := gitClient.CurrentBranch(repoPath)
			if err != nil {
				if warnings != nil {
					*warnings = append(*warnings, fmt.Sprintf("repo %s: failed reading branch (%v)", repo.Name, err))
				}
			} else if ok && branch != "" {
				alias.DefaultBranch = branch
				updated = true
			}
		}
		if updated {
			cfg.Repos[repo.Name] = alias
			recovered = append(recovered, repo.Name)
		}
	}
	return recovered
}

func recoverRemoteName(repoPath string, gitClient git.Client, preferred string, warnings *[]string) string {
	remotes, err := gitClient.RemoteNames(repoPath)
	if err != nil {
		if warnings != nil {
			*warnings = append(*warnings, fmt.Sprintf("repo %s: remote names unavailable (%v)", repoPath, err))
		}
		return ""
	}
	if preferred != "" {
		if slices.Contains(remotes, preferred) {
			return preferred
		}
	}
	if len(remotes) == 1 {
		return remotes[0]
	}
	return ""
}

func recoverRemoteURL(repoPath string, gitClient git.Client, remote string, warnings *[]string) string {
	if remote != "" {
		urls, err := gitClient.RemoteURLs(repoPath, remote)
		if err != nil {
			if warnings != nil {
				*warnings = append(*warnings, fmt.Sprintf("repo %s: remote %s URL unavailable (%v)", repoPath, remote, err))
			}
			return ""
		}
		if len(urls) > 0 {
			return urls[0]
		}
		return ""
	}
	remotes, err := gitClient.RemoteNames(repoPath)
	if err != nil || len(remotes) != 1 {
		return ""
	}
	urls, err := gitClient.RemoteURLs(repoPath, remotes[0])
	if err != nil {
		if warnings != nil {
			*warnings = append(*warnings, fmt.Sprintf("repo %s: remote %s URL unavailable (%v)", repoPath, remotes[0], err))
		}
		return ""
	}
	if len(urls) > 0 {
		return urls[0]
	}
	return ""
}
