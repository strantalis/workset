package main

import (
	"context"
	"sync"

	"github.com/strantalis/workset/pkg/terminalservice"
	"github.com/strantalis/workset/pkg/worksetapi"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const mainWindowName = "main"

// App struct
type App struct {
	ctx                   context.Context
	runtimeApp            *application.App
	mainWindowName        string
	service               *worksetapi.Service
	serviceOnce           sync.Once
	repoFileIndexMu       sync.Mutex
	repoFileIndexes       map[string]repoFileIndexCacheEntry
	repoHoverMu           sync.Mutex
	repoHoverClients      map[string]repoHoverBackend
	repoDiffSummaryMu     sync.Mutex
	repoDiffSummaries     map[string]repoDiffSummaryCacheEntry
	terminalMu            sync.Mutex
	terminals             map[string]*terminalSession
	terminalServiceMu     sync.Mutex
	terminalServiceServer *terminalservice.Server
	terminalServiceCancel context.CancelFunc
	terminalServiceDone   chan struct{}
	terminalServiceClient *terminalservice.Client
	terminalServiceInfo   *terminalservice.InfoResponse
	terminalServiceReady  bool
	terminalServiceStart  *terminalServiceStartState
	repoDiffWatchers      *repoDiffWatchManager
	githubOps             *githubOperationManager
	popoutMu              sync.Mutex
	popouts               map[string]string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		service:              nil,
		mainWindowName:       mainWindowName,
		repoFileIndexes:      map[string]repoFileIndexCacheEntry{},
		repoHoverClients:     map[string]repoHoverBackend{},
		repoDiffSummaries:    map[string]repoDiffSummaryCacheEntry{},
		terminals:            map[string]*terminalSession{},
		terminalServiceStart: &terminalServiceStartState{},
		repoDiffWatchers:     newRepoDiffWatchManager(),
		githubOps:            newGitHubOperationManager(),
		popouts:              map[string]string{},
	}
}

func (a *App) setRuntime(app *application.App) {
	a.runtimeApp = app
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logTerminalServicef("app_startup build_marker=restart-logging-v2")
	ensureDevConfig()
	_, _ = worksetapi.EnsureLoginEnv(ctx)
	ensureDevTerminalServiceSocket()
	ensureLegacySessiondRetired()
	ensureTerminalServiceStarted(a)
}

func (a *App) shutdown(_ context.Context) {
	if a.repoDiffWatchers != nil {
		a.repoDiffWatchers.shutdown()
	}
	a.popoutMu.Lock()
	a.popouts = map[string]string{}
	a.popoutMu.Unlock()
	a.repoFileIndexMu.Lock()
	a.repoFileIndexes = map[string]repoFileIndexCacheEntry{}
	a.repoFileIndexMu.Unlock()
	a.repoHoverMu.Lock()
	for _, client := range a.repoHoverClients {
		_ = client.Close()
	}
	a.repoHoverClients = map[string]repoHoverBackend{}
	a.repoHoverMu.Unlock()
	a.repoDiffSummaryMu.Lock()
	a.repoDiffSummaries = map[string]repoDiffSummaryCacheEntry{}
	a.repoDiffSummaryMu.Unlock()
	a.terminalMu.Lock()
	for _, session := range a.terminals {
		_ = session.CloseWithReason("shutdown")
	}
	a.terminals = map[string]*terminalSession{}
	a.terminalMu.Unlock()
	a.stopEmbeddedTerminalService()
}

func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.startup(ctx)
	return nil
}

func (a *App) ServiceShutdown() error {
	a.shutdown(a.ctx)
	return nil
}
