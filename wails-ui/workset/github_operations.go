package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/strantalis/workset/pkg/worksetapi"
)

type GitHubOperationType string

const (
	GitHubOperationTypeCreatePR   GitHubOperationType = "create_pr"
	GitHubOperationTypeCommitPush GitHubOperationType = "commit_push"
)

type GitHubOperationStage string

const (
	GitHubOperationStageQueued            GitHubOperationStage = "queued"
	GitHubOperationStageGenerating        GitHubOperationStage = "generating"
	GitHubOperationStageCreating          GitHubOperationStage = "creating"
	GitHubOperationStageGeneratingMessage GitHubOperationStage = "generating_message"
	GitHubOperationStageStaging           GitHubOperationStage = "staging"
	GitHubOperationStageCommitting        GitHubOperationStage = "committing"
	GitHubOperationStagePushing           GitHubOperationStage = "pushing"
	GitHubOperationStageCompleted         GitHubOperationStage = "completed"
	GitHubOperationStageFailed            GitHubOperationStage = "failed"
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
	now    func() time.Time
	status map[githubOperationKey]githubOperationRecord
}

type githubOperationRecord struct {
	status      GitHubOperationStatusPayload
	lastUpdated time.Time
}

const (
	githubOperationRetention  = 6 * time.Hour
	githubOperationMaxEntries = 256
)

func newGitHubOperationManager() *githubOperationManager {
	return &githubOperationManager{
		now:    time.Now,
		status: map[githubOperationKey]githubOperationRecord{},
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

	m.cleanupLocked(m.now())

	for currentKey, currentStatus := range m.status {
		if currentKey.workspaceID != workspaceID || currentKey.repoID != repoID {
			continue
		}
		if currentStatus.status.State == GitHubOperationStateRunning {
			return githubOperationKey{}, GitHubOperationStatusPayload{}, worksetapi.ValidationError{
				Message: fmt.Sprintf("operation already running for repo (%s)", currentStatus.status.Type),
			}
		}
	}

	now := m.now()
	status := GitHubOperationStatusPayload{
		OperationID: m.nextOperationID(opType),
		WorkspaceID: workspaceID,
		RepoID:      repoID,
		Type:        opType,
		Stage:       GitHubOperationStageQueued,
		State:       GitHubOperationStateRunning,
		StartedAt:   now.UTC().Format(time.RFC3339),
	}
	m.status[key] = githubOperationRecord{
		status:      status,
		lastUpdated: now,
	}
	return key, status, nil
}

func (m *githubOperationManager) get(key githubOperationKey) (GitHubOperationStatusPayload, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.status[key]
	if !ok {
		return GitHubOperationStatusPayload{}, false
	}
	return record.status, true
}

func (m *githubOperationManager) setStage(key githubOperationKey, stage GitHubOperationStage) (GitHubOperationStatusPayload, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.status[key]
	if !ok {
		return GitHubOperationStatusPayload{}, false
	}
	status := record.status
	status.Stage = stage
	status.State = GitHubOperationStateRunning
	status.Error = ""
	status.FinishedAt = ""
	m.status[key] = githubOperationRecord{
		status:      status,
		lastUpdated: m.now(),
	}
	return status, true
}

func (m *githubOperationManager) completeCreatePR(key githubOperationKey, result worksetapi.PullRequestCreatedJSON) (GitHubOperationStatusPayload, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.status[key]
	if !ok {
		return GitHubOperationStatusPayload{}, false
	}
	status := record.status
	status.Stage = GitHubOperationStageCompleted
	status.State = GitHubOperationStateCompleted
	status.Error = ""
	status.FinishedAt = m.now().UTC().Format(time.RFC3339)
	status.PullRequest = &result
	status.CommitPush = nil
	now := m.now()
	m.status[key] = githubOperationRecord{
		status:      status,
		lastUpdated: now,
	}
	m.cleanupLocked(now)
	return status, true
}

func (m *githubOperationManager) completeCommitPush(key githubOperationKey, result worksetapi.CommitAndPushResultJSON) (GitHubOperationStatusPayload, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.status[key]
	if !ok {
		return GitHubOperationStatusPayload{}, false
	}
	status := record.status
	status.Stage = GitHubOperationStageCompleted
	status.State = GitHubOperationStateCompleted
	status.Error = ""
	status.FinishedAt = m.now().UTC().Format(time.RFC3339)
	status.PullRequest = nil
	status.CommitPush = &result
	now := m.now()
	m.status[key] = githubOperationRecord{
		status:      status,
		lastUpdated: now,
	}
	m.cleanupLocked(now)
	return status, true
}

func (m *githubOperationManager) fail(key githubOperationKey, err error) (GitHubOperationStatusPayload, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.status[key]
	if !ok {
		return GitHubOperationStatusPayload{}, false
	}
	status := record.status
	status.Stage = GitHubOperationStageFailed
	status.State = GitHubOperationStateFailed
	if err != nil {
		status.Error = strings.TrimSpace(err.Error())
	} else {
		status.Error = ""
	}
	status.FinishedAt = m.now().UTC().Format(time.RFC3339)
	now := m.now()
	m.status[key] = githubOperationRecord{
		status:      status,
		lastUpdated: now,
	}
	m.cleanupLocked(now)
	return status, true
}

func (m *githubOperationManager) cleanupLocked(now time.Time) {
	ttlCutoff := now.Add(-githubOperationRetention)
	for key, record := range m.status {
		if record.status.State == GitHubOperationStateRunning {
			continue
		}
		if record.lastUpdated.Before(ttlCutoff) {
			delete(m.status, key)
		}
	}

	if len(m.status) <= githubOperationMaxEntries {
		return
	}

	type candidate struct {
		key         githubOperationKey
		lastUpdated time.Time
	}
	candidates := make([]candidate, 0, len(m.status))
	for key, record := range m.status {
		if record.status.State == GitHubOperationStateRunning {
			continue
		}
		candidates = append(candidates, candidate{
			key:         key,
			lastUpdated: record.lastUpdated,
		})
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		if a.lastUpdated.Equal(b.lastUpdated) {
			return 0
		}
		if a.lastUpdated.Before(b.lastUpdated) {
			return -1
		}
		return 1
	})

	for _, candidate := range candidates {
		if len(m.status) <= githubOperationMaxEntries {
			break
		}
		delete(m.status, candidate.key)
	}
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

var githubOperationEmit = emitRuntimeEvent

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
	githubOperationEmit(a.ctx, EventGitHubOperation, status)
}

func (a *App) runCreatePullRequestAsync(ctx context.Context, key githubOperationKey, repoName string, input StartCreatePullRequestAsyncRequest) {
	manager := a.ensureGitHubOperationManager()
	svc := a.ensureService()
	if status, ok := manager.setStage(key, GitHubOperationStageGenerating); ok {
		a.emitGitHubOperation(status)
	}

	generated, err := svc.GeneratePullRequestText(ctx, worksetapi.PullRequestGenerateInput{
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

	created, err := svc.CreatePullRequest(ctx, worksetapi.PullRequestCreateInput{
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
	svc := a.ensureService()

	result, err := svc.CommitAndPush(ctx, worksetapi.CommitAndPushInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		Message:   input.Message,
		OnStage: func(stage worksetapi.CommitAndPushStage) {
			statusStage := GitHubOperationStageCommitting
			switch stage {
			case worksetapi.CommitAndPushStageGeneratingMessage:
				statusStage = GitHubOperationStageGeneratingMessage
			case worksetapi.CommitAndPushStageStaging:
				statusStage = GitHubOperationStageStaging
			case worksetapi.CommitAndPushStageCommitting:
				statusStage = GitHubOperationStageCommitting
			case worksetapi.CommitAndPushStagePushing:
				statusStage = GitHubOperationStagePushing
			}
			if status, ok := manager.setStage(key, statusStage); ok {
				a.emitGitHubOperation(status)
			}
		},
	})
	if err != nil {
		if status, ok := manager.fail(key, err); ok {
			a.emitGitHubOperation(status)
		}
		return
	}

	if status, ok := manager.completeCommitPush(key, result.Payload); ok {
		a.emitGitHubOperation(status)
	}
}
