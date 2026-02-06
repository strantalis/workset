package worksetapi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

func TestListWorkspacesSorted(t *testing.T) {
	env := newTestEnv(t)
	cfg := env.loadConfig()
	cfg.Workspaces = map[string]config.WorkspaceRef{
		"beta":  {Path: "/tmp/beta"},
		"alpha": {Path: "/tmp/alpha"},
	}
	env.saveConfig(cfg)

	result, err := env.svc.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	if len(result.Workspaces) != 2 {
		t.Fatalf("expected 2 workspaces")
	}
	if result.Workspaces[0].Name != "alpha" || result.Workspaces[1].Name != "beta" {
		t.Fatalf("unexpected order: %+v", result.Workspaces)
	}
}

func TestListWorkspacesFiltersArchived(t *testing.T) {
	env := newTestEnv(t)
	cfg := env.loadConfig()
	cfg.Workspaces = map[string]config.WorkspaceRef{
		"alpha": {Path: "/tmp/alpha"},
		"beta":  {Path: "/tmp/beta", ArchivedAt: "2024-01-02T03:04:05Z"},
	}
	env.saveConfig(cfg)

	result, err := env.svc.ListWorkspacesWithOptions(context.Background(), WorkspaceListOptions{IncludeArchived: false})
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	if len(result.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(result.Workspaces))
	}
	if result.Workspaces[0].Name != "alpha" {
		t.Fatalf("unexpected workspace: %+v", result.Workspaces[0])
	}
}

func TestCreateWorkspaceDefaultPath(t *testing.T) {
	env := newTestEnv(t)
	result, err := env.svc.CreateWorkspace(context.Background(), WorkspaceCreateInput{Name: "demo"})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if result.Workspace.Name != "demo" {
		t.Fatalf("unexpected name: %s", result.Workspace.Name)
	}
	if result.Workspace.Path == "" || result.Workspace.Workset == "" {
		t.Fatalf("missing path/workset")
	}
	if _, err := os.Stat(result.Workspace.Workset); err != nil {
		t.Fatalf("workset file missing: %v", err)
	}
	cfg := env.loadConfig()
	if _, ok := cfg.Workspaces["demo"]; !ok {
		t.Fatalf("workspace not registered")
	}
}

func TestCreateWorkspaceDefaultPathSanitizesName(t *testing.T) {
	env := newTestEnv(t)
	result, err := env.svc.CreateWorkspace(context.Background(), WorkspaceCreateInput{Name: "fix/ws-test"})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if result.Workspace.Name != "fix/ws-test" {
		t.Fatalf("unexpected name: %s", result.Workspace.Name)
	}
	expected := filepath.Join(env.workspaceRoot, workspace.WorkspaceDirName("fix/ws-test"))
	if result.Workspace.Path != expected {
		t.Fatalf("unexpected path: got %s want %s", result.Workspace.Path, expected)
	}
	cfg := env.loadConfig()
	if _, ok := cfg.Workspaces["fix/ws-test"]; !ok {
		t.Fatalf("workspace not registered")
	}
}

func TestCreateWorkspaceValidation(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.svc.CreateWorkspace(context.Background(), WorkspaceCreateInput{})
	_ = requireErrorType[ValidationError](t, err)
}

func TestCreateWorkspaceWithGroupRepos(t *testing.T) {
	env := newTestEnv(t)
	local := env.createLocalRepo("repo-a")
	cfg := env.loadConfig()
	cfg.Repos = map[string]config.RegisteredRepo{
		"repo-a": {Path: local},
	}
	cfg.Groups = map[string]config.Group{
		"core": {
			Members: []config.GroupMember{
				{Repo: "repo-a"},
			},
		},
	}
	env.saveConfig(cfg)

	result, err := env.svc.CreateWorkspace(context.Background(), WorkspaceCreateInput{
		Name:   "demo",
		Groups: []string{"core"},
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(result.Workspace.Path))
	if err != nil {
		t.Fatalf("load workspace config: %v", err)
	}
	if len(wsCfg.Repos) != 1 || wsCfg.Repos[0].Name != "repo-a" {
		t.Fatalf("expected repo from group")
	}
}

func TestCreateWorkspaceWarnsOutsideRoot(t *testing.T) {
	env := newTestEnv(t)
	outside := filepath.Join(env.root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	result, err := env.svc.CreateWorkspace(context.Background(), WorkspaceCreateInput{
		Name: "outside",
		Path: outside,
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected warning for outside workspace root")
	}
}

func TestCreateWorkspacePendingHooks(t *testing.T) {
	env := newTestEnv(t)
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
	cfg.Repos = map[string]config.RegisteredRepo{
		"repo-a": {Path: local},
	}
	env.saveConfig(cfg)

	result, err := env.svc.CreateWorkspace(context.Background(), WorkspaceCreateInput{
		Name:  "demo",
		Repos: []string{"repo-a"},
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if len(result.PendingHooks) != 1 {
		t.Fatalf("expected pending hooks")
	}
	if result.PendingHooks[0].Repo != "repo-a" {
		t.Fatalf("unexpected pending repo: %s", result.PendingHooks[0].Repo)
	}
	if result.PendingHooks[0].Status != HookRunStatusSkipped || result.PendingHooks[0].Reason != "untrusted" {
		t.Fatalf("expected skipped/untrusted status")
	}
}

func TestCreateWorkspaceRunsTrustedHooks(t *testing.T) {
	env := newTestEnv(t)
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
	cfg.Repos = map[string]config.RegisteredRepo{
		"repo-a": {Path: local},
	}
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

	result, err := env.svc.CreateWorkspace(context.Background(), WorkspaceCreateInput{
		Name:  "demo",
		Repos: []string{"repo-a"},
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if len(result.PendingHooks) != 0 {
		t.Fatalf("expected no pending hooks")
	}
	if len(result.HookRuns) != 1 {
		t.Fatalf("expected hook runs, got %d", len(result.HookRuns))
	}
	if result.HookRuns[0].Repo != "repo-a" || result.HookRuns[0].Event != "worktree.created" {
		t.Fatalf("unexpected hook run payload: %+v", result.HookRuns[0])
	}
	if runner.calls == 0 {
		t.Fatalf("expected hook runner to run")
	}
}

func TestCreateWorkspaceRegistersOnRepoFailure(t *testing.T) {
	env := newTestEnv(t)
	local := env.createLocalRepo("repo-a")
	env.git.worktreeAddHook = func(string) error {
		return errors.New("no auth")
	}

	cfg := env.loadConfig()
	cfg.Repos = map[string]config.RegisteredRepo{
		"repo-a": {Path: local},
	}
	env.saveConfig(cfg)

	_, err := env.svc.CreateWorkspace(context.Background(), WorkspaceCreateInput{
		Name:  "demo",
		Repos: []string{"repo-a"},
	})
	if err == nil {
		t.Fatalf("expected create workspace to fail")
	}

	cfg = env.loadConfig()
	if _, ok := cfg.Workspaces["demo"]; !ok {
		t.Fatalf("workspace not registered after failure")
	}
}

func TestDeleteWorkspaceRequiresConfirmation(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	_, err := env.svc.DeleteWorkspace(context.Background(), WorkspaceDeleteInput{
		Selector:    WorkspaceSelector{Value: root},
		DeleteFiles: true,
		Confirmed:   false,
	})
	_ = requireErrorType[ConfirmationRequired](t, err)
}

func TestDeleteWorkspaceOutsideRootUnsafe(t *testing.T) {
	env := newTestEnv(t)
	outside := filepath.Join(env.root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := env.loadConfig()
	cfg.Workspaces = map[string]config.WorkspaceRef{
		"outside": {Path: outside},
	}
	env.saveConfig(cfg)

	_, err := env.svc.DeleteWorkspace(context.Background(), WorkspaceDeleteInput{
		Selector:    WorkspaceSelector{Value: "outside"},
		DeleteFiles: true,
	})
	_ = requireErrorType[UnsafeOperation](t, err)
}

func TestDeleteWorkspaceRefusesConfigPath(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	_, err := env.svc.CreateWorkspace(ctx, WorkspaceCreateInput{
		Name: "config-root",
		Path: env.root,
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	_, err = env.svc.DeleteWorkspace(ctx, WorkspaceDeleteInput{
		Selector:    WorkspaceSelector{Value: "config-root"},
		DeleteFiles: true,
		Force:       true,
		Confirmed:   true,
	})
	_ = requireErrorType[UnsafeOperation](t, err)
}

func TestDeleteWorkspaceMissingConfigDoesNotWriteDefaults(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("HOME", env.root)
	if err := os.Remove(env.configPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove config: %v", err)
	}
	target := filepath.Join(env.root, ".workset", "workspaces", "orphan")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir orphan: %v", err)
	}

	_, err := env.svc.DeleteWorkspace(context.Background(), WorkspaceDeleteInput{
		Selector:    WorkspaceSelector{Value: "orphan"},
		DeleteFiles: false,
	})
	if err != nil {
		t.Fatalf("delete workspace: %v", err)
	}
	if _, err := os.Stat(env.configPath); err == nil {
		t.Fatalf("expected config to remain missing")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat config: %v", err)
	}
}

func TestDeleteWorkspaceContainingOtherWorkspaceUnsafe(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	childRoot := env.createWorkspace(ctx, "child")

	_, err := env.svc.CreateWorkspace(ctx, WorkspaceCreateInput{
		Name: "parent",
		Path: env.workspaceRoot,
	})
	if err != nil {
		t.Fatalf("create parent workspace: %v", err)
	}
	if !strings.HasPrefix(childRoot, env.workspaceRoot+string(os.PathSeparator)) {
		t.Fatalf("expected child workspace under %s, got %s", env.workspaceRoot, childRoot)
	}

	_, err = env.svc.DeleteWorkspace(ctx, WorkspaceDeleteInput{
		Selector:    WorkspaceSelector{Value: "parent"},
		DeleteFiles: true,
		Force:       true,
		Confirmed:   true,
	})
	_ = requireErrorType[UnsafeOperation](t, err)
}

func TestDeleteWorkspaceRemovesRegistration(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	result, err := env.svc.DeleteWorkspace(context.Background(), WorkspaceDeleteInput{
		Selector:    WorkspaceSelector{Value: "demo"},
		DeleteFiles: false,
	})
	if err != nil {
		t.Fatalf("delete workspace: %v", err)
	}
	if result.Payload.Path != root {
		t.Fatalf("unexpected payload: %+v", result.Payload)
	}
	cfg := env.loadConfig()
	if _, ok := cfg.Workspaces["demo"]; ok {
		t.Fatalf("workspace registration not removed")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("workspace should still exist: %v", err)
	}
}

func TestStatusWorkspaceReportsDirtyAndMissing(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("load workspace config: %v", err)
	}
	wsCfg.Repos = []config.RepoConfig{
		{Name: "repo-a", RepoDir: "repo-a"},
		{Name: "repo-b", RepoDir: "repo-b"},
	}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "repo-a"), 0o755); err != nil {
		t.Fatalf("mkdir repo-a: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "repo-b"), 0o755); err != nil {
		t.Fatalf("mkdir repo-b: %v", err)
	}
	env.git.status[filepath.Join(root, "repo-a")] = git.StatusSummary{Dirty: true}
	env.git.status[filepath.Join(root, "repo-b")] = git.StatusSummary{Missing: true}

	result, err := env.svc.StatusWorkspace(context.Background(), WorkspaceSelector{Value: root})
	if err != nil {
		t.Fatalf("status workspace: %v", err)
	}
	if len(result.Statuses) != 2 {
		t.Fatalf("expected 2 statuses")
	}
	if !result.Statuses[0].Dirty && !result.Statuses[1].Dirty {
		t.Fatalf("expected dirty status")
	}
	if !result.Statuses[0].Missing && !result.Statuses[1].Missing {
		t.Fatalf("expected missing status")
	}
}

func TestDeleteWorkspaceDeletesFiles(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	_, err := env.svc.DeleteWorkspace(context.Background(), WorkspaceDeleteInput{
		Selector:    WorkspaceSelector{Value: root},
		DeleteFiles: true,
		Force:       true,
		Confirmed:   true,
	})
	if err != nil {
		t.Fatalf("delete workspace: %v", err)
	}
	if _, err := os.Stat(root); err == nil {
		t.Fatalf("expected workspace deleted")
	}
}

func TestResolveWorkspaceErrors(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	cfg := env.loadConfig()

	_, _, err := env.svc.resolveWorkspace(ctx, &cfg, env.configPath, WorkspaceSelector{Value: "missing"})
	_ = requireErrorType[NotFoundError](t, err)

	root := env.createWorkspace(ctx, "demo")
	cfg = env.loadConfig()
	cfg.Workspaces["demo"] = config.WorkspaceRef{Path: filepath.Join(env.root, "other")}
	_, _, err = env.svc.resolveWorkspace(ctx, &cfg, env.configPath, WorkspaceSelector{Value: root})
	_ = requireErrorType[ConflictError](t, err)
}

func TestWarnOutsideWorkspaceRoot(t *testing.T) {
	warnings := warnOutsideWorkspaceRoot("/tmp/demo", "/tmp")
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	warnings = warnOutsideWorkspaceRoot("/var/demo", "/tmp")
	if len(warnings) == 0 {
		t.Fatalf("expected warning outside workspace root")
	}
}
