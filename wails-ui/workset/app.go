package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/strantalis/workset/pkg/sessiond"
	"github.com/strantalis/workset/pkg/worksetapi"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const mainWindowName = "main"

// App struct
type App struct {
	ctx              context.Context
	runtimeApp       *application.App
	mainWindowName   string
	service          *worksetapi.Service
	serviceOnce      sync.Once
	terminalMu       sync.Mutex
	terminals        map[string]*terminalSession
	sessiondMu       sync.Mutex
	sessiondClient   *sessiond.Client
	sessiondStart    *sessiondStartState
	sessiondRestart  *sessiondRestartState
	repoDiffWatchers *repoDiffWatchManager
	githubOps        *githubOperationManager
	popoutMu         sync.Mutex
	popouts          map[string]string
	terminalOwners   map[string]string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		service:          nil,
		mainWindowName:   mainWindowName,
		terminals:        map[string]*terminalSession{},
		sessiondStart:    &sessiondStartState{},
		sessiondRestart:  &sessiondRestartState{},
		repoDiffWatchers: newRepoDiffWatchManager(),
		githubOps:        newGitHubOperationManager(),
		popouts:          map[string]string{},
		terminalOwners:   map[string]string{},
	}
}

func (a *App) setRuntime(app *application.App) {
	a.runtimeApp = app
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logRestartf("app_startup build_marker=restart-logging-v2")
	ensureDevConfig()
	_, _ = worksetapi.EnsureLoginEnv(ctx)
	ensureDevSessiondSocket()
	setSessiondPathFromCwd()
	ensureSessiondUpToDate(a)
	ensureSessiondStarted(a)
}

func (a *App) shutdown(_ context.Context) {
	if a.repoDiffWatchers != nil {
		a.repoDiffWatchers.shutdown()
	}
	a.popoutMu.Lock()
	a.popouts = map[string]string{}
	a.terminalOwners = map[string]string{}
	a.popoutMu.Unlock()
	a.terminalMu.Lock()
	defer a.terminalMu.Unlock()
	for _, session := range a.terminals {
		_ = session.CloseWithReason("shutdown")
	}
	a.terminals = map[string]*terminalSession{}
}

func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.startup(ctx)
	return nil
}

func (a *App) ServiceShutdown() error {
	a.shutdown(a.ctx)
	return nil
}

func setSessiondPathFromCwd() {
	if os.Getenv("WORKSET_SESSIOND_PATH") != "" {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	exeName := "workset-sessiond"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	candidates := []string{
		filepath.Join(cwd, "build", "sessiond", exeName),
		filepath.Join(cwd, "wails-ui", "workset", "build", "sessiond", exeName),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			_ = os.Setenv("WORKSET_SESSIOND_PATH", candidate)
			return
		}
	}
}
