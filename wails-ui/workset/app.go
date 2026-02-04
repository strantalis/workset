package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/strantalis/workset/pkg/sessiond"
	"github.com/strantalis/workset/pkg/worksetapi"
)

// App struct
type App struct {
	ctx             context.Context
	service         *worksetapi.Service
	terminalMu      sync.Mutex
	terminals       map[string]*terminalSession
	restoredModes   map[string]terminalModeState
	sessiondMu      sync.Mutex
	sessiondClient  *sessiond.Client
	sessiondStart   *sessiondStartState
	sessiondRestart *sessiondRestartState
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		service:         worksetapi.NewService(worksetapi.Options{}),
		terminals:       map[string]*terminalSession{},
		restoredModes:   map[string]terminalModeState{},
		sessiondStart:   &sessiondStartState{},
		sessiondRestart: &sessiondRestartState{},
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logRestartf("app_startup build_marker=restart-logging-v2")
	_, _ = worksetapi.EnsureLoginEnv(ctx)
	ensureDevSessiondSocket()
	setSessiondPathFromCwd()
	ensureSessiondUpToDate(a)
	ensureSessiondStarted(a)
	go a.restoreTerminalSessions(ctx)
}

func (a *App) shutdown(_ context.Context) {
	_ = a.persistTerminalState()
	a.terminalMu.Lock()
	defer a.terminalMu.Unlock()
	for _, session := range a.terminals {
		_ = session.CloseWithReason("shutdown")
	}
	a.terminals = map[string]*terminalSession{}
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
