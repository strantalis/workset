package worksetapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/session"
)

type testEnv struct {
	t             *testing.T
	root          string
	configPath    string
	workspaceRoot string
	repoRoot      string
	now           time.Time
	git           *fakeGit
	runner        *stubRunner
	svc           *Service
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspaces")
	repoRoot := filepath.Join(root, "repos")
	cfgPath := filepath.Join(root, "config.yaml")
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	cfg := config.GlobalConfig{
		Defaults: config.Defaults{
			BaseBranch:        "main",
			WorkspaceRoot:     workspaceRoot,
			RepoStoreRoot:     repoRoot,
			SessionNameFormat: "workset-{workspace}",
			Agent:             "codex",
		},
	}
	if err := config.SaveGlobal(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	fakeGit := newFakeGit()
	stubRunner := newStubRunner()
	svc := NewService(Options{
		ConfigPath:    cfgPath,
		Git:           fakeGit,
		SessionRunner: stubRunner,
		Clock:         func() time.Time { return now },
		Logf:          func(string, ...any) {},
	})

	return &testEnv{
		t:             t,
		root:          root,
		configPath:    cfgPath,
		workspaceRoot: workspaceRoot,
		repoRoot:      repoRoot,
		now:           now,
		git:           fakeGit,
		runner:        stubRunner,
		svc:           svc,
	}
}

func (e *testEnv) loadConfig() config.GlobalConfig {
	e.t.Helper()
	cfg, _, err := config.LoadGlobalWithInfo(e.configPath)
	if err != nil {
		e.t.Fatalf("load config: %v", err)
	}
	return cfg
}

func (e *testEnv) saveConfig(cfg config.GlobalConfig) {
	e.t.Helper()
	if err := config.SaveGlobal(e.configPath, cfg); err != nil {
		e.t.Fatalf("save config: %v", err)
	}
}

func (e *testEnv) createLocalRepo(name string) string {
	e.t.Helper()
	path := filepath.Join(e.root, "local", name)
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		e.t.Fatalf("create repo dir: %v", err)
	}
	return path
}

func (e *testEnv) createWorkspace(ctx context.Context, name string) string {
	e.t.Helper()
	result, err := e.svc.CreateWorkspace(ctx, WorkspaceCreateInput{Name: name})
	if err != nil {
		e.t.Fatalf("create workspace: %v", err)
	}
	return result.Workspace.Path
}

type fakeGit struct {
	status          map[string]git.StatusSummary
	statusErr       map[string]error
	refs            map[string]bool
	remotes         map[string][]string
	remoteExists    map[string]map[string]bool
	ancestors       map[string]bool
	contentMerged   map[string]bool
	currentBranch   map[string]string
	currentOK       map[string]bool
	worktreeAdds    []git.WorktreeAddOptions
	worktreeRemovs  []worktreeRemoveCall
	worktreeAddHook func(path string) error
}

type worktreeRemoveCall struct {
	repoPath string
	name     string
}

func newFakeGit() *fakeGit {
	return &fakeGit{
		status:        map[string]git.StatusSummary{},
		statusErr:     map[string]error{},
		refs:          map[string]bool{},
		remotes:       map[string][]string{},
		remoteExists:  map[string]map[string]bool{},
		ancestors:     map[string]bool{},
		contentMerged: map[string]bool{},
		currentBranch: map[string]string{},
		currentOK:     map[string]bool{},
	}
}

func (f *fakeGit) Clone(_ context.Context, _ string, path, _ string) error {
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		return err
	}
	return nil
}

func (f *fakeGit) CloneBare(_ context.Context, _ string, path, _ string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(path, "HEAD"), []byte("ref: refs/heads/main"), 0o644)
}

func (f *fakeGit) AddRemote(_ string, _ string, _ string) error {
	return nil
}

func (f *fakeGit) RemoteNames(repoPath string) ([]string, error) {
	if remotes, ok := f.remotes[repoPath]; ok {
		return remotes, nil
	}
	return []string{"origin"}, nil
}

func (f *fakeGit) ReferenceExists(repoPath, ref string) (bool, error) {
	if ok, exists := f.refs[refKey(repoPath, ref)]; exists {
		return ok, nil
	}
	return true, nil
}

func (f *fakeGit) Fetch(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *fakeGit) Status(path string) (git.StatusSummary, error) {
	if err, ok := f.statusErr[path]; ok {
		return git.StatusSummary{}, err
	}
	if status, ok := f.status[path]; ok {
		return status, nil
	}
	return git.StatusSummary{}, nil
}

func (f *fakeGit) IsRepo(path string) (bool, error) {
	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		return true, nil
	}
	return false, nil
}

func (f *fakeGit) IsAncestor(repoPath, ancestorRef, descendantRef string) (bool, error) {
	if ok, exists := f.ancestors[refKey(repoPath, ancestorRef+"->"+descendantRef)]; exists {
		return ok, nil
	}
	return true, nil
}

func (f *fakeGit) IsContentMerged(repoPath, branchRef, baseRef string) (bool, error) {
	if ok, exists := f.contentMerged[refKey(repoPath, branchRef+"->"+baseRef)]; exists {
		return ok, nil
	}
	return true, nil
}

func (f *fakeGit) CurrentBranch(repoPath string) (string, bool, error) {
	if branch, ok := f.currentBranch[repoPath]; ok {
		return branch, f.currentOK[repoPath], nil
	}
	return "", false, nil
}

func (f *fakeGit) RemoteExists(repoPath, remoteName string) (bool, error) {
	if remoteName == "" {
		return false, nil
	}
	if repoRemotes, ok := f.remoteExists[repoPath]; ok {
		if exists, ok := repoRemotes[remoteName]; ok {
			return exists, nil
		}
	}
	return true, nil
}

func (f *fakeGit) WorktreeAdd(_ context.Context, opts git.WorktreeAddOptions) error {
	if err := os.MkdirAll(opts.WorktreePath, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(opts.WorktreePath, ".git"), 0o755); err != nil {
		return err
	}
	if f.worktreeAddHook != nil {
		if err := f.worktreeAddHook(opts.WorktreePath); err != nil {
			return err
		}
	}
	f.worktreeAdds = append(f.worktreeAdds, opts)
	return nil
}

func (f *fakeGit) WorktreeRemove(repoPath, worktreeName string) error {
	f.worktreeRemovs = append(f.worktreeRemovs, worktreeRemoveCall{
		repoPath: repoPath,
		name:     worktreeName,
	})
	return nil
}

func (f *fakeGit) WorktreeList(_ string) ([]string, error) {
	return nil, nil
}

type stubRunner struct {
	lookPath map[string]error
	results  map[string]session.CommandResult
	errors   map[string]error
	calls    []session.CommandSpec
}

func newStubRunner() *stubRunner {
	return &stubRunner{
		lookPath: map[string]error{},
		results:  map[string]session.CommandResult{},
		errors:   map[string]error{},
	}
}

func (r *stubRunner) LookPath(name string) error {
	if err, ok := r.lookPath[name]; ok {
		return err
	}
	return nil
}

func (r *stubRunner) Run(_ context.Context, spec session.CommandSpec) (session.CommandResult, error) {
	r.calls = append(r.calls, spec)
	key := commandKey(spec.Name, spec.Args)
	if result, ok := r.results[key]; ok {
		return result, r.errors[key]
	}
	if err, ok := r.errors[key]; ok {
		return session.CommandResult{}, err
	}
	return session.CommandResult{ExitCode: 0}, nil
}

func commandKey(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

func refKey(repoPath, ref string) string {
	return fmt.Sprintf("%s::%s", repoPath, ref)
}

func requireErrorType[T error](t *testing.T, err error) T {
	t.Helper()
	var target T
	if !errors.As(err, &target) {
		t.Fatalf("expected %T, got %v", target, err)
	}
	return target
}
