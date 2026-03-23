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
)

const (
	repoDiffLocalPollInterval = 2 * time.Second
	repoDiffPrPollInterval    = 30 * time.Second
	repoDiffDebounceWindow    = 400 * time.Millisecond
)

var (
	repoDiffEmit            = emitRuntimeEvent
	repoDiffResolveRepoPath = func(ctx context.Context, app *App, workspaceID, repoID string) (string, error) {
		return app.resolveRepoPath(ctx, workspaceID, repoID)
	}
	repoDiffNewWatcher             = fsnotify.NewWatcher
	repoDiffResolveGitWatchTargets = resolveRepoDiffGitWatchTargets
)

var repoDiffResolveRepoAlias = func(workspaceID, repoID string) (string, error) {
	return resolveRepoAlias(workspaceID, repoID)
}

var repoDiffRunWatch = func(w *repoDiffWatch) {
	w.run()
}

type repoDiffPrStatusResult struct {
	pullRequest worksetapi.PullRequestStatusJSON
	checks      []worksetapi.PullRequestCheckJSON
}

type repoDiffGitWatchTargets struct {
	adminDir   string
	exactFiles map[string]struct{}
}

func (t repoDiffGitWatchTargets) matches(path string) bool {
	cleanPath := filepath.Clean(path)
	if _, ok := t.exactFiles[cleanPath]; ok {
		return true
	}
	return filepath.Dir(cleanPath) == t.adminDir
}

var repoDiffGetLocalStatus = func(ctx context.Context, _ *App, _ repoDiffWatchKey, _ string, repoPath string) (repoLocalStatusSnapshot, error) {
	return loadRepoLocalStatus(ctx, repoPath, "")
}

var repoDiffCollectLocalSummary = func(
	ctx context.Context,
	app *App,
	_ repoDiffWatchKey,
	_ string,
	repoPath string,
	summarySignature string,
) (RepoDiffSummary, error) {
	if app != nil {
		app.repoDiffSummaryMu.Lock()
		cached, ok := app.repoDiffSummaries[repoPath]
		app.repoDiffSummaryMu.Unlock()
		if ok && cached.signature == summarySignature {
			return cached.summary, nil
		}
	}
	files, err := collectRepoDiffSummary(ctx, repoPath)
	if err != nil {
		return RepoDiffSummary{}, err
	}
	summary := RepoDiffSummary{Files: files}
	for _, file := range files {
		summary.TotalAdded += file.Added
		summary.TotalRemoved += file.Removed
	}
	if app != nil {
		app.repoDiffSummaryMu.Lock()
		app.repoDiffSummaries[repoPath] = repoDiffSummaryCacheEntry{
			signature: summarySignature,
			summary:   summary,
		}
		app.repoDiffSummaryMu.Unlock()
	}
	return summary, nil
}

var repoDiffGetTrackedPR = func(ctx context.Context, app *App, key repoDiffWatchKey, repoName string) (worksetapi.PullRequestCreatedJSON, bool, error) {
	svc := app.ensureService()
	result, err := svc.GetTrackedPullRequest(ctx, worksetapi.PullRequestTrackedInput{
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
	svc := app.ensureService()
	result, err := svc.GetPullRequestStatus(ctx, worksetapi.PullRequestStatusInput{
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
	svc := app.ensureService()
	result, err := svc.ListRemotes(ctx, worksetapi.ListRemotesInput{
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
	svc := app.ensureService()
	result, err := svc.ListPullRequestReviewComments(ctx, worksetapi.PullRequestReviewsInput{
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

type repoDiffSubscription struct {
	key      repoDiffWatchKey
	repoName string

	mu       sync.Mutex
	refs     int
	fullRefs int

	prNumber int
	prBranch string

	lastSummaryHash      string
	lastLocalSummaryHash string
	lastLocalStatusHash  string
	lastPrStatusHash     string
	lastPrReviewsHash    string
	lastPrStatus         *worksetapi.PullRequestStatusJSON
}

func newRepoDiffSubscription(key repoDiffWatchKey, repoName string, localOnly bool) *repoDiffSubscription {
	fullRefs := 1
	if localOnly {
		fullRefs = 0
	}
	return &repoDiffSubscription{
		key:      key,
		repoName: repoName,
		refs:     1,
		fullRefs: fullRefs,
	}
}

func (s *repoDiffSubscription) increment(localOnly bool) {
	s.mu.Lock()
	s.refs++
	if !localOnly {
		s.fullRefs++
	}
	s.mu.Unlock()
}

func (s *repoDiffSubscription) decrement(localOnly bool) int {
	s.mu.Lock()
	s.refs--
	if !localOnly && s.fullRefs > 0 {
		s.fullRefs--
	}
	if s.fullRefs == 0 {
		s.prNumber = 0
		s.prBranch = ""
		s.lastPrStatus = nil
	}
	refs := s.refs
	s.mu.Unlock()
	return refs
}

func (s *repoDiffSubscription) hasFullWatch() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fullRefs > 0
}

func (s *repoDiffSubscription) updatePrInfo(number int, branch string) bool {
	trimmedBranch := strings.TrimSpace(branch)
	if number == 0 && trimmedBranch == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fullRefs == 0 {
		return false
	}
	s.prNumber = number
	s.prBranch = trimmedBranch
	return true
}

func (s *repoDiffSubscription) currentPrInfo() (int, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prNumber, s.prBranch
}

func (s *repoDiffSubscription) hasActivePr() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastPrStatus != nil
}

func (s *repoDiffSubscription) setLastPrStatus(status *worksetapi.PullRequestStatusJSON) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status == nil {
		s.lastPrStatus = nil
		return
	}
	cloned := *status
	s.lastPrStatus = &cloned
}

func (s *repoDiffSubscription) repo() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repoName
}

func (s *repoDiffSubscription) setRepoName(repoName string) {
	if repoName == "" {
		return
	}
	s.mu.Lock()
	s.repoName = repoName
	s.mu.Unlock()
}

func (s *repoDiffSubscription) emitSummary(ctx context.Context, summary RepoDiffSummary) {
	ensureSummaryFiles(&summary)
	hash := hashPayload(summary)
	s.mu.Lock()
	if s.refs == 0 || hash == s.lastSummaryHash {
		s.mu.Unlock()
		return
	}
	s.lastSummaryHash = hash
	key := s.key
	s.mu.Unlock()
	repoDiffEmit(ctx, EventRepoDiffSummary, RepoDiffSummaryEvent{
		WorkspaceID: key.workspaceID,
		RepoID:      key.repoID,
		Summary:     summary,
	})
}

func (s *repoDiffSubscription) emitLocalSummary(ctx context.Context, summary RepoDiffSummary) {
	ensureSummaryFiles(&summary)
	hash := hashPayload(summary)
	s.mu.Lock()
	if s.refs == 0 || hash == s.lastLocalSummaryHash {
		s.mu.Unlock()
		return
	}
	s.lastLocalSummaryHash = hash
	key := s.key
	s.mu.Unlock()
	repoDiffEmit(ctx, EventRepoDiffLocalSummary, RepoDiffSummaryEvent{
		WorkspaceID: key.workspaceID,
		RepoID:      key.repoID,
		Summary:     summary,
	})
}

func (s *repoDiffSubscription) emitLocalStatus(ctx context.Context, status worksetapi.RepoLocalStatusJSON) {
	hash := hashPayload(status)
	s.mu.Lock()
	if s.refs == 0 || hash == s.lastLocalStatusHash {
		s.mu.Unlock()
		return
	}
	s.lastLocalStatusHash = hash
	key := s.key
	s.mu.Unlock()
	repoDiffEmit(ctx, EventRepoDiffLocalStatus, RepoDiffLocalStatusEvent{
		WorkspaceID: key.workspaceID,
		RepoID:      key.repoID,
		Status:      status,
	})
}

func (s *repoDiffSubscription) emitPrStatus(ctx context.Context, status PullRequestStatusPayload) {
	hash := hashPayload(status)
	s.mu.Lock()
	if s.refs == 0 || hash == s.lastPrStatusHash {
		s.mu.Unlock()
		return
	}
	s.lastPrStatusHash = hash
	key := s.key
	s.mu.Unlock()
	repoDiffEmit(ctx, EventRepoDiffPRStatus, RepoDiffPrStatusEvent{
		WorkspaceID: key.workspaceID,
		RepoID:      key.repoID,
		Status:      status,
	})
}

func (s *repoDiffSubscription) emitPrReviews(ctx context.Context, comments []worksetapi.PullRequestReviewCommentJSON) {
	hash := hashPayload(comments)
	s.mu.Lock()
	if s.refs == 0 || hash == s.lastPrReviewsHash {
		s.mu.Unlock()
		return
	}
	s.lastPrReviewsHash = hash
	key := s.key
	s.mu.Unlock()
	repoDiffEmit(ctx, EventRepoDiffPRReviews, RepoDiffPrReviewsEvent{
		WorkspaceID: key.workspaceID,
		RepoID:      key.repoID,
		Comments:    comments,
	})
}

type repoDiffWatchManager struct {
	mu            sync.Mutex
	watches       map[string]*repoDiffWatch
	subscriptions map[repoDiffWatchKey]*repoDiffWatch
}

func newRepoDiffWatchManager() *repoDiffWatchManager {
	return &repoDiffWatchManager{
		watches:       map[string]*repoDiffWatch{},
		subscriptions: map[repoDiffWatchKey]*repoDiffWatch{},
	}
}

func (m *repoDiffWatchManager) start(ctx context.Context, app *App, input RepoDiffWatchRequest) (bool, error) {
	if input.WorkspaceID == "" || input.RepoID == "" {
		return false, errors.New("workspace and repo are required")
	}

	key := repoDiffWatchKey{workspaceID: input.WorkspaceID, repoID: input.RepoID}

	m.mu.Lock()
	existing := m.subscriptions[key]
	if existing != nil {
		existing.addSubscriber(key, "", input.LocalOnly)
		if !input.LocalOnly {
			existing.updatePrInfo(key, input.PrNumber, input.PrBranch)
		}
		m.mu.Unlock()
		existing.enqueueLocalRefresh()
		return false, nil
	}
	m.mu.Unlock()

	repoPath, err := repoDiffResolveRepoPath(ctx, app, input.WorkspaceID, input.RepoID)
	if err != nil {
		return false, err
	}
	repoName, err := repoDiffResolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return false, err
	}
	identity := filepath.Clean(repoPath)

	m.mu.Lock()
	existing = m.subscriptions[key]
	if existing != nil {
		existing.addSubscriber(key, repoName, input.LocalOnly)
		if !input.LocalOnly {
			existing.updatePrInfo(key, input.PrNumber, input.PrBranch)
		}
		m.mu.Unlock()
		existing.enqueueLocalRefresh()
		return false, nil
	}
	watch := m.watches[identity]
	started := false
	if watch == nil {
		watchCtx, cancel := context.WithCancel(ctx)
		watch = newRepoDiffWatch(app, watchCtx, cancel, key, repoName, repoPath, input.LocalOnly)
		m.watches[identity] = watch
		started = true
	} else {
		watch.addSubscriber(key, repoName, input.LocalOnly)
	}
	m.subscriptions[key] = watch
	m.mu.Unlock()

	watch.enqueueLocalRefresh()
	if !input.LocalOnly {
		watch.updatePrInfo(key, input.PrNumber, input.PrBranch)
	}
	if started {
		go repoDiffRunWatch(watch)
	}

	return started, nil
}

func (m *repoDiffWatchManager) update(input RepoDiffWatchRequest) bool {
	if input.WorkspaceID == "" || input.RepoID == "" {
		return false
	}
	key := repoDiffWatchKey{workspaceID: input.WorkspaceID, repoID: input.RepoID}
	m.mu.Lock()
	existing := m.subscriptions[key]
	m.mu.Unlock()
	if existing == nil {
		return false
	}
	if !input.LocalOnly {
		existing.updatePrInfo(key, input.PrNumber, input.PrBranch)
	}
	return true
}

func (m *repoDiffWatchManager) stop(input RepoDiffWatchRequest) bool {
	if input.WorkspaceID == "" || input.RepoID == "" {
		return false
	}
	key := repoDiffWatchKey{workspaceID: input.WorkspaceID, repoID: input.RepoID}
	m.mu.Lock()
	existing := m.subscriptions[key]
	if existing == nil {
		m.mu.Unlock()
		return false
	}
	remaining, empty := existing.removeSubscriber(key, input.LocalOnly)
	if remaining == 0 {
		delete(m.subscriptions, key)
	}
	if !empty {
		m.mu.Unlock()
		return false
	}
	delete(m.watches, existing.identity)
	m.mu.Unlock()
	existing.stop()
	return true
}

func (m *repoDiffWatchManager) shutdown() {
	m.mu.Lock()
	watches := make([]*repoDiffWatch, 0, len(m.watches))
	for _, watch := range m.watches {
		watches = append(watches, watch)
	}
	m.watches = map[string]*repoDiffWatch{}
	m.subscriptions = map[repoDiffWatchKey]*repoDiffWatch{}
	m.mu.Unlock()
	for _, watch := range watches {
		watch.stop()
	}
}

type repoDiffWatch struct {
	app      *App
	ctx      context.Context
	cancel   context.CancelFunc
	identity string
	repoPath string

	subMu       sync.Mutex
	subscribers map[repoDiffWatchKey]*repoDiffSubscription

	refreshMu    sync.Mutex
	refreshTimer *time.Timer
	watchMu      sync.Mutex
	watchedPaths map[string]struct{}

	localRefreshCh chan struct{}
	prRefreshCh    chan struct{}

	lastLocalSummarySignature string
	lastLocalSummary          RepoDiffSummary
	remotes                   []worksetapi.RemoteInfoJSON
	remotesMu                 sync.Mutex
	watchTargets              repoDiffGitWatchTargets
}

func newRepoDiffWatch(app *App, ctx context.Context, cancel context.CancelFunc, key repoDiffWatchKey, repoName, repoPath string, localOnly bool) *repoDiffWatch {
	identity := filepath.Clean(repoPath)
	watch := &repoDiffWatch{
		app:            app,
		ctx:            ctx,
		cancel:         cancel,
		identity:       identity,
		repoPath:       repoPath,
		subscribers:    map[repoDiffWatchKey]*repoDiffSubscription{},
		localRefreshCh: make(chan struct{}, 1),
		prRefreshCh:    make(chan struct{}, 1),
		watchedPaths:   map[string]struct{}{},
	}
	watch.addSubscriber(key, repoName, localOnly)
	return watch
}

func (w *repoDiffWatch) addSubscriber(key repoDiffWatchKey, repoName string, localOnly bool) {
	w.subMu.Lock()
	defer w.subMu.Unlock()
	if existing := w.subscribers[key]; existing != nil {
		existing.setRepoName(repoName)
		existing.increment(localOnly)
		return
	}
	w.subscribers[key] = newRepoDiffSubscription(key, repoName, localOnly)
}

func (w *repoDiffWatch) getSubscriber(key repoDiffWatchKey) *repoDiffSubscription {
	w.subMu.Lock()
	defer w.subMu.Unlock()
	return w.subscribers[key]
}

func (w *repoDiffWatch) removeSubscriber(key repoDiffWatchKey, localOnly bool) (int, bool) {
	w.subMu.Lock()
	defer w.subMu.Unlock()
	existing := w.subscribers[key]
	if existing == nil {
		return 0, len(w.subscribers) == 0
	}
	remaining := existing.decrement(localOnly)
	if remaining <= 0 {
		delete(w.subscribers, key)
		remaining = 0
	}
	return remaining, len(w.subscribers) == 0
}

func (w *repoDiffWatch) subscribersSnapshot() []*repoDiffSubscription {
	w.subMu.Lock()
	defer w.subMu.Unlock()
	subscribers := make([]*repoDiffSubscription, 0, len(w.subscribers))
	for _, subscriber := range w.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	return subscribers
}

func (w *repoDiffWatch) fullSubscribers() []*repoDiffSubscription {
	subscribers := w.subscribersSnapshot()
	full := make([]*repoDiffSubscription, 0, len(subscribers))
	for _, subscriber := range subscribers {
		if subscriber.hasFullWatch() {
			full = append(full, subscriber)
		}
	}
	return full
}

func (w *repoDiffWatch) hasFullWatch() bool {
	return len(w.fullSubscribers()) > 0
}

func (w *repoDiffWatch) stop() {
	w.cancel()
	w.stopRefreshTimer()
}

func (w *repoDiffWatch) updatePrInfo(key repoDiffWatchKey, number int, branch string) {
	subscriber := w.getSubscriber(key)
	if subscriber == nil {
		return
	}
	if !subscriber.updatePrInfo(number, branch) {
		return
	}
	w.enqueuePrRefresh()
}

func (w *repoDiffWatch) run() {
	w.enqueueLocalRefresh()
	w.enqueuePrRefresh()

	go w.localRefreshLoop()
	go w.prRefreshLoop()
	go w.localPollLoop()
	go w.prPollLoop()

	var watch *fsnotify.Watcher
	createdWatch, err := repoDiffNewWatcher()
	if err == nil {
		targets, resolveErr := repoDiffResolveGitWatchTargets(w.ctx, w.repoPath)
		if resolveErr == nil {
			watch = createdWatch
			w.watchTargets = targets
			if w.addGitWatchTargets(watch, targets) == nil {
				go w.fsnotifyLoop(watch, targets)
			} else {
				_ = watch.Close()
				watch = nil
			}
		} else {
			_ = createdWatch.Close()
		}
	}

	<-w.ctx.Done()
	w.stopRefreshTimer()
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

func (w *repoDiffWatch) fsnotifyLoop(watcher *fsnotify.Watcher, targets repoDiffGitWatchTargets) {
	for {
		select {
		case <-w.ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			w.handleFsnotifyEvent(watcher, targets, event)
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
			// ignore watcher errors; fallback polling keeps state fresh
		}
	}
}

func (w *repoDiffWatch) scheduleLocalRefresh() {
	w.refreshMu.Lock()
	defer w.refreshMu.Unlock()
	if w.refreshTimer == nil {
		w.refreshTimer = time.AfterFunc(repoDiffDebounceWindow, func() {
			w.enqueueLocalRefresh()
		})
		return
	}
	w.refreshTimer.Reset(repoDiffDebounceWindow)
}

func (w *repoDiffWatch) handleFsnotifyEvent(watcher *fsnotify.Watcher, targets repoDiffGitWatchTargets, event fsnotify.Event) {
	if !targets.matches(event.Name) {
		return
	}
	w.scheduleLocalRefresh()
	if w.hasFullWatch() {
		w.enqueuePrRefresh()
	}
}

func (w *repoDiffWatch) enqueueLocalRefresh() {
	select {
	case <-w.ctx.Done():
		return
	default:
	}
	select {
	case w.localRefreshCh <- struct{}{}:
	default:
	}
}

func (w *repoDiffWatch) enqueuePrRefresh() {
	select {
	case <-w.ctx.Done():
		return
	default:
	}
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
	subscribers := w.subscribersSnapshot()
	if len(subscribers) == 0 {
		return
	}
	primary := subscribers[0]
	primaryRepo := primary.repo()

	statusSnapshot, err := repoDiffGetLocalStatus(w.ctx, w.app, primary.key, primaryRepo, w.repoPath)
	if err != nil {
		return
	}
	status := statusSnapshot.payload

	summary := emptyRepoDiffSummary()
	if status.HasUncommitted {
		if statusSnapshot.summarySignature == w.lastLocalSummarySignature {
			summary = w.lastLocalSummary
		} else {
			updated, err := repoDiffCollectLocalSummary(
				w.ctx,
				w.app,
				primary.key,
				primaryRepo,
				w.repoPath,
				statusSnapshot.summarySignature,
			)
			if err != nil {
				return
			}
			summary = updated
			w.lastLocalSummarySignature = statusSnapshot.summarySignature
			w.lastLocalSummary = updated
		}
	} else {
		w.lastLocalSummarySignature = ""
		w.lastLocalSummary = emptyRepoDiffSummary()
	}
	for _, subscriber := range subscribers {
		subscriber.emitLocalStatus(w.app.ctx, status)
		subscriber.emitLocalSummary(w.app.ctx, summary)
		if subscriber.hasFullWatch() && !subscriber.hasActivePr() {
			subscriber.emitSummary(w.app.ctx, summary)
		}
	}
}

func (w *repoDiffWatch) refreshPr() {
	fullSubscribers := w.fullSubscribers()
	if len(fullSubscribers) == 0 {
		return
	}
	for _, subscriber := range fullSubscribers {
		repoName := subscriber.repo()
		prNumber, prBranch := subscriber.currentPrInfo()
		if prNumber == 0 && prBranch == "" {
			tracked, found, err := repoDiffGetTrackedPR(w.ctx, w.app, subscriber.key, repoName)
			if err != nil || !found {
				continue
			}
			prNumber = tracked.Number
			if prBranch == "" {
				prBranch = tracked.HeadBranch
			}
		}

		result, err := repoDiffGetPrStatus(w.ctx, w.app, subscriber.key, repoName, prNumber, prBranch)
		if err != nil {
			continue
		}

		statusPayload := PullRequestStatusPayload{
			PullRequest: result.pullRequest,
			Checks:      result.checks,
		}
		subscriber.emitPrStatus(w.app.ctx, statusPayload)
		subscriber.setLastPrStatus(&result.pullRequest)

		remotes := w.loadRemotes(subscriber)
		baseRef, headRef := resolveBranchRefs(remotes, result.pullRequest)
		if baseRef != "" && headRef != "" {
			summary, err := repoDiffCollectBranchSummary(w.ctx, w.repoPath, baseRef, headRef)
			if err == nil {
				subscriber.emitSummary(w.app.ctx, summary)
			}
		}

		reviews, err := repoDiffGetPrReviews(w.ctx, w.app, subscriber.key, repoName, prNumber, prBranch)
		if err != nil {
			continue
		}
		subscriber.emitPrReviews(w.app.ctx, reviews)
	}
}

func (w *repoDiffWatch) loadRemotes(subscriber *repoDiffSubscription) []worksetapi.RemoteInfoJSON {
	w.remotesMu.Lock()
	if w.remotes != nil {
		remotes := w.remotes
		w.remotesMu.Unlock()
		return remotes
	}
	w.remotesMu.Unlock()

	remotes, err := repoDiffListRemotes(w.ctx, w.app, subscriber.key, subscriber.repo())
	if err != nil {
		return nil
	}

	w.remotesMu.Lock()
	if w.remotes == nil {
		w.remotes = remotes
	}
	cached := w.remotes
	w.remotesMu.Unlock()
	return cached
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

func (w *repoDiffWatch) addGitWatchTargets(watcher *fsnotify.Watcher, targets repoDiffGitWatchTargets) error {
	if targets.adminDir == "" {
		return errors.New("git admin dir required")
	}
	if err := w.addWatchPath(watcher, targets.adminDir); err != nil {
		return err
	}

	for path := range targets.exactFiles {
		if err := w.addWatchPath(watcher, path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (w *repoDiffWatch) addWatchPath(watcher *fsnotify.Watcher, path string) error {
	cleanPath := filepath.Clean(path)
	w.watchMu.Lock()
	if w.watchedPaths == nil {
		w.watchedPaths = map[string]struct{}{}
	}
	if _, exists := w.watchedPaths[cleanPath]; exists {
		w.watchMu.Unlock()
		return nil
	}
	if err := watcher.Add(cleanPath); err != nil {
		w.watchMu.Unlock()
		return err
	}
	w.watchedPaths[cleanPath] = struct{}{}
	w.watchMu.Unlock()
	return nil
}

func (w *repoDiffWatch) stopRefreshTimer() {
	w.refreshMu.Lock()
	if w.refreshTimer != nil {
		w.refreshTimer.Stop()
		w.refreshTimer = nil
	}
	w.refreshMu.Unlock()
}

func resolveRepoDiffGitWatchTargets(ctx context.Context, repoPath string) (repoDiffGitWatchTargets, error) {
	adminDir, err := repoDiffRevParsePath(ctx, repoPath, "--git-dir")
	if err != nil {
		return repoDiffGitWatchTargets{}, err
	}
	exactFiles := map[string]struct{}{}
	for _, spec := range []string{"HEAD", "index", "packed-refs", "FETCH_HEAD"} {
		resolved, resolveErr := repoDiffRevParsePath(ctx, repoPath, "--git-path", spec)
		if resolveErr != nil {
			return repoDiffGitWatchTargets{}, resolveErr
		}
		exactFiles[resolved] = struct{}{}
	}

	return repoDiffGitWatchTargets{
		adminDir:   adminDir,
		exactFiles: exactFiles,
	}, nil
}

func repoDiffRevParsePath(ctx context.Context, repoPath string, args ...string) (string, error) {
	cmdArgs := []string{"-C", repoPath, "rev-parse"}
	cmdArgs = append(cmdArgs, args...)
	cmd := newGitCommandContext(ctx, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.New(strings.TrimSpace(string(output)))
	}
	resolved := strings.TrimSpace(string(output))
	if resolved == "" {
		return "", errors.New("git rev-parse returned empty path")
	}
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(repoPath, resolved)
	}
	return filepath.Clean(resolved), nil
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
