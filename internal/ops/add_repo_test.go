package ops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

func TestAddRepoLinksLocal(t *testing.T) {
	source := setupRepo(t)
	addRemote(t, source, "origin", source)
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, resolvedRemote, warnings, err := AddRepo(context.Background(), AddRepoInput{
		WorkspaceRoot: root,
		Name:          "demo-repo",
		URL:           source,
		Defaults:      defaults,
		Remote:        defaults.Remote,
		DefaultBranch: defaults.BaseBranch,
		AllowFallback: false,
		Git:           git.NewCLIClient(),
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	if resolvedRemote != "origin" {
		t.Fatalf("expected resolved remote origin, got %q", resolvedRemote)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	ws, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if len(ws.Repos) != 1 {
		t.Fatalf("expected repo in workspace")
	}
	expectedPath, err := filepath.EvalSymlinks(source)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if ws.Repos[0].LocalPath != expectedPath {
		t.Fatalf("expected local_path %s, got %s", expectedPath, ws.Repos[0].LocalPath)
	}

	agentsContent, err := os.ReadFile(workspace.AgentsFile(root))
	if err != nil {
		t.Fatalf("agents file missing: %v", err)
	}
	if !strings.Contains(string(agentsContent), "Configured Repos (from workset.yaml)") {
		t.Fatalf("agents file missing configured repos section")
	}
	if !strings.Contains(string(agentsContent), "demo-repo") {
		t.Fatalf("agents file missing repo entry")
	}
}

func TestAddRepoMissingRemoteErrors(t *testing.T) {
	source := setupRepo(t)
	addRemote(t, source, "upstream", source)
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, _, _, err := AddRepo(context.Background(), AddRepoInput{
		WorkspaceRoot: root,
		Name:          "demo-repo",
		URL:           source,
		Defaults:      defaults,
		Remote:        defaults.Remote,
		DefaultBranch: defaults.BaseBranch,
		AllowFallback: false,
		Git:           git.NewCLIClient(),
	})
	if err == nil {
		t.Fatalf("expected missing remote error")
	}
}

func TestStatusDirty(t *testing.T) {
	source := setupRepo(t)
	addRemote(t, source, "origin", source)
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, _, _, err := AddRepo(context.Background(), AddRepoInput{
		WorkspaceRoot: root,
		Name:          "demo-repo",
		URL:           source,
		Defaults:      defaults,
		Remote:        defaults.Remote,
		DefaultBranch: defaults.BaseBranch,
		AllowFallback: false,
		Git:           git.NewCLIClient(),
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	worktreePath := workspace.RepoWorktreePath(root, "demo", "demo-repo")
	if err := os.WriteFile(filepath.Join(worktreePath, "extra.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	statuses, err := Status(context.Background(), StatusInput{
		WorkspaceRoot:       root,
		Defaults:            defaults,
		RepoDefaultBranches: map[string]string{"demo-repo": defaults.BaseBranch},
		Git:                 git.NewCLIClient(),
	})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if !statuses[0].Dirty {
		t.Fatalf("expected dirty status")
	}
}

func TestAddRepoFetchesAndFastForwardsBaseBranch(t *testing.T) {
	repoRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	gitDir := filepath.Join(repoRoot, ".git")
	fake := newFakeGitClient()
	fake.remotes[gitDir] = []string{defaults.Remote}
	fake.remoteExists[key(gitDir, defaults.Remote)] = true
	fake.refs[key(gitDir, "refs/heads/"+defaults.BaseBranch)] = true
	fake.refs[key(gitDir, "refs/remotes/"+defaults.Remote+"/"+defaults.BaseBranch)] = true
	fake.ancestors[key(gitDir, "refs/heads/"+defaults.BaseBranch+"->refs/remotes/"+defaults.Remote+"/"+defaults.BaseBranch)] = true

	_, _, warnings, err := AddRepo(context.Background(), AddRepoInput{
		WorkspaceRoot: root,
		Name:          "demo-repo",
		SourcePath:    repoRoot,
		Defaults:      defaults,
		Remote:        defaults.Remote,
		DefaultBranch: defaults.BaseBranch,
		AllowFallback: false,
		Git:           fake,
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(fake.fetchCalls) != 1 {
		t.Fatalf("expected 1 fetch call, got %d", len(fake.fetchCalls))
	}
	if fake.fetchCalls[0].repoPath != gitDir || fake.fetchCalls[0].remote != defaults.Remote {
		t.Fatalf("unexpected fetch call: %+v", fake.fetchCalls[0])
	}
	if len(fake.updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(fake.updateCalls))
	}
	if fake.updateCalls[0].repoPath != gitDir ||
		fake.updateCalls[0].branch != defaults.BaseBranch ||
		fake.updateCalls[0].target != defaults.Remote+"/"+defaults.BaseBranch {
		t.Fatalf("unexpected update call: %+v", fake.updateCalls[0])
	}
}

func TestAddRepoWarnsWhenBaseBranchDiverges(t *testing.T) {
	repoRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	gitDir := filepath.Join(repoRoot, ".git")
	fake := newFakeGitClient()
	fake.remotes[gitDir] = []string{defaults.Remote}
	fake.remoteExists[key(gitDir, defaults.Remote)] = true
	fake.refs[key(gitDir, "refs/heads/"+defaults.BaseBranch)] = true
	fake.refs[key(gitDir, "refs/remotes/"+defaults.Remote+"/"+defaults.BaseBranch)] = true
	fake.ancestors[key(gitDir, "refs/heads/"+defaults.BaseBranch+"->refs/remotes/"+defaults.Remote+"/"+defaults.BaseBranch)] = false

	_, _, warnings, err := AddRepo(context.Background(), AddRepoInput{
		WorkspaceRoot: root,
		Name:          "demo-repo",
		SourcePath:    repoRoot,
		Defaults:      defaults,
		Remote:        defaults.Remote,
		DefaultBranch: defaults.BaseBranch,
		AllowFallback: false,
		Git:           fake,
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	if len(fake.updateCalls) != 0 {
		t.Fatalf("expected no update calls, got %d", len(fake.updateCalls))
	}
	found := false
	for _, warning := range warnings {
		if strings.Contains(warning, "does not fast-forward") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected fast-forward warning, got %v", warnings)
	}
}

func setupRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.name", "Tester")
	runGit(t, root, "config", "user.email", "tester@example.com")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "initial")
	return root
}

type fakeGitClient struct {
	fetchCalls []fetchCall
	updateCalls []updateCall
	refs        map[string]bool
	remotes     map[string][]string
	remoteExists map[string]bool
	ancestors   map[string]bool
}

type fetchCall struct {
	repoPath string
	remote   string
}

type updateCall struct {
	repoPath string
	branch   string
	target   string
}

func newFakeGitClient() *fakeGitClient {
	return &fakeGitClient{
		refs:         map[string]bool{},
		remotes:      map[string][]string{},
		remoteExists: map[string]bool{},
		ancestors:    map[string]bool{},
	}
}

func (f *fakeGitClient) Clone(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (f *fakeGitClient) CloneBare(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (f *fakeGitClient) AddRemote(_ string, _ string, _ string) error {
	return nil
}

func (f *fakeGitClient) RemoteNames(repoPath string) ([]string, error) {
	if remotes, ok := f.remotes[repoPath]; ok {
		return remotes, nil
	}
	return []string{}, nil
}

func (f *fakeGitClient) RemoteURLs(_ string, _ string) ([]string, error) {
	return nil, nil
}

func (f *fakeGitClient) ReferenceExists(repoPath, ref string) (bool, error) {
	if ok, exists := f.refs[key(repoPath, ref)]; exists {
		return ok, nil
	}
	return false, nil
}

func (f *fakeGitClient) Fetch(_ context.Context, repoPath, remoteName string) error {
	f.fetchCalls = append(f.fetchCalls, fetchCall{repoPath: repoPath, remote: remoteName})
	return nil
}

func (f *fakeGitClient) UpdateBranch(_ context.Context, repoPath, branchName, targetRef string) error {
	f.updateCalls = append(f.updateCalls, updateCall{repoPath: repoPath, branch: branchName, target: targetRef})
	return nil
}

func (f *fakeGitClient) Status(_ string) (git.StatusSummary, error) {
	return git.StatusSummary{}, nil
}

func (f *fakeGitClient) IsRepo(_ string) (bool, error) {
	return true, nil
}

func (f *fakeGitClient) IsAncestor(repoPath, ancestorRef, descendantRef string) (bool, error) {
	if ok, exists := f.ancestors[key(repoPath, ancestorRef+"->"+descendantRef)]; exists {
		return ok, nil
	}
	return false, nil
}

func (f *fakeGitClient) IsContentMerged(_ string, _ string, _ string) (bool, error) {
	return false, nil
}

func (f *fakeGitClient) CurrentBranch(_ string) (string, bool, error) {
	return "", false, nil
}

func (f *fakeGitClient) RemoteExists(repoPath, remoteName string) (bool, error) {
	if ok, exists := f.remoteExists[key(repoPath, remoteName)]; exists {
		return ok, nil
	}
	return false, nil
}

func (f *fakeGitClient) WorktreeAdd(_ context.Context, _ git.WorktreeAddOptions) error {
	return nil
}

func (f *fakeGitClient) WorktreeRemove(_ git.WorktreeRemoveOptions) error {
	return nil
}

func (f *fakeGitClient) WorktreeList(_ string) ([]string, error) {
	return nil, nil
}

func key(repoPath, ref string) string {
	return repoPath + "::" + ref
}

func addRemote(t *testing.T, repoPath, name, url string) {
	t.Helper()
	runGit(t, repoPath, "remote", "add", name, url)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
}
