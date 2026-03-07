package worksetapi

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/git"
)

type localMergeContext struct {
	resolution       repoResolution
	localPath        string
	baseBranch       string
	featureBranch    string
	featureCommitted bool
	featureMessage   string
}

type localMergeBaseTarget struct {
	integrationPath  string
	tempWorktreeName string
	tempBranch       string
	baseRef          string
	baseExists       bool
	baseSHA          string
	cleanup          func(context.Context)
}

func (s *Service) LocalMerge(ctx context.Context, input LocalMergeInput) (LocalMergeResult, error) {
	emitStage := func(stage LocalMergeStage) {
		if input.OnStage != nil {
			input.OnStage(stage)
		}
	}

	mergeCtx, err := s.prepareLocalMergeContext(ctx, input, emitStage)
	if err != nil {
		return LocalMergeResult{}, err
	}
	emitStage(LocalMergeStagePreparingBase)
	baseTarget, err := s.prepareLocalMergeBaseTarget(ctx, mergeCtx)
	if err != nil {
		return LocalMergeResult{}, err
	}
	if baseTarget.cleanup != nil {
		defer baseTarget.cleanup(ctx)
	}
	baseMessage, err := s.commitLocalMergeToBase(ctx, mergeCtx, baseTarget, strings.TrimSpace(input.Message), emitStage)
	if err != nil {
		return LocalMergeResult{}, err
	}
	finalSHA, err := s.finalizeLocalMerge(ctx, mergeCtx, baseTarget)
	if err != nil {
		return LocalMergeResult{}, err
	}
	return buildLocalMergeResult(mergeCtx, baseMessage, finalSHA), nil
}

func (s *Service) prepareLocalMergeContext(
	ctx context.Context,
	input LocalMergeInput,
	emitStage func(LocalMergeStage),
) (localMergeContext, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return localMergeContext{}, err
	}
	localPath := strings.TrimSpace(resolution.Repo.LocalPath)
	if localPath == "" {
		return localMergeContext{}, ValidationError{Message: "local_path required for local merge"}
	}
	baseBranch := strings.TrimSpace(input.Base)
	if baseBranch == "" {
		baseBranch = strings.TrimSpace(resolution.RepoDefaults.DefaultBranch)
	}
	if baseBranch == "" {
		return localMergeContext{}, ValidationError{Message: "base branch required"}
	}
	featureBranch, err := s.resolveCurrentBranch(resolution)
	if err != nil {
		return localMergeContext{}, err
	}
	featureBranch = strings.TrimSpace(strings.TrimPrefix(featureBranch, "refs/heads/"))
	if featureBranch == "" {
		return localMergeContext{}, ValidationError{Message: "feature branch required"}
	}
	if featureBranch == baseBranch {
		return localMergeContext{}, ValidationError{Message: "feature branch already matches base branch"}
	}
	featureCommitted, featureMessage, err := s.commitFeatureBranchIfNeeded(
		ctx,
		resolution,
		featureBranch,
		emitStage,
	)
	if err != nil {
		return localMergeContext{}, err
	}
	if err := ensureLocalMergeSourceClean(ctx, localPath, s.commands); err != nil {
		return localMergeContext{}, err
	}
	return localMergeContext{
		resolution:       resolution,
		localPath:        localPath,
		baseBranch:       baseBranch,
		featureBranch:    featureBranch,
		featureCommitted: featureCommitted,
		featureMessage:   featureMessage,
	}, nil
}

func (s *Service) commitFeatureBranchIfNeeded(
	ctx context.Context,
	resolution repoResolution,
	featureBranch string,
	emitStage func(LocalMergeStage),
) (bool, string, error) {
	hasUncommitted, err := gitHasUncommittedChanges(ctx, resolution.RepoPath, s.commands)
	if err != nil || !hasUncommitted {
		return false, "", err
	}
	emitStage(LocalMergeStageGeneratingMessage)
	featureMessage, err := s.generateCommitMessage(ctx, resolution, resolution.RepoPath, featureBranch)
	if err != nil {
		return false, "", err
	}
	emitStage(LocalMergeStageCommittingWorktree)
	if err := gitAddAll(ctx, resolution.RepoPath, s.commands); err != nil {
		return false, "", err
	}
	hasStaged, err := gitHasStagedChanges(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		return false, "", err
	}
	if !hasStaged {
		return false, "", ValidationError{Message: "no changes staged after git add"}
	}
	if err := gitCommitMessage(ctx, resolution.RepoPath, featureMessage, s.commands); err != nil {
		return false, "", err
	}
	return true, featureMessage, nil
}

func ensureLocalMergeSourceClean(ctx context.Context, localPath string, runner CommandRunner) error {
	sourceDirty, err := gitHasUncommittedChanges(ctx, localPath, runner)
	if err != nil {
		return err
	}
	if sourceDirty {
		return ValidationError{Message: "source repo must be clean before local merge"}
	}
	return nil
}

func (s *Service) prepareLocalMergeBaseTarget(
	ctx context.Context,
	mergeCtx localMergeContext,
) (localMergeBaseTarget, error) {
	sourceBranch, sourceOnBranch, err := s.git.CurrentBranch(mergeCtx.localPath)
	if err != nil {
		return localMergeBaseTarget{}, err
	}
	baseRef := "refs/heads/" + mergeCtx.baseBranch
	baseExists, baseSHA, err := s.resolveLocalMergeBaseRef(ctx, mergeCtx.localPath, baseRef)
	if err != nil {
		return localMergeBaseTarget{}, err
	}
	target := localMergeBaseTarget{
		integrationPath: mergeCtx.localPath,
		baseRef:         baseRef,
		baseExists:      baseExists,
		baseSHA:         baseSHA,
	}
	if sourceOnBranch && strings.TrimSpace(sourceBranch) == mergeCtx.baseBranch {
		return target, nil
	}
	tempWorktreeName := fmt.Sprintf("local-merge-%d", s.clock().UnixNano())
	tempBranch := "workset/" + tempWorktreeName
	tempPath := filepath.Join(mergeCtx.resolution.WorkspaceRoot, ".workset", "tmp", tempWorktreeName)
	if err := s.git.WorktreeAdd(ctx, git.WorktreeAddOptions{
		RepoPath:      mergeCtx.localPath,
		WorktreePath:  tempPath,
		WorktreeName:  tempWorktreeName,
		BranchName:    tempBranch,
		StartRemote:   strings.TrimSpace(mergeCtx.resolution.RepoDefaults.Remote),
		StartBranch:   mergeCtx.baseBranch,
		ForceCheckout: false,
	}); err != nil {
		return localMergeBaseTarget{}, err
	}
	target.integrationPath = tempPath
	target.tempWorktreeName = tempWorktreeName
	target.tempBranch = tempBranch
	target.cleanup = func(cleanupCtx context.Context) {
		_ = s.git.WorktreeRemove(git.WorktreeRemoveOptions{
			RepoPath:     mergeCtx.localPath,
			WorktreeName: tempWorktreeName,
			Force:        true,
		})
		_ = gitDeleteBranch(cleanupCtx, mergeCtx.localPath, tempBranch, s.commands)
	}
	return target, nil
}

func (s *Service) resolveLocalMergeBaseRef(
	ctx context.Context,
	localPath string,
	baseRef string,
) (bool, string, error) {
	baseExists, err := s.git.ReferenceExists(ctx, localPath, baseRef)
	if err != nil {
		return false, "", err
	}
	if !baseExists {
		return false, "", nil
	}
	baseSHA, err := gitResolveRef(ctx, localPath, baseRef, s.commands)
	if err != nil {
		return false, "", err
	}
	return true, baseSHA, nil
}

func (s *Service) commitLocalMergeToBase(
	ctx context.Context,
	mergeCtx localMergeContext,
	target localMergeBaseTarget,
	requestedMessage string,
	emitStage func(LocalMergeStage),
) (string, error) {
	emitStage(LocalMergeStageMerging)
	if err := gitMergeSquash(ctx, target.integrationPath, mergeCtx.featureBranch, s.commands); err != nil {
		return "", s.resetLocalMergeBaseOnError(ctx, target.integrationPath, err)
	}
	baseMessage := requestedMessage
	var err error
	if baseMessage == "" {
		emitStage(LocalMergeStageGeneratingMessage)
		baseMessage, err = s.generateCommitMessage(
			ctx,
			mergeCtx.resolution,
			target.integrationPath,
			mergeCtx.baseBranch,
		)
		if err != nil {
			return "", s.resetLocalMergeBaseOnError(ctx, target.integrationPath, err)
		}
	}
	emitStage(LocalMergeStageCommittingBase)
	if err := s.commitLocalMergeBaseChanges(ctx, target.integrationPath, baseMessage); err != nil {
		return "", s.resetLocalMergeBaseOnError(ctx, target.integrationPath, err)
	}
	return baseMessage, nil
}

func (s *Service) commitLocalMergeBaseChanges(
	ctx context.Context,
	integrationPath string,
	baseMessage string,
) error {
	if err := gitAddAll(ctx, integrationPath, s.commands); err != nil {
		return err
	}
	hasStaged, err := gitHasStagedChanges(ctx, integrationPath, s.commands)
	if err != nil {
		return err
	}
	if !hasStaged {
		return ValidationError{Message: "no changes to merge into base branch"}
	}
	return gitCommitMessage(ctx, integrationPath, baseMessage, s.commands)
}

func (s *Service) resetLocalMergeBaseOnError(
	ctx context.Context,
	integrationPath string,
	err error,
) error {
	if integrationPath == "" {
		return err
	}
	if resetErr := gitResetHardHead(ctx, integrationPath, s.commands); resetErr != nil && err == nil {
		return resetErr
	}
	return err
}

func (s *Service) finalizeLocalMerge(
	ctx context.Context,
	mergeCtx localMergeContext,
	target localMergeBaseTarget,
) (string, error) {
	if target.tempBranch != "" {
		if err := s.updateLocalMergeBaseBranch(ctx, mergeCtx, target); err != nil {
			return "", err
		}
	}
	return gitResolveRef(ctx, mergeCtx.localPath, target.baseRef, s.commands)
}

func (s *Service) updateLocalMergeBaseBranch(
	ctx context.Context,
	mergeCtx localMergeContext,
	target localMergeBaseTarget,
) error {
	if target.baseExists {
		currentBaseSHA, err := gitResolveRef(ctx, mergeCtx.localPath, target.baseRef, s.commands)
		if err != nil {
			return err
		}
		if currentBaseSHA != target.baseSHA {
			return ValidationError{Message: "base branch changed during local merge; retry after syncing"}
		}
	}
	return s.git.UpdateBranch(ctx, mergeCtx.localPath, mergeCtx.baseBranch, target.tempBranch)
}

func buildLocalMergeResult(
	mergeCtx localMergeContext,
	baseMessage string,
	finalSHA string,
) LocalMergeResult {
	pushRemote := strings.TrimSpace(mergeCtx.resolution.RepoDefaults.Remote)
	return LocalMergeResult{
		Payload: LocalMergeResultJSON{
			BaseBranch:           mergeCtx.baseBranch,
			BaseBranchPushed:     false,
			FeatureBranch:        mergeCtx.featureBranch,
			FeatureCommitted:     mergeCtx.featureCommitted,
			FeatureCommitMessage: mergeCtx.featureMessage,
			BaseCommitMessage:    baseMessage,
			BaseSHA:              finalSHA,
			PushRemote:           pushRemote,
			Pushable:             pushRemote != "",
		},
		Config: mergeCtx.resolution.ConfigInfo,
	}
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
