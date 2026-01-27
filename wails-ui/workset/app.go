package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	normalizePathFromShell()
	logPathState()
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

func normalizePathFromShell() {
	if runtime.GOOS == "windows" {
		return
	}
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}
	loginPath, err := pathFromShell(shell)
	if err != nil {
		if envTruthy(os.Getenv("WORKSET_DEBUG_PATH")) {
			logRestartf("env_path_shell_failed shell=%q err=%v", shell, err)
		}
		return
	}
	if loginPath == "" {
		if envTruthy(os.Getenv("WORKSET_DEBUG_PATH")) {
			logRestartf("env_path_shell_empty shell=%q", shell)
		}
		return
	}
	currentPath := os.Getenv("PATH")
	merged := mergePathEntries(currentPath, loginPath)
	if merged != "" && merged != currentPath {
		_ = os.Setenv("PATH", merged)
	}
}

const pathOutputPrefix = "__WORKSET_PATH__"

func pathFromShell(shell string) (string, error) {
	command := "printf '__WORKSET_PATH__%s\\n' \"$PATH\""
	args := []string{"-lc", command}
	isZsh := strings.HasSuffix(filepath.Base(shell), "zsh")
	if isZsh {
		args = []string{"-lic", command}
	}
	output, err := exec.Command(shell, args...).Output()
	if err != nil && isZsh {
		output, err = exec.Command(shell, "-lc", command).Output()
	}
	if err != nil {
		return "", err
	}
	return extractPathFromShellOutput(string(output)), nil
}

func extractPathFromShellOutput(output string) string {
	var path string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, pathOutputPrefix) {
			path = strings.TrimPrefix(line, pathOutputPrefix)
		}
	}
	return strings.TrimSpace(path)
}

func mergePathEntries(current, fromLogin string) string {
	if current == "" {
		return fromLogin
	}
	seen := make(map[string]struct{})
	merged := make([]string, 0, 16)
	for _, value := range strings.Split(current, ":") {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		merged = append(merged, value)
	}
	for _, value := range strings.Split(fromLogin, ":") {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		merged = append(merged, value)
	}
	return strings.Join(merged, ":")
}

func logPathState() {
	if !envTruthy(os.Getenv("WORKSET_DEBUG_PATH")) {
		return
	}
	logRestartf("env_path shell=%q path=%q", os.Getenv("SHELL"), os.Getenv("PATH"))
}
