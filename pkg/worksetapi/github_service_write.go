package worksetapi

import (
	"context"
	"fmt"
	"strings"
)

// CreatePullRequest opens a pull request against the resolved upstream repo.
func (s *Service) CreatePullRequest(ctx context.Context, input PullRequestCreateInput) (PullRequestCreateResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return PullRequestCreateResult{}, err
	}
	if strings.TrimSpace(input.Title) == "" {
		return PullRequestCreateResult{}, ValidationError{Message: "title required"}
	}

	headInfo, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, input.BaseRemote)
	if err != nil {
		return PullRequestCreateResult{}, err
	}
	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return PullRequestCreateResult{}, err
	}

	baseBranch := strings.TrimSpace(input.Base)
	if baseBranch == "" {
		baseBranch, err = s.resolveDefaultBranch(ctx, client, baseInfo, resolution)
		if err != nil {
			return PullRequestCreateResult{}, err
		}
	}
	headBranch := strings.TrimSpace(input.Head)
	if headBranch == "" {
		headBranch, err = s.resolveCurrentBranch(resolution)
		if err != nil {
			return PullRequestCreateResult{}, err
		}
	}
	headBranch = strings.TrimPrefix(headBranch, "refs/heads/")
	if strings.Contains(headBranch, ":") {
		return PullRequestCreateResult{}, ValidationError{Message: "head should be a branch name without an owner prefix"}
	}
	if input.AutoCommit {
		if err := s.commitPullRequestChanges(ctx, resolution, headBranch); err != nil {
			return PullRequestCreateResult{}, err
		}
	}
	if input.AutoPush {
		if err := gitPushBranch(ctx, resolution.RepoPath, headInfo.Remote, headBranch, s.commands); err != nil {
			return PullRequestCreateResult{}, err
		}
	} else {
		exists, err := remoteBranchExists(ctx, resolution.RepoPath, headInfo.Remote, headBranch, s.commands)
		if err != nil {
			return PullRequestCreateResult{}, err
		}
		if !exists {
			return PullRequestCreateResult{}, ValidationError{Message: fmt.Sprintf("head branch %q not found on remote %q; push it first: git push -u %s %s", headBranch, headInfo.Remote, headInfo.Remote, headBranch)}
		}
	}

	headRef := fmt.Sprintf("%s:%s", headInfo.Owner, headBranch)
	pr, err := client.CreatePullRequest(ctx, baseInfo.Owner, baseInfo.Repo, GitHubNewPullRequest{
		Title: input.Title,
		Head:  headRef,
		Base:  baseBranch,
		Body:  strings.TrimSpace(input.Body),
		Draft: input.Draft,
	})
	if err != nil {
		if isInvalidHeadError(err) {
			return PullRequestCreateResult{}, ValidationError{Message: fmt.Sprintf("GitHub rejected head %q; ensure the branch exists on %s/%s and that remote %q points to your fork", headRef, headInfo.Owner, headInfo.Repo, headInfo.Remote)}
		}
		return PullRequestCreateResult{}, ValidationError{Message: formatGitHubAPIError(err)}
	}

	payload := PullRequestCreatedJSON{
		Repo:       resolution.Repo.Name,
		Number:     pr.Number,
		URL:        pr.URL,
		Title:      pr.Title,
		Body:       pr.Body,
		Draft:      pr.Draft,
		State:      pr.State,
		BaseRepo:   fmt.Sprintf("%s/%s", baseInfo.Owner, baseInfo.Repo),
		BaseBranch: baseBranch,
		HeadRepo:   fmt.Sprintf("%s/%s", headInfo.Owner, headInfo.Repo),
		HeadBranch: headBranch,
	}
	s.recordPullRequest(ctx, resolution, payload)
	return PullRequestCreateResult{Payload: payload, Config: resolution.ConfigInfo}, nil
}

// ReplyToReviewComment creates a reply to an existing review comment.
func (s *Service) ReplyToReviewComment(ctx context.Context, input ReplyToReviewCommentInput) (ReviewCommentResult, error) {
	if input.CommentID <= 0 {
		return ReviewCommentResult{}, ValidationError{Message: "comment ID required"}
	}
	if strings.TrimSpace(input.Body) == "" {
		return ReviewCommentResult{}, ValidationError{Message: "body required"}
	}

	statusInput := PullRequestStatusInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
		Number:    input.Number,
		Branch:    input.Branch,
	}
	pr, _, baseInfo, client, resolution, err := s.resolvePullRequest(ctx, statusInput)
	if err != nil {
		return ReviewCommentResult{}, err
	}

	comment, err := client.CreateReplyComment(
		ctx,
		baseInfo.Owner,
		baseInfo.Repo,
		pr.Number,
		input.CommentID,
		strings.TrimSpace(input.Body),
	)
	if err != nil {
		return ReviewCommentResult{}, ValidationError{Message: formatGitHubAPIError(err)}
	}

	return ReviewCommentResult{
		Comment: comment,
		Config:  resolution.ConfigInfo,
	}, nil
}

// EditReviewComment updates an existing review comment.
func (s *Service) EditReviewComment(ctx context.Context, input EditReviewCommentInput) (ReviewCommentResult, error) {
	if input.CommentID <= 0 {
		return ReviewCommentResult{}, ValidationError{Message: "comment ID required"}
	}
	if strings.TrimSpace(input.Body) == "" {
		return ReviewCommentResult{}, ValidationError{Message: "body required"}
	}

	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return ReviewCommentResult{}, err
	}

	_, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return ReviewCommentResult{}, err
	}

	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return ReviewCommentResult{}, err
	}

	comment, err := client.EditReviewComment(
		ctx,
		baseInfo.Owner,
		baseInfo.Repo,
		input.CommentID,
		strings.TrimSpace(input.Body),
	)
	if err != nil {
		return ReviewCommentResult{}, ValidationError{Message: formatGitHubAPIError(err)}
	}

	return ReviewCommentResult{
		Comment: comment,
		Config:  resolution.ConfigInfo,
	}, nil
}

// DeleteReviewComment deletes a review comment.
func (s *Service) DeleteReviewComment(ctx context.Context, input DeleteReviewCommentInput) (DeleteReviewCommentResult, error) {
	if input.CommentID <= 0 {
		return DeleteReviewCommentResult{}, ValidationError{Message: "comment ID required"}
	}

	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return DeleteReviewCommentResult{}, err
	}

	_, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return DeleteReviewCommentResult{}, err
	}

	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return DeleteReviewCommentResult{}, err
	}

	err = client.DeleteReviewComment(ctx, baseInfo.Owner, baseInfo.Repo, input.CommentID)
	if err != nil {
		return DeleteReviewCommentResult{}, ValidationError{Message: formatGitHubAPIError(err)}
	}

	return DeleteReviewCommentResult{
		Success: true,
		Config:  resolution.ConfigInfo,
	}, nil
}

// ResolveReviewThread resolves or unresolves a review thread using GraphQL.
func (s *Service) ResolveReviewThread(ctx context.Context, input ResolveReviewThreadInput) (ResolveReviewThreadResult, error) {
	threadID := strings.TrimSpace(input.ThreadID)
	if threadID == "" {
		return ResolveReviewThreadResult{}, ValidationError{Message: "thread ID required"}
	}

	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return ResolveReviewThreadResult{}, err
	}

	_, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return ResolveReviewThreadResult{}, err
	}

	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return ResolveReviewThreadResult{}, err
	}

	if strings.HasPrefix(threadID, "PRRC_") {
		resolvedThreadID, err := client.GetReviewThreadID(ctx, threadID)
		if err != nil {
			return ResolveReviewThreadResult{}, err
		}
		threadID = resolvedThreadID
	}

	resolved, err := client.ResolveReviewThread(ctx, threadID, input.Resolve)
	if err != nil {
		return ResolveReviewThreadResult{}, err
	}

	return ResolveReviewThreadResult{
		Resolved: resolved,
		Config:   resolution.ConfigInfo,
	}, nil
}

// GeneratePullRequestText runs the default agent to propose a title/body.
func (s *Service) GeneratePullRequestText(ctx context.Context, input PullRequestGenerateInput) (PullRequestGenerateResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return PullRequestGenerateResult{}, err
	}
	headBranch, err := s.resolveCurrentBranch(resolution)
	if err != nil {
		return PullRequestGenerateResult{}, err
	}

	diffLimit := input.MaxDiffBytes
	if diffLimit <= 0 {
		diffLimit = defaultDiffLimit
	}
	patch, err := buildRepoPatch(ctx, resolution.RepoPath, diffLimit, s.commands)
	if err != nil {
		return PullRequestGenerateResult{}, err
	}
	if strings.TrimSpace(patch) == "" {
		return PullRequestGenerateResult{}, ValidationError{Message: "no diff available to summarize"}
	}

	agent := strings.TrimSpace(resolution.Defaults.Agent)
	if agent == "" {
		return PullRequestGenerateResult{}, ValidationError{Message: "defaults.agent is not configured"}
	}
	model := strings.TrimSpace(resolution.Defaults.AgentModel)

	prompt := formatPRPrompt(resolution.Repo.Name, headBranch, patch)
	result, err := s.runAgentPrompt(ctx, resolution.RepoPath, agent, prompt, model)
	if err != nil {
		return PullRequestGenerateResult{}, err
	}
	return PullRequestGenerateResult{Payload: result, Config: resolution.ConfigInfo}, nil
}

// CommitAndPush commits all changes and pushes to the remote.
func (s *Service) CommitAndPush(ctx context.Context, input CommitAndPushInput) (CommitAndPushResult, error) {
	emitStage := func(stage CommitAndPushStage) {
		if input.OnStage != nil {
			input.OnStage(stage)
		}
	}

	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	}) // Cannot use type conversion here due to Message field
	if err != nil {
		return CommitAndPushResult{}, err
	}

	branch, err := s.resolveCurrentBranch(resolution)
	if err != nil {
		return CommitAndPushResult{}, err
	}

	// Check for changes to commit
	hasUncommitted, err := gitHasUncommittedChanges(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		return CommitAndPushResult{}, err
	}
	if !hasUncommitted {
		return CommitAndPushResult{}, ValidationError{Message: "no changes to commit"}
	}
	if err := s.preflightSSHAuth(ctx, resolution); err != nil {
		return CommitAndPushResult{}, err
	}

	// Generate commit message if not provided
	message := strings.TrimSpace(input.Message)
	if message == "" {
		emitStage(CommitAndPushStageGeneratingMessage)
		agent := strings.TrimSpace(resolution.Defaults.Agent)
		if agent == "" {
			return CommitAndPushResult{}, ValidationError{Message: "defaults.agent is not configured; cannot auto-generate commit message"}
		}
		patch, err := buildRepoPatch(ctx, resolution.RepoPath, defaultDiffLimit, s.commands)
		if err != nil {
			return CommitAndPushResult{}, err
		}
		prompt := formatCommitPrompt(resolution.Repo.Name, branch, patch)
		message, err = s.runCommitMessageWithModel(ctx, resolution.RepoPath, agent, prompt, resolution.Defaults.AgentModel)
		if err != nil {
			return CommitAndPushResult{}, err
		}
	}

	// Stage all changes
	emitStage(CommitAndPushStageStaging)
	if err := gitAddAll(ctx, resolution.RepoPath, s.commands); err != nil {
		return CommitAndPushResult{}, err
	}

	// Verify staged changes exist
	hasStaged, err := gitHasStagedChanges(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		return CommitAndPushResult{}, err
	}
	if !hasStaged {
		return CommitAndPushResult{}, ValidationError{Message: "no changes staged after git add"}
	}

	// Commit
	emitStage(CommitAndPushStageCommitting)
	if err := gitCommitMessage(ctx, resolution.RepoPath, message, s.commands); err != nil {
		return CommitAndPushResult{}, err
	}

	// Get the new commit SHA
	sha, err := gitHeadSHA(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		sha = ""
	}

	// Resolve remote for push
	headInfo, _, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return CommitAndPushResult{
			Payload: CommitAndPushResultJSON{
				Committed: true,
				Pushed:    false,
				Message:   message,
				SHA:       sha,
			},
			Config: resolution.ConfigInfo,
		}, err
	}

	// Push
	emitStage(CommitAndPushStagePushing)
	if err := gitPushBranch(ctx, resolution.RepoPath, headInfo.Remote, branch, s.commands); err != nil {
		return CommitAndPushResult{
			Payload: CommitAndPushResultJSON{
				Committed: true,
				Pushed:    false,
				Message:   message,
				SHA:       sha,
			},
			Config: resolution.ConfigInfo,
		}, err
	}

	return CommitAndPushResult{
		Payload: CommitAndPushResultJSON{
			Committed: true,
			Pushed:    true,
			Message:   message,
			SHA:       sha,
		},
		Config: resolution.ConfigInfo,
	}, nil
}
