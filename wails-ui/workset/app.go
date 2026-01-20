package main

import (
	"context"
	"sync"

	"github.com/strantalis/workset/pkg/worksetapi"
)

// App struct
type App struct {
	ctx        context.Context
	service    *worksetapi.Service
	terminalMu sync.Mutex
	terminals  map[string]*terminalSession
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		service:   worksetapi.NewService(worksetapi.Options{}),
		terminals: map[string]*terminalSession{},
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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
