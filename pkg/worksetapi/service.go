package worksetapi

import (
	"context"
	"time"

	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/hooks"
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
	CommandRunner  CommandRunner
	GitHubProvider GitHubProvider
	TokenStore     TokenStore
	// ExecFunc overrides how Exec runs commands (useful for embedding/tests).
	ExecFunc func(ctx context.Context, root string, command []string, env []string) error
	// HookRunner overrides how hooks run commands (useful for embedding/tests).
	HookRunner hooks.Runner
	Clock      func() time.Time
	Logf       func(format string, args ...any)
}

// Service provides the public API for workspace, repo, alias, group, session,
// and exec operations.
type Service struct {
	configPath string
	configs    ConfigStore
	workspaces WorkspaceStore
	git        git.Client
	runner     session.Runner
	commands   CommandRunner
	exec       func(ctx context.Context, root string, command []string, env []string) error
	hookRunner hooks.Runner
	clock      func() time.Time
	logf       func(format string, args ...any)
	github     GitHubProvider
}

// NewService constructs a Service with injected dependencies or defaults.
func NewService(opts Options) *Service {
	ensureLoginEnv()
	git.EnsureSSHAuthSock()

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
		gitClient = git.NewCLIClient()
	}
	runner := opts.SessionRunner
	if runner == nil {
		runner = session.ExecRunner{}
	}
	commandRunner := opts.CommandRunner
	if commandRunner == nil {
		commandRunner = runCommandCapture
	}
	execFunc := opts.ExecFunc
	if execFunc == nil {
		execFunc = runExecCommand
	}
	hookRunner := opts.HookRunner
	if hookRunner == nil {
		hookRunner = hooks.ExecRunner{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	tokenStore := opts.TokenStore
	if tokenStore == nil {
		tokenStore = KeyringTokenStore{}
	}
	githubProvider := opts.GitHubProvider
	if githubProvider == nil {
		githubProvider = NewGitHubProviderSelector(tokenStore)
	}
	return &Service{
		configPath: opts.ConfigPath,
		configs:    cfgStore,
		workspaces: wsStore,
		git:        gitClient,
		runner:     runner,
		commands:   commandRunner,
		exec:       execFunc,
		hookRunner: hookRunner,
		clock:      clock,
		logf:       opts.Logf,
		github:     githubProvider,
	}
}
