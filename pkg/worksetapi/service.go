package worksetapi

import (
	"context"
	"time"

	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/session"
)

// Options configures a Service instance.
// Zero values select defaults that match the CLI behavior.
type Options struct {
	ConfigPath     string
	ConfigStore    ConfigStore
	WorkspaceStore WorkspaceStore
	Git            git.Client
	SessionRunner  session.Runner
	// ExecFunc overrides how Exec runs commands (useful for embedding/tests).
	ExecFunc func(ctx context.Context, root string, command []string, env []string) error
	Clock    func() time.Time
	Logf     func(format string, args ...any)
}

// Service provides the public API for workspace, repo, alias, group, session,
// and exec operations.
type Service struct {
	configPath string
	configs    ConfigStore
	workspaces WorkspaceStore
	git        git.Client
	runner     session.Runner
	exec       func(ctx context.Context, root string, command []string, env []string) error
	clock      func() time.Time
	logf       func(format string, args ...any)
}

// NewService constructs a Service with injected dependencies or defaults.
func NewService(opts Options) *Service {
	cfgStore := opts.ConfigStore
	if cfgStore == nil {
		cfgStore = FileConfigStore{}
	}
	wsStore := opts.WorkspaceStore
	if wsStore == nil {
		wsStore = FileWorkspaceStore{}
	}
	gitClient := opts.Git
	if gitClient == nil {
		gitClient = git.NewGoGitClient()
	}
	runner := opts.SessionRunner
	if runner == nil {
		runner = session.ExecRunner{}
	}
	execFunc := opts.ExecFunc
	if execFunc == nil {
		execFunc = runExecCommand
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Service{
		configPath: opts.ConfigPath,
		configs:    cfgStore,
		workspaces: wsStore,
		git:        gitClient,
		runner:     runner,
		exec:       execFunc,
		clock:      clock,
		logf:       opts.Logf,
	}
}
