package worksetapi

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/git"
)

func (s *Service) LocalMerge(ctx context.Context, input LocalMergeInput) (LocalMergeResult, error) {
	emitStage := func(stage LocalMergeStage) {
		if input.OnStage != nil {
			input.OnStage(stage)
		}
	}

	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return LocalMergeResult{}, err
	}

	localPath := strings.TrimSpace(resolution.Repo.LocalPath)
	if localPath == "" {
		return LocalMergeResult{}, ValidationError{Message: "local_path required for local merge"}
	}

	baseBranch := strings.TrimSpace(input.Base)
	if baseBranch == "" {
		baseBranch = strings.TrimSpace(resolution.RepoDefaults.DefaultBranch)
	}
	if baseBranch == "" {
		return LocalMergeResult{}, ValidationError{Message: "base branch required"}
	}

	featureBranch, err := s.resolveCurrentBranch(resolution)
	if err != nil {
		return LocalMergeResult{}, err
	}
	featureBranch = strings.TrimSpace(strings.TrimPrefix(featureBranch, "refs/heads/"))
	if featureBranch == "" {
		return LocalMergeResult{}, ValidationError{Message: "feature branch required"}
	}
	if featureBranch == baseBranch {
		return LocalMergeResult{}, ValidationError{Message: "feature branch already matches base branch"}
	}

	hasUncommitted, err := gitHasUncommittedChanges(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		return LocalMergeResult{}, err
	}

	featureCommitted := false
	featureMessage := ""
	if hasUncommitted {
		emitStage(LocalMergeStageGeneratingMessage)
		featureMessage, err = s.generateCommitMessage(ctx, resolution, resolution.RepoPath, featureBranch)
		if err != nil {
			return LocalMergeResult{}, err
		}
		emitStage(LocalMergeStageCommittingWorktree)
		if err := gitAddAll(ctx, resolution.RepoPath, s.commands); err != nil {
			return LocalMergeResult{}, err
		}
		hasStaged, err := gitHasStagedChanges(ctx, resolution.RepoPath, s.commands)
		if err != nil {
			return LocalMergeResult{}, err
		}
		if !hasStaged {
			return LocalMergeResult{}, ValidationError{Message: "no changes staged after git add"}
		}
		if err := gitCommitMessage(ctx, resolution.RepoPath, featureMessage, s.commands); err != nil {
			return LocalMergeResult{}, err
		}
		featureCommitted = true
	}

	sourceDirty, err := gitHasUncommittedChanges(ctx, localPath, s.commands)
	if err != nil {
		return LocalMergeResult{}, err
	}
	if sourceDirty {
		return LocalMergeResult{}, ValidationError{Message: "source repo must be clean before local merge"}
	}

	sourceBranch, sourceOnBranch, err := s.git.CurrentBranch(localPath)
	if err != nil {
		return LocalMergeResult{}, err
	}

	baseRef := "refs/heads/" + baseBranch
	baseExists, err := s.git.ReferenceExists(ctx, localPath, baseRef)
	if err != nil {
		return LocalMergeResult{}, err
	}
	baseSHA := ""
	if baseExists {
		baseSHA, err = gitResolveRef(ctx, localPath, baseRef, s.commands)
		if err != nil {
			return LocalMergeResult{}, err
		}
	}

	emitStage(LocalMergeStagePreparingBase)

	integrationPath := localPath
	tempWorktreeName := ""
	tempBranch := ""
	var cleanup func(context.Context)
	if !sourceOnBranch || strings.TrimSpace(sourceBranch) != baseBranch {
		tempWorktreeName = fmt.Sprintf("local-merge-%d", s.clock().UnixNano())
		tempBranch = "workset/" + tempWorktreeName
		tempPath := filepath.Join(resolution.WorkspaceRoot, ".workset", "tmp", tempWorktreeName)
		if err := s.git.WorktreeAdd(ctx, git.WorktreeAddOptions{
			RepoPath:      localPath,
			WorktreePath:  tempPath,
			WorktreeName:  tempWorktreeName,
			BranchName:    tempBranch,
			StartRemote:   strings.TrimSpace(resolution.RepoDefaults.Remote),
			StartBranch:   baseBranch,
			ForceCheckout: false,
		}); err != nil {
			return LocalMergeResult{}, err
		}
		integrationPath = tempPath
		cleanup = func(cleanupCtx context.Context) {
			_ = s.git.WorktreeRemove(git.WorktreeRemoveOptions{
				RepoPath:     localPath,
				WorktreeName: tempWorktreeName,
				Force:        true,
			})
			_ = gitDeleteBranch(cleanupCtx, localPath, tempBranch, s.commands)
		}
		defer cleanup(ctx)
	}

	emitStage(LocalMergeStageMerging)
	if err := gitMergeSquash(ctx, integrationPath, featureBranch, s.commands); err != nil {
		if integrationPath == localPath {
			_ = gitResetHardHead(ctx, integrationPath, s.commands)
		}
		return LocalMergeResult{}, err
	}

	emitStage(LocalMergeStageGeneratingMessage)
	baseMessage := strings.TrimSpace(input.Message)
	if baseMessage == "" {
		baseMessage, err = s.generateCommitMessage(ctx, resolution, integrationPath, baseBranch)
		if err != nil {
			if integrationPath == localPath {
				_ = gitResetHardHead(ctx, integrationPath, s.commands)
			}
			return LocalMergeResult{}, err
		}
	}

	emitStage(LocalMergeStageCommittingBase)
	if err := gitAddAll(ctx, integrationPath, s.commands); err != nil {
		if integrationPath == localPath {
			_ = gitResetHardHead(ctx, integrationPath, s.commands)
		}
		return LocalMergeResult{}, err
	}
	hasStaged, err := gitHasStagedChanges(ctx, integrationPath, s.commands)
	if err != nil {
		if integrationPath == localPath {
			_ = gitResetHardHead(ctx, integrationPath, s.commands)
		}
		return LocalMergeResult{}, err
	}
	if !hasStaged {
		if integrationPath == localPath {
			_ = gitResetHardHead(ctx, integrationPath, s.commands)
		}
		return LocalMergeResult{}, ValidationError{Message: "no changes to merge into base branch"}
	}
	if err := gitCommitMessage(ctx, integrationPath, baseMessage, s.commands); err != nil {
		if integrationPath == localPath {
			_ = gitResetHardHead(ctx, integrationPath, s.commands)
		}
		return LocalMergeResult{}, err
	}

	if tempBranch != "" {
		currentBaseSHA := ""
		if baseExists {
			currentBaseSHA, err = gitResolveRef(ctx, localPath, baseRef, s.commands)
			if err != nil {
				return LocalMergeResult{}, err
			}
			if currentBaseSHA != baseSHA {
				return LocalMergeResult{}, ValidationError{Message: "base branch changed during local merge; retry after syncing"}
			}
		}
		if err := s.git.UpdateBranch(ctx, localPath, baseBranch, tempBranch); err != nil {
			return LocalMergeResult{}, err
		}
	}

	finalSHA, err := gitResolveRef(ctx, localPath, baseRef, s.commands)
	if err != nil {
		return LocalMergeResult{}, err
	}

	return LocalMergeResult{
		Payload: LocalMergeResultJSON{
			BaseBranch:           baseBranch,
			BaseBranchPushed:     false,
			FeatureBranch:        featureBranch,
			FeatureCommitted:     featureCommitted,
			FeatureCommitMessage: featureMessage,
			BaseCommitMessage:    baseMessage,
			BaseSHA:              finalSHA,
			PushRemote:           strings.TrimSpace(resolution.RepoDefaults.Remote),
			Pushable:             strings.TrimSpace(resolution.RepoDefaults.Remote) != "",
		},
		Config: resolution.ConfigInfo,
	}, nil
}

func (s *Service) PushBranch(ctx context.Context, input PushBranchInput) (PushBranchResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return PushBranchResult{}, err
	}

	localPath := strings.TrimSpace(resolution.Repo.LocalPath)
	if localPath == "" {
		localPath = resolution.RepoPath
	}

	branch := strings.TrimSpace(input.Branch)
	if branch == "" {
		return PushBranchResult{}, ValidationError{Message: "branch required"}
	}
	remote := strings.TrimSpace(resolution.RepoDefaults.Remote)
	if remote == "" {
		return PushBranchResult{}, ValidationError{Message: "remote required to push branch"}
	}
	if err := s.preflightSSHAuth(ctx, repoResolution{
		RepoPath:      localPath,
		Repo:          resolution.Repo,
		WorkspaceRoot: resolution.WorkspaceRoot,
		Defaults:      resolution.Defaults,
		RepoDefaults:  resolution.RepoDefaults,
	}); err != nil {
		return PushBranchResult{}, err
	}
	if err := gitPushBranch(ctx, localPath, remote, branch, s.commands); err != nil {
		return PushBranchResult{}, err
	}
	return PushBranchResult{
		Payload: PushBranchResultJSON{
			Branch: branch,
			Remote: remote,
			Pushed: true,
		},
		Config: resolution.ConfigInfo,
	}, nil
}

func (s *Service) generateCommitMessage(
	ctx context.Context,
	resolution repoResolution,
	repoPath string,
	branch string,
) (string, error) {
	agent := strings.TrimSpace(resolution.Defaults.Agent)
	if agent == "" {
		return "", ValidationError{Message: "defaults.agent is not configured; cannot auto-generate commit message"}
	}
	patch, err := buildRepoPatch(ctx, repoPath, defaultDiffLimit, s.commands)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(patch) == "" {
		return "", ValidationError{Message: "no changes to commit"}
	}
	return s.runCommitMessageWithModel(
		ctx,
		repoPath,
		agent,
		formatCommitPrompt(resolution.Repo.Name, branch, patch),
		resolution.Defaults.AgentModel,
	)
}
