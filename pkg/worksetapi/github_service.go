package worksetapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v75/github"
	"github.com/strantalis/workset/internal/workspace"
)

const (
	defaultGitHubHost = "github.com"
	defaultDiffLimit  = 120000
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
	token, err := s.resolveGitHubToken(ctx, resolution.WorkspaceRoot)
	if err != nil {
		return PullRequestCreateResult{}, err
	}

	headInfo, baseInfo, err := s.resolveRemoteInfo(ctx, resolution)
	if err != nil {
		return PullRequestCreateResult{}, err
	}
	client, err := s.newGitHubClient(token, baseInfo.Host)
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
	newPR := &github.NewPullRequest{
		Title: github.Ptr(input.Title),
		Head:  github.Ptr(headRef),
		Base:  github.Ptr(baseBranch),
		Body:  github.Ptr(strings.TrimSpace(input.Body)),
		Draft: github.Ptr(input.Draft),
	}
	pr, _, err := client.PullRequests.Create(ctx, baseInfo.Owner, baseInfo.Repo, newPR)
	if err != nil {
		if isInvalidHeadError(err) {
			return PullRequestCreateResult{}, ValidationError{Message: fmt.Sprintf("GitHub rejected head %q; ensure the branch exists on %s/%s and that remote %q points to your fork", headRef, headInfo.Owner, headInfo.Repo, headInfo.Remote)}
		}
		return PullRequestCreateResult{}, ValidationError{Message: formatGitHubAPIError(err)}
	}

	payload := PullRequestCreatedJSON{
		Repo:       resolution.Repo.Name,
		Number:     pr.GetNumber(),
		URL:        pr.GetHTMLURL(),
		Title:      pr.GetTitle(),
		Body:       pr.GetBody(),
		Draft:      pr.GetDraft(),
		State:      pr.GetState(),
		BaseRepo:   fmt.Sprintf("%s/%s", baseInfo.Owner, baseInfo.Repo),
		BaseBranch: baseBranch,
		HeadRepo:   fmt.Sprintf("%s/%s", headInfo.Owner, headInfo.Repo),
		HeadBranch: headBranch,
	}
	s.recordPullRequest(ctx, resolution, payload)
	return PullRequestCreateResult{Payload: payload, Config: resolution.ConfigInfo}, nil
}

// GetPullRequestStatus returns the PR summary and checks.
func (s *Service) GetPullRequestStatus(ctx context.Context, input PullRequestStatusInput) (PullRequestStatusResult, error) {
	pr, headInfo, baseInfo, client, resolution, err := s.resolvePullRequest(ctx, input)
	if err != nil {
		return PullRequestStatusResult{}, err
	}

	mergeable := ""
	if pr.Mergeable != nil {
		switch {
		case pr.GetMergeable():
			mergeable = "mergeable"
		default:
			mergeable = "conflicts"
		}
	}

	checks, err := s.listCheckRuns(ctx, client, baseInfo, pr)
	if err != nil {
		return PullRequestStatusResult{}, err
	}

	status := PullRequestStatusJSON{
		Repo:       resolution.Repo.Name,
		Number:     pr.GetNumber(),
		URL:        pr.GetHTMLURL(),
		Title:      pr.GetTitle(),
		State:      pr.GetState(),
		Draft:      pr.GetDraft(),
		BaseRepo:   fmt.Sprintf("%s/%s", baseInfo.Owner, baseInfo.Repo),
		BaseBranch: pr.Base.GetRef(),
		HeadRepo:   fmt.Sprintf("%s/%s", headInfo.Owner, headInfo.Repo),
		HeadBranch: pr.Head.GetRef(),
		Mergeable:  mergeable,
	}

	return PullRequestStatusResult{
		PullRequest: status,
		Checks:      checks,
		Config:      resolution.ConfigInfo,
	}, nil
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

	comments := make([]PullRequestReviewCommentJSON, 0)
	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		page, resp, err := client.PullRequests.ListComments(ctx, baseInfo.Owner, baseInfo.Repo, pr.GetNumber(), opts)
		if err != nil {
			return PullRequestReviewCommentsResult{}, err
		}
		for _, comment := range page {
			comments = append(comments, mapReviewComment(comment))
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return PullRequestReviewCommentsResult{
		Comments: comments,
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

	prompt := formatPRPrompt(resolution.Repo.Name, headBranch, patch)
	result, err := s.runAgentPrompt(ctx, resolution.RepoPath, agent, prompt)
	if err != nil {
		return PullRequestGenerateResult{}, err
	}
	return PullRequestGenerateResult{Payload: result, Config: resolution.ConfigInfo}, nil
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

// CommitAndPush commits all changes and pushes to the remote.
func (s *Service) CommitAndPush(ctx context.Context, input CommitAndPushInput) (CommitAndPushResult, error) {
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

	// Generate commit message if not provided
	message := strings.TrimSpace(input.Message)
	if message == "" {
		agent := strings.TrimSpace(resolution.Defaults.Agent)
		if agent == "" {
			return CommitAndPushResult{}, ValidationError{Message: "defaults.agent is not configured; cannot auto-generate commit message"}
		}
		patch, err := buildRepoPatch(ctx, resolution.RepoPath, defaultDiffLimit, s.commands)
		if err != nil {
			return CommitAndPushResult{}, err
		}
		prompt := formatCommitPrompt(resolution.Repo.Name, branch, patch)
		schema, err := ensureCommitSchema()
		if err != nil {
			return CommitAndPushResult{}, err
		}
		output, err := s.runAgentPromptRaw(ctx, resolution.RepoPath, agent, prompt, schema)
		if err != nil {
			return CommitAndPushResult{}, err
		}
		message, err = parseCommitJSON(output)
		if err != nil {
			return CommitAndPushResult{}, err
		}
	}

	// Stage all changes
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
	if err := gitCommitMessage(ctx, resolution.RepoPath, message, s.commands); err != nil {
		return CommitAndPushResult{}, err
	}

	// Get the new commit SHA
	sha, err := gitHeadSHA(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		sha = ""
	}

	// Resolve remote for push
	headInfo, _, err := s.resolveRemoteInfo(ctx, resolution)
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

func (s *Service) resolveGitHubToken(ctx context.Context, root string) (string, error) {
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token, nil
	}
	if token := strings.TrimSpace(os.Getenv("GH_TOKEN")); token != "" {
		return token, nil
	}
	result, err := s.commands(ctx, root, []string{"gh", "auth", "token"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			message = "GitHub CLI not found; install `gh` and run `gh auth login`"
		} else if message == "" {
			message = "run `gh auth login` to authenticate GitHub CLI"
		}
		return "", ValidationError{Message: message}
	}
	token := strings.TrimSpace(result.Stdout)
	if token == "" {
		return "", ValidationError{Message: "gh auth token returned empty output"}
	}
	return token, nil
}

func (s *Service) resolveRemoteInfo(ctx context.Context, resolution repoResolution) (remoteInfo, remoteInfo, error) {
	headRemote := strings.TrimSpace(resolution.RepoDefaults.Remote)
	if headRemote == "" {
		headRemote = "origin"
	}
	headInfo, err := s.remoteInfoFor(resolution.RepoPath, headRemote)
	if err != nil {
		return remoteInfo{}, remoteInfo{}, err
	}
	baseRemote := headRemote
	if exists, err := s.git.RemoteExists(resolution.RepoPath, "upstream"); err == nil && exists {
		baseRemote = "upstream"
	}
	baseInfo, err := s.remoteInfoFor(resolution.RepoPath, baseRemote)
	if err != nil {
		return remoteInfo{}, remoteInfo{}, err
	}
	if headInfo.Host != baseInfo.Host {
		return remoteInfo{}, remoteInfo{}, ValidationError{Message: "head and base remotes must share the same GitHub host"}
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

func (s *Service) resolveDefaultBranch(ctx context.Context, client *github.Client, base remoteInfo, resolution repoResolution) (string, error) {
	repo, _, err := client.Repositories.Get(ctx, base.Owner, base.Repo)
	if err == nil && repo != nil && repo.GetDefaultBranch() != "" {
		return repo.GetDefaultBranch(), nil
	}
	if resolution.RepoDefaults.DefaultBranch != "" {
		return resolution.RepoDefaults.DefaultBranch, nil
	}
	return "", ValidationError{Message: "base branch required"}
}

func (s *Service) newGitHubClient(token, host string) (*github.Client, error) {
	if host == "" || host == defaultGitHubHost {
		return github.NewClient(nil).WithAuthToken(token), nil
	}
	baseURL := fmt.Sprintf("https://%s/api/v3/", host)
	uploadURL := fmt.Sprintf("https://%s/api/uploads/", host)
	client, err := github.NewClient(nil).WithEnterpriseURLs(baseURL, uploadURL)
	if err != nil {
		return nil, err
	}
	return client.WithAuthToken(token), nil
}

func (s *Service) resolvePullRequest(ctx context.Context, input PullRequestStatusInput) (*github.PullRequest, remoteInfo, remoteInfo, *github.Client, repoResolution, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return nil, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}
	token, err := s.resolveGitHubToken(ctx, resolution.WorkspaceRoot)
	if err != nil {
		return nil, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}
	headInfo, baseInfo, err := s.resolveRemoteInfo(ctx, resolution)
	if err != nil {
		return nil, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}
	client, err := s.newGitHubClient(token, baseInfo.Host)
	if err != nil {
		return nil, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}

	number := input.Number
	if number == 0 {
		branch := strings.TrimSpace(input.Branch)
		if branch == "" {
			branch, err = s.resolveCurrentBranch(resolution)
			if err != nil {
				return nil, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
			}
		}
		number, err = s.findPullRequestNumber(ctx, client, baseInfo, headInfo, branch)
		if err != nil {
			return nil, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
		}
	}
	pr, _, err := client.PullRequests.Get(ctx, baseInfo.Owner, baseInfo.Repo, number)
	if err != nil {
		return nil, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}
	return pr, headInfo, baseInfo, client, resolution, nil
}

func (s *Service) findPullRequestNumber(ctx context.Context, client *github.Client, base remoteInfo, head remoteInfo, branch string) (int, error) {
	headRef := fmt.Sprintf("%s:%s", head.Owner, branch)
	opts := &github.PullRequestListOptions{
		State:       "open",
		Head:        headRef,
		ListOptions: github.ListOptions{PerPage: 50},
	}
	for {
		prs, resp, err := client.PullRequests.List(ctx, base.Owner, base.Repo, opts)
		if err != nil {
			return 0, err
		}
		if len(prs) > 0 {
			return prs[0].GetNumber(), nil
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return 0, NotFoundError{Message: "pull request not found for current branch"}
}

func (s *Service) listCheckRuns(ctx context.Context, client *github.Client, base remoteInfo, pr *github.PullRequest) ([]PullRequestCheckJSON, error) {
	sha := pr.GetHead().GetSHA()
	if sha == "" {
		return nil, nil
	}
	opts := &github.ListCheckRunsOptions{ListOptions: github.ListOptions{PerPage: 100}}
	checks := make([]PullRequestCheckJSON, 0)
	for {
		result, resp, err := client.Checks.ListCheckRunsForRef(ctx, base.Owner, base.Repo, sha, opts)
		if err != nil {
			return nil, err
		}
		for _, run := range result.CheckRuns {
			checks = append(checks, PullRequestCheckJSON{
				Name:        run.GetName(),
				Status:      run.GetStatus(),
				Conclusion:  run.GetConclusion(),
				DetailsURL:  run.GetDetailsURL(),
				StartedAt:   formatTimestamp(run.StartedAt),
				CompletedAt: formatTimestamp(run.CompletedAt),
			})
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return checks, nil
}

func mapReviewComment(comment *github.PullRequestComment) PullRequestReviewCommentJSON {
	outdated := comment.GetPosition() == 0 && comment.GetOriginalPosition() != 0
	return PullRequestReviewCommentJSON{
		ID:             comment.GetID(),
		ReviewID:       comment.GetPullRequestReviewID(),
		Author:         comment.User.GetLogin(),
		Body:           comment.GetBody(),
		Path:           comment.GetPath(),
		Line:           comment.GetLine(),
		Side:           comment.GetSide(),
		CommitID:       comment.GetCommitID(),
		OriginalCommit: comment.GetOriginalCommitID(),
		OriginalLine:   comment.GetOriginalLine(),
		OriginalStart:  comment.GetOriginalStartLine(),
		Outdated:       outdated,
		URL:            comment.GetHTMLURL(),
		CreatedAt:      formatTimestamp(comment.CreatedAt),
		UpdatedAt:      formatTimestamp(comment.UpdatedAt),
		InReplyTo:      comment.GetInReplyTo(),
		ReplyToComment: comment.GetInReplyTo() != 0,
	}
}

func formatTimestamp(ts *github.Timestamp) string {
	if ts == nil || ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339)
}

func formatPRPrompt(repoName, branch, patch string) string {
	builder := strings.Builder{}
	builder.WriteString("Generate a pull request title and body based on this diff.\n")
	builder.WriteString("Return JSON only: {\"title\":\"...\",\"body\":\"...\"}.\n")
	builder.WriteString(fmt.Sprintf("Repo: %s\n", repoName))
	builder.WriteString(fmt.Sprintf("Branch: %s\n\n", branch))
	builder.WriteString("Diff:\n")
	builder.WriteString(patch)
	builder.WriteString("\n")
	return builder.String()
}

func formatCommitPrompt(repoName, branch, patch string) string {
	builder := strings.Builder{}
	builder.WriteString("Generate a conventional commit message for this diff.\n")
	builder.WriteString("Use format: type(scope): subject. Keep it concise.\n")
	builder.WriteString("Return JSON only: {\"message\":\"...\"}.\n")
	builder.WriteString(fmt.Sprintf("Repo: %s\n", repoName))
	builder.WriteString(fmt.Sprintf("Branch: %s\n\n", branch))
	builder.WriteString("Diff:\n")
	builder.WriteString(patch)
	builder.WriteString("\n")
	return builder.String()
}

func (s *Service) runAgentPrompt(ctx context.Context, repoPath, agent, prompt string) (PullRequestGeneratedJSON, error) {
	schema, err := ensurePRSchema()
	if err != nil {
		return PullRequestGeneratedJSON{}, err
	}
	output, err := s.runAgentPromptRaw(ctx, repoPath, agent, prompt, schema)
	if err != nil {
		return PullRequestGeneratedJSON{}, err
	}
	return parseAgentJSON(output)
}

func (s *Service) runAgentPromptRaw(ctx context.Context, repoPath, agent, prompt, schema string) (string, error) {
	command := strings.Fields(agent)
	if len(command) == 0 {
		return "", ValidationError{Message: "agent command required"}
	}
	command, env, stdin, err := prepareAgentCommand(command, prompt, schema)
	if err != nil {
		return "", err
	}
	result, err := s.commands(ctx, repoPath, command, env, stdin)
	if err != nil || result.ExitCode != 0 {
		if shouldRetryWithPTY(err, result) {
			ptyResult, ptyErr := runCommandWithPTY(ctx, repoPath, command, env, stdin)
			if ptyErr == nil && ptyResult.ExitCode == 0 {
				return ptyResult.Stdout, nil
			}
			if ptyErr != nil && err == nil {
				err = ptyErr
			}
			if ptyResult.ExitCode != 0 && ptyResult.Stdout != "" {
				result = ptyResult
			}
		}
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			message = fmt.Sprintf("agent command not found: %s", command[0])
		} else if message == "" {
			message = "agent command failed"
		}
		return "", ValidationError{Message: message}
	}
	return result.Stdout, nil
}

func parseAgentJSON(output string) (PullRequestGeneratedJSON, error) {
	output = strings.TrimSpace(stripANSI(output))
	if output == "" {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "agent returned empty output"}
	}
	payload, err := decodeJSON(output)
	if err != nil {
		return PullRequestGeneratedJSON{}, err
	}
	if strings.TrimSpace(payload.Title) == "" {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "agent output missing title"}
	}
	return payload, nil
}

func parseCommitJSON(output string) (string, error) {
	output = strings.TrimSpace(stripANSI(output))
	if output == "" {
		return "", ValidationError{Message: "agent returned empty output"}
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		re := regexp.MustCompile(`\{[\s\S]*\}`)
		match := re.FindString(output)
		if match == "" {
			return "", ValidationError{Message: "unable to parse agent JSON output"}
		}
		if err := json.Unmarshal([]byte(match), &payload); err != nil {
			return "", ValidationError{Message: "invalid agent JSON output"}
		}
	}
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		return "", ValidationError{Message: "agent output missing commit message"}
	}
	return message, nil
}

func decodeJSON(output string) (PullRequestGeneratedJSON, error) {
	var payload PullRequestGeneratedJSON
	if err := json.Unmarshal([]byte(output), &payload); err == nil {
		return payload, nil
	}
	re := regexp.MustCompile(`\{[\s\S]*\}`)
	match := re.FindString(output)
	if match == "" {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "unable to parse agent JSON output"}
	}
	if err := json.Unmarshal([]byte(match), &payload); err != nil {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "invalid agent JSON output"}
	}
	return payload, nil
}

func stripANSI(value string) string {
	if value == "" {
		return value
	}
	re := regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	return re.ReplaceAllString(value, "")
}

func isStdinNotTerminal(err error, result CommandResult) bool {
	message := strings.ToLower(strings.TrimSpace(result.Stderr))
	if message == "" {
		message = strings.ToLower(strings.TrimSpace(result.Stdout))
	}
	if err != nil && message == "" {
		message = strings.ToLower(err.Error())
	}
	return strings.Contains(message, "stdin is not a terminal") || strings.Contains(message, "not a tty")
}

func shouldRetryWithPTY(err error, result CommandResult) bool {
	if isStdinNotTerminal(err, result) {
		return true
	}
	if err == nil || result.ExitCode == 0 {
		return false
	}
	output := strings.TrimSpace(result.Stdout + result.Stderr)
	return output == ""
}

func (s *Service) commitPullRequestChanges(ctx context.Context, resolution repoResolution, branch string) error {
	agent := strings.TrimSpace(resolution.Defaults.Agent)
	if agent == "" {
		return ValidationError{Message: "defaults.agent is not configured"}
	}
	diffLimit := defaultDiffLimit
	patch, err := buildRepoPatch(ctx, resolution.RepoPath, diffLimit, s.commands)
	if err != nil {
		return err
	}
	if strings.TrimSpace(patch) == "" {
		return nil
	}
	prompt := formatCommitPrompt(resolution.Repo.Name, branch, patch)
	schema, err := ensureCommitSchema()
	if err != nil {
		return err
	}
	output, err := s.runAgentPromptRaw(ctx, resolution.RepoPath, agent, prompt, schema)
	if err != nil {
		return err
	}
	message, err := parseCommitJSON(output)
	if err != nil {
		return err
	}
	if err := gitAddAll(ctx, resolution.RepoPath, s.commands); err != nil {
		return err
	}
	hasStaged, err := gitHasStagedChanges(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		return err
	}
	if !hasStaged {
		return ValidationError{Message: "no changes to commit"}
	}
	if err := gitCommitMessage(ctx, resolution.RepoPath, message, s.commands); err != nil {
		return err
	}
	return nil
}

func isInvalidHeadError(err error) bool {
	var ghErr *github.ErrorResponse
	if !errors.As(err, &ghErr) {
		return false
	}
	for _, entry := range ghErr.Errors {
		if strings.EqualFold(entry.Resource, "PullRequest") && strings.EqualFold(entry.Field, "head") && strings.EqualFold(entry.Code, "invalid") {
			return true
		}
	}
	return false
}

func formatGitHubAPIError(err error) string {
	var ghErr *github.ErrorResponse
	if !errors.As(err, &ghErr) {
		if err != nil {
			return err.Error()
		}
		return "GitHub API error"
	}
	details := make([]string, 0, len(ghErr.Errors))
	for _, entry := range ghErr.Errors {
		detail := strings.TrimSpace(entry.Message)
		if detail == "" {
			parts := []string{}
			if entry.Resource != "" {
				parts = append(parts, entry.Resource)
			}
			if entry.Field != "" {
				parts = append(parts, entry.Field)
			}
			if entry.Code != "" {
				parts = append(parts, entry.Code)
			}
			detail = strings.TrimSpace(strings.Join(parts, " "))
		}
		if detail != "" {
			details = append(details, detail)
		}
	}
	message := strings.TrimSpace(ghErr.Message)
	if len(details) == 0 {
		if message != "" {
			return message
		}
		if err != nil {
			return err.Error()
		}
		return "GitHub API error"
	}
	if message == "" {
		return strings.Join(details, "; ")
	}
	return fmt.Sprintf("%s (%s)", message, strings.Join(details, "; "))
}

var (
	prSchemaOnce     sync.Once
	prSchemaPath     string
	prSchemaErr      error
	commitSchemaOnce sync.Once
	commitSchemaPath string
	commitSchemaErr  error
)

func prepareAgentCommand(command []string, prompt string, schema string) ([]string, []string, string, error) {
	env := append(os.Environ(),
		"WORKSET_PR_PROMPT="+prompt,
		"WORKSET_PR_JSON=1",
	)
	if len(command) == 0 {
		return nil, nil, "", errors.New("agent command required")
	}
	if filepath.Base(command[0]) != "codex" {
		return command, env, prompt, nil
	}
	if schema == "" {
		return nil, nil, "", errors.New("agent schema required")
	}

	args := command[1:]
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		args = append([]string{"exec"}, args...)
	} else if args[0] == "exec" || args[0] == "e" {
		// ok
	} else {
		// Any other subcommand should pass through unchanged.
		return command, env, prompt, nil
	}

	promptProvided := hasPromptArg(args)
	if !hasFlag(args, "--color") {
		args = append(args, "--color", "never")
	}
	if !hasFlag(args, "--output-schema") {
		args = append(args, "--output-schema", schema)
	}
	// In non-interactive mode, read the prompt from stdin.
	if !promptProvided {
		args = append(args, "-")
	}
	return append([]string{"codex"}, args...), env, prompt, nil
}

func hasFlag(args []string, name string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

func hasPromptArg(args []string) bool {
	sawExec := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if !sawExec && (arg == "exec" || arg == "e") {
			sawExec = true
			continue
		}
		if arg == "-" {
			return true
		}
		// first non-flag arg is prompt; treat it as present
		return true
	}
	return false
}

func ensurePRSchema() (string, error) {
	prSchemaOnce.Do(func() {
		path := filepath.Join(os.TempDir(), "workset-pr-schema.json")
		payload := `{"type":"object","properties":{"title":{"type":"string"},"body":{"type":"string"}},"required":["title","body"],"additionalProperties":false}`
		prSchemaErr = os.WriteFile(path, []byte(payload), 0o644)
		if prSchemaErr == nil {
			prSchemaPath = path
		}
	})
	return prSchemaPath, prSchemaErr
}

func ensureCommitSchema() (string, error) {
	commitSchemaOnce.Do(func() {
		path := filepath.Join(os.TempDir(), "workset-commit-schema.json")
		payload := `{"type":"object","properties":{"message":{"type":"string"}},"required":["message"],"additionalProperties":false}`
		commitSchemaErr = os.WriteFile(path, []byte(payload), 0o644)
		if commitSchemaErr == nil {
			commitSchemaPath = path
		}
	})
	return commitSchemaPath, commitSchemaErr
}

func gitAddAll(ctx context.Context, repoPath string, runner CommandRunner) error {
	result, err := runner(ctx, repoPath, []string{"git", "add", "-A"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "git add failed"
		}
		return ValidationError{Message: message}
	}
	return nil
}

func gitHasStagedChanges(ctx context.Context, repoPath string, runner CommandRunner) (bool, error) {
	result, err := runner(ctx, repoPath, []string{"git", "diff", "--cached", "--name-only"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "unable to check staged changes"
		}
		return false, ValidationError{Message: message}
	}
	return strings.TrimSpace(result.Stdout) != "", nil
}

func gitCommitMessage(ctx context.Context, repoPath, message string, runner CommandRunner) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return ValidationError{Message: "commit message required"}
	}
	result, err := runner(ctx, repoPath, []string{"git", "commit", "-m", message}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		msg := strings.TrimSpace(result.Stderr)
		if msg == "" && err != nil {
			msg = err.Error()
		}
		if msg == "" {
			msg = "git commit failed"
		}
		return ValidationError{Message: msg}
	}
	return nil
}

func gitPushBranch(ctx context.Context, repoPath, remote, branch string, runner CommandRunner) error {
	if strings.TrimSpace(remote) == "" {
		return ValidationError{Message: "remote name required to push head branch"}
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return ValidationError{Message: "head branch required"}
	}
	result, err := runner(ctx, repoPath, []string{"git", "push", "-u", remote, branch}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "git push failed"
		}
		return ValidationError{Message: message}
	}
	return nil
}

func (s *Service) recordPullRequest(ctx context.Context, resolution repoResolution, payload PullRequestCreatedJSON) {
	state, err := s.workspaces.LoadState(ctx, resolution.WorkspaceRoot)
	if err != nil {
		if s.logf != nil {
			s.logf("workset: unable to load workspace state for PR tracking: %v", err)
		}
		return
	}
	if state.PullRequests == nil {
		state.PullRequests = map[string]workspace.PullRequestState{}
	}
	state.PullRequests[resolution.Repo.Name] = workspace.PullRequestState{
		Repo:       payload.Repo,
		Number:     payload.Number,
		URL:        payload.URL,
		Title:      payload.Title,
		Body:       payload.Body,
		Draft:      payload.Draft,
		State:      payload.State,
		BaseRepo:   payload.BaseRepo,
		BaseBranch: payload.BaseBranch,
		HeadRepo:   payload.HeadRepo,
		HeadBranch: payload.HeadBranch,
		UpdatedAt:  s.clock().Format(time.RFC3339),
	}
	if err := s.workspaces.SaveState(ctx, resolution.WorkspaceRoot, state); err != nil && s.logf != nil {
		s.logf("workset: unable to save workspace state for PR tracking: %v", err)
	}
}

func remoteBranchExists(ctx context.Context, repoPath, remote, branch string, runner CommandRunner) (bool, error) {
	if strings.TrimSpace(repoPath) == "" {
		return false, errors.New("repo path required")
	}
	if strings.TrimSpace(remote) == "" {
		return false, ValidationError{Message: "remote name required to verify head branch"}
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false, ValidationError{Message: "head branch required"}
	}
	result, err := runner(ctx, repoPath, []string{"git", "ls-remote", "--heads", remote, branch}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "unable to verify remote head branch"
		}
		return false, ValidationError{Message: message}
	}
	return strings.TrimSpace(result.Stdout) != "", nil
}

func buildRepoPatch(ctx context.Context, repoPath string, limit int, runner CommandRunner) (string, error) {
	if repoPath == "" {
		return "", errors.New("repo path required")
	}
	parts := []string{}
	staged, err := runGitDiff(ctx, repoPath, runner, true, "")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(staged) != "" {
		parts = append(parts, staged)
	}
	unstaged, err := runGitDiff(ctx, repoPath, runner, false, "")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(unstaged) != "" {
		parts = append(parts, unstaged)
	}
	untracked, err := gitUntracked(ctx, repoPath, runner)
	if err != nil {
		return "", err
	}
	for _, file := range untracked {
		diff, err := gitDiffNoIndex(ctx, repoPath, runner, file)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(diff) != "" {
			parts = append(parts, diff)
		}
	}
	patch := strings.Join(parts, "\n")
	if limit > 0 && len(patch) > limit {
		patch = patch[:limit] + "\n... (diff truncated)\n"
	}
	return patch, nil
}

func runGitDiff(ctx context.Context, repoPath string, runner CommandRunner, staged bool, file string) (string, error) {
	args := []string{"git", "diff"}
	if staged {
		args = append(args, "--cached")
	}
	if file != "" {
		args = append(args, "--", file)
	}
	result, err := runner(ctx, repoPath, args, os.Environ(), "")
	if err != nil && result.ExitCode != 1 {
		return "", err
	}
	return result.Stdout, nil
}

func gitDiffNoIndex(ctx context.Context, repoPath string, runner CommandRunner, file string) (string, error) {
	if strings.TrimSpace(file) == "" {
		return "", nil
	}
	args := []string{"git", "diff", "--no-index", "--", "/dev/null", file}
	result, err := runner(ctx, repoPath, args, os.Environ(), "")
	if err != nil && result.ExitCode != 1 {
		return "", err
	}
	return result.Stdout, nil
}

func gitUntracked(ctx context.Context, repoPath string, runner CommandRunner) ([]string, error) {
	result, err := runner(ctx, repoPath, []string{"git", "ls-files", "--others", "--exclude-standard"}, os.Environ(), "")
	if err != nil && result.ExitCode != 1 {
		return nil, err
	}
	lines := strings.Split(result.Stdout, "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, line)
	}
	return files, nil
}

func gitHasUncommittedChanges(ctx context.Context, repoPath string, runner CommandRunner) (bool, error) {
	result, err := runner(ctx, repoPath, []string{"git", "status", "--porcelain"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "unable to check uncommitted changes"
		}
		return false, ValidationError{Message: message}
	}
	return strings.TrimSpace(result.Stdout) != "", nil
}

func gitAheadBehind(ctx context.Context, repoPath, branch string, runner CommandRunner) (int, int, error) {
	// Get upstream tracking branch
	upstreamResult, err := runner(ctx, repoPath, []string{"git", "rev-parse", "--abbrev-ref", branch + "@{upstream}"}, os.Environ(), "")
	if err != nil || upstreamResult.ExitCode != 0 {
		return 0, 0, ValidationError{Message: "no upstream tracking branch configured"}
	}
	upstream := strings.TrimSpace(upstreamResult.Stdout)
	if upstream == "" {
		return 0, 0, ValidationError{Message: "no upstream tracking branch configured"}
	}

	// Get ahead count
	aheadResult, err := runner(ctx, repoPath, []string{"git", "rev-list", "--count", upstream + ".." + branch}, os.Environ(), "")
	ahead := 0
	if err == nil && aheadResult.ExitCode == 0 {
		if parsed, parseErr := parseCount(aheadResult.Stdout); parseErr == nil {
			ahead = parsed
		}
	}

	// Get behind count
	behindResult, err := runner(ctx, repoPath, []string{"git", "rev-list", "--count", branch + ".." + upstream}, os.Environ(), "")
	behind := 0
	if err == nil && behindResult.ExitCode == 0 {
		if parsed, parseErr := parseCount(behindResult.Stdout); parseErr == nil {
			behind = parsed
		}
	}

	return ahead, behind, nil
}

func parseCount(output string) (int, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return 0, errors.New("empty output")
	}
	var count int
	_, err := fmt.Sscanf(output, "%d", &count)
	return count, err
}

func gitHeadSHA(ctx context.Context, repoPath string, runner CommandRunner) (string, error) {
	result, err := runner(ctx, repoPath, []string{"git", "rev-parse", "HEAD"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		return "", errors.New("unable to get HEAD SHA")
	}
	return strings.TrimSpace(result.Stdout), nil
}
