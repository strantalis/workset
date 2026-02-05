package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestShouldIgnorePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{path: "", want: true},
		{path: ".git", want: true},
		{path: "/tmp/repo/.git", want: true},
		{path: "/tmp/repo/node_modules", want: true},
		{path: "/tmp/repo/src/main.go", want: false},
	}

	for _, tc := range cases {
		if got := shouldIgnorePath(tc.path); got != tc.want {
			t.Fatalf("path %q expected %v got %v", tc.path, tc.want, got)
		}
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
	watch := &repoDiffWatch{}
	var last string

	if !watch.shouldEmit(&last, map[string]string{"a": "b"}) {
		t.Fatal("expected first payload to emit")
	}
	if watch.shouldEmit(&last, map[string]string{"a": "b"}) {
		t.Fatal("expected duplicate payload to be suppressed")
	}
	if !watch.shouldEmit(&last, map[string]string{"a": "c"}) {
		t.Fatal("expected changed payload to emit")
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
		if name == "repodiff:summary" && len(data) > 0 {
			if payload, ok := data[0].(RepoDiffSummaryEvent); ok {
				summaryPayload = payload.Summary
			}
		}
	}

	repoDiffGetLocalStatus = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string) (worksetapi.RepoLocalStatusJSON, error) {
		return worksetapi.RepoLocalStatusJSON{
			HasUncommitted: true,
			Ahead:          1,
			Behind:         0,
			CurrentBranch:  "main",
		}, nil
	}

	repoDiffCollectLocalSummary = func(_ context.Context, _ string) (RepoDiffSummary, error) {
		return RepoDiffSummary{
			Files: []RepoDiffFile{
				{Path: "file.txt", Added: 3, Removed: 1, Status: "modified"},
			},
			TotalAdded:   3,
			TotalRemoved: 1,
		}, nil
	}

	watch := &repoDiffWatch{
		app:      &App{ctx: context.Background()},
		ctx:      context.Background(),
		key:      repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"},
		repoName: "repo",
		fullRefs: 1,
	}

	watch.refreshLocal()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(events), events)
	}
	if events[0] != "repodiff:local-status" || events[1] != "repodiff:local-summary" || events[2] != "repodiff:summary" {
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
		app:      &App{ctx: context.Background()},
		ctx:      context.Background(),
		key:      repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"},
		repoName: "repo",
		fullRefs: 1,
	}
	watch.prNumber = 42

	watch.refreshPr()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(events), events)
	}
	if events[0] != "repodiff:pr-status" || events[1] != "repodiff:summary" || events[2] != "repodiff:pr-reviews" {
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

	repoDiffGetLocalStatus = func(_ context.Context, _ *App, _ repoDiffWatchKey, _ string) (worksetapi.RepoLocalStatusJSON, error) {
		return worksetapi.RepoLocalStatusJSON{
			HasUncommitted: true,
			Ahead:          0,
			Behind:         0,
			CurrentBranch:  "main",
		}, nil
	}

	repoDiffCollectLocalSummary = func(_ context.Context, _ string) (RepoDiffSummary, error) {
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
		app:      &App{ctx: context.Background()},
		ctx:      context.Background(),
		key:      repoDiffWatchKey{workspaceID: "ws-1", repoID: "repo-1"},
		repoName: "repo",
	}

	watch.refreshLocal()
	if len(events) != 1 || events[0] != "repodiff:local-status" {
		t.Fatalf("expected only local-status event, got %v", events)
	}

	watch.refreshPr()
	if len(events) != 1 {
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
