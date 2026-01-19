package worksetapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/strantalis/workset/internal/config"
)

func TestExecUsesProvidedRunner(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	var gotRoot string
	var gotCommand []string
	var gotEnv []string
	svc := NewService(Options{
		ConfigPath:     env.configPath,
		ConfigStore:    FileConfigStore{},
		WorkspaceStore: FileWorkspaceStore{},
		Git:            env.git,
		SessionRunner:  env.runner,
		ExecFunc: func(_ context.Context, root string, command []string, env []string) error {
			gotRoot = root
			gotCommand = command
			gotEnv = env
			return nil
		},
		Clock: func() time.Time { return env.now },
		Logf:  func(string, ...any) {},
	})

	if err := svc.Exec(context.Background(), ExecInput{
		Workspace: WorkspaceSelector{Value: root},
		Command:   []string{"echo", "hi"},
	}); err != nil {
		t.Fatalf("exec: %v", err)
	}

	if gotRoot != root {
		t.Fatalf("unexpected root: %s", gotRoot)
	}
	if len(gotCommand) != 2 || gotCommand[0] != "echo" {
		t.Fatalf("unexpected command: %v", gotCommand)
	}
	foundRoot := false
	foundConfig := false
	for _, entry := range gotEnv {
		if strings.HasPrefix(entry, "WORKSET_ROOT=") {
			foundRoot = true
		}
		if strings.HasPrefix(entry, "WORKSET_CONFIG=") {
			foundConfig = true
		}
	}
	if !foundRoot || !foundConfig {
		t.Fatalf("missing WORKSET env vars: %v", gotEnv)
	}
}

func TestExecRequiresWorkspace(t *testing.T) {
	env := newTestEnv(t)
	err := env.svc.Exec(context.Background(), ExecInput{})
	_ = requireErrorType[ValidationError](t, err)
}

func TestExecMissingWorksetFile(t *testing.T) {
	env := newTestEnv(t)
	missing := filepath.Join(env.root, "empty")
	if err := os.MkdirAll(missing, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cfg := env.loadConfig()
	cfg.Workspaces = map[string]config.WorkspaceRef{
		"empty": {Path: missing},
	}
	env.saveConfig(cfg)

	err := env.svc.Exec(context.Background(), ExecInput{
		Workspace: WorkspaceSelector{Value: "empty"},
		Command:   []string{"echo", "hi"},
	})
	_ = requireErrorType[NotFoundError](t, err)
}

func TestExecUsesDefaultWorkspace(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	cfg := env.loadConfig()
	cfg.Defaults.Workspace = "demo"
	env.saveConfig(cfg)

	called := false
	svc := NewService(Options{
		ConfigPath:     env.configPath,
		ConfigStore:    FileConfigStore{},
		WorkspaceStore: FileWorkspaceStore{},
		Git:            env.git,
		SessionRunner:  env.runner,
		ExecFunc: func(_ context.Context, gotRoot string, _ []string, _ []string) error {
			if gotRoot != root {
				t.Fatalf("unexpected root: %s", gotRoot)
			}
			called = true
			return nil
		},
		Clock: func() time.Time { return env.now },
		Logf:  func(string, ...any) {},
	})

	if err := svc.Exec(context.Background(), ExecInput{Command: []string{"echo", "hi"}}); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !called {
		t.Fatalf("expected exec func called")
	}
}
