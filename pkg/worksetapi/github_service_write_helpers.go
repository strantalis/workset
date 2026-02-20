package worksetapi

import (
	"context"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/workspace"
)

func (s *Service) commitPullRequestChanges(ctx context.Context, resolution repoResolution, branch string) error {
	agent := strings.TrimSpace(resolution.Defaults.Agent)
	if agent == "" {
		return ValidationError{Message: "defaults.agent is not configured"}
	}
	model := strings.TrimSpace(resolution.Defaults.AgentModel)
	diffLimit := defaultDiffLimit
	patch, err := buildRepoPatch(ctx, resolution.RepoPath, diffLimit, s.commands)
	if err != nil {
		return err
	}
	if strings.TrimSpace(patch) == "" {
		return nil
	}
	if err := s.preflightSSHAuth(ctx, resolution); err != nil {
		return err
	}
	prompt := formatCommitPrompt(resolution.Repo.Name, branch, patch)
	message, err := s.runCommitMessageWithModel(ctx, resolution.RepoPath, agent, prompt, model)
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
		Merged:     payload.Merged,
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

func (s *Service) clearTrackedPullRequestIfMatchingNumber(
	ctx context.Context,
	resolution repoResolution,
	number int,
) {
	state, err := s.workspaces.LoadState(ctx, resolution.WorkspaceRoot)
	if err != nil {
		if s.logf != nil {
			s.logf("workset: unable to load workspace state for PR untracking: %v", err)
		}
		return
	}
	if len(state.PullRequests) == 0 {
		return
	}
	tracked, ok := state.PullRequests[resolution.Repo.Name]
	if !ok || tracked.Number != number {
		return
	}
	delete(state.PullRequests, resolution.Repo.Name)
	if err := s.workspaces.SaveState(ctx, resolution.WorkspaceRoot, state); err != nil && s.logf != nil {
		s.logf("workset: unable to save workspace state for PR untracking: %v", err)
	}
}
