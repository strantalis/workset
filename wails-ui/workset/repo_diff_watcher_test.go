package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/strantalis/workset/pkg/worksetapi"
)

func TestResolveRemoteRef(t *testing.T) {
	remotes := []worksetapi.RemoteInfoJSON{
		{Name: "origin", Owner: "octo", Repo: "repo"},
		{Name: "upstream", Owner: "source", Repo: "repo"},
	}

	if got := resolveRemoteRef(remotes, "octo/repo", "main"); got != "origin/main" {
		t.Fatalf("expected origin/main, got %q", got)
	}

	if got := resolveRemoteRef(remotes, "unknown/repo", "dev"); got != "dev" {
		t.Fatalf("expected fallback branch, got %q", got)
	}

	if got := resolveRemoteRef(remotes, "", "main"); got != "" {
		t.Fatalf("expected empty for missing repo, got %q", got)
	}
}

func TestResolveBranchRefs(t *testing.T) {
	remotes := []worksetapi.RemoteInfoJSON{
		{Name: "origin", Owner: "octo", Repo: "repo"},
	}

	pr := worksetapi.PullRequestStatusJSON{
		BaseRepo:   "octo/repo",
		BaseBranch: "main",
		HeadRepo:   "octo/repo",
		HeadBranch: "feature",
	}

	base, head := resolveBranchRefs(remotes, pr)
	if base != "origin/main" || head != "origin/feature" {
		t.Fatalf("unexpected refs: %q %q", base, head)
	}
}

func TestRepoDiffGitWatchTargetsMatches(t *testing.T) {
	targets := repoDiffGitWatchTargets{
		adminDir: filepath.Clean("/tmp/repo/.git"),
		exactFiles: map[string]struct{}{
			filepath.Clean("/tmp/repo/.git/HEAD"):  {},
			filepath.Clean("/tmp/repo/.git/index"): {},
		},
	}

	if !targets.matches("/tmp/repo/.git/HEAD") {
		t.Fatal("expected exact git admin file to match")
	}
	if !targets.matches("/tmp/repo/.git/index.lock") {
		t.Fatal("expected direct git admin child to match")
	}
	if targets.matches("/tmp/repo/.git/refs/heads/main") {
		t.Fatal("expected nested refs path to rely on polling")
	}
	if targets.matches("/tmp/repo/src/main.go") {
		t.Fatal("expected unrelated path to be ignored")
	}
}

func TestHashPayloadDeterminism(t *testing.T) {
	first := hashPayload(map[string]string{"a": "b"})
	second := hashPayload(map[string]string{"a": "b"})
	if first == "" || second == "" || first != second {
		t.Fatalf("expected deterministic hash, got %q and %q", first, second)
	}

	third := hashPayload(map[string]string{"a": "c"})
	if third == first {
		t.Fatalf("expected different hash for different payloads")
	}
}

func TestShouldEmitDedupes(t *testing.T) {
	origEmit := repoDiffEmit
	defer func() {
		repoDiffEmit = origEmit
	}()

	var events int
	repoDiffEmit = func(_ context.Context, _ string, _ ...interface{}) {
		events++
	}

	subscriber := newRepoDiffSubscription(repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"}, "repo", true)
	status := worksetapi.RepoLocalStatusJSON{CurrentBranch: "main"}
	subscriber.emitLocalStatus(context.Background(), status)
	subscriber.emitLocalStatus(context.Background(), status)
	subscriber.emitLocalStatus(context.Background(), worksetapi.RepoLocalStatusJSON{CurrentBranch: "feature"})

	if events != 2 {
		t.Fatalf("expected duplicate payloads to be suppressed, got %d events", events)
	}
}

func TestResolveRepoDiffGitWatchTargetsUsesGitAdminPaths(t *testing.T) {
	repo := initRepoDiffGitRepo(t)

	targets, err := resolveRepoDiffGitWatchTargets(context.Background(), repo)
	if err != nil {
		t.Fatalf("resolve targets: %v", err)
	}

	if targets.adminDir != gitRevParsePath(t, repo, "--git-dir") {
		t.Fatalf("unexpected admin dir: %q", targets.adminDir)
	}
	for _, spec := range []string{"HEAD", "index", "packed-refs", "FETCH_HEAD"} {
		resolved := gitRevParsePath(t, repo, "--git-path", spec)
		if _, ok := targets.exactFiles[resolved]; !ok {
			t.Fatalf("expected exact file %q to be tracked", resolved)
		}
	}
}

func TestResolveRepoDiffGitWatchTargetsSupportsWorktrees(t *testing.T) {
	repo := initRepoDiffGitRepo(t)
	writeRepoDiffFile(t, filepath.Join(repo, "README.md"), []byte("hello\n"))
	runRepoDiffGit(t, repo, "add", "README.md")
	runRepoDiffGit(t, repo, "commit", "-m", "feat: seed")

	worktree := filepath.Join(t.TempDir(), "wt")
	runRepoDiffGit(t, repo, "worktree", "add", "-b", "feature/watch", worktree)

	targets, err := resolveRepoDiffGitWatchTargets(context.Background(), worktree)
	if err != nil {
		t.Fatalf("resolve worktree targets: %v", err)
	}

	if targets.adminDir != gitRevParsePath(t, worktree, "--git-dir") {
		t.Fatalf("unexpected worktree admin dir: %q", targets.adminDir)
	}
}

func TestRefreshLocalEmitsEventsOnce(t *testing.T) {
	origEmit := repoDiffEmit
	origGetLocalStatus := repoDiffGetLocalStatus
	origCollectLocalSummary := repoDiffCollectLocalSummary
	defer func() {
		repoDiffEmit = origEmit
		repoDiffGetLocalStatus = origGetLocalStatus
		repoDiffCollectLocalSummary = origCollectLocalSummary
	}()

	events := []string{}
	var summaryPayload RepoDiffSummary
	repoDiffEmit = func(_ context.Context, name string, data ...interface{}) {
		events = append(events, name)
		if name == EventRepoDiffSummary && len(data) > 0 {
			if payload, ok := data[0].(RepoDiffSummaryEvent); ok {
				summaryPayload = payload.Summary
			}
		}
	}

	repoDiffGetLocalStatus = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string, _ string) (repoLocalStatusSnapshot, error) {
		return repoLocalStatusSnapshot{
			payload: worksetapi.RepoLocalStatusJSON{
				HasUncommitted: true,
				Ahead:          1,
				Behind:         0,
				CurrentBranch:  "main",
			},
			summarySignature: "dirty-1",
		}, nil
	}

	repoDiffCollectLocalSummary = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string, _ string, _ string) (RepoDiffSummary, error) {
		return RepoDiffSummary{
			Files: []RepoDiffFile{
				{Path: "file.txt", Added: 3, Removed: 1, Status: "modified"},
			},
			TotalAdded:   3,
			TotalRemoved: 1,
		}, nil
	}

	watch := &repoDiffWatch{
		app:         &App{ctx: context.Background()},
		ctx:         context.Background(),
		repoPath:    "/tmp/repo",
		subscribers: map[repoDiffWatchKey]*repoDiffSubscription{},
	}
	watch.addSubscriber(repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"}, "repo", false)

	watch.refreshLocal()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(events), events)
	}
	if events[0] != EventRepoDiffLocalStatus || events[1] != EventRepoDiffLocalSummary || events[2] != EventRepoDiffSummary {
		t.Fatalf("unexpected event order: %v", events)
	}

	watch.refreshLocal()
	if len(events) != 3 {
		t.Fatalf("expected no duplicate events, got %d", len(events))
	}
	if summaryPayload.Files == nil {
		t.Fatal("expected summary files to be non-nil")
	}
}

func TestRefreshLocalReusesSummaryForUnchangedDirtySignature(t *testing.T) {
	origGetLocalStatus := repoDiffGetLocalStatus
	origCollectLocalSummary := repoDiffCollectLocalSummary
	defer func() {
		repoDiffGetLocalStatus = origGetLocalStatus
		repoDiffCollectLocalSummary = origCollectLocalSummary
	}()

	collectCalls := 0
	repoDiffGetLocalStatus = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string, _ string) (repoLocalStatusSnapshot, error) {
		return repoLocalStatusSnapshot{
			payload: worksetapi.RepoLocalStatusJSON{
				HasUncommitted: true,
				CurrentBranch:  "main",
			},
			summarySignature: "same-dirty-state",
		}, nil
	}
	repoDiffCollectLocalSummary = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string, _ string, _ string) (RepoDiffSummary, error) {
		collectCalls++
		return RepoDiffSummary{
			Files:        []RepoDiffFile{{Path: "file.txt", Added: 1, Removed: 0, Status: "modified"}},
			TotalAdded:   1,
			TotalRemoved: 0,
		}, nil
	}

	watch := &repoDiffWatch{
		app:              &App{ctx: context.Background(), repoDiffSummaries: map[string]repoDiffSummaryCacheEntry{}},
		ctx:              context.Background(),
		repoPath:         "/tmp/repo",
		subscribers:      map[repoDiffWatchKey]*repoDiffSubscription{},
		lastLocalSummary: emptyRepoDiffSummary(),
	}
	watch.addSubscriber(repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"}, "repo", false)

	watch.refreshLocal()
	watch.refreshLocal()

	if collectCalls != 1 {
		t.Fatalf("expected one summary recomputation, got %d", collectCalls)
	}
}

func TestRefreshPrEmitsEventsOnce(t *testing.T) {
	origEmit := repoDiffEmit
	origGetPrStatus := repoDiffGetPrStatus
	origListRemotes := repoDiffListRemotes
	origCollectBranchSummary := repoDiffCollectBranchSummary
	origGetPrReviews := repoDiffGetPrReviews
	defer func() {
		repoDiffEmit = origEmit
		repoDiffGetPrStatus = origGetPrStatus
		repoDiffListRemotes = origListRemotes
		repoDiffCollectBranchSummary = origCollectBranchSummary
		repoDiffGetPrReviews = origGetPrReviews
	}()

	events := []string{}
	repoDiffEmit = func(_ context.Context, name string, _ ...interface{}) {
		events = append(events, name)
	}

	repoDiffGetPrStatus = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string, _ int, _ string) (repoDiffPrStatusResult, error) {
		return repoDiffPrStatusResult{
			pullRequest: worksetapi.PullRequestStatusJSON{
				Repo:       "octo/repo",
				Number:     42,
				URL:        "https://github.com/octo/repo/pull/42",
				Title:      "PR",
				State:      "open",
				Draft:      false,
				BaseRepo:   "octo/repo",
				BaseBranch: "main",
				HeadRepo:   "octo/repo",
				HeadBranch: "feature",
			},
			checks: []worksetapi.PullRequestCheckJSON{
				{Name: "ci", Status: "completed", Conclusion: "success"},
			},
		}, nil
	}

	repoDiffListRemotes = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string) ([]worksetapi.RemoteInfoJSON, error) {
		return []worksetapi.RemoteInfoJSON{{Name: "origin", Owner: "octo", Repo: "repo"}}, nil
	}

	repoDiffCollectBranchSummary = func(_ context.Context, _ string, _ string, _ string) (RepoDiffSummary, error) {
		return RepoDiffSummary{
			Files:        []RepoDiffFile{{Path: "file.go", Added: 2, Removed: 0, Status: "modified"}},
			TotalAdded:   2,
			TotalRemoved: 0,
		}, nil
	}

	repoDiffGetPrReviews = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string, _ int, _ string) ([]worksetapi.PullRequestReviewCommentJSON, error) {
		return []worksetapi.PullRequestReviewCommentJSON{
			{ID: 1, Body: "LGTM", Path: "file.go", Outdated: false},
		}, nil
	}

	watch := &repoDiffWatch{
		app:         &App{ctx: context.Background()},
		ctx:         context.Background(),
		repoPath:    "/tmp/repo",
		subscribers: map[repoDiffWatchKey]*repoDiffSubscription{},
	}
	key := repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"}
	watch.addSubscriber(key, "repo", false)
	watch.updatePrInfo(key, 42, "feature")

	watch.refreshPr()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(events), events)
	}
	if events[0] != EventRepoDiffPRStatus || events[1] != EventRepoDiffSummary || events[2] != EventRepoDiffPRReviews {
		t.Fatalf("unexpected event order: %v", events)
	}

	watch.refreshPr()
	if len(events) != 3 {
		t.Fatalf("expected no duplicate events, got %d", len(events))
	}
}

func TestLocalOnlySkipsSummaryAndPr(t *testing.T) {
	origEmit := repoDiffEmit
	origGetLocalStatus := repoDiffGetLocalStatus
	origCollectLocalSummary := repoDiffCollectLocalSummary
	origGetPrStatus := repoDiffGetPrStatus
	defer func() {
		repoDiffEmit = origEmit
		repoDiffGetLocalStatus = origGetLocalStatus
		repoDiffCollectLocalSummary = origCollectLocalSummary
		repoDiffGetPrStatus = origGetPrStatus
	}()

	events := []string{}
	repoDiffEmit = func(_ context.Context, name string, _ ...interface{}) {
		events = append(events, name)
	}

	repoDiffGetLocalStatus = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string, _ string) (repoLocalStatusSnapshot, error) {
		return repoLocalStatusSnapshot{
			payload: worksetapi.RepoLocalStatusJSON{
				HasUncommitted: true,
				Ahead:          0,
				Behind:         0,
				CurrentBranch:  "main",
			},
			summarySignature: "dirty-1",
		}, nil
	}

	repoDiffCollectLocalSummary = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string, _ string, _ string) (RepoDiffSummary, error) {
		return RepoDiffSummary{
			Files:        []RepoDiffFile{{Path: "file.txt", Added: 1, Removed: 0, Status: "modified"}},
			TotalAdded:   1,
			TotalRemoved: 0,
		}, nil
	}

	repoDiffGetPrStatus = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string, _ int, _ string) (repoDiffPrStatusResult, error) {
		t.Fatal("pr status should not be fetched in local-only mode")
		return repoDiffPrStatusResult{}, nil
	}

	watch := &repoDiffWatch{
		app:         &App{ctx: context.Background()},
		ctx:         context.Background(),
		repoPath:    "/tmp/repo",
		subscribers: map[repoDiffWatchKey]*repoDiffSubscription{},
	}
	watch.addSubscriber(repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"}, "repo", true)

	watch.refreshLocal()
	if len(events) != 2 {
		t.Fatalf("expected local-status and local-summary events, got %v", events)
	}
	if events[0] != EventRepoDiffLocalStatus || events[1] != EventRepoDiffLocalSummary {
		t.Fatalf("unexpected local-only event order: %v", events)
	}

	watch.refreshPr()
	if len(events) != 2 {
		t.Fatalf("expected no pr events, got %v", events)
	}
}

func TestRepoDiffWatchManagerStartDedupesConcurrentStarts(t *testing.T) {
	origRun := repoDiffRunWatch
	origResolvePath := repoDiffResolveRepoPath
	origResolveAlias := repoDiffResolveRepoAlias
	defer func() {
		repoDiffRunWatch = origRun
		repoDiffResolveRepoPath = origResolvePath
		repoDiffResolveRepoAlias = origResolveAlias
	}()

	repoDiffResolveRepoPath = func(_ context.Context, _ *App, _ string, _ string) (string, error) {
		return t.TempDir(), nil
	}
	repoDiffResolveRepoAlias = func(_ string, _ string) (string, error) {
		return "repo", nil
	}

	var runCount int32
	repoDiffRunWatch = func(_ *repoDiffWatch) {
		atomic.AddInt32(&runCount, 1)
	}

	manager := newRepoDiffWatchManager()
	app := &App{ctx: context.Background()}
	input := RepoDiffWatchRequest{
		WorkspaceID: "ws-1",
		RepoID:      "ws-1::repo",
		LocalOnly:   true,
	}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = manager.start(context.Background(), app, input)
		}()
	}
	wg.Wait()

	for i := 0; i < 10 && atomic.LoadInt32(&runCount) == 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&runCount); got != 1 {
		t.Fatalf("expected 1 watcher run, got %d", got)
	}

	manager.stop(input)
	manager.stop(input)
}

func TestRepoDiffWatchManagerSharesWatchAcrossWorkspaceKeysForSameRepoPath(t *testing.T) {
	origRun := repoDiffRunWatch
	origResolvePath := repoDiffResolveRepoPath
	origResolveAlias := repoDiffResolveRepoAlias
	defer func() {
		repoDiffRunWatch = origRun
		repoDiffResolveRepoPath = origResolvePath
		repoDiffResolveRepoAlias = origResolveAlias
	}()

	repoPath := t.TempDir()
	repoDiffResolveRepoPath = func(_ context.Context, _ *App, _ string, _ string) (string, error) {
		return repoPath, nil
	}
	repoDiffResolveRepoAlias = func(_ string, _ string) (string, error) {
		return "repo", nil
	}

	var runCount int32
	repoDiffRunWatch = func(_ *repoDiffWatch) {
		atomic.AddInt32(&runCount, 1)
	}

	manager := newRepoDiffWatchManager()
	app := &App{ctx: context.Background()}

	first := RepoDiffWatchRequest{WorkspaceID: "ws-1", RepoID: "ws-1::repo", LocalOnly: true}
	second := RepoDiffWatchRequest{WorkspaceID: "ws-2", RepoID: "ws-2::repo", LocalOnly: true}

	if _, err := manager.start(context.Background(), app, first); err != nil {
		t.Fatalf("start first watch: %v", err)
	}
	if _, err := manager.start(context.Background(), app, second); err != nil {
		t.Fatalf("start second watch: %v", err)
	}

	for i := 0; i < 10 && atomic.LoadInt32(&runCount) == 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&runCount); got != 1 {
		t.Fatalf("expected one shared watcher run, got %d", got)
	}
	if got := len(manager.watches); got != 1 {
		t.Fatalf("expected one shared watcher entry, got %d", got)
	}
	if got := len(manager.subscriptions); got != 2 {
		t.Fatalf("expected two subscriptions, got %d", got)
	}

	if stopped := manager.stop(first); stopped {
		t.Fatal("expected first stop to keep shared watcher alive")
	}
	if stopped := manager.stop(second); !stopped {
		t.Fatal("expected last stop to tear down shared watcher")
	}
}

func TestRepoDiffWatchStopStopsRefreshTimer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	watch := &repoDiffWatch{
		ctx:            ctx,
		cancel:         cancel,
		localRefreshCh: make(chan struct{}, 1),
	}
	watch.refreshTimer = time.AfterFunc(500*time.Millisecond, func() {
		watch.enqueueLocalRefresh()
	})

	watch.stop()

	if watch.refreshTimer != nil {
		t.Fatal("expected refresh timer to be cleared on stop")
	}
	if got := len(watch.localRefreshCh); got != 0 {
		t.Fatalf("expected no pending refresh events after stop, got %d", got)
	}
}

func TestRepoDiffWatchUpdatePrInfoEmptyInputDoesNotClearState(t *testing.T) {
	key := repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"}
	watch := &repoDiffWatch{
		subscribers: map[repoDiffWatchKey]*repoDiffSubscription{},
	}
	watch.addSubscriber(key, "repo", false)
	subscriber := watch.getSubscriber(key)
	if subscriber == nil {
		t.Fatal("expected subscriber to exist")
	}
	subscriber.updatePrInfo(42, "feature/foo")
	subscriber.setLastPrStatus(&worksetapi.PullRequestStatusJSON{
		Number: 42,
		State:  "open",
	})

	watch.updatePrInfo(key, 0, "")

	if got, _ := subscriber.currentPrInfo(); got != 42 {
		t.Fatalf("expected pr number to remain unchanged, got %d", got)
	}
	if _, branch := subscriber.currentPrInfo(); branch != "feature/foo" {
		t.Fatalf("expected pr branch to remain unchanged, got %q", branch)
	}
	if !subscriber.hasActivePr() {
		t.Fatal("expected last PR status to remain set")
	}
}

func TestRepoDiffWatchHandleFsnotifyEventTriggersRefreshes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watch := &repoDiffWatch{
		ctx:            ctx,
		localRefreshCh: make(chan struct{}, 1),
		prRefreshCh:    make(chan struct{}, 1),
		subscribers:    map[repoDiffWatchKey]*repoDiffSubscription{},
	}
	watch.addSubscriber(repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"}, "repo", false)
	targets := repoDiffGitWatchTargets{
		exactFiles: map[string]struct{}{filepath.Clean("/tmp/repo/.git/index"): {}},
	}

	watch.handleFsnotifyEvent(nil, targets, fsnotify.Event{
		Name: filepath.Clean("/tmp/repo/.git/index"),
		Op:   fsnotify.Write,
	})

	select {
	case <-watch.prRefreshCh:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected pr refresh to be queued")
	}

	select {
	case <-watch.localRefreshCh:
	case <-time.After(1 * time.Second):
		t.Fatal("expected local refresh after debounce")
	}

	watch.stopRefreshTimer()
}

func TestRepoDiffWatchHandleFsnotifyEventIgnoresUnrelatedPaths(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watch := &repoDiffWatch{
		ctx:            ctx,
		localRefreshCh: make(chan struct{}, 1),
		prRefreshCh:    make(chan struct{}, 1),
		subscribers:    map[repoDiffWatchKey]*repoDiffSubscription{},
	}
	watch.addSubscriber(repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"}, "repo", false)
	targets := repoDiffGitWatchTargets{
		exactFiles: map[string]struct{}{filepath.Clean("/tmp/repo/.git/index"): {}},
	}

	watch.handleFsnotifyEvent(nil, targets, fsnotify.Event{
		Name: filepath.Clean("/tmp/repo/src/main.go"),
		Op:   fsnotify.Write,
	})

	select {
	case <-watch.prRefreshCh:
		t.Fatal("expected unrelated event to skip pr refresh")
	case <-time.After(250 * time.Millisecond):
	}
	select {
	case <-watch.localRefreshCh:
		t.Fatal("expected unrelated event to skip local refresh")
	case <-time.After(500 * time.Millisecond):
	}
}

func TestRepoDiffWatchRunLocalOnlyCreatesGitWatcher(t *testing.T) {
	origNewWatcher := repoDiffNewWatcher
	origResolveTargets := repoDiffResolveGitWatchTargets
	origGetLocalStatus := repoDiffGetLocalStatus
	defer func() {
		repoDiffNewWatcher = origNewWatcher
		repoDiffResolveGitWatchTargets = origResolveTargets
		repoDiffGetLocalStatus = origGetLocalStatus
	}()

	var watcherCreateCount int32
	repoDiffNewWatcher = func() (*fsnotify.Watcher, error) {
		atomic.AddInt32(&watcherCreateCount, 1)
		return fsnotify.NewWatcher()
	}
	adminDir := filepath.Join(t.TempDir(), ".git")
	if err := os.MkdirAll(adminDir, 0o755); err != nil {
		t.Fatalf("mkdir admin dir: %v", err)
	}
	repoDiffResolveGitWatchTargets = func(_ context.Context, _ string) (repoDiffGitWatchTargets, error) {
		return repoDiffGitWatchTargets{
			adminDir: adminDir,
			exactFiles: map[string]struct{}{
				filepath.Join(adminDir, "HEAD"): {},
			},
		}, nil
	}
	repoDiffGetLocalStatus = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string, _ string) (repoLocalStatusSnapshot, error) {
		return repoLocalStatusSnapshot{
			payload: worksetapi.RepoLocalStatusJSON{
				HasUncommitted: false,
				CurrentBranch:  "main",
			},
		}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	watch := newRepoDiffWatch(
		&App{ctx: context.Background()},
		ctx,
		cancel,
		repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"},
		"repo",
		t.TempDir(),
		true,
	)

	done := make(chan struct{})
	go func() {
		watch.run()
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch to stop")
	}

	if got := atomic.LoadInt32(&watcherCreateCount); got != 1 {
		t.Fatalf("expected one git metadata watcher for local-only run, got %d", got)
	}
}

func TestRepoDiffWatchAddGitWatchTargetsDedupesPaths(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("create watcher: %v", err)
	}
	defer watcher.Close()

	root := filepath.Join(t.TempDir(), ".git")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	headPath := filepath.Join(root, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("write head: %v", err)
	}
	watch := &repoDiffWatch{watchedPaths: map[string]struct{}{}}
	targets := repoDiffGitWatchTargets{
		adminDir: root,
		exactFiles: map[string]struct{}{
			headPath: {},
		},
	}
	if err := watch.addGitWatchTargets(watcher, targets); err != nil {
		t.Fatalf("add first git watch: %v", err)
	}
	firstCount := len(watch.watchedPaths)
	if firstCount == 0 {
		t.Fatal("expected at least one watched path")
	}

	if err := watch.addGitWatchTargets(watcher, targets); err != nil {
		t.Fatalf("add second git watch: %v", err)
	}
	if got := len(watch.watchedPaths); got != firstCount {
		t.Fatalf("expected deduped watch count %d, got %d", firstCount, got)
	}
}

func initRepoDiffGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runRepoDiffGit(t, repo, "init")
	runRepoDiffGit(t, repo, "config", "user.name", "Workset Test")
	runRepoDiffGit(t, repo, "config", "user.email", "workset@example.com")
	return repo
}

func gitRevParsePath(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmdArgs := []string{"-C", repo, "rev-parse"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse %v failed: %v (%s)", args, err, string(output))
	}
	resolved := filepath.Clean(strings.TrimSpace(string(output)))
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(repo, resolved)
	}
	return filepath.Clean(resolved)
}

func runRepoDiffGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, string(output))
	}
}

func writeRepoDiffFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
