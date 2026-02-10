package worksetapi

import (
	"context"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/hooks"
)

type repoHooksPreviewSource struct {
	resolvedSource string
	localPath      string
	remoteURL      string
}

// PreviewRepoHooks discovers repository hook definitions from local paths or GitHub remotes.
func (s *Service) PreviewRepoHooks(ctx context.Context, input RepoHooksPreviewInput) (RepoHooksPreviewResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return RepoHooksPreviewResult{}, err
	}

	source := strings.TrimSpace(input.Source)
	if source == "" {
		return RepoHooksPreviewResult{}, ValidationError{Message: "repo source required"}
	}

	resolved, err := resolveRepoHooksPreviewSource(cfg, source)
	if err != nil {
		return RepoHooksPreviewResult{}, err
	}

	result := RepoHooksPreviewResult{
		Payload: RepoHooksPreviewJSON{
			Source:         source,
			ResolvedSource: resolved.resolvedSource,
			Ref:            strings.TrimSpace(input.Ref),
			Exists:         false,
		},
		Config: info,
	}

	if resolved.localPath != "" {
		file, exists, err := hooks.LoadRepoHooks(resolved.localPath)
		if err != nil {
			return RepoHooksPreviewResult{}, err
		}
		if !exists {
			return result, nil
		}
		result.Payload.Exists = true
		result.Payload.Hooks = mapRepoHookPreviewHooks(file)
		return result, nil
	}

	remote, err := parseGitHubRemoteURL(resolved.remoteURL)
	if err != nil {
		return RepoHooksPreviewResult{}, ValidationError{Message: "repo source must be a valid GitHub-style URL"}
	}
	host := strings.TrimSpace(remote.Host)
	if host == "" {
		host = defaultGitHubHost
	}

	result.Payload.Host = host
	result.Payload.Owner = remote.Owner
	result.Payload.Repo = remote.Repo

	client, err := s.githubClient(ctx, host)
	if err != nil {
		return RepoHooksPreviewResult{}, err
	}
	data, exists, err := client.GetFileContent(ctx, remote.Owner, remote.Repo, hooks.RepoHooksPath, strings.TrimSpace(input.Ref))
	if err != nil {
		return RepoHooksPreviewResult{}, err
	}
	if !exists {
		return result, nil
	}

	file, err := hooks.ParseRepoHooks(data)
	if err != nil {
		return RepoHooksPreviewResult{}, err
	}
	result.Payload.Exists = true
	result.Payload.Hooks = mapRepoHookPreviewHooks(file)
	return result, nil
}

func resolveRepoHooksPreviewSource(cfg config.GlobalConfig, source string) (repoHooksPreviewSource, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return repoHooksPreviewSource{}, ValidationError{Message: "repo source required"}
	}

	if alias, ok := cfg.Repos[source]; ok {
		if path := strings.TrimSpace(alias.Path); path != "" {
			resolvedPath, err := resolveLocalPathInput(path)
			if err != nil {
				return repoHooksPreviewSource{}, err
			}
			return repoHooksPreviewSource{
				resolvedSource: resolvedPath,
				localPath:      resolvedPath,
			}, nil
		}
		if url := strings.TrimSpace(alias.URL); url != "" {
			return repoHooksPreviewSource{
				resolvedSource: url,
				remoteURL:      url,
			}, nil
		}
		return repoHooksPreviewSource{}, ValidationError{Message: "registered repo source is empty"}
	}

	if looksLikeURL(source) {
		return repoHooksPreviewSource{
			resolvedSource: source,
			remoteURL:      source,
		}, nil
	}

	if !looksLikeLocalPath(source) {
		return repoHooksPreviewSource{}, ValidationError{Message: "repo source must be a registered alias, local path, or git URL"}
	}

	resolvedPath, err := resolveLocalPathInput(source)
	if err != nil {
		return repoHooksPreviewSource{}, err
	}
	return repoHooksPreviewSource{
		resolvedSource: resolvedPath,
		localPath:      resolvedPath,
	}, nil
}

func mapRepoHookPreviewHooks(file hooks.File) []RepoHookPreviewJSON {
	if len(file.Hooks) == 0 {
		return nil
	}
	result := make([]RepoHookPreviewJSON, 0, len(file.Hooks))
	for _, hook := range file.Hooks {
		on := make([]string, 0, len(hook.On))
		for _, event := range hook.On {
			on = append(on, string(event))
		}
		run := append([]string(nil), hook.Run...)
		result = append(result, RepoHookPreviewJSON{
			ID:      hook.ID,
			On:      on,
			Run:     run,
			Cwd:     hook.Cwd,
			OnError: hook.OnError,
		})
	}
	return result
}
