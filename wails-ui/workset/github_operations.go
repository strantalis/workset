package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/strantalis/workset/pkg/worksetapi"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type GitHubOperationType string

const (
	GitHubOperationTypeCreatePR   GitHubOperationType = "create_pr"
	GitHubOperationTypeCommitPush GitHubOperationType = "commit_push"
)

type GitHubOperationStage string

const (
	GitHubOperationStageQueued     GitHubOperationStage = "queued"
	GitHubOperationStageGenerating GitHubOperationStage = "generating"
	GitHubOperationStageCreating   GitHubOperationStage = "creating"
	GitHubOperationStageCommitting GitHubOperationStage = "committing"
	GitHubOperationStagePushing    GitHubOperationStage = "pushing"
	GitHubOperationStageCompleted  GitHubOperationStage = "completed"
	GitHubOperationStageFailed     GitHubOperationStage = "failed"
)

type GitHubOperationState string

const (
	GitHubOperationStateRunning   GitHubOperationState = "running"
	GitHubOperationStateCompleted GitHubOperationState = "completed"
	GitHubOperationStateFailed    GitHubOperationState = "failed"
)

type GitHubOperationStatusPayload struct {
	OperationID string                              `json:"operationId"`
	WorkspaceID string                              `json:"workspaceId"`
	RepoID      string                              `json:"repoId"`
	Type        GitHubOperationType                 `json:"type"`
	Stage       GitHubOperationStage                `json:"stage"`
	State       GitHubOperationState                `json:"state"`
	StartedAt   string                              `json:"startedAt"`
	FinishedAt  string                              `json:"finishedAt,omitempty"`
	Error       string                              `json:"error,omitempty"`
	PullRequest *worksetapi.PullRequestCreatedJSON  `json:"pullRequest,omitempty"`
	CommitPush  *worksetapi.CommitAndPushResultJSON `json:"commitPush,omitempty"`
}

type githubOperationKey struct {
	workspaceID string
	repoID      string
	opType      GitHubOperationType
}

type githubOperationManager struct {
	mu     sync.Mutex
	seq    uint64
	status map[githubOperationKey]GitHubOperationStatusPayload
}

func newGitHubOperationManager() *githubOperationManager {
	return &githubOperationManager{
		status: map[githubOperationKey]GitHubOperationStatusPayload{},
	}
}

func (m *githubOperationManager) start(workspaceID, repoID string, opType GitHubOperationType) (githubOperationKey, GitHubOperationStatusPayload, error) {
	key := githubOperationKey{
		workspaceID: workspaceID,
		repoID:      repoID,
		opType:      opType,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for currentKey, currentStatus := range m.status {
		if currentKey.workspaceID != workspaceID || currentKey.repoID != repoID {
			continue
		}
		if currentStatus.State == GitHubOperationStateRunning {
			return githubOperationKey{}, GitHubOperationStatusPayload{}, worksetapi.ValidationError{
				Message: fmt.Sprintf("operation already running for repo (%s)", currentStatus.Type),
			}
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	status := GitHubOperationStatusPayload{
		OperationID: m.nextOperationID(opType),
		WorkspaceID: workspaceID,
		RepoID:      repoID,
		Type:        opType,
		Stage:       GitHubOperationStageQueued,
		State:       GitHubOperationStateRunning,
		StartedAt:   now,
	}
	m.status[key] = status
	return key, status, nil
}

func (m *githubOperationManager) get(key githubOperationKey) (GitHubOperationStatusPayload, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status, ok := m.status[key]
	return status, ok
}

func (m *githubOperationManager) setStage(key githubOperationKey, stage GitHubOperationStage) (GitHubOperationStatusPayload, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status, ok := m.status[key]
	if !ok {
		return GitHubOperationStatusPayload{}, false
	}
	status.Stage = stage
	status.State = GitHubOperationStateRunning
	status.Error = ""
	status.FinishedAt = ""
	m.status[key] = status
	return status, true
}

func (m *githubOperationManager) completeCreatePR(key githubOperationKey, result worksetapi.PullRequestCreatedJSON) (GitHubOperationStatusPayload, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status, ok := m.status[key]
	if !ok {
		return GitHubOperationStatusPayload{}, false
	}
	status.Stage = GitHubOperationStageCompleted
	status.State = GitHubOperationStateCompleted
	status.Error = ""
	status.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	status.PullRequest = &result
	status.CommitPush = nil
	m.status[key] = status
	return status, true
}

func (m *githubOperationManager) completeCommitPush(key githubOperationKey, result worksetapi.CommitAndPushResultJSON) (GitHubOperationStatusPayload, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status, ok := m.status[key]
	if !ok {
		return GitHubOperationStatusPayload{}, false
	}
	status.Stage = GitHubOperationStageCompleted
	status.State = GitHubOperationStateCompleted
	status.Error = ""
	status.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	status.PullRequest = nil
	status.CommitPush = &result
	m.status[key] = status
	return status, true
}

func (m *githubOperationManager) fail(key githubOperationKey, err error) (GitHubOperationStatusPayload, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status, ok := m.status[key]
	if !ok {
		return GitHubOperationStatusPayload{}, false
	}
	status.Stage = GitHubOperationStageFailed
	status.State = GitHubOperationStateFailed
	if err != nil {
		status.Error = strings.TrimSpace(err.Error())
	} else {
		status.Error = ""
	}
	status.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	m.status[key] = status
	return status, true
}

func (m *githubOperationManager) nextOperationID(opType GitHubOperationType) string {
	next := atomic.AddUint64(&m.seq, 1)
	return fmt.Sprintf("%s-%d-%d", opType, time.Now().UTC().UnixNano(), next)
}

func parseGitHubOperationType(raw string) (GitHubOperationType, error) {
	switch GitHubOperationType(strings.TrimSpace(raw)) {
	case GitHubOperationTypeCreatePR:
		return GitHubOperationTypeCreatePR, nil
	case GitHubOperationTypeCommitPush:
		return GitHubOperationTypeCommitPush, nil
	default:
		return "", worksetapi.ValidationError{
			Message: fmt.Sprintf("invalid operation type %q", strings.TrimSpace(raw)),
		}
	}
}

var githubOperationEmit = wruntime.EventsEmit

func (a *App) ensureGitHubOperationManager() *githubOperationManager {
	if a.githubOps == nil {
		a.githubOps = newGitHubOperationManager()
	}
	return a.githubOps
}

func (a *App) emitGitHubOperation(status GitHubOperationStatusPayload) {
	if a == nil || a.ctx == nil {
		return
	}
	githubOperationEmit(a.ctx, "github:operation", status)
}

func (a *App) runCreatePullRequestAsync(ctx context.Context, key githubOperationKey, repoName string, input StartCreatePullRequestAsyncRequest) {
	manager := a.ensureGitHubOperationManager()
	if status, ok := manager.setStage(key, GitHubOperationStageGenerating); ok {
		a.emitGitHubOperation(status)
	}

	generated, err := a.service.GeneratePullRequestText(ctx, worksetapi.PullRequestGenerateInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
	})
	if err != nil {
		if status, ok := manager.fail(key, err); ok {
			a.emitGitHubOperation(status)
		}
		return
	}

	if status, ok := manager.setStage(key, GitHubOperationStageCreating); ok {
		a.emitGitHubOperation(status)
	}

	created, err := a.service.CreatePullRequest(ctx, worksetapi.PullRequestCreateInput{
		Workspace:  worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:       repoName,
		Base:       input.Base,
		Head:       input.Head,
		BaseRemote: input.BaseRemote,
		Title:      generated.Payload.Title,
		Body:       generated.Payload.Body,
		Draft:      input.Draft,
		AutoCommit: true,
		AutoPush:   true,
	})
	if err != nil {
		if status, ok := manager.fail(key, err); ok {
			a.emitGitHubOperation(status)
		}
		return
	}

	if status, ok := manager.completeCreatePR(key, created.Payload); ok {
		a.emitGitHubOperation(status)
	}
}

func (a *App) runCommitAndPushAsync(ctx context.Context, key githubOperationKey, repoName string, input StartCommitAndPushAsyncRequest) {
	manager := a.ensureGitHubOperationManager()
	if status, ok := manager.setStage(key, GitHubOperationStageCommitting); ok {
		a.emitGitHubOperation(status)
	}

	result, err := a.service.CommitAndPush(ctx, worksetapi.CommitAndPushInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		Message:   input.Message,
	})
	if err != nil {
		if status, ok := manager.fail(key, err); ok {
			a.emitGitHubOperation(status)
		}
		return
	}

	if status, ok := manager.setStage(key, GitHubOperationStagePushing); ok {
		a.emitGitHubOperation(status)
	}
	if status, ok := manager.completeCommitPush(key, result.Payload); ok {
		a.emitGitHubOperation(status)
	}
}
