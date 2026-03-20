package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
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
	refsDir    string
	exactFiles map[string]struct{}
}

func (t repoDiffGitWatchTargets) matches(path string) bool {
	cleanPath := filepath.Clean(path)
	if _, ok := t.exactFiles[cleanPath]; ok {
		return true
	}
	if t.refsDir == "" {
		return false
	}
	return pathWithin(cleanPath, t.refsDir)
}

func (t repoDiffGitWatchTargets) refsParentDir() string {
	if t.refsDir == "" {
		return ""
	}
	return filepath.Dir(t.refsDir)
}

var repoDiffGetLocalStatus = func(ctx context.Context, app *App, key repoDiffWatchKey, repoName string) (worksetapi.RepoLocalStatusJSON, error) {
	svc := app.ensureService()
	result, err := svc.GetRepoLocalStatus(ctx, worksetapi.RepoLocalStatusInput{
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

	repoPath, err := repoDiffResolveRepoPath(ctx, app, input.WorkspaceID, input.RepoID)
	if err != nil {
		return false, err
	}
	repoName, err := repoDiffResolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return false, err
	}

	watchCtx, cancel := context.WithCancel(ctx)
	watch := newRepoDiffWatch(app, watchCtx, cancel, key, repoName, repoPath, input.LocalOnly)
	if !input.LocalOnly {
		watch.updatePrInfo(input.PrNumber, input.PrBranch)
	}

	m.mu.Lock()
	existing = m.watches[key]
	if existing != nil {
		existing.increment(input.LocalOnly)
		if !input.LocalOnly {
			existing.updatePrInfo(input.PrNumber, input.PrBranch)
		}
		m.mu.Unlock()
		watch.stop()
		return false, nil
	}
	m.watches[key] = watch
	m.mu.Unlock()

	go repoDiffRunWatch(watch)

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
	watchMu      sync.Mutex
	watchedPaths map[string]struct{}

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
	watchTargets repoDiffGitWatchTargets
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
		watchedPaths:   map[string]struct{}{},
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
	w.stopRefreshTimer()
}

func (w *repoDiffWatch) updatePrInfo(number int, branch string) {
	if !w.hasFullWatch() {
		return
	}
	trimmedBranch := strings.TrimSpace(branch)
	if number == 0 && trimmedBranch == "" {
		return
	}
	w.prMu.Lock()
	w.prNumber = number
	w.prBranch = trimmedBranch
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
	if event.Op&fsnotify.Create == fsnotify.Create {
		if w.shouldAddRefsWatch(event.Name, targets) {
			if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
				_ = w.addWatchRecursive(watcher, event.Name)
			}
		}
	}
	if !targets.matches(event.Name) {
		return
	}
	w.scheduleLocalRefresh()
	if w.hasFullWatch() {
		w.enqueuePrRefresh()
	}
}

func (w *repoDiffWatch) shouldAddRefsWatch(path string, targets repoDiffGitWatchTargets) bool {
	if targets.refsDir == "" {
		return false
	}
	cleanPath := filepath.Clean(path)
	if cleanPath == targets.refsDir {
		return true
	}
	return pathWithin(cleanPath, targets.refsDir)
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
	status, err := repoDiffGetLocalStatus(w.ctx, w.app, w.key, w.repoName)
	if err != nil {
		return
	}
	w.emitLocalStatus(status)

	summary := RepoDiffSummary{Files: []RepoDiffFile{}}
	if status.HasUncommitted {
		updated, err := repoDiffCollectLocalSummary(w.ctx, w.repoPath)
		if err != nil {
			return
		}
		summary = updated
	}
	w.emitLocalSummary(summary)

	if !w.hasFullWatch() {
		return
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
	repoDiffEmit(w.app.ctx, EventRepoDiffSummary, RepoDiffSummaryEvent{
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
	repoDiffEmit(w.app.ctx, EventRepoDiffLocalSummary, RepoDiffSummaryEvent{
		WorkspaceID: w.key.workspaceID,
		RepoID:      w.key.repoID,
		Summary:     summary,
	})
}

func (w *repoDiffWatch) emitLocalStatus(status worksetapi.RepoLocalStatusJSON) {
	if !w.shouldEmit(&w.lastLocalStatusHash, status) {
		return
	}
	repoDiffEmit(w.app.ctx, EventRepoDiffLocalStatus, RepoDiffLocalStatusEvent{
		WorkspaceID: w.key.workspaceID,
		RepoID:      w.key.repoID,
		Status:      status,
	})
}

func (w *repoDiffWatch) emitPrStatus(status PullRequestStatusPayload) {
	if !w.shouldEmit(&w.lastPrStatusHash, status) {
		return
	}
	repoDiffEmit(w.app.ctx, EventRepoDiffPRStatus, RepoDiffPrStatusEvent{
		WorkspaceID: w.key.workspaceID,
		RepoID:      w.key.repoID,
		Status:      status,
	})
}

func (w *repoDiffWatch) emitPrReviews(comments []worksetapi.PullRequestReviewCommentJSON) {
	if !w.shouldEmit(&w.lastPrReviewsHash, comments) {
		return
	}
	repoDiffEmit(w.app.ctx, EventRepoDiffPRReviews, RepoDiffPrReviewsEvent{
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

func (w *repoDiffWatch) addGitWatchTargets(watcher *fsnotify.Watcher, targets repoDiffGitWatchTargets) error {
	if targets.adminDir == "" {
		return errors.New("git admin dir required")
	}
	if err := w.addWatchPath(watcher, targets.adminDir); err != nil {
		return err
	}

	refsParent := targets.refsParentDir()
	if refsParent != "" {
		if err := w.addWatchPath(watcher, refsParent); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if targets.refsDir != "" {
		if err := w.addWatchRecursive(watcher, targets.refsDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (w *repoDiffWatch) addWatchRecursive(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			_ = w.addWatchPath(watcher, path)
		}
		return nil
	})
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
	refsDir, err := repoDiffRevParsePath(ctx, repoPath, "--git-path", "refs")
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
		refsDir:    refsDir,
		exactFiles: exactFiles,
	}, nil
}

func repoDiffRevParsePath(ctx context.Context, repoPath string, args ...string) (string, error) {
	cmdArgs := []string{"-C", repoPath, "rev-parse"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
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

func pathWithin(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
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
