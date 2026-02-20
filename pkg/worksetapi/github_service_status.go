package worksetapi

import (
	"context"
	"fmt"
	"strings"
)

// GetPullRequestStatus returns the PR summary and checks.
func (s *Service) GetPullRequestStatus(ctx context.Context, input PullRequestStatusInput) (PullRequestStatusResult, error) {
	pr, headInfo, baseInfo, client, resolution, err := s.resolvePullRequest(ctx, input)
	if err != nil {
		return PullRequestStatusResult{}, err
	}

	mergeable := ""
	if pr.Mergeable != nil {
		switch {
		case *pr.Mergeable:
			mergeable = "mergeable"
		default:
			mergeable = "conflicts"
		}
	}
	s.reconcileTrackedPullRequest(ctx, resolution, pr, baseInfo, headInfo)

	checks, err := s.listCheckRuns(ctx, client, baseInfo, pr)
	if err != nil {
		return PullRequestStatusResult{}, err
	}

	status := PullRequestStatusJSON{
		Repo:       resolution.Repo.Name,
		Number:     pr.Number,
		URL:        pr.URL,
		Title:      pr.Title,
		State:      pr.State,
		Draft:      pr.Draft,
		Merged:     pr.Merged,
		BaseRepo:   fmt.Sprintf("%s/%s", baseInfo.Owner, baseInfo.Repo),
		BaseBranch: pr.BaseRef,
		HeadRepo:   fmt.Sprintf("%s/%s", headInfo.Owner, headInfo.Repo),
		HeadBranch: pr.HeadRef,
		Mergeable:  mergeable,
	}

	return PullRequestStatusResult{
		PullRequest: status,
		Checks:      checks,
		Config:      resolution.ConfigInfo,
	}, nil
}

func (s *Service) reconcileTrackedPullRequest(
	ctx context.Context,
	resolution repoResolution,
	pr GitHubPullRequest,
	baseInfo remoteInfo,
	headInfo remoteInfo,
) {
	if strings.EqualFold(pr.State, "open") {
		s.recordPullRequest(ctx, resolution, PullRequestCreatedJSON{
			Repo:       resolution.Repo.Name,
			Number:     pr.Number,
			URL:        pr.URL,
			Title:      pr.Title,
			Body:       pr.Body,
			Draft:      pr.Draft,
			State:      pr.State,
			Merged:     false,
			BaseRepo:   fmt.Sprintf("%s/%s", baseInfo.Owner, baseInfo.Repo),
			BaseBranch: pr.BaseRef,
			HeadRepo:   fmt.Sprintf("%s/%s", headInfo.Owner, headInfo.Repo),
			HeadBranch: pr.HeadRef,
		})
		return
	}
	if pr.Merged {
		s.recordPullRequest(ctx, resolution, PullRequestCreatedJSON{
			Repo:       resolution.Repo.Name,
			Number:     pr.Number,
			URL:        pr.URL,
			Title:      pr.Title,
			Body:       pr.Body,
			Draft:      pr.Draft,
			State:      pr.State,
			Merged:     true,
			BaseRepo:   fmt.Sprintf("%s/%s", baseInfo.Owner, baseInfo.Repo),
			BaseBranch: pr.BaseRef,
			HeadRepo:   fmt.Sprintf("%s/%s", headInfo.Owner, headInfo.Repo),
			HeadBranch: pr.HeadRef,
		})
		return
	}
	s.clearTrackedPullRequestIfMatchingNumber(ctx, resolution, pr.Number)
}

// GetCheckAnnotations returns annotations for a specific check run.
func (s *Service) GetCheckAnnotations(ctx context.Context, input GetCheckAnnotationsInput) (CheckAnnotationsResult, error) {
	client, err := s.githubClient(ctx, "")
	if err != nil {
		return CheckAnnotationsResult{}, err
	}
	annotations, err := client.GetCheckRunAnnotations(ctx, input.Owner, input.Repo, input.CheckRunID)
	if err != nil {
		return CheckAnnotationsResult{}, err
	}
	return CheckAnnotationsResult{Annotations: annotations}, nil
}

// GetTrackedPullRequest returns the last recorded PR for a repo.
func (s *Service) GetTrackedPullRequest(ctx context.Context, input PullRequestTrackedInput) (PullRequestTrackedResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput(input))
	if err != nil {
		return PullRequestTrackedResult{}, err
	}
	state, err := s.workspaces.LoadState(ctx, resolution.WorkspaceRoot)
	if err != nil {
		return PullRequestTrackedResult{}, err
	}
	pr, ok := state.PullRequests[resolution.Repo.Name]
	if !ok {
		return PullRequestTrackedResult{
			Payload: PullRequestTrackedJSON{Found: false},
			Config:  resolution.ConfigInfo,
		}, nil
	}
	return PullRequestTrackedResult{
		Payload: PullRequestTrackedJSON{
			Found: true,
			PullRequest: PullRequestCreatedJSON{
				Repo:       pr.Repo,
				Number:     pr.Number,
				URL:        pr.URL,
				Title:      pr.Title,
				Body:       pr.Body,
				Draft:      pr.Draft,
				State:      pr.State,
				Merged:     pr.Merged,
				BaseRepo:   pr.BaseRepo,
				BaseBranch: pr.BaseBranch,
				HeadRepo:   pr.HeadRepo,
				HeadBranch: pr.HeadBranch,
			},
		},
		Config: resolution.ConfigInfo,
	}, nil
}

// ListPullRequestReviewComments returns review comments for a PR.
func (s *Service) ListPullRequestReviewComments(ctx context.Context, input PullRequestReviewsInput) (PullRequestReviewCommentsResult, error) {
	statusInput := PullRequestStatusInput(input)
	pr, _, baseInfo, client, resolution, err := s.resolvePullRequest(ctx, statusInput)
	if err != nil {
		return PullRequestReviewCommentsResult{}, err
	}

	threadMap := map[string]threadInfo{}
	if client != nil {
		if mapResult, err := client.ReviewThreadMap(ctx, baseInfo.Owner, baseInfo.Repo, pr.Number); err == nil {
			threadMap = mapResult
		}
	}

	comments := make([]PullRequestReviewCommentJSON, 0)
	page := 1
	for {
		pageComments, nextPage, err := client.ListReviewComments(ctx, baseInfo.Owner, baseInfo.Repo, pr.Number, page, 100)
		if err != nil {
			return PullRequestReviewCommentsResult{}, err
		}
		for _, comment := range pageComments {
			mapped := comment
			if mapped.NodeID != "" {
				if info, ok := threadMap[mapped.NodeID]; ok {
					mapped.ThreadID = info.ThreadID
					mapped.Resolved = info.IsResolved
				}
			}
			comments = append(comments, mapped)
		}
		if nextPage == 0 {
			break
		}
		page = nextPage
	}

	return PullRequestReviewCommentsResult{
		Comments: comments,
		Config:   resolution.ConfigInfo,
	}, nil
}

// GetCurrentGitHubUser returns the authenticated GitHub user.
func (s *Service) GetCurrentGitHubUser(ctx context.Context, input GitHubUserInput) (GitHubUserResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput(input))
	if err != nil {
		return GitHubUserResult{}, err
	}

	_, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return GitHubUserResult{}, err
	}

	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return GitHubUserResult{}, err
	}

	user, _, err := client.GetCurrentUser(ctx)
	if err != nil {
		return GitHubUserResult{}, ValidationError{Message: formatGitHubAPIError(err)}
	}

	return GitHubUserResult{
		User:   user,
		Config: resolution.ConfigInfo,
	}, nil
}

// GetRepoLocalStatus returns the local uncommitted/ahead/behind status for a repo.
func (s *Service) GetRepoLocalStatus(ctx context.Context, input RepoLocalStatusInput) (RepoLocalStatusResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput(input))
	if err != nil {
		return RepoLocalStatusResult{}, err
	}

	// Get current branch
	branch, err := s.resolveCurrentBranch(resolution)
	if err != nil {
		return RepoLocalStatusResult{}, err
	}

	// Check for uncommitted changes via git status --porcelain
	hasUncommitted, err := gitHasUncommittedChanges(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		return RepoLocalStatusResult{}, err
	}

	// Get ahead/behind counts
	ahead, behind, err := gitAheadBehind(ctx, resolution.RepoPath, branch, s.commands)
	if err != nil {
		// Non-fatal: upstream tracking may not be configured
		ahead, behind = 0, 0
	}

	return RepoLocalStatusResult{
		Payload: RepoLocalStatusJSON{
			HasUncommitted: hasUncommitted,
			Ahead:          ahead,
			Behind:         behind,
			CurrentBranch:  branch,
		},
		Config: resolution.ConfigInfo,
	}, nil
}

// ListRemotes returns the list of git remotes for a repo with owner/repo info.
func (s *Service) ListRemotes(ctx context.Context, input ListRemotesInput) (ListRemotesResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput(input))
	if err != nil {
		return ListRemotesResult{}, err
	}

	remoteNames, err := s.git.RemoteNames(resolution.RepoPath)
	if err != nil {
		return ListRemotesResult{}, err
	}

	remotes := make([]RemoteInfoJSON, 0, len(remoteNames))
	for _, name := range remoteNames {
		info, err := s.remoteInfoFor(resolution.RepoPath, name)
		if err != nil {
			// Skip remotes that can't be parsed (e.g., non-GitHub remotes)
			continue
		}
		remotes = append(remotes, RemoteInfoJSON{
			Name:  name,
			Owner: info.Owner,
			Repo:  info.Repo,
		})
	}

	return ListRemotesResult{Remotes: remotes}, nil
}
