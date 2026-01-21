package worksetapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/hooks"
	"github.com/strantalis/workset/internal/workspace"
)

func TestListRepos(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("load workspace config: %v", err)
	}
	wsCfg.Repos = []config.RepoConfig{
		{
			Name:      "repo-a",
			RepoDir:   "repo-a",
			Remotes:   config.Remotes{Base: config.RemoteConfig{Name: "origin", DefaultBranch: "main"}, Write: config.RemoteConfig{Name: "origin"}},
			LocalPath: "/tmp/repo-a",
		},
	}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace config: %v", err)
	}

	result, err := env.svc.ListRepos(context.Background(), WorkspaceSelector{Value: root})
	if err != nil {
		t.Fatalf("list repos: %v", err)
	}
	if len(result.Repos) != 1 {
		t.Fatalf("expected 1 repo")
	}
	if result.Repos[0].Base != "origin/main" {
		t.Fatalf("unexpected base: %s", result.Repos[0].Base)
	}
}

func TestAddRepoFromLocalPath(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")

	result, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		Name:       "repo-a",
		NameSet:    true,
		SourcePath: local,
	})
	if err != nil {
		t.Fatalf("add repo: %v", err)
	}
	if result.Payload.Repo != "repo-a" {
		t.Fatalf("unexpected repo name: %s", result.Payload.Repo)
	}
	if result.Payload.Managed {
		t.Fatalf("expected unmanaged repo for local path")
	}
	if _, err := os.Stat(result.WorktreePath); err != nil {
		t.Fatalf("worktree path missing: %v", err)
	}
}

func TestAddRepoReattachesExistingWorktree(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")

	result, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		Name:       "repo-a",
		NameSet:    true,
		SourcePath: local,
	})
	if err != nil {
		t.Fatalf("add repo: %v", err)
	}

	if _, err := env.svc.RemoveRepo(context.Background(), RepoRemoveInput{
		Workspace:       WorkspaceSelector{Value: root},
		Name:            "repo-a",
		DeleteWorktrees: false,
		DeleteLocal:     false,
		Confirmed:       true,
	}); err != nil {
		t.Fatalf("remove repo: %v", err)
	}

	if _, err := os.Stat(result.WorktreePath); err != nil {
		t.Fatalf("worktree path missing after removal: %v", err)
	}

	if _, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		Name:       "repo-a",
		NameSet:    true,
		SourcePath: local,
	}); err != nil {
		t.Fatalf("re-add repo: %v", err)
	}

	if len(env.git.worktreeAdds) != 1 {
		t.Fatalf("expected no new worktree additions on reattach")
	}
}

func TestAddRepoFromAliasURL(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	cfg := env.loadConfig()
	cfg.Repos = map[string]config.RepoAlias{
		"repo-b": {URL: "https://example.com/repo-b.git", DefaultBranch: "dev"},
	}
	env.saveConfig(cfg)

	result, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace: WorkspaceSelector{Value: root},
		Source:    "repo-b",
	})
	if err != nil {
		t.Fatalf("add repo: %v", err)
	}
	if !result.Payload.Managed {
		t.Fatalf("expected managed repo for URL clone")
	}
	if result.Payload.LocalPath != filepath.Join(env.repoRoot, "repo-b") {
		t.Fatalf("unexpected local path: %s", result.Payload.LocalPath)
	}
}

func TestUpdateRepoRemotes(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")

	if _, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		Name:       "repo-a",
		NameSet:    true,
		SourcePath: local,
	}); err != nil {
		t.Fatalf("add repo: %v", err)
	}

	result, _, err := env.svc.UpdateRepoRemotes(context.Background(), RepoRemotesUpdateInput{
		Workspace:      WorkspaceSelector{Value: root},
		Name:           "repo-a",
		BaseRemote:     "upstream",
		WriteRemote:    "origin",
		BaseRemoteSet:  true,
		WriteRemoteSet: true,
	})
	if err != nil {
		t.Fatalf("update remotes: %v", err)
	}
	if result.Base != "upstream/main" || result.Write != "origin/main" {
		t.Fatalf("unexpected remotes: %+v", result)
	}
}

func TestRemoveRepoSafetyAndConfirmation(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")
	var err error

	if _, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		Name:       "repo-a",
		NameSet:    true,
		SourcePath: local,
	}); err != nil {
		t.Fatalf("add repo: %v", err)
	}

	worktreePath := filepath.Join(root, "repo-a")
	env.git.status[worktreePath] = git.StatusSummary{Dirty: true}

	_, err = env.svc.RemoveRepo(context.Background(), RepoRemoveInput{
		Workspace:       WorkspaceSelector{Value: root},
		Name:            "repo-a",
		DeleteWorktrees: true,
	})
	_ = requireErrorType[UnsafeOperation](t, err)

	_, err = env.svc.RemoveRepo(context.Background(), RepoRemoveInput{
		Workspace:       WorkspaceSelector{Value: root},
		Name:            "repo-a",
		DeleteWorktrees: true,
		Force:           true,
		Confirmed:       false,
	})
	_ = requireErrorType[ConfirmationRequired](t, err)

	_, err = env.svc.RemoveRepo(context.Background(), RepoRemoveInput{
		Workspace:       WorkspaceSelector{Value: root},
		Name:            "repo-a",
		DeleteWorktrees: true,
		Force:           true,
		Confirmed:       true,
	})
	if err != nil {
		t.Fatalf("remove repo: %v", err)
	}
}

func TestAddRepoRequiresSource(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	_, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace: WorkspaceSelector{Value: root},
	})
	_ = requireErrorType[ValidationError](t, err)
}

func TestAddRepoInvalidLocalPath(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	_, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		SourcePath: filepath.Join(env.root, "missing"),
	})
	if err == nil {
		t.Fatalf("expected error for missing local path")
	}
}

func TestAddRepoUpdatesAliasFromLocalPath(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")

	cfg := env.loadConfig()
	cfg.Repos = map[string]config.RepoAlias{
		"repo-a": {URL: local},
	}
	env.saveConfig(cfg)

	_, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:     WorkspaceSelector{Value: root},
		Source:        "repo-a",
		UpdateAliases: true,
	})
	if err != nil {
		t.Fatalf("add repo: %v", err)
	}

	updated := env.loadConfig()
	alias := updated.Repos["repo-a"]
	if alias.Path == "" || alias.URL != "" {
		t.Fatalf("expected alias updated with path: %+v", alias)
	}
}

func TestAddRepoPendingHooksWhenUntrusted(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")
	env.git.worktreeAddHook = func(path string) error {
		hooksDir := filepath.Join(path, ".workset")
		if err := os.MkdirAll(hooksDir, 0o755); err != nil {
			return err
		}
		data := []byte("hooks:\n  - id: bootstrap\n    on: [worktree.created]\n    run: [\"npm\", \"ci\"]\n")
		return os.WriteFile(filepath.Join(hooksDir, "hooks.yaml"), data, 0o644)
	}

	result, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		Name:       "repo-a",
		NameSet:    true,
		SourcePath: local,
	})
	if err != nil {
		t.Fatalf("add repo: %v", err)
	}
	if len(result.PendingHooks) != 1 {
		t.Fatalf("expected pending hooks")
	}
	if len(result.Payload.PendingHooks) != 1 {
		t.Fatalf("expected pending hooks in payload")
	}
	if result.PendingHooks[0].Repo != "repo-a" {
		t.Fatalf("unexpected pending repo: %s", result.PendingHooks[0].Repo)
	}
	if result.PendingHooks[0].Status != HookRunStatusSkipped || result.PendingHooks[0].Reason != "untrusted" {
		t.Fatalf("expected skipped/untrusted status")
	}
}

func TestAddRepoRunsTrustedHooks(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")
	env.git.worktreeAddHook = func(path string) error {
		hooksDir := filepath.Join(path, ".workset")
		if err := os.MkdirAll(hooksDir, 0o755); err != nil {
			return err
		}
		data := []byte("hooks:\n  - id: bootstrap\n    on: [worktree.created]\n    run: [\"npm\", \"ci\"]\n")
		return os.WriteFile(filepath.Join(hooksDir, "hooks.yaml"), data, 0o644)
	}

	cfg := env.loadConfig()
	cfg.Hooks.RepoHooks.TrustedRepos = []string{"repo-a"}
	env.saveConfig(cfg)

	runner := &stubHookRunner{}
	env.svc = NewService(Options{
		ConfigPath:    env.configPath,
		Git:           env.git,
		SessionRunner: env.runner,
		HookRunner:    runner,
		Clock:         func() time.Time { return env.now },
		Logf:          func(string, ...any) {},
	})

	result, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		Name:       "repo-a",
		NameSet:    true,
		SourcePath: local,
	})
	if err != nil {
		t.Fatalf("add repo: %v", err)
	}
	if len(result.PendingHooks) != 0 {
		t.Fatalf("expected no pending hooks")
	}
	if runner.calls == 0 {
		t.Fatalf("expected hook runner to run")
	}
}

func TestAddRepoPendingHooksWhenDisabled(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")
	env.git.worktreeAddHook = func(path string) error {
		hooksDir := filepath.Join(path, ".workset")
		if err := os.MkdirAll(hooksDir, 0o755); err != nil {
			return err
		}
		data := []byte("hooks:\n  - id: bootstrap\n    on: [worktree.created]\n    run: [\"npm\", \"ci\"]\n")
		return os.WriteFile(filepath.Join(hooksDir, "hooks.yaml"), data, 0o644)
	}

	cfg := env.loadConfig()
	cfg.Hooks.Enabled = false
	env.saveConfig(cfg)

	result, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		Name:       "repo-a",
		NameSet:    true,
		SourcePath: local,
	})
	if err != nil {
		t.Fatalf("add repo: %v", err)
	}
	if len(result.PendingHooks) != 1 {
		t.Fatalf("expected pending hooks")
	}
	if result.PendingHooks[0].Reason != "disabled" || result.PendingHooks[0].Status != HookRunStatusSkipped {
		t.Fatalf("expected skipped/disabled status")
	}
}

func TestRemoveRepoUnmanagedLocalRequiresForce(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")

	if _, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		Name:       "repo-a",
		NameSet:    true,
		SourcePath: local,
	}); err != nil {
		t.Fatalf("add repo: %v", err)
	}

	_, err := env.svc.RemoveRepo(context.Background(), RepoRemoveInput{
		Workspace:   WorkspaceSelector{Value: root},
		Name:        "repo-a",
		DeleteLocal: true,
	})
	_ = requireErrorType[UnsafeOperation](t, err)
}

type stubHookRunner struct {
	calls int
}

func (s *stubHookRunner) Run(_ context.Context, _ hooks.RunRequest) error {
	s.calls++
	return nil
}

func TestRemoveRepoUnmergedUnsafe(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")

	if _, err := env.svc.AddRepo(context.Background(), RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		Name:       "repo-a",
		NameSet:    true,
		SourcePath: local,
	}); err != nil {
		t.Fatalf("add repo: %v", err)
	}

	repoPath, err := resolveLocalPathInput(local)
	if err != nil {
		t.Fatalf("resolve local path: %v", err)
	}
	branch := "demo"
	baseRef := "refs/remotes/origin/main"
	branchRef := "refs/heads/" + branch
	writeRef := "refs/remotes/origin/" + branch
	env.git.ancestors[refKey(repoPath, branchRef+"->"+baseRef)] = false
	env.git.contentMerged[refKey(repoPath, branchRef+"->"+baseRef)] = false
	env.git.refs[refKey(repoPath, writeRef)] = false

	_, err = env.svc.RemoveRepo(context.Background(), RepoRemoveInput{
		Workspace:       WorkspaceSelector{Value: root},
		Name:            "repo-a",
		DeleteWorktrees: true,
	})
	_ = requireErrorType[UnsafeOperation](t, err)
}

func TestUpdateRepoRemotesValidation(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	_, _, err := env.svc.UpdateRepoRemotes(context.Background(), RepoRemotesUpdateInput{
		Workspace: WorkspaceSelector{Value: root},
	})
	_ = requireErrorType[ValidationError](t, err)
}
