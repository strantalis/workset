package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/strantalis/workset/pkg/worksetapi"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	repoDiffLocalPollInterval = 60 * time.Second
	repoDiffPrPollInterval    = 30 * time.Second
	repoDiffDebounceWindow    = 400 * time.Millisecond
)

var repoDiffEmit = wruntime.EventsEmit

type repoDiffPrStatusResult struct {
	pullRequest worksetapi.PullRequestStatusJSON
	checks      []worksetapi.PullRequestCheckJSON
}

var repoDiffGetLocalStatus = func(ctx context.Context, app *App, key repoDiffWatchKey, repoName string) (worksetapi.RepoLocalStatusJSON, error) {
	result, err := app.service.GetRepoLocalStatus(ctx, worksetapi.RepoLocalStatusInput{
		Workspace: worksetapi.WorkspaceSelector{Value: key.workspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return worksetapi.RepoLocalStatusJSON{}, err
	}
	return result.Payload, nil
}

var repoDiffCollectLocalSummary = func(ctx context.Context, repoPath string) (RepoDiffSummary, error) {
	files, err := collectRepoDiffSummary(ctx, repoPath)
	if err != nil {
		return RepoDiffSummary{}, err
	}
	summary := RepoDiffSummary{Files: files}
	for _, file := range files {
		summary.TotalAdded += file.Added
		summary.TotalRemoved += file.Removed
	}
	return summary, nil
}

var repoDiffGetTrackedPR = func(ctx context.Context, app *App, key repoDiffWatchKey, repoName string) (worksetapi.PullRequestCreatedJSON, bool, error) {
	result, err := app.service.GetTrackedPullRequest(ctx, worksetapi.PullRequestTrackedInput{
		Workspace: worksetapi.WorkspaceSelector{Value: key.workspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return worksetapi.PullRequestCreatedJSON{}, false, err
	}
	if !result.Payload.Found {
		return worksetapi.PullRequestCreatedJSON{}, false, nil
	}
	return result.Payload.PullRequest, true, nil
}

var repoDiffGetPrStatus = func(ctx context.Context, app *App, key repoDiffWatchKey, repoName string, prNumber int, prBranch string) (repoDiffPrStatusResult, error) {
	result, err := app.service.GetPullRequestStatus(ctx, worksetapi.PullRequestStatusInput{
		Workspace: worksetapi.WorkspaceSelector{Value: key.workspaceID},
		Repo:      repoName,
		Number:    prNumber,
		Branch:    prBranch,
	})
	if err != nil {
		return repoDiffPrStatusResult{}, err
	}
	return repoDiffPrStatusResult{
		pullRequest: result.PullRequest,
		checks:      result.Checks,
	}, nil
}

var repoDiffListRemotes = func(ctx context.Context, app *App, key repoDiffWatchKey, repoName string) ([]worksetapi.RemoteInfoJSON, error) {
	result, err := app.service.ListRemotes(ctx, worksetapi.ListRemotesInput{
		Workspace: worksetapi.WorkspaceSelector{Value: key.workspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return nil, err
	}
	return result.Remotes, nil
}

var repoDiffCollectBranchSummary = func(ctx context.Context, repoPath, base, head string) (RepoDiffSummary, error) {
	files, err := collectBranchDiffSummary(ctx, repoPath, base, head)
	if err != nil {
		return RepoDiffSummary{}, err
	}
	summary := RepoDiffSummary{Files: files}
	for _, file := range files {
		summary.TotalAdded += file.Added
		summary.TotalRemoved += file.Removed
	}
	return summary, nil
}

var repoDiffGetPrReviews = func(ctx context.Context, app *App, key repoDiffWatchKey, repoName string, prNumber int, prBranch string) ([]worksetapi.PullRequestReviewCommentJSON, error) {
	result, err := app.service.ListPullRequestReviewComments(ctx, worksetapi.PullRequestReviewsInput{
		Workspace: worksetapi.WorkspaceSelector{Value: key.workspaceID},
		Repo:      repoName,
		Number:    prNumber,
		Branch:    prBranch,
	})
	if err != nil {
		return nil, err
	}
	return result.Comments, nil
}

type RepoDiffWatchRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	PrNumber    int    `json:"prNumber,omitempty"`
	PrBranch    string `json:"prBranch,omitempty"`
	LocalOnly   bool   `json:"localOnly,omitempty"`
}

type RepoDiffSummaryEvent struct {
	WorkspaceID string          `json:"workspaceId"`
	RepoID      string          `json:"repoId"`
	Summary     RepoDiffSummary `json:"summary"`
}

type RepoDiffLocalStatusEvent struct {
	WorkspaceID string                         `json:"workspaceId"`
	RepoID      string                         `json:"repoId"`
	Status      worksetapi.RepoLocalStatusJSON `json:"status"`
}

type RepoDiffPrStatusEvent struct {
	WorkspaceID string                   `json:"workspaceId"`
	RepoID      string                   `json:"repoId"`
	Status      PullRequestStatusPayload `json:"status"`
}

type RepoDiffPrReviewsEvent struct {
	WorkspaceID string                                    `json:"workspaceId"`
	RepoID      string                                    `json:"repoId"`
	Comments    []worksetapi.PullRequestReviewCommentJSON `json:"comments"`
}

type repoDiffWatchKey struct {
	workspaceID string
	repoID      string
}

type repoDiffWatchManager struct {
	mu      sync.Mutex
	watches map[repoDiffWatchKey]*repoDiffWatch
}

func newRepoDiffWatchManager() *repoDiffWatchManager {
	return &repoDiffWatchManager{watches: map[repoDiffWatchKey]*repoDiffWatch{}}
}

func (m *repoDiffWatchManager) start(ctx context.Context, app *App, input RepoDiffWatchRequest) (bool, error) {
	if input.WorkspaceID == "" || input.RepoID == "" {
		return false, errors.New("workspace and repo are required")
	}

	key := repoDiffWatchKey{workspaceID: input.WorkspaceID, repoID: input.RepoID}

	m.mu.Lock()
	existing := m.watches[key]
	if existing != nil {
		existing.increment(input.LocalOnly)
		if !input.LocalOnly {
			existing.updatePrInfo(input.PrNumber, input.PrBranch)
		}
		m.mu.Unlock()
		return false, nil
	}
	m.mu.Unlock()

	repoPath, err := app.resolveRepoPath(ctx, input.WorkspaceID, input.RepoID)
	if err != nil {
		return false, err
	}
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return false, err
	}

	watchCtx, cancel := context.WithCancel(ctx)
	watch := newRepoDiffWatch(app, watchCtx, cancel, key, repoName, repoPath, input.LocalOnly)
	if !input.LocalOnly {
		watch.updatePrInfo(input.PrNumber, input.PrBranch)
	}

	m.mu.Lock()
	m.watches[key] = watch
	m.mu.Unlock()

	go watch.run()

	return true, nil
}

func (m *repoDiffWatchManager) update(input RepoDiffWatchRequest) bool {
	if input.WorkspaceID == "" || input.RepoID == "" {
		return false
	}
	key := repoDiffWatchKey{workspaceID: input.WorkspaceID, repoID: input.RepoID}
	m.mu.Lock()
	defer m.mu.Unlock()
	existing := m.watches[key]
	if existing == nil {
		return false
	}
	if !input.LocalOnly {
		existing.updatePrInfo(input.PrNumber, input.PrBranch)
	}
	return true
}

func (m *repoDiffWatchManager) stop(input RepoDiffWatchRequest) bool {
	if input.WorkspaceID == "" || input.RepoID == "" {
		return false
	}
	key := repoDiffWatchKey{workspaceID: input.WorkspaceID, repoID: input.RepoID}
	m.mu.Lock()
	defer m.mu.Unlock()
	existing := m.watches[key]
	if existing == nil {
		return false
	}
	if existing.decrement(input.LocalOnly) > 0 {
		return false
	}
	delete(m.watches, key)
	existing.stop()
	return true
}

func (m *repoDiffWatchManager) shutdown() {
	m.mu.Lock()
	watches := make([]*repoDiffWatch, 0, len(m.watches))
	for _, watch := range m.watches {
		watches = append(watches, watch)
	}
	m.watches = map[repoDiffWatchKey]*repoDiffWatch{}
	m.mu.Unlock()
	for _, watch := range watches {
		watch.stop()
	}
}

type repoDiffWatch struct {
	app      *App
	ctx      context.Context
	cancel   context.CancelFunc
	key      repoDiffWatchKey
	repoName string
	repoPath string

	refMu    sync.Mutex
	refs     int
	fullRefs int

	prMu     sync.Mutex
	prNumber int
	prBranch string

	refreshMu    sync.Mutex
	refreshTimer *time.Timer

	localRefreshCh chan struct{}
	prRefreshCh    chan struct{}

	hashMu               sync.Mutex
	lastSummaryHash      string
	lastLocalSummaryHash string
	lastLocalStatusHash  string
	lastPrStatusHash     string
	lastPrReviewsHash    string

	lastPrStatus *worksetapi.PullRequestStatusJSON
	remotes      []worksetapi.RemoteInfoJSON
}

func newRepoDiffWatch(app *App, ctx context.Context, cancel context.CancelFunc, key repoDiffWatchKey, repoName, repoPath string, localOnly bool) *repoDiffWatch {
	fullRefs := 1
	if localOnly {
		fullRefs = 0
	}
	return &repoDiffWatch{
		app:            app,
		ctx:            ctx,
		cancel:         cancel,
		key:            key,
		repoName:       repoName,
		repoPath:       repoPath,
		refs:           1,
		fullRefs:       fullRefs,
		localRefreshCh: make(chan struct{}, 1),
		prRefreshCh:    make(chan struct{}, 1),
	}
}

func (w *repoDiffWatch) increment(localOnly bool) {
	w.refMu.Lock()
	w.refs++
	if !localOnly {
		w.fullRefs++
	}
	w.refMu.Unlock()
}

func (w *repoDiffWatch) decrement(localOnly bool) int {
	var fullRefs int
	w.refMu.Lock()
	w.refs--
	if !localOnly && w.fullRefs > 0 {
		w.fullRefs--
	}
	refs := w.refs
	fullRefs = w.fullRefs
	w.refMu.Unlock()
	if fullRefs == 0 {
		w.clearPrInfo()
	}
	return refs
}

func (w *repoDiffWatch) hasFullWatch() bool {
	w.refMu.Lock()
	defer w.refMu.Unlock()
	return w.fullRefs > 0
}

func (w *repoDiffWatch) clearPrInfo() {
	w.prMu.Lock()
	w.prNumber = 0
	w.prBranch = ""
	w.lastPrStatus = nil
	w.prMu.Unlock()
}

func (w *repoDiffWatch) stop() {
	w.cancel()
}

func (w *repoDiffWatch) updatePrInfo(number int, branch string) {
	if !w.hasFullWatch() {
		return
	}
	w.prMu.Lock()
	w.prNumber = number
	w.prBranch = strings.TrimSpace(branch)
	if w.prNumber == 0 && w.prBranch == "" {
		w.lastPrStatus = nil
	}
	w.prMu.Unlock()
	w.enqueuePrRefresh()
}

func (w *repoDiffWatch) run() {
	w.enqueueLocalRefresh()
	w.enqueuePrRefresh()

	go w.localRefreshLoop()
	go w.prRefreshLoop()
	go w.localPollLoop()
	go w.prPollLoop()

	watch, err := fsnotify.NewWatcher()
	if err == nil {
		_ = w.addWatchRecursive(watch, w.repoPath)
		go w.fsnotifyLoop(watch)
	}

	<-w.ctx.Done()
	if watch != nil {
		_ = watch.Close()
	}
}

func (w *repoDiffWatch) localPollLoop() {
	ticker := time.NewTicker(repoDiffLocalPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.enqueueLocalRefresh()
		}
	}
}

func (w *repoDiffWatch) prPollLoop() {
	ticker := time.NewTicker(repoDiffPrPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.enqueuePrRefresh()
		}
	}
}

func (w *repoDiffWatch) fsnotifyLoop(watcher *fsnotify.Watcher) {
	for {
		select {
		case <-w.ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if shouldIgnorePath(event.Name) {
				continue
			}
			if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = w.addWatchRecursive(watcher, event.Name)
				}
			}
			w.scheduleLocalRefresh()
		case <-watcher.Errors:
			// ignore watcher errors; fallback polling keeps state fresh
		}
	}
}

func (w *repoDiffWatch) scheduleLocalRefresh() {
	w.refreshMu.Lock()
	defer w.refreshMu.Unlock()
	if w.refreshTimer == nil {
		w.refreshTimer = time.AfterFunc(repoDiffDebounceWindow, w.enqueueLocalRefresh)
		return
	}
	w.refreshTimer.Reset(repoDiffDebounceWindow)
}

func (w *repoDiffWatch) enqueueLocalRefresh() {
	select {
	case w.localRefreshCh <- struct{}{}:
	default:
	}
}

func (w *repoDiffWatch) enqueuePrRefresh() {
	if !w.hasFullWatch() {
		return
	}
	select {
	case w.prRefreshCh <- struct{}{}:
	default:
	}
}

func (w *repoDiffWatch) localRefreshLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.localRefreshCh:
			w.refreshLocal()
		}
	}
}

func (w *repoDiffWatch) prRefreshLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.prRefreshCh:
			w.refreshPr()
		}
	}
}

func (w *repoDiffWatch) refreshLocal() {
	status, err := repoDiffGetLocalStatus(w.ctx, w.app, w.key, w.repoName)
	if err != nil {
		return
	}
	w.emitLocalStatus(status)

	if !w.hasFullWatch() {
		return
	}

	summary := RepoDiffSummary{Files: []RepoDiffFile{}}
	if status.HasUncommitted {
		updated, err := repoDiffCollectLocalSummary(w.ctx, w.repoPath)
		if err != nil {
			return
		}
		summary = updated
		w.emitLocalSummary(summary)
	}

	if w.hasActivePr() {
		return
	}
	w.emitSummary(summary)
}

func (w *repoDiffWatch) refreshPr() {
	if !w.hasFullWatch() {
		return
	}
	prNumber, prBranch := w.currentPrInfo()
	if prNumber == 0 && prBranch == "" {
		tracked, found, err := repoDiffGetTrackedPR(w.ctx, w.app, w.key, w.repoName)
		if err != nil || !found {
			return
		}
		prNumber = tracked.Number
		if prBranch == "" {
			prBranch = tracked.HeadBranch
		}
	}

	result, err := repoDiffGetPrStatus(w.ctx, w.app, w.key, w.repoName, prNumber, prBranch)
	if err != nil {
		return
	}

	statusPayload := PullRequestStatusPayload{
		PullRequest: result.pullRequest,
		Checks:      result.checks,
	}
	w.emitPrStatus(statusPayload)

	w.prMu.Lock()
	w.lastPrStatus = &result.pullRequest
	w.prMu.Unlock()

	if w.remotes == nil {
		remotes, err := repoDiffListRemotes(w.ctx, w.app, w.key, w.repoName)
		if err == nil {
			w.remotes = remotes
		}
	}

	baseRef, headRef := resolveBranchRefs(w.remotes, result.pullRequest)
	if baseRef != "" && headRef != "" {
		summary, err := repoDiffCollectBranchSummary(w.ctx, w.repoPath, baseRef, headRef)
		if err == nil {
			w.emitSummary(summary)
		}
	}

	reviews, err := repoDiffGetPrReviews(w.ctx, w.app, w.key, w.repoName, prNumber, prBranch)
	if err != nil {
		return
	}
	w.emitPrReviews(reviews)
}

func (w *repoDiffWatch) currentPrInfo() (int, string) {
	w.prMu.Lock()
	defer w.prMu.Unlock()
	return w.prNumber, w.prBranch
}

func (w *repoDiffWatch) hasActivePr() bool {
	w.prMu.Lock()
	defer w.prMu.Unlock()
	return w.lastPrStatus != nil
}

func (w *repoDiffWatch) emitSummary(summary RepoDiffSummary) {
	ensureSummaryFiles(&summary)
	if !w.shouldEmit(&w.lastSummaryHash, summary) {
		return
	}
	repoDiffEmit(w.app.ctx, "repodiff:summary", RepoDiffSummaryEvent{
		WorkspaceID: w.key.workspaceID,
		RepoID:      w.key.repoID,
		Summary:     summary,
	})
}

func (w *repoDiffWatch) emitLocalSummary(summary RepoDiffSummary) {
	ensureSummaryFiles(&summary)
	if !w.shouldEmit(&w.lastLocalSummaryHash, summary) {
		return
	}
	repoDiffEmit(w.app.ctx, "repodiff:local-summary", RepoDiffSummaryEvent{
		WorkspaceID: w.key.workspaceID,
		RepoID:      w.key.repoID,
		Summary:     summary,
	})
}

func (w *repoDiffWatch) emitLocalStatus(status worksetapi.RepoLocalStatusJSON) {
	if !w.shouldEmit(&w.lastLocalStatusHash, status) {
		return
	}
	repoDiffEmit(w.app.ctx, "repodiff:local-status", RepoDiffLocalStatusEvent{
		WorkspaceID: w.key.workspaceID,
		RepoID:      w.key.repoID,
		Status:      status,
	})
}

func (w *repoDiffWatch) emitPrStatus(status PullRequestStatusPayload) {
	if !w.shouldEmit(&w.lastPrStatusHash, status) {
		return
	}
	repoDiffEmit(w.app.ctx, "repodiff:pr-status", RepoDiffPrStatusEvent{
		WorkspaceID: w.key.workspaceID,
		RepoID:      w.key.repoID,
		Status:      status,
	})
}

func (w *repoDiffWatch) emitPrReviews(comments []worksetapi.PullRequestReviewCommentJSON) {
	if !w.shouldEmit(&w.lastPrReviewsHash, comments) {
		return
	}
	repoDiffEmit(w.app.ctx, "repodiff:pr-reviews", RepoDiffPrReviewsEvent{
		WorkspaceID: w.key.workspaceID,
		RepoID:      w.key.repoID,
		Comments:    comments,
	})
}

func (w *repoDiffWatch) shouldEmit(lastHash *string, payload any) bool {
	hash := hashPayload(payload)
	w.hashMu.Lock()
	defer w.hashMu.Unlock()
	if hash == *lastHash {
		return false
	}
	*lastHash = hash
	return true
}

func hashPayload(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ensureSummaryFiles(summary *RepoDiffSummary) {
	if summary != nil && summary.Files == nil {
		summary.Files = []RepoDiffFile{}
	}
}

func shouldIgnorePath(path string) bool {
	if path == "" {
		return true
	}
	base := filepath.Base(path)
	switch base {
	case ".git", ".workset", "node_modules":
		return true
	default:
		return false
	}
}

func (w *repoDiffWatch) addWatchRecursive(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if shouldIgnorePath(path) {
				return filepath.SkipDir
			}
			_ = watcher.Add(path)
		}
		return nil
	})
}

func resolveBranchRefs(remotes []worksetapi.RemoteInfoJSON, pr worksetapi.PullRequestStatusJSON) (string, string) {
	base := resolveRemoteRef(remotes, pr.BaseRepo, pr.BaseBranch)
	head := resolveRemoteRef(remotes, pr.HeadRepo, pr.HeadBranch)
	return base, head
}

func resolveRemoteRef(remotes []worksetapi.RemoteInfoJSON, repoFullName, branch string) string {
	repoFullName = strings.TrimSpace(repoFullName)
	branch = strings.TrimSpace(branch)
	if repoFullName == "" || branch == "" {
		return ""
	}
	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return branch
	}
	owner := parts[0]
	repo := parts[1]
	for _, remote := range remotes {
		if remote.Owner == owner && remote.Repo == repo {
			return remote.Name + "/" + branch
		}
	}
	return branch
}

func (a *App) StartRepoDiffWatch(input RepoDiffWatchRequest) (bool, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	if a.repoDiffWatchers == nil {
		a.repoDiffWatchers = newRepoDiffWatchManager()
	}
	return a.repoDiffWatchers.start(ctx, a, input)
}

func (a *App) UpdateRepoDiffWatch(input RepoDiffWatchRequest) (bool, error) {
	if a.repoDiffWatchers == nil {
		return false, nil
	}
	return a.repoDiffWatchers.update(input), nil
}

func (a *App) StopRepoDiffWatch(input RepoDiffWatchRequest) (bool, error) {
	if a.repoDiffWatchers == nil {
		return false, nil
	}
	return a.repoDiffWatchers.stop(input), nil
}
