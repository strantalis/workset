package worksetapi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/session"
	"github.com/strantalis/workset/internal/workspace"
)

func TestStopWorkspaceSessions(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	env.runner.results[commandKey("tmux", []string{"has-session", "-t", "workset-demo"})] = session.CommandResult{ExitCode: 0}
	env.runner.results[commandKey("tmux", []string{"kill-session", "-t", "workset-demo"})] = session.CommandResult{ExitCode: 0}

	if err := env.svc.stopWorkspaceSessions(context.Background(), root, false); err != nil {
		t.Fatalf("stop sessions: %v", err)
	}
}

func TestStopWorkspaceSessionsForceSkipsInvalid(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"broken": {Backend: "unknown"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if err := env.svc.stopWorkspaceSessions(context.Background(), root, true); err != nil {
		t.Fatalf("stop sessions force: %v", err)
	}
}

func TestStopWorkspaceSessionsMissingState(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	if err := os.Remove(workspace.StatePath(root)); err != nil {
		t.Fatalf("remove state: %v", err)
	}
	if err := env.svc.stopWorkspaceSessions(context.Background(), root, false); err != nil {
		t.Fatalf("stop sessions: %v", err)
	}
}

func TestStopWorkspaceSessionsMissingBackend(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := env.svc.stopWorkspaceSessions(context.Background(), root, false); err == nil {
		t.Fatalf("expected missing backend error")
	}
}

func TestStopWorkspaceSessionsMissingBackendForce(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := env.svc.stopWorkspaceSessions(context.Background(), root, true); err != nil {
		t.Fatalf("stop sessions force: %v", err)
	}
}

func TestStopWorkspaceSessionsLookPathError(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	env.runner.lookPath["tmux"] = errors.New("missing")

	if err := env.svc.stopWorkspaceSessions(context.Background(), root, false); err == nil {
		t.Fatalf("expected lookpath error")
	}
}

func TestStopWorkspaceSessionsLookPathErrorForce(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	env.runner.lookPath["tmux"] = errors.New("missing")

	if err := env.svc.stopWorkspaceSessions(context.Background(), root, true); err != nil {
		t.Fatalf("expected force to skip lookpath error, got %v", err)
	}
}

func TestStopWorkspaceSessionsUnsupportedBackend(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "exec", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if err := env.svc.stopWorkspaceSessions(context.Background(), root, false); err == nil {
		t.Fatalf("expected unsupported backend error")
	}
}

func TestStopWorkspaceSessionsInvalidBackend(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "bogus", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if err := env.svc.stopWorkspaceSessions(context.Background(), root, false); err == nil {
		t.Fatalf("expected invalid backend error")
	}
}

func TestStopWorkspaceSessionsUnsupportedBackendForce(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "exec", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if err := env.svc.stopWorkspaceSessions(context.Background(), root, true); err != nil {
		t.Fatalf("expected force to skip exec backend, got %v", err)
	}
}

func TestStopWorkspaceSessionsExistsError(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	env.runner.results[commandKey("tmux", []string{"has-session", "-t", "workset-demo"})] = session.CommandResult{ExitCode: 2}
	env.runner.errors[commandKey("tmux", []string{"has-session", "-t", "workset-demo"})] = errors.New("boom")

	if err := env.svc.stopWorkspaceSessions(context.Background(), root, false); err == nil {
		t.Fatalf("expected exists error")
	}
}

func TestStopWorkspaceSessionsStopErrorForce(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	env.runner.results[commandKey("tmux", []string{"has-session", "-t", "workset-demo"})] = session.CommandResult{ExitCode: 0}
	env.runner.errors[commandKey("tmux", []string{"kill-session", "-t", "workset-demo"})] = errors.New("fail stop")

	if err := env.svc.stopWorkspaceSessions(context.Background(), root, true); err != nil {
		t.Fatalf("expected force to skip stop errors, got %v", err)
	}
}

func TestStopWorkspaceSessionsStopError(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	env.runner.results[commandKey("tmux", []string{"has-session", "-t", "workset-demo"})] = session.CommandResult{ExitCode: 0}
	env.runner.errors[commandKey("tmux", []string{"kill-session", "-t", "workset-demo"})] = errors.New("fail stop")

	if err := env.svc.stopWorkspaceSessions(context.Background(), root, false); err == nil {
		t.Fatalf("expected stop error")
	}
}

func TestRemoveWorkspaceRepoWorktrees(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("load workspace config: %v", err)
	}
	wsCfg.Repos = []config.RepoConfig{
		{
			Name:      "repo-a",
			RepoDir:   "repo-a",
			LocalPath: local,
			Remotes:   config.Remotes{Base: config.RemoteConfig{Name: "origin"}},
		},
	}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "repo-a"), 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	// Create a leftover worktree with .git file so cleanup path is exercised.
	extra := filepath.Join(root, "extra")
	if err := os.MkdirAll(extra, 0o755); err != nil {
		t.Fatalf("mkdir extra: %v", err)
	}
	gitDir := filepath.Join(root, ".git-worktrees", "extra")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir gitdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extra, ".git"), []byte("gitdir: "+gitDir), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}

	if err := env.svc.removeWorkspaceRepoWorktrees(context.Background(), root, env.loadConfig().Defaults, false); err != nil {
		t.Fatalf("remove worktrees: %v", err)
	}
	if len(env.git.worktreeRemovs) == 0 {
		t.Fatalf("expected worktree removals")
	}
}

func TestRemoveWorkspaceRepoWorktreesMissingWorkspace(t *testing.T) {
	env := newTestEnv(t)
	missing := filepath.Join(env.root, "missing")
	if err := env.svc.removeWorkspaceRepoWorktrees(context.Background(), missing, env.loadConfig().Defaults, false); err != nil {
		t.Fatalf("expected nil for missing workspace, got %v", err)
	}
	if err := env.svc.removeWorkspaceRepoWorktrees(context.Background(), missing, env.loadConfig().Defaults, true); err != nil {
		t.Fatalf("expected nil for missing workspace with force, got %v", err)
	}
}

func TestRemoveWorkspaceRepoWorktreesForceSkipsErrors(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	local := filepath.Join(env.root, "broken")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatalf("mkdir local: %v", err)
	}
	if err := os.WriteFile(filepath.Join(local, ".git"), []byte("invalid"), 0o644); err != nil {
		t.Fatalf("write invalid git: %v", err)
	}

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("load workspace config: %v", err)
	}
	wsCfg.Repos = []config.RepoConfig{
		{
			Name:      "broken",
			RepoDir:   "broken",
			LocalPath: local,
			Remotes:   config.Remotes{Base: config.RemoteConfig{Name: "origin"}},
		},
	}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace config: %v", err)
	}

	if err := env.svc.removeWorkspaceRepoWorktrees(context.Background(), root, env.loadConfig().Defaults, true); err != nil {
		t.Fatalf("expected force to skip errors, got %v", err)
	}
}

func TestRemoveWorkspaceRepoWorktreesErrorWithoutForce(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	local := filepath.Join(env.root, "broken2")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatalf("mkdir local: %v", err)
	}
	if err := os.WriteFile(filepath.Join(local, ".git"), []byte("invalid"), 0o644); err != nil {
		t.Fatalf("write invalid git: %v", err)
	}

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("load workspace config: %v", err)
	}
	wsCfg.Repos = []config.RepoConfig{
		{
			Name:      "broken2",
			RepoDir:   "broken2",
			LocalPath: local,
			Remotes:   config.Remotes{Base: config.RemoteConfig{Name: "origin"}},
		},
	}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace config: %v", err)
	}

	if err := env.svc.removeWorkspaceRepoWorktrees(context.Background(), root, env.loadConfig().Defaults, false); err == nil {
		t.Fatalf("expected error without force")
	}
}

func TestRemoveWorkspaceRepoWorktreesForceSkipsLoadError(t *testing.T) {
	env := newTestEnv(t)
	root := filepath.Join(env.root, "bad-workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.WriteFile(workspace.WorksetFile(root), []byte(":::invalid"), 0o644); err != nil {
		t.Fatalf("write workset: %v", err)
	}

	if err := env.svc.removeWorkspaceRepoWorktrees(context.Background(), root, env.loadConfig().Defaults, true); err != nil {
		t.Fatalf("expected force to skip load error, got %v", err)
	}
}
