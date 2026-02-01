package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

type fakeGit struct {
	statuses            map[string]git.StatusSummary
	statusErrs          map[string]error
	remoteExists        bool
	referenceExists     map[string]bool
	ancestors           map[string]bool
	contentMerged       bool
	worktreeRemoveCalls []git.WorktreeRemoveOptions
	worktreeRemoveErr   error
}

func newFakeGit() *fakeGit {
	return &fakeGit{
		statuses:        map[string]git.StatusSummary{},
		statusErrs:      map[string]error{},
		referenceExists: map[string]bool{},
		ancestors:       map[string]bool{},
	}
}

func (f *fakeGit) Clone(_ context.Context, _ string, _ string, _ string) error { return nil }
func (f *fakeGit) CloneBare(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
func (f *fakeGit) AddRemote(_ string, _ string, _ string) error { return nil }
func (f *fakeGit) RemoteNames(_ string) ([]string, error)       { return nil, nil }
func (f *fakeGit) RemoteURLs(_ string, _ string) ([]string, error) {
	return nil, nil
}

func (f *fakeGit) ReferenceExists(_ context.Context, repoPath, ref string) (bool, error) {
	if ok, exists := f.referenceExists[repoPath+"|"+ref]; exists {
		return ok, nil
	}
	return true, nil
}
func (f *fakeGit) Fetch(_ context.Context, _ string, _ string) error { return nil }
func (f *fakeGit) UpdateBranch(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (f *fakeGit) Status(path string) (git.StatusSummary, error) {
	if err, ok := f.statusErrs[path]; ok {
		return git.StatusSummary{}, err
	}
	if status, ok := f.statuses[path]; ok {
		return status, nil
	}
	return git.StatusSummary{}, nil
}
func (f *fakeGit) IsRepo(_ string) (bool, error) { return true, nil }
func (f *fakeGit) IsAncestor(repoPath, ancestorRef, descendantRef string) (bool, error) {
	if ok, exists := f.ancestors[repoPath+"|"+ancestorRef+"->"+descendantRef]; exists {
		return ok, nil
	}
	return true, nil
}

func (f *fakeGit) IsContentMerged(_ string, _ string, _ string) (bool, error) {
	return f.contentMerged, nil
}
func (f *fakeGit) CurrentBranch(_ string) (string, bool, error)  { return "", false, nil }
func (f *fakeGit) RemoteExists(_ string, _ string) (bool, error) { return f.remoteExists, nil }
func (f *fakeGit) WorktreeAdd(_ context.Context, _ git.WorktreeAddOptions) error {
	return nil
}

func (f *fakeGit) WorktreeRemove(opts git.WorktreeRemoveOptions) error {
	f.worktreeRemoveCalls = append(f.worktreeRemoveCalls, opts)
	return f.worktreeRemoveErr
}
func (f *fakeGit) WorktreeList(_ string) ([]string, error) { return nil, nil }

func TestListBranchesUsesWorkspaceStateWhenMissingWorktrees(t *testing.T) {
	root := t.TempDir()
	defaults := config.DefaultConfig().Defaults
	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}
	branches, err := listBranches(root, defaults)
	if err != nil {
		t.Fatalf("listBranches: %v", err)
	}
	if len(branches) != 1 || branches[0] != "demo" {
		t.Fatalf("expected demo branch, got %v", branches)
	}
}

func TestListBranchesUsesMetaAndDirNames(t *testing.T) {
	root := t.TempDir()
	defaults := config.DefaultConfig().Defaults
	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}
	worktrees := workspace.WorktreesPath(root)
	if err := os.MkdirAll(worktrees, 0o755); err != nil {
		t.Fatalf("mkdir worktrees: %v", err)
	}
	metaDir := filepath.Join(worktrees, "feature__one")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatalf("mkdir meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, ".workset-branch"), []byte("feature/one"), 0o644); err != nil {
		t.Fatalf("write meta: %v", err)
	}
	dirOnly := filepath.Join(worktrees, "hotfix__two")
	if err := os.MkdirAll(dirOnly, 0o755); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}
	branches, err := listBranches(root, defaults)
	if err != nil {
		t.Fatalf("listBranches: %v", err)
	}
	sort.Strings(branches)
	expected := []string{"feature/one", "hotfix/two"}
	if strings.Join(branches, ",") != strings.Join(expected, ",") {
		t.Fatalf("expected %v, got %v", expected, branches)
	}
}

func TestRemoveIfEmpty(t *testing.T) {
	root := t.TempDir()
	empty := filepath.Join(root, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatalf("mkdir empty: %v", err)
	}
	if err := removeIfEmpty(empty); err != nil {
		t.Fatalf("removeIfEmpty: %v", err)
	}
	if _, err := os.Stat(empty); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected empty dir removed, err=%v", err)
	}

	nonEmpty := filepath.Join(root, "non-empty")
	if err := os.MkdirAll(nonEmpty, 0o755); err != nil {
		t.Fatalf("mkdir non-empty: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nonEmpty, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := removeIfEmpty(nonEmpty); err != nil {
		t.Fatalf("removeIfEmpty non-empty: %v", err)
	}
	if _, err := os.Stat(nonEmpty); err != nil {
		t.Fatalf("expected non-empty dir to remain: %v", err)
	}
}

func TestWorktreeAdminFromPath(t *testing.T) {
	root := t.TempDir()
	worktreePath := filepath.Join(root, "wt")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	gitDir := filepath.Join(root, "repo", ".git", "worktrees", "wt")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir gitdir: %v", err)
	}
	gitFile := filepath.Join(worktreePath, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: "+gitDir), 0o644); err != nil {
		t.Fatalf("write git file: %v", err)
	}
	commonDir, worktreeName, ok, err := worktreeAdminFromPath(worktreePath)
	if err != nil {
		t.Fatalf("worktreeAdminFromPath: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if commonDir != gitDir {
		t.Fatalf("expected commonDir %s, got %s", gitDir, commonDir)
	}
	if worktreeName != "wt" {
		t.Fatalf("expected worktree name wt, got %s", worktreeName)
	}
}

func TestWorktreeAdminFromPathInvalidGitFile(t *testing.T) {
	root := t.TempDir()
	worktreePath := filepath.Join(root, "wt")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("oops"), 0o644); err != nil {
		t.Fatalf("write git file: %v", err)
	}
	_, _, _, err := worktreeAdminFromPath(worktreePath)
	if err == nil {
		t.Fatalf("expected invalid .git error")
	}
}

func TestFindWorktreePathsSkipsDepthAndWorkset(t *testing.T) {
	root := t.TempDir()
	worktreePath := filepath.Join(root, "wt1")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /tmp/common/worktrees/wt1"), 0o644); err != nil {
		t.Fatalf("write git file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".workset", "skip"), 0o755); err != nil {
		t.Fatalf("mkdir workset: %v", err)
	}
	deep := filepath.Join(root, "a", "b", "c", "d")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deep, ".git"), []byte("gitdir: /tmp/common/worktrees/deep"), 0o644); err != nil {
		t.Fatalf("write deep git file: %v", err)
	}
	paths, err := findWorktreePaths(root)
	if err != nil {
		t.Fatalf("findWorktreePaths: %v", err)
	}
	if len(paths) != 1 || paths[0] != worktreePath {
		t.Fatalf("expected %s only, got %v", worktreePath, paths)
	}
}

func TestFindWorktreeNameByPath(t *testing.T) {
	root := t.TempDir()
	commonGitDir := filepath.Join(root, "repo", ".git")
	worktreesDir := filepath.Join(commonGitDir, "worktrees", "wt1")
	if err := os.MkdirAll(worktreesDir, 0o755); err != nil {
		t.Fatalf("mkdir worktrees: %v", err)
	}
	worktreePath := filepath.Join(root, "worktree1")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	gitdirPath := filepath.Join(worktreesDir, "gitdir")
	if err := os.WriteFile(gitdirPath, []byte(filepath.Join(worktreePath, ".git")), 0o644); err != nil {
		t.Fatalf("write gitdir: %v", err)
	}
	name, ok, err := findWorktreeNameByPath(commonGitDir, worktreePath)
	if err != nil {
		t.Fatalf("findWorktreeNameByPath: %v", err)
	}
	if !ok || name != "wt1" {
		t.Fatalf("expected wt1, got %q ok=%t", name, ok)
	}
}

func TestCleanupWorkspaceWorktreesRemovesWorktree(t *testing.T) {
	root := t.TempDir()
	worktreePath := filepath.Join(root, "wt1")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	commonGitDir := filepath.Join(root, "repo", ".git")
	worktreeAdmin := filepath.Join(commonGitDir, "worktrees", "wt1")
	if err := os.MkdirAll(worktreeAdmin, 0o755); err != nil {
		t.Fatalf("mkdir admin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: "+worktreeAdmin), 0o644); err != nil {
		t.Fatalf("write git file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreeAdmin, "commondir"), []byte(commonGitDir), 0o644); err != nil {
		t.Fatalf("write commondir: %v", err)
	}
	fake := newFakeGit()
	if err := CleanupWorkspaceWorktrees(CleanupWorkspaceWorktreesInput{
		WorkspaceRoot: root,
		Git:           fake,
		Force:         false,
	}); err != nil {
		t.Fatalf("CleanupWorkspaceWorktrees: %v", err)
	}
	if len(fake.worktreeRemoveCalls) != 1 {
		t.Fatalf("expected 1 worktree remove, got %d", len(fake.worktreeRemoveCalls))
	}
	call := fake.worktreeRemoveCalls[0]
	if call.RepoPath != commonGitDir || call.WorktreeName != "wt1" {
		t.Fatalf("unexpected remove call: %+v", call)
	}
}

func TestCheckRepoSafetyHappyPath(t *testing.T) {
	root := t.TempDir()
	defaults := config.DefaultConfig().Defaults
	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}
	worktrees := workspace.WorktreesPath(root)
	if err := os.MkdirAll(worktrees, 0o755); err != nil {
		t.Fatalf("mkdir worktrees: %v", err)
	}
	branchDir := filepath.Join(worktrees, "main")
	repoDir := filepath.Join(branchDir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	fake := newFakeGit()
	fake.remoteExists = true
	fake.statuses[repoDir] = git.StatusSummary{Dirty: true}
	report, err := CheckRepoSafety(context.Background(), RepoSafetyInput{
		WorkspaceRoot: root,
		Repo:          config.RepoConfig{Name: "repo", RepoDir: "repo"},
		Defaults:      defaults,
		RepoDefaults:  RepoDefaults{Remote: defaults.Remote, DefaultBranch: defaults.BaseBranch},
		Git:           fake,
	})
	if err != nil {
		t.Fatalf("CheckRepoSafety: %v", err)
	}
	if len(report.Branches) != 1 {
		t.Fatalf("expected 1 branch, got %d", len(report.Branches))
	}
	if !report.Branches[0].Dirty {
		t.Fatalf("expected dirty branch")
	}
}

func TestCheckRepoSafetyMissingRemote(t *testing.T) {
	root := t.TempDir()
	defaults := config.Defaults{}
	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}
	worktrees := workspace.WorktreesPath(root)
	if err := os.MkdirAll(worktrees, 0o755); err != nil {
		t.Fatalf("mkdir worktrees: %v", err)
	}
	branchDir := filepath.Join(worktrees, "main")
	repoDir := filepath.Join(branchDir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	_, err := CheckRepoSafety(context.Background(), RepoSafetyInput{
		WorkspaceRoot: root,
		Repo:          config.RepoConfig{Name: "repo", RepoDir: "repo"},
		Defaults:      defaults,
		Git:           newFakeGit(),
	})
	if err == nil {
		t.Fatalf("expected remote required error")
	}
}

func TestCheckWorkspaceSafetyAggregatesRepos(t *testing.T) {
	root := t.TempDir()
	defaults := config.DefaultConfig().Defaults
	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}
	repos := []config.RepoConfig{
		{Name: "repo-a", RepoDir: "repo-a"},
		{Name: "repo-b", RepoDir: "repo-b"},
	}
	wsConfig, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	wsConfig.Repos = repos
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsConfig); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	for _, repo := range repos {
		if err := os.MkdirAll(filepath.Join(root, repo.RepoDir), 0o755); err != nil {
			t.Fatalf("mkdir repo: %v", err)
		}
	}
	fake := newFakeGit()
	fake.remoteExists = true
	report, err := CheckWorkspaceSafety(context.Background(), WorkspaceSafetyInput{
		WorkspaceRoot: root,
		Defaults:      defaults,
		RepoDefaults: map[string]RepoDefaults{
			"repo-a": {Remote: defaults.Remote, DefaultBranch: defaults.BaseBranch},
			"repo-b": {Remote: defaults.Remote, DefaultBranch: defaults.BaseBranch},
		},
		Git: fake,
	})
	if err != nil {
		t.Fatalf("CheckWorkspaceSafety: %v", err)
	}
	if len(report.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(report.Repos))
	}
}

func TestRemoveRepoInputValidation(t *testing.T) {
	ctx := context.Background()
	if _, err := RemoveRepo(ctx, RemoveRepoInput{Name: "repo", Git: newFakeGit()}); err == nil {
		t.Fatalf("expected workspace root error")
	}
	if _, err := RemoveRepo(ctx, RemoveRepoInput{WorkspaceRoot: "root", Git: newFakeGit()}); err == nil {
		t.Fatalf("expected repo name error")
	}
	if _, err := RemoveRepo(ctx, RemoveRepoInput{WorkspaceRoot: "root", Name: "repo"}); err == nil {
		t.Fatalf("expected git client error")
	}
}

func TestRemoveRepoNotFound(t *testing.T) {
	root := t.TempDir()
	defaults := config.DefaultConfig().Defaults
	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}
	_, err := RemoveRepo(context.Background(), RemoveRepoInput{
		WorkspaceRoot: root,
		Name:          "missing",
		Defaults:      defaults,
		Git:           newFakeGit(),
	})
	if err == nil {
		t.Fatalf("expected repo not found error")
	}
}

func TestRemoveRepoDeleteLocalGuards(t *testing.T) {
	root := t.TempDir()
	defaults := config.DefaultConfig().Defaults
	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}
	repoPath := filepath.Join(root, "repo-a")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	wsConfig, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	wsConfig.Repos = []config.RepoConfig{
		{Name: "repo-a", RepoDir: "repo-a", LocalPath: repoPath, Managed: false},
	}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsConfig); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	_, err = RemoveRepo(context.Background(), RemoveRepoInput{
		WorkspaceRoot: root,
		Name:          "repo-a",
		Defaults:      defaults,
		Git:           newFakeGit(),
		DeleteLocal:   true,
	})
	if err == nil {
		t.Fatalf("expected unmanaged repo delete refusal")
	}

	wsConfig.Repos[0].Managed = true
	wsConfig.Repos[0].LocalPath = ""
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsConfig); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	_, err = RemoveRepo(context.Background(), RemoveRepoInput{
		WorkspaceRoot: root,
		Name:          "repo-a",
		Defaults:      defaults,
		Git:           newFakeGit(),
		DeleteLocal:   true,
	})
	if err == nil {
		t.Fatalf("expected missing local_path error")
	}
}

func TestRemoveRepoDeleteLocalRemovesRepo(t *testing.T) {
	root := t.TempDir()
	defaults := config.DefaultConfig().Defaults
	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}
	repoPath := filepath.Join(root, "repo-a")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	wsConfig, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	wsConfig.Repos = []config.RepoConfig{
		{Name: "repo-a", RepoDir: "repo-a", LocalPath: repoPath, Managed: true},
	}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsConfig); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	_, err = RemoveRepo(context.Background(), RemoveRepoInput{
		WorkspaceRoot: root,
		Name:          "repo-a",
		Defaults:      defaults,
		Git:           newFakeGit(),
		DeleteLocal:   true,
	})
	if err != nil {
		t.Fatalf("RemoveRepo: %v", err)
	}
	if _, err := os.Stat(repoPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected repo path removed, err=%v", err)
	}
	updated, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if len(updated.Repos) != 0 {
		t.Fatalf("expected repo removed from config")
	}
}
