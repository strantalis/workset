package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ggit "github.com/go-git/go-git/v6"
	ggitcfg "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
)

var worksetBin string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "workset-e2e-*")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = os.RemoveAll(tmp)
	}()

	worksetBin = filepath.Join(tmp, "workset")
	repoRoot, err := findRepoRoot()
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("go", "build", "-o", worksetBin, "./cmd/workset")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"GOMODCACHE="+filepath.Join(tmp, "gomodcache"),
		"GOCACHE="+filepath.Join(tmp, "gocache"),
		"GOSUMDB=off",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		panic(string(output))
	}

	os.Exit(m.Run())
}

func TestRepoAddFromRelativePath(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "test-repo-1"))
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}

	workDir := filepath.Join(runner.root, "run")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	relSource, err := filepath.Rel(workDir, source)
	if err != nil {
		t.Fatalf("rel path: %v", err)
	}
	if _, err := runner.runDir(workDir, "repo", "add", relSource, "-w", "demo"); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	worktreePath := filepath.Join(runner.workspaceRoot(), "demo", "test-repo-1")
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("expected worktree at repo path, got err=%v", err)
	}

	out, err := runner.run("repo", "ls", "-w", "demo")
	if err != nil {
		t.Fatalf("repo ls: %v", err)
	}
	if !strings.Contains(out, "test-repo-1") {
		t.Fatalf("repo ls missing repo: %s", out)
	}

	out, err = runner.run("repo", "ls", "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("repo ls --json: %v", err)
	}
	if !strings.Contains(out, "\"name\": \"test-repo-1\"") {
		t.Fatalf("repo ls --json missing repo: %s", out)
	}
	if !strings.Contains(out, "\"local_path\":") {
		t.Fatalf("repo ls --json missing local_path: %s", out)
	}
}

func TestRepoRemoveDeletesFiles(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "demo-repo"))
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	if _, err := runner.run("repo", "rm", "-w", "demo", "--delete-worktrees", "--yes", "demo-repo"); err != nil {
		t.Fatalf("repo rm: %v", err)
	}

	if _, err := os.Stat(source); err != nil {
		t.Fatalf("expected local repo to remain, got err=%v", err)
	}
}

func TestInterspersedFlags(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "flag-repo"))

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}

	out, err := runner.run("repo", "add", source, "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("repo add with interspersed flags: %v", err)
	}
	if !strings.Contains(out, "\"status\": \"ok\"") {
		t.Fatalf("repo add json missing status: %s", out)
	}

	if _, err := runner.run("repo", "rm", "--delete-worktrees", "--yes", "flag-repo", "-w", "demo"); err != nil {
		t.Fatalf("repo rm with interspersed -w: %v", err)
	}
}

func TestRepoAddFromURLUsesRepoStore(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "url-repo"))
	store := filepath.Join(runner.root, "repo-store")

	if _, err := runner.run("config", "set", "defaults.repo_store_root", store); err != nil {
		t.Fatalf("config set repo_store_root: %v", err)
	}
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}

	url := fileURL(source)
	if _, err := runner.run("repo", "add", "-w", "demo", url, "--name", "url-repo"); err != nil {
		t.Fatalf("repo add url: %v", err)
	}

	out, err := runner.run("repo", "ls", "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("repo ls --json: %v", err)
	}
	if !strings.Contains(out, "\"managed\": true") {
		t.Fatalf("repo ls missing managed=true: %s", out)
	}
	if !strings.Contains(out, store) {
		t.Fatalf("repo ls missing repo_store_root path: %s", out)
	}

	if _, err := runner.run("repo", "rm", "-w", "demo", "url-repo", "--delete-local", "--yes"); err != nil {
		t.Fatalf("repo rm --delete-local: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store, "url-repo")); !os.IsNotExist(err) {
		t.Fatalf("expected repo store deleted, got err=%v", err)
	}
}

func TestRepoAliasLocalAndURLFlow(t *testing.T) {
	runner := newRunner(t)
	localRepo := setupRepo(t, filepath.Join(runner.root, "src", "alias-local"))
	urlRepo := setupRepo(t, filepath.Join(runner.root, "src", "alias-url"))
	store := filepath.Join(runner.root, "repo-store")

	if _, err := runner.run("config", "set", "defaults.repo_store_root", store); err != nil {
		t.Fatalf("config set repo_store_root: %v", err)
	}
	if _, err := runner.run("repo", "alias", "add", "local-alias", localRepo); err != nil {
		t.Fatalf("alias add local: %v", err)
	}
	if _, err := runner.run("repo", "alias", "add", "url-alias", fileURL(urlRepo)); err != nil {
		t.Fatalf("alias add url: %v", err)
	}
	out, err := runner.run("repo", "alias", "ls", "--json")
	if err != nil {
		t.Fatalf("alias ls: %v", err)
	}
	if !strings.Contains(out, "\"name\": \"local-alias\"") || !strings.Contains(out, "\"name\": \"url-alias\"") {
		t.Fatalf("alias ls missing entries: %s", out)
	}
	if !strings.Contains(out, "\"default_branch\": \"main\"") {
		t.Fatalf("alias ls missing default_branch: %s", out)
	}

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", "local-alias"); err != nil {
		t.Fatalf("repo add local alias: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", "url-alias"); err != nil {
		t.Fatalf("repo add url alias: %v", err)
	}

	out, err = runner.run("repo", "ls", "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("repo ls: %v", err)
	}
	if !strings.Contains(out, localRepo) {
		t.Fatalf("repo ls missing local path: %s", out)
	}
	if !strings.Contains(out, filepath.Join(store, "url-alias")) {
		t.Fatalf("repo ls missing repo store path: %s", out)
	}

	if _, err := runner.run("repo", "alias", "set", "--default-branch", "main", "url-alias", fileURL(urlRepo)); err != nil {
		t.Fatalf("alias set default branch: %v", err)
	}
	if _, err := runner.run("repo", "alias", "rm", "local-alias"); err != nil {
		t.Fatalf("alias rm local: %v", err)
	}
	if _, err := runner.run("repo", "alias", "rm", "url-alias"); err != nil {
		t.Fatalf("alias rm url: %v", err)
	}
}

func TestRepoAddWithRepoDirCreatesWorktree(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "dir-repo"))

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source, "--repo-dir", "custom-dir"); err != nil {
		t.Fatalf("repo add --repo-dir: %v", err)
	}

	worktreePath := filepath.Join(runner.workspaceRoot(), "demo", "custom-dir")
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("expected worktree at custom dir: %v", err)
	}
}

func TestRepoRemotesSet(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "remotes-repo"))

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}
	if _, err := runner.run(
		"repo", "remotes", "set",
		"-w", "demo",
		"remotes-repo",
		"--base-remote", "upstream",
		"--write-remote", "origin",
		"--base-branch", "trunk",
	); err != nil {
		t.Fatalf("repo remotes set: %v", err)
	}

	out, err := runner.run("repo", "ls", "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("repo ls: %v", err)
	}
	if !strings.Contains(out, "\"base\": \"upstream/trunk\"") {
		t.Fatalf("repo ls missing base remote update: %s", out)
	}
	if !strings.Contains(out, "\"write\": \"origin/trunk\"") {
		t.Fatalf("repo ls missing write remote update: %s", out)
	}
}

func TestVersionCommand(t *testing.T) {
	runner := newRunner(t)

	out, err := runner.run("version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if strings.TrimSpace(out) != "dev" {
		t.Fatalf("unexpected version output: %q", out)
	}

	out, err = runner.run("--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	if !strings.Contains(out, "dev") {
		t.Fatalf("unexpected --version output: %q", out)
	}
}

func TestRepoRemoveWorktreesSafety(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "dirty-repo"))

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}

	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	worktreePath := filepath.Join(runner.workspaceRoot(), "demo", "dirty-repo")
	if err := os.WriteFile(filepath.Join(worktreePath, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty: %v", err)
	}

	if _, err := runner.run("repo", "rm", "-w", "demo", "--delete-worktrees", "--yes", "dirty-repo"); err == nil {
		t.Fatalf("expected repo rm to fail when dirty")
	}

	if _, err := runner.run("repo", "rm", "-w", "demo", "--delete-worktrees", "--yes", "--force", "dirty-repo"); err != nil {
		t.Fatalf("repo rm --force: %v", err)
	}

	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree deleted, got err=%v", err)
	}
}

func TestWorkspaceInitAndConfig(t *testing.T) {
	runner := newRunner(t)
	target := filepath.Join(runner.root, "init-ws")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir init-ws: %v", err)
	}
	if _, err := runner.run("new", "init-ws", "--path", target); err != nil {
		t.Fatalf("workset new --path: %v", err)
	}
	if _, err := runner.run("config", "set", "defaults.parallelism", "4"); err != nil {
		t.Fatalf("config set: %v", err)
	}
	repoStore := filepath.Join(runner.root, "repo-store")
	if _, err := runner.run("config", "set", "defaults.repo_store_root", repoStore); err != nil {
		t.Fatalf("config set repo_store_root: %v", err)
	}
	out, err := runner.run("config", "show", "--json")
	if err != nil {
		t.Fatalf("config show: %v", err)
	}
	if !strings.Contains(out, "\"parallelism\": 4") {
		t.Fatalf("config show missing parallelism: %s", out)
	}
	if !strings.Contains(out, repoStore) {
		t.Fatalf("config show missing repo_store_root: %s", out)
	}
	out, err = runner.run("ls", "--plain")
	if err != nil {
		t.Fatalf("workset ls: %v", err)
	}
	if !strings.Contains(out, "init-ws") {
		t.Fatalf("workset ls missing init-ws: %s", out)
	}
}

func TestWorkspaceListJSON(t *testing.T) {
	runner := newRunner(t)
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	out, err := runner.run("ls", "--json")
	if err != nil {
		t.Fatalf("workset ls --json: %v", err)
	}
	if !strings.Contains(out, "\"name\": \"demo\"") {
		t.Fatalf("workset ls json missing demo: %s", out)
	}
}

func TestTemplateFlowWithMultipleWorkspaces(t *testing.T) {
	runner := newRunner(t)
	repoA := setupRepo(t, filepath.Join(runner.root, "src", "repo-a"))
	repoB := setupRepo(t, filepath.Join(runner.root, "src", "repo-b"))

	if _, err := runner.run("repo", "alias", "add", "repo-a", repoA); err != nil {
		t.Fatalf("alias add repo-a: %v", err)
	}
	if _, err := runner.run("repo", "alias", "add", "repo-b", repoB); err != nil {
		t.Fatalf("alias add repo-b: %v", err)
	}
	if _, err := runner.run("repo", "alias", "set", "--default-branch", "main", "repo-a", repoA); err != nil {
		t.Fatalf("alias set default branch: %v", err)
	}
	out, err := runner.run("repo", "alias", "ls", "--json")
	if err != nil {
		t.Fatalf("alias ls: %v", err)
	}
	if !strings.Contains(out, "\"name\": \"repo-a\"") {
		t.Fatalf("alias ls missing repo-a: %s", out)
	}

	if _, err := runner.run("template", "create", "stack", "--description", "demo template"); err != nil {
		t.Fatalf("template create: %v", err)
	}
	if _, err := runner.run("template", "add", "stack", "repo-a"); err != nil {
		t.Fatalf("template add repo-a: %v", err)
	}
	if _, err := runner.run("template", "add", "stack", "repo-b"); err != nil {
		t.Fatalf("template add repo-b: %v", err)
	}
	out, err = runner.run("template", "ls", "--plain")
	if err != nil {
		t.Fatalf("template ls: %v", err)
	}
	if !strings.Contains(out, "stack") {
		t.Fatalf("template ls missing stack: %s", out)
	}
	out, err = runner.run("template", "show", "stack", "--plain")
	if err != nil {
		t.Fatalf("template show: %v", err)
	}
	if !strings.Contains(out, "repo-a") || !strings.Contains(out, "repo-b") {
		t.Fatalf("template show missing repos: %s", out)
	}

	if _, err := runner.run("new", "alpha"); err != nil {
		t.Fatalf("workset new alpha: %v", err)
	}
	if _, err := runner.run("new", "beta"); err != nil {
		t.Fatalf("workset new beta: %v", err)
	}
	if _, err := runner.run("template", "apply", "-w", "alpha", "stack"); err != nil {
		t.Fatalf("template apply alpha: %v", err)
	}
	if _, err := runner.run("template", "apply", "-w", "beta", "stack"); err != nil {
		t.Fatalf("template apply beta: %v", err)
	}
	out, err = runner.run("repo", "ls", "-w", "alpha", "--plain")
	if err != nil {
		t.Fatalf("repo ls alpha: %v", err)
	}
	if !strings.Contains(out, "repo-a") || !strings.Contains(out, "repo-b") {
		t.Fatalf("repo ls alpha missing repos: %s", out)
	}
	out, err = runner.run("repo", "ls", "-w", "beta", "--plain")
	if err != nil {
		t.Fatalf("repo ls beta: %v", err)
	}
	if !strings.Contains(out, "repo-a") || !strings.Contains(out, "repo-b") {
		t.Fatalf("repo ls beta missing repos: %s", out)
	}

	if _, err := runner.run("template", "remove", "stack", "repo-b"); err != nil {
		t.Fatalf("template remove: %v", err)
	}
	if _, err := runner.run("template", "rm", "stack"); err != nil {
		t.Fatalf("template rm: %v", err)
	}
	if _, err := runner.run("repo", "alias", "rm", "repo-b"); err != nil {
		t.Fatalf("alias rm repo-b: %v", err)
	}
}

func TestGroupAliasCommands(t *testing.T) {
	runner := newRunner(t)
	repoA := setupRepo(t, filepath.Join(runner.root, "src", "group-repo-a"))

	if _, err := runner.run("repo", "alias", "add", "group-repo-a", repoA); err != nil {
		t.Fatalf("alias add: %v", err)
	}
	if _, err := runner.run("group", "create", "group-stack", "--description", "group demo"); err != nil {
		t.Fatalf("group create: %v", err)
	}
	if _, err := runner.run("group", "add", "group-stack", "group-repo-a"); err != nil {
		t.Fatalf("group add: %v", err)
	}
	out, err := runner.run("group", "ls", "--plain")
	if err != nil {
		t.Fatalf("group ls: %v", err)
	}
	if !strings.Contains(out, "group-stack") {
		t.Fatalf("group ls missing group-stack: %s", out)
	}
	out, err = runner.run("group", "show", "group-stack", "--plain")
	if err != nil {
		t.Fatalf("group show: %v", err)
	}
	if !strings.Contains(out, "group-repo-a") {
		t.Fatalf("group show missing repo: %s", out)
	}
	out, err = runner.run("group", "show", "group-stack", "--json")
	if err != nil {
		t.Fatalf("group show --json: %v", err)
	}
	if !strings.Contains(out, "\"name\": \"origin\"") {
		t.Fatalf("group show missing default base remote: %s", out)
	}
	if !strings.Contains(out, "\"default_branch\": \"main\"") {
		t.Fatalf("group show missing default branch: %s", out)
	}
	if _, err := runner.run("group", "rm", "group-stack"); err != nil {
		t.Fatalf("group rm: %v", err)
	}
	if _, err := runner.run("repo", "alias", "rm", "group-repo-a"); err != nil {
		t.Fatalf("alias rm: %v", err)
	}
}

func TestWorkspaceNewWithGroupAndRepo(t *testing.T) {
	runner := newRunner(t)
	repoA := setupRepo(t, filepath.Join(runner.root, "src", "new-group-repo-a"))
	repoB := setupRepo(t, filepath.Join(runner.root, "src", "new-group-repo-b"))

	if _, err := runner.run("repo", "alias", "add", "new-repo-a", repoA); err != nil {
		t.Fatalf("alias add repo-a: %v", err)
	}
	if _, err := runner.run("repo", "alias", "add", "new-repo-b", repoB); err != nil {
		t.Fatalf("alias add repo-b: %v", err)
	}
	if _, err := runner.run("group", "create", "new-group"); err != nil {
		t.Fatalf("group create: %v", err)
	}
	if _, err := runner.run("group", "add", "new-group", "new-repo-a"); err != nil {
		t.Fatalf("group add: %v", err)
	}

	if _, err := runner.run("new", "demo", "--group", "new-group", "--repo", "new-repo-b"); err != nil {
		t.Fatalf("workset new with group+repo: %v", err)
	}
	out, err := runner.run("repo", "ls", "-w", "demo", "--plain")
	if err != nil {
		t.Fatalf("repo ls: %v", err)
	}
	if !strings.Contains(out, "new-repo-a") || !strings.Contains(out, "new-repo-b") {
		t.Fatalf("repo ls missing group/repos: %s", out)
	}
}

func TestWorkspaceNewConflictAcrossGroupAndRepo(t *testing.T) {
	runner := newRunner(t)
	repoA := setupRepo(t, filepath.Join(runner.root, "src", "conflict-repo-a"))

	if _, err := runner.run("repo", "alias", "add", "conflict-repo-a", repoA); err != nil {
		t.Fatalf("alias add repo-a: %v", err)
	}
	if _, err := runner.run("group", "create", "conflict-group"); err != nil {
		t.Fatalf("group create: %v", err)
	}
	if _, err := runner.run("group", "add", "conflict-group", "conflict-repo-a", "--base-remote", "upstream"); err != nil {
		t.Fatalf("group add: %v", err)
	}

	if _, err := runner.run("new", "demo", "--group", "conflict-group", "--repo", "conflict-repo-a"); err == nil {
		t.Fatalf("expected conflict error")
	} else if !strings.Contains(err.Error(), "conflicting repo") {
		t.Fatalf("expected conflict error, got: %v", err)
	}
}

func TestShellCompletionCommand(t *testing.T) {
	runner := newRunner(t)
	out, err := runner.run("completion", "bash")
	if err != nil {
		t.Fatalf("completion bash: %v", err)
	}
	if !strings.Contains(out, "workset") {
		t.Fatalf("completion output missing workset: %s", out)
	}
}

func TestWorkspaceRemoveDelete(t *testing.T) {
	runner := newRunner(t)
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("rm", "-w", "demo"); err != nil {
		t.Fatalf("workset rm: %v", err)
	}

	wsPath := filepath.Join(runner.workspaceRoot(), "demo")
	if _, err := os.Stat(wsPath); err != nil {
		t.Fatalf("expected workspace to remain: %v", err)
	}

	if _, err := runner.run("rm", "-w", "demo", "--delete", "--yes"); err != nil {
		t.Fatalf("workset rm --delete: %v", err)
	}
	if _, err := os.Stat(wsPath); !os.IsNotExist(err) {
		t.Fatalf("expected workspace deleted, got err=%v", err)
	}
}

func TestWorkspaceRemoveDirtyWorktree(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "ws-dirty-repo"))

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	worktreePath := filepath.Join(runner.workspaceRoot(), "demo", "ws-dirty-repo")
	if err := os.WriteFile(filepath.Join(worktreePath, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty: %v", err)
	}

	if _, err := runner.run("rm", "-w", "demo", "--delete", "--yes"); err == nil {
		t.Fatalf("expected workspace rm to fail when dirty")
	}

	if _, err := runner.run("rm", "-w", "demo", "--delete", "--yes", "--force"); err != nil {
		t.Fatalf("workspace rm --force: %v", err)
	}

	if _, err := os.Stat(filepath.Join(runner.workspaceRoot(), "demo")); !os.IsNotExist(err) {
		t.Fatalf("expected workspace deleted, got err=%v", err)
	}
}

func TestWorkspaceRemoveSquashMergedBranch(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "squash-merged-repo"))

	if _, err := runner.run("new", "ws-switch"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "ws-switch", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	repoName := filepath.Base(source)
	worktreePath := filepath.Join(runner.workspaceRoot(), "ws-switch", repoName)

	commitFile(t, worktreePath, "", "feature.txt", "feature", "feat: add feature")
	commitFile(t, source, "main", "feature.txt", "feature", "chore: squash merge feature")
	commitFile(t, source, "main", "feature.txt", "feature v2", "chore: tweak after merge")

	if _, err := runner.run("rm", "-w", "ws-switch", "--delete", "--yes"); err != nil {
		t.Fatalf("expected workspace rm after squash merge: %v", err)
	}
}

func TestStatusJSONOutput(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "status-repo"))
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}
	out, err := runner.run("status", "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}
	if !strings.Contains(out, "\"name\": \"status-repo\"") {
		t.Fatalf("status json missing repo: %s", out)
	}
	if !strings.Contains(out, "\"path\":") {
		t.Fatalf("status json missing path: %s", out)
	}
}

func TestStatusPlainOutput(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "status-plain-repo"))
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}
	out, err := runner.run("status", "-w", "demo", "--plain")
	if err != nil {
		t.Fatalf("status --plain: %v", err)
	}
	if !strings.Contains(out, "status-plain-repo") {
		t.Fatalf("status plain missing repo: %s", out)
	}
}

func TestRepoListPlainOutput(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "repo-list-plain"))
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}
	out, err := runner.run("repo", "ls", "-w", "demo", "--plain")
	if err != nil {
		t.Fatalf("repo ls --plain: %v", err)
	}
	if !strings.Contains(out, "repo-list-plain") {
		t.Fatalf("repo ls plain missing repo: %s", out)
	}
}

type runner struct {
	t      *testing.T
	root   string
	home   string
	config string
}

func newRunner(t *testing.T) *runner {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	config := filepath.Join(home, ".config", "workset", "config.yaml")
	return &runner{
		t:      t,
		root:   root,
		home:   home,
		config: config,
	}
}

func (r *runner) env() []string {
	return append(os.Environ(),
		"HOME="+r.home,
		"XDG_CONFIG_HOME="+filepath.Join(r.home, ".config"),
	)
}

func (r *runner) workspaceRoot() string {
	return filepath.Join(r.home, ".workset", "workspaces")
}

func (r *runner) run(args ...string) (string, error) {
	return r.runDir(r.root, args...)
}

func (r *runner) runDir(dir string, args ...string) (string, error) {
	cmd := exec.Command(worksetBin, args...)
	cmd.Dir = dir
	cmd.Env = r.env()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &execError{err: err, stderr: stderr.String(), stdout: stdout.String()}
	}
	if stderr.Len() > 0 {
		return stdout.String() + stderr.String(), nil
	}
	return stdout.String(), nil
}

type execError struct {
	err    error
	stdout string
	stderr string
}

func (e *execError) Error() string {
	msg := e.err.Error()
	if e.stderr != "" {
		msg += ": " + e.stderr
	}
	if e.stdout != "" {
		msg += ": " + e.stdout
	}
	return msg
}

func setupRepo(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	repo, err := ggit.PlainInit(path, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	if _, err := repo.CreateRemote(&ggitcfg.RemoteConfig{
		Name: "origin",
		URLs: []string{path},
	}); err != nil {
		t.Fatalf("CreateRemote: %v", err)
	}
	if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("main"))); err != nil {
		t.Fatalf("set HEAD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if _, err := worktree.Add("README.md"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	_, err = worktree.Commit("initial", &ggit.CommitOptions{
		Author: &object.Signature{
			Name:  "Tester",
			Email: "tester@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return path
}

func commitFile(t *testing.T, repoPath, branch, filename, contents, message string) {
	t.Helper()

	repo, err := ggit.PlainOpenWithOptions(repoPath, &ggit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	if branch != "" {
		branchRef := plumbing.NewBranchReferenceName(branch)
		if err := worktree.Checkout(&ggit.CheckoutOptions{Branch: branchRef}); err != nil {
			if err := worktree.Checkout(&ggit.CheckoutOptions{Branch: branchRef, Create: true}); err != nil {
				t.Fatalf("checkout %s: %v", branch, err)
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(filepath.Join(repoPath, filename)), 0o755); err != nil {
		t.Fatalf("mkdir file dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, filename), []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := worktree.Add(filename); err != nil {
		t.Fatalf("add file: %v", err)
	}
	if _, err := worktree.Commit(message, &ggit.CommitOptions{
		Author: &object.Signature{
			Name:  "Tester",
			Email: "tester@example.com",
			When:  time.Now(),
		},
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func fileURL(path string) string {
	return "file://" + filepath.ToSlash(path)
}
