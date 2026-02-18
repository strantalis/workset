package worksetapi

import (
	"context"
	"fmt"
	"strings"
)

func (s *Service) resolveRemoteInfo(ctx context.Context, resolution repoResolution, baseRemoteOverride string) (remoteInfo, remoteInfo, error) {
	headRemote := strings.TrimSpace(resolution.RepoDefaults.Remote)
	if headRemote == "" {
		headRemote = "origin"
	}
	headInfo, err := s.remoteInfoFor(resolution.RepoPath, headRemote)
	if err != nil {
		return remoteInfo{}, remoteInfo{}, err
	}
	baseRemote := strings.TrimSpace(baseRemoteOverride)
	if baseRemote == "" {
		// Auto-detect: use upstream if it exists, otherwise use head remote
		baseRemote = headRemote
		if exists, err := s.git.RemoteExists(resolution.RepoPath, "upstream"); err == nil && exists {
			baseRemote = "upstream"
		}
	}
	baseInfo, err := s.remoteInfoFor(resolution.RepoPath, baseRemote)
	if err != nil {
		return remoteInfo{}, remoteInfo{}, err
	}
	if headInfo.Host != baseInfo.Host {
		return remoteInfo{}, remoteInfo{}, ValidationError{Message: "head and base remotes must share the same GitHub host"}
	}
	if headInfo.Host != defaultGitHubHost {
		return remoteInfo{}, remoteInfo{}, ValidationError{Message: fmt.Sprintf("unsupported GitHub host %q: only github.com is supported in this release", headInfo.Host)}
	}
	return headInfo, baseInfo, nil
}

func (s *Service) remoteInfoFor(repoPath, remoteName string) (remoteInfo, error) {
	urls, err := s.git.RemoteURLs(repoPath, remoteName)
	if err != nil {
		return remoteInfo{}, err
	}
	if len(urls) == 0 {
		return remoteInfo{}, ValidationError{Message: fmt.Sprintf("remote %q has no URL configured", remoteName)}
	}
	info, err := parseGitHubRemoteURL(urls[0])
	if err != nil {
		return remoteInfo{}, err
	}
	info.Remote = remoteName
	info.URL = urls[0]
	if info.Host == "" {
		info.Host = defaultGitHubHost
	}
	return info, nil
}

func (s *Service) resolveCurrentBranch(resolution repoResolution) (string, error) {
	branch, ok, err := s.git.CurrentBranch(resolution.RepoPath)
	if err != nil {
		return "", err
	}
	if !ok || strings.TrimSpace(branch) == "" {
		if resolution.Branch != "" {
			return resolution.Branch, nil
		}
		return "", ValidationError{Message: "unable to resolve current branch"}
	}
	return branch, nil
}

func (s *Service) githubClient(ctx context.Context, host string) (GitHubClient, error) {
	if s.github == nil {
		return nil, AuthRequiredError{Message: "GitHub authentication required"}
	}
	if err := s.importGitHubPATFromEnv(ctx); err != nil {
		return nil, err
	}
	return s.github.Client(ctx, host)
}

func (s *Service) resolveDefaultBranch(ctx context.Context, client GitHubClient, base remoteInfo, resolution repoResolution) (string, error) {
	branch, err := client.GetRepoDefaultBranch(ctx, base.Owner, base.Repo)
	if err == nil && strings.TrimSpace(branch) != "" {
		return branch, nil
	}
	if resolution.RepoDefaults.DefaultBranch != "" {
		return resolution.RepoDefaults.DefaultBranch, nil
	}
	return "", ValidationError{Message: "base branch required"}
}

func (s *Service) resolvePullRequest(ctx context.Context, input PullRequestStatusInput) (GitHubPullRequest, remoteInfo, remoteInfo, GitHubClient, repoResolution, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}
	headInfo, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}
	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}

	number := input.Number
	if number == 0 {
		branch := strings.TrimSpace(input.Branch)
		if branch == "" {
			branch, err = s.resolveCurrentBranch(resolution)
			if err != nil {
				return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
			}
		}
		number, err = s.findPullRequestNumber(ctx, client, baseInfo, headInfo, branch)
		if err != nil {
			return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
		}
	}
	pr, err := client.GetPullRequest(ctx, baseInfo.Owner, baseInfo.Repo, number)
	if err != nil {
		return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}
	return pr, headInfo, baseInfo, client, resolution, nil
}

func (s *Service) findPullRequestNumber(ctx context.Context, client GitHubClient, base remoteInfo, head remoteInfo, branch string) (int, error) {
	headRef := fmt.Sprintf("%s:%s", head.Owner, branch)
	page := 1
	for {
		prs, next, err := client.ListPullRequests(ctx, base.Owner, base.Repo, headRef, "open", page, 50)
		if err != nil {
			return 0, err
		}
		if len(prs) > 0 {
			return prs[0].Number, nil
		}
		if next == 0 {
			break
		}
		page = next
	}
	return 0, NotFoundError{Message: "pull request not found for current branch"}
}

func (s *Service) listCheckRuns(ctx context.Context, client GitHubClient, base remoteInfo, pr GitHubPullRequest) ([]PullRequestCheckJSON, error) {
	sha := pr.HeadSHA
	if sha == "" {
		return nil, nil
	}
	checks := make([]PullRequestCheckJSON, 0)
	page := 1
	for {
		pageChecks, next, err := client.ListCheckRuns(ctx, base.Owner, base.Repo, sha, page, 100)
		if err != nil {
			return nil, err
		}
		checks = append(checks, pageChecks...)
		if next == 0 {
			break
		}
		page = next
	}
	return checks, nil
}
