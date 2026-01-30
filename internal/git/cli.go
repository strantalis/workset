package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// CLIClient implements Git operations via the git CLI.
type CLIClient struct {
	gitPath string
}

// NewCLIClient constructs a git CLI client.
func NewCLIClient() CLIClient {
	return CLIClient{gitPath: "git"}
}

type gitResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func (c CLIClient) run(ctx context.Context, repoPath string, args ...string) (gitResult, error) {
	cmdArgs := make([]string, 0, len(args)+4)
	if repoPath != "" {
		workDir, extraArgs, err := normalizeRepoPath(repoPath)
		if err != nil {
			return gitResult{}, err
		}
		if workDir != "" {
			cmdArgs = append(cmdArgs, "-C", workDir)
		}
		cmdArgs = append(cmdArgs, extraArgs...)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, c.gitPath, cmdArgs...)
	cmd.Env = os.Environ()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return gitResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
	}, err
}

func (c CLIClient) Clone(ctx context.Context, url, path, remoteName string) error {
	if remoteName == "" {
		remoteName = "origin"
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	_, err := c.run(ctx, "", "clone", "--origin", remoteName, url, path)
	return err
}

func (c CLIClient) CloneBare(ctx context.Context, url, path, remoteName string) error {
	if remoteName == "" {
		remoteName = "origin"
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	_, err := c.run(ctx, "", "clone", "--bare", "--origin", remoteName, url, path)
	return err
}

func (c CLIClient) AddRemote(path, name, url string) error {
	if name == "" {
		return errors.New("remote name required")
	}
	if url == "" {
		return errors.New("remote url required")
	}
	result, err := c.run(context.Background(), path, "remote", "add", name, url)
	if err != nil {
		if strings.Contains(result.stderr, "already exists") {
			return nil
		}
		return err
	}
	return nil
}

func (c CLIClient) RemoteNames(repoPath string) ([]string, error) {
	if repoPath == "" {
		return nil, errors.New("repo path required")
	}
	result, err := c.run(context.Background(), repoPath, "remote")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(result.stdout), "\n")
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func (c CLIClient) RemoteURLs(repoPath, remoteName string) ([]string, error) {
	if repoPath == "" {
		return nil, errors.New("repo path required")
	}
	if remoteName == "" {
		return nil, errors.New("remote name required")
	}
	result, err := c.run(context.Background(), repoPath, "remote", "get-url", "--all", remoteName)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(result.stdout), "\n")
	urls := make([]string, 0, len(lines))
	for _, line := range lines {
		url := strings.TrimSpace(line)
		if url != "" {
			urls = append(urls, url)
		}
	}
	if len(urls) == 0 {
		return nil, errors.New("remote has no URLs configured")
	}
	return urls, nil
}

func (c CLIClient) ReferenceExists(repoPath, ref string) (bool, error) {
	if repoPath == "" || ref == "" {
		return false, errors.New("repo path and ref required")
	}
	result, err := c.run(context.Background(), repoPath, "show-ref", "--verify", ref)
	if err != nil {
		if result.exitCode == 1 || isMissingRef(result.stderr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c CLIClient) Fetch(ctx context.Context, repoPath, remoteName string) error {
	if repoPath == "" {
		return errors.New("repo path required")
	}
	if remoteName == "" {
		return errors.New("remote name required")
	}
	_, err := c.run(ctx, repoPath, "fetch", remoteName)
	return err
}

func (c CLIClient) Status(path string) (StatusSummary, error) {
	if path == "" {
		return StatusSummary{}, errors.New("repo path required")
	}
	result, err := c.run(context.Background(), path, "status", "--porcelain=v1", "-z")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return StatusSummary{Missing: true}, nil
		}
		if isNotRepo(result) {
			return StatusSummary{Missing: true}, nil
		}
		return StatusSummary{}, err
	}
	return StatusSummary{Dirty: len(result.stdout) > 0}, nil
}

func (c CLIClient) IsRepo(path string) (bool, error) {
	result, err := c.run(context.Background(), path, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if result.exitCode != 0 {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(result.stdout) == "true", nil
}

func (c CLIClient) IsAncestor(repoPath, ancestorRef, descendantRef string) (bool, error) {
	if ancestorRef == "" || descendantRef == "" {
		return false, errors.New("ancestor and descendant refs required")
	}
	result, err := c.run(context.Background(), repoPath, "merge-base", "--is-ancestor", ancestorRef, descendantRef)
	if err != nil {
		if result.exitCode == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c CLIClient) IsContentMerged(repoPath, branchRef, baseRef string) (bool, error) {
	if repoPath == "" {
		return false, errors.New("repo path required")
	}
	if branchRef == "" || baseRef == "" {
		return false, errors.New("branch and base refs required")
	}
	if equal, err := c.refsMatch(repoPath, baseRef, branchRef); err == nil && equal {
		return true, nil
	} else if err != nil {
		return false, err
	}

	bases, err := c.mergeBases(repoPath, branchRef, baseRef)
	if err != nil {
		return false, err
	}
	if len(bases) == 0 {
		return false, nil
	}
	var lastErr error
	for _, mergeBase := range bases {
		merged, err := c.changesAppliedInBase(repoPath, mergeBase, branchRef, baseRef)
		if err != nil {
			lastErr = err
			continue
		}
		if merged {
			return true, nil
		}
	}
	merged, err := c.treeSeenInHistory(repoPath, baseRef, branchRef)
	if err != nil {
		if lastErr != nil {
			return false, lastErr
		}
		return false, err
	}
	if merged {
		return true, nil
	}
	if lastErr != nil {
		return false, lastErr
	}
	return false, nil
}

func (c CLIClient) CurrentBranch(repoPath string) (string, bool, error) {
	if repoPath == "" {
		return "", false, errors.New("repo path required")
	}
	result, err := c.run(context.Background(), repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", false, err
	}
	name := strings.TrimSpace(result.stdout)
	if name == "" || name == "HEAD" {
		return "", false, nil
	}
	return name, true, nil
}

func (c CLIClient) RemoteExists(repoPath, remoteName string) (bool, error) {
	if repoPath == "" || remoteName == "" {
		return false, errors.New("repo path and remote name required")
	}
	result, err := c.run(context.Background(), repoPath, "remote", "get-url", remoteName)
	if err != nil {
		if strings.Contains(result.stderr, "No such remote") || result.exitCode != 0 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c CLIClient) WorktreeAdd(ctx context.Context, opts WorktreeAddOptions) error {
	if opts.RepoPath == "" {
		return errors.New("repo path required")
	}
	if opts.WorktreePath == "" {
		return errors.New("worktree path required")
	}
	if opts.WorktreeName == "" {
		return errors.New("worktree name required")
	}
	if opts.BranchName == "" {
		return errors.New("branch name required")
	}

	if err := os.MkdirAll(opts.WorktreePath, 0o755); err != nil {
		return err
	}

	startRef := ""
	if opts.StartBranch != "" {
		branchRef := fmt.Sprintf("refs/heads/%s", opts.StartBranch)
		branchExists, err := c.ReferenceExists(opts.RepoPath, branchRef)
		if err != nil {
			return err
		}
		if branchExists {
			startRef = opts.StartBranch
		}
	}
	if opts.StartRemote != "" && opts.StartBranch != "" {
		remoteRef := fmt.Sprintf("refs/remotes/%s/%s", opts.StartRemote, opts.StartBranch)
		remoteExists, err := c.ReferenceExists(opts.RepoPath, remoteRef)
		if err != nil {
			return err
		}
		if remoteExists {
			startRef = fmt.Sprintf("%s/%s", opts.StartRemote, opts.StartBranch)
		}
	}

	refExists, err := c.ReferenceExists(opts.RepoPath, fmt.Sprintf("refs/heads/%s", opts.BranchName))
	if err != nil {
		return err
	}

	args := []string{"worktree", "add"}
	if opts.ForceCheckout {
		args = append(args, "--force")
	}
	if refExists {
		args = append(args, opts.WorktreePath, opts.BranchName)
	} else {
		args = append(args, "-b", opts.BranchName, opts.WorktreePath)
		if startRef != "" {
			args = append(args, startRef)
		}
	}
	result, err := c.run(ctx, opts.RepoPath, args...)
	if err != nil {
		message := strings.TrimSpace(result.stderr)
		if message == "" {
			message = strings.TrimSpace(result.stdout)
		}
		if message != "" {
			return fmt.Errorf("%w: %s", err, message)
		}
		return err
	}
	return nil
}

func (c CLIClient) WorktreeRemove(opts WorktreeRemoveOptions) error {
	if opts.RepoPath == "" {
		return errors.New("repo path required")
	}
	if opts.WorktreeName == "" {
		return errors.New("worktree name required")
	}
	worktreePath, err := c.worktreePathFromName(opts.RepoPath, opts.WorktreeName)
	if err != nil {
		if errors.Is(err, ErrWorktreeNotFound) {
			return ErrWorktreeNotFound
		}
		return err
	}
	args := []string{"worktree", "remove"}
	if opts.Force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)
	result, err := c.run(context.Background(), opts.RepoPath, args...)
	if err != nil {
		if strings.Contains(result.stderr, "is not a working tree") || strings.Contains(result.stderr, "not a working tree") {
			return ErrWorktreeNotFound
		}
		message := strings.TrimSpace(result.stderr)
		if message == "" {
			message = strings.TrimSpace(result.stdout)
		}
		if message != "" {
			return fmt.Errorf("%w: %s", err, message)
		}
		return err
	}
	return nil
}

func (c CLIClient) WorktreeList(repoPath string) ([]string, error) {
	if repoPath == "" {
		return nil, errors.New("repo path required")
	}
	worktreesPath, err := c.gitAdminPath(repoPath, filepath.Join("worktrees"))
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(worktreesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func (c CLIClient) refsMatch(repoPath, left, right string) (bool, error) {
	result, err := c.run(context.Background(), repoPath, "diff", "--quiet", left, right)
	if err == nil {
		return true, nil
	}
	if result.exitCode == 1 {
		return false, nil
	}
	return false, err
}

func (c CLIClient) mergeBases(repoPath, left, right string) ([]string, error) {
	result, err := c.run(context.Background(), repoPath, "merge-base", "--all", left, right)
	if err != nil {
		return nil, err
	}
	lines := strings.Fields(strings.TrimSpace(result.stdout))
	return lines, nil
}

func (c CLIClient) changesAppliedInBase(repoPath, mergeBase, branchRef, baseRef string) (bool, error) {
	result, err := c.run(context.Background(), repoPath, "diff", "--name-status", "-z", mergeBase, branchRef)
	if err != nil {
		return false, err
	}
	entries, err := parseNameStatus([]byte(result.stdout))
	if err != nil {
		return false, err
	}
	if len(entries) == 0 {
		return true, nil
	}
	for _, entry := range entries {
		switch entry.status {
		case "A", "M", "T":
			baseEntry, baseOK, err := c.readTreeEntry(repoPath, baseRef, entry.oldPath)
			if err != nil {
				return false, err
			}
			branchEntry, branchOK, err := c.readTreeEntry(repoPath, branchRef, entry.oldPath)
			if err != nil {
				return false, err
			}
			if !baseOK || !branchOK {
				return false, nil
			}
			if baseEntry.hash != branchEntry.hash || baseEntry.mode != branchEntry.mode {
				return false, nil
			}
		case "D":
			baseEntry, baseOK, err := c.readTreeEntry(repoPath, baseRef, entry.oldPath)
			if err != nil {
				return false, err
			}
			if baseOK && baseEntry.hash != "" {
				return false, nil
			}
		case "R", "C":
			baseEntry, baseOK, err := c.readTreeEntry(repoPath, baseRef, entry.newPath)
			if err != nil {
				return false, err
			}
			branchEntry, branchOK, err := c.readTreeEntry(repoPath, branchRef, entry.newPath)
			if err != nil {
				return false, err
			}
			if !baseOK || !branchOK {
				return false, nil
			}
			if baseEntry.hash != branchEntry.hash || baseEntry.mode != branchEntry.mode {
				return false, nil
			}
			oldEntry, oldOK, err := c.readTreeEntry(repoPath, baseRef, entry.oldPath)
			if err != nil {
				return false, err
			}
			if oldOK && oldEntry.hash != "" {
				return false, nil
			}
		default:
			return false, fmt.Errorf("unsupported diff status %s", entry.status)
		}
	}
	return true, nil
}

func (c CLIClient) treeSeenInHistory(repoPath, baseRef, branchRef string) (bool, error) {
	treeRes, err := c.run(context.Background(), repoPath, "rev-parse", branchRef+"^{tree}")
	if err != nil {
		return false, err
	}
	treeHash := strings.TrimSpace(treeRes.stdout)
	if treeHash == "" {
		return false, errors.New("tree hash required")
	}
	logRes, err := c.run(context.Background(), repoPath, "log", "--pretty=%T", baseRef)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(logRes.stdout, "\n") {
		if strings.TrimSpace(line) == treeHash {
			return true, nil
		}
	}
	return false, nil
}

func (c CLIClient) readTreeEntry(repoPath, ref, path string) (treeEntry, bool, error) {
	result, err := c.run(context.Background(), repoPath, "ls-tree", "-z", ref, "--", path)
	if err != nil {
		return treeEntry{}, false, err
	}
	if result.stdout == "" {
		return treeEntry{}, false, nil
	}
	entry := strings.TrimSuffix(result.stdout, "\x00")
	parts := strings.SplitN(entry, "\t", 2)
	if len(parts) != 2 {
		return treeEntry{}, false, fmt.Errorf("unexpected ls-tree output: %s", entry)
	}
	fields := strings.Fields(parts[0])
	if len(fields) < 3 {
		return treeEntry{}, false, fmt.Errorf("unexpected ls-tree entry: %s", entry)
	}
	return treeEntry{
		mode: fields[0],
		hash: fields[2],
	}, true, nil
}

func (c CLIClient) worktreePathFromName(repoPath, worktreeName string) (string, error) {
	worktreeAdmin, err := c.gitAdminPath(repoPath, filepath.Join("worktrees", worktreeName))
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(worktreeAdmin, "gitdir"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrWorktreeNotFound
		}
		return "", err
	}
	gitDir := strings.TrimSpace(string(data))
	if gitDir == "" {
		return "", ErrWorktreeNotFound
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(worktreeAdmin, gitDir)
	}
	return filepath.Dir(gitDir), nil
}

func (c CLIClient) gitAdminPath(repoPath, path string) (string, error) {
	result, err := c.run(context.Background(), repoPath, "rev-parse", "--git-path", path)
	if err != nil {
		if result.exitCode != 0 {
			return "", ErrWorktreeNotFound
		}
		return "", err
	}
	gitPath := strings.TrimSpace(result.stdout)
	if gitPath == "" {
		return "", ErrWorktreeNotFound
	}
	return gitPath, nil
}

func isNotRepo(result gitResult) bool {
	if result.exitCode == 0 {
		return false
	}
	msg := strings.ToLower(result.stderr)
	return strings.Contains(msg, "not a git repository") || strings.Contains(msg, "not a git repo")
}

func isMissingRef(stderr string) bool {
	msg := strings.ToLower(strings.TrimSpace(stderr))
	return strings.Contains(msg, "not a valid ref") ||
		strings.Contains(msg, "unknown revision") ||
		strings.Contains(msg, "bad object") ||
		strings.Contains(msg, "ambiguous argument")
}

func normalizeRepoPath(repoPath string) (string, []string, error) {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return "", nil, nil
	}
	info, err := os.Stat(repoPath)
	if err != nil {
		return "", nil, err
	}
	if !info.IsDir() {
		gitDir, ok, err := parseGitDirFile(repoPath)
		if err != nil {
			return "", nil, err
		}
		if ok {
			return filepath.Dir(repoPath), []string{"--git-dir", gitDir}, nil
		}
		return "", nil, fmt.Errorf("repo path %q is not a directory", repoPath)
	}

	if isGitDir(repoPath) {
		if worktreeRoot, ok, err := worktreeRootFromGitDir(repoPath); err != nil {
			return "", nil, err
		} else if ok {
			return worktreeRoot, []string{"--git-dir", repoPath}, nil
		}
		if !isWorktreeAdminDir(repoPath) {
			return repoPath, nil, nil
		}
		return "", []string{"--git-dir", repoPath}, nil
	}
	return repoPath, nil, nil
}

func parseGitDirFile(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	line := strings.TrimSpace(string(data))
	const prefix = "gitdir:"
	if !strings.HasPrefix(line, prefix) {
		return "", false, nil
	}
	gitDir := strings.TrimSpace(line[len(prefix):])
	if gitDir == "" {
		return "", false, nil
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(filepath.Dir(path), gitDir)
	}
	return filepath.Clean(gitDir), true, nil
}

func isGitDir(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "HEAD")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "config")); err != nil {
		return false
	}
	return true
}

func isWorktreeAdminDir(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "gitdir")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(path, "commondir")); err == nil {
		return true
	}
	return false
}

func worktreeRootFromGitDir(path string) (string, bool, error) {
	if gitDir, ok, err := parseGitDirFile(filepath.Join(path, "gitdir")); err != nil {
		return "", false, err
	} else if ok {
		return filepath.Dir(gitDir), true, nil
	}
	if filepath.Base(path) == ".git" {
		parent := filepath.Dir(path)
		if samePath(filepath.Join(parent, ".git"), path) {
			return parent, true, nil
		}
	}
	return "", false, nil
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if left == right {
		return true
	}
	leftEval, errLeft := filepath.EvalSymlinks(left)
	rightEval, errRight := filepath.EvalSymlinks(right)
	if errLeft != nil || errRight != nil {
		return false
	}
	return filepath.Clean(leftEval) == filepath.Clean(rightEval)
}
