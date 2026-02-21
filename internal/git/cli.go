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
	"strconv"
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
	result := gitResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
	}
	if err != nil {
		message := strings.TrimSpace(result.stderr)
		if message == "" {
			message = strings.TrimSpace(result.stdout)
		}
		if message != "" {
			err = fmt.Errorf("%w: %s", err, message)
		}
	}
	return result, err
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

func (c CLIClient) ReferenceExists(ctx context.Context, repoPath, ref string) (bool, error) {
	if repoPath == "" || ref == "" {
		return false, errors.New("repo path and ref required")
	}
	result, err := c.run(ctx, repoPath, "show-ref", "--verify", ref)
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
	result, err := c.run(ctx, repoPath, "fetch", "--no-write-fetch-head", remoteName)
	if err == nil {
		return nil
	}
	if isNoWriteFetchHeadUnsupported(result.stderr) {
		_, fallbackErr := c.run(ctx, repoPath, "fetch", remoteName)
		return fallbackErr
	}
	return err
}

func isNoWriteFetchHeadUnsupported(stderr string) bool {
	text := strings.ToLower(stderr)
	return strings.Contains(text, "unknown option") && strings.Contains(text, "no-write-fetch-head")
}

// UpdateBranch force-updates branchName to targetRef. Callers must ensure the update is safe.
func (c CLIClient) UpdateBranch(ctx context.Context, repoPath, branchName, targetRef string) error {
	if repoPath == "" {
		return errors.New("repo path required")
	}
	if branchName == "" {
		return errors.New("branch name required")
	}
	if targetRef == "" {
		return errors.New("target ref required")
	}
	_, err := c.run(ctx, repoPath, "branch", "-f", branchName, targetRef)
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

	ancestor, err := c.IsAncestor(repoPath, branchRef, baseRef)
	if err == nil && ancestor {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	if equal, err := c.refsMatch(repoPath, baseRef, branchRef); err == nil && equal {
		return true, nil
	} else if err != nil {
		return false, err
	}

	uniqueByPatch, cherryErr := c.rightOnlyCherryCount(repoPath, baseRef, branchRef)
	if cherryErr == nil && uniqueByPatch == 0 {
		return true, nil
	}
	// Continue with tree-based fallbacks when patch-id matching cannot run.

	bases, err := c.mergeBases(repoPath, branchRef, baseRef)
	if err != nil {
		return false, err
	}
	if len(bases) == 0 {
		return false, nil
	}
	mergedByMergeTree, err := c.mergeTreeLeavesBaseUnchanged(repoPath, baseRef, branchRef)
	if err == nil && mergedByMergeTree {
		return true, nil
	}
	var lastErr error
	if err != nil {
		lastErr = err
	}
	for _, mergeBase := range bases {
		merged, err := c.changesAppliedInBase(repoPath, mergeBase, branchRef, baseRef)
		if err != nil {
			lastErr = err
			continue
		}
		if merged {
			return true, nil
		}
		merged, err = c.squashPatchAppliedInBase(repoPath, mergeBase, branchRef, baseRef)
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

func (c CLIClient) mergeTreeLeavesBaseUnchanged(repoPath, baseRef, branchRef string) (bool, error) {
	result, err := c.run(context.Background(), repoPath, "merge-tree", "--write-tree", baseRef, branchRef)
	if err != nil {
		if result.exitCode == 1 || isMergeTreeUnsupported(result.stderr) || isMergeTreeTransientFailure(result.stderr) {
			return false, nil
		}
		return false, err
	}
	mergedTree := strings.TrimSpace(result.stdout)
	if mergedTree == "" {
		return false, errors.New("empty merge-tree output")
	}
	baseTreeResult, err := c.run(context.Background(), repoPath, "rev-parse", baseRef+"^{tree}")
	if err != nil {
		return false, err
	}
	return mergedTree == strings.TrimSpace(baseTreeResult.stdout), nil
}

func isMergeTreeUnsupported(stderr string) bool {
	text := strings.ToLower(stderr)
	return strings.Contains(text, "unknown option") && strings.Contains(text, "write-tree")
}

func isMergeTreeTransientFailure(stderr string) bool {
	text := strings.ToLower(stderr)
	return strings.Contains(text, "unable to create temporary file") || strings.Contains(text, "operation not permitted")
}

func (c CLIClient) squashPatchAppliedInBase(repoPath, mergeBase, branchRef, baseRef string) (bool, error) {
	branchPatchID, hasBranchPatch, err := c.rangePatchID(repoPath, mergeBase, branchRef)
	if err != nil {
		return false, err
	}
	if !hasBranchPatch {
		return true, nil
	}
	result, err := c.run(context.Background(), repoPath, "rev-list", mergeBase+".."+baseRef)
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(result.stdout, "\n") {
		commit := strings.TrimSpace(line)
		if commit == "" {
			continue
		}
		commitPatchID, hasCommitPatch, err := c.commitPatchID(repoPath, commit)
		if err != nil {
			return false, err
		}
		if !hasCommitPatch {
			continue
		}
		if commitPatchID == branchPatchID {
			return true, nil
		}
	}
	return false, nil
}

func (c CLIClient) CurrentBranch(repoPath string) (string, bool, error) {
	if repoPath == "" {
		return "", false, errors.New("repo path required")
	}
	result, err := c.run(context.Background(), repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		if result.exitCode != 0 && isMissingRef(result.stderr) {
			return "", false, nil
		}
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
		branchRef := "refs/heads/" + opts.StartBranch
		branchExists, err := c.ReferenceExists(ctx, opts.RepoPath, branchRef)
		if err != nil {
			return err
		}
		if branchExists {
			startRef = opts.StartBranch
		}
	}
	if opts.StartRemote != "" && opts.StartBranch != "" {
		remoteRef := fmt.Sprintf("refs/remotes/%s/%s", opts.StartRemote, opts.StartBranch)
		remoteExists, err := c.ReferenceExists(ctx, opts.RepoPath, remoteRef)
		if err != nil {
			return err
		}
		if remoteExists {
			startRef = fmt.Sprintf("%s/%s", opts.StartRemote, opts.StartBranch)
		}
	}

	refExists, err := c.ReferenceExists(ctx, opts.RepoPath, "refs/heads/"+opts.BranchName)
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
	forced := opts.Force
	for {
		args := []string{"worktree", "remove"}
		if forced {
			args = append(args, "--force")
		}
		args = append(args, worktreePath)
		result, err := c.run(context.Background(), opts.RepoPath, args...)
		if err == nil {
			return nil
		}
		if strings.Contains(result.stderr, "is not a working tree") || strings.Contains(result.stderr, "not a working tree") {
			return ErrWorktreeNotFound
		}
		if !forced && shouldRetryWorktreeRemoveWithForce(result.stderr) {
			forced = true
			continue
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
}

func (c CLIClient) WorktreeList(repoPath string) ([]string, error) {
	if repoPath == "" {
		return nil, errors.New("repo path required")
	}
	worktreesPath, err := c.gitAdminPath(repoPath, "worktrees")
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

func (c CLIClient) rightOnlyCherryCount(repoPath, leftRef, rightRef string) (int, error) {
	result, err := c.run(context.Background(), repoPath, "rev-list", "--right-only", "--cherry-pick", "--count", leftRef+"..."+rightRef)
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(result.stdout)
	if value == "" {
		return 0, errors.New("empty rev-list --count output")
	}
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse rev-list count %q: %w", value, err)
	}
	return count, nil
}

func (c CLIClient) rangePatchID(repoPath, startRef, endRef string) (string, bool, error) {
	result, err := c.run(
		context.Background(),
		repoPath,
		"diff",
		"--no-ext-diff",
		"--full-index",
		"--binary",
		startRef+".."+endRef,
	)
	if err != nil {
		return "", false, err
	}
	return c.patchIDFromDiff(result.stdout)
}

func (c CLIClient) commitPatchID(repoPath, commit string) (string, bool, error) {
	result, err := c.run(
		context.Background(),
		repoPath,
		"show",
		"--no-ext-diff",
		"--full-index",
		"--binary",
		"--pretty=format:",
		commit,
	)
	if err != nil {
		return "", false, err
	}
	return c.patchIDFromDiff(result.stdout)
}

func (c CLIClient) patchIDFromDiff(diffText string) (string, bool, error) {
	if strings.TrimSpace(diffText) == "" {
		return "", false, nil
	}
	cmd := exec.CommandContext(context.Background(), c.gitPath, "patch-id", "--stable")
	cmd.Env = os.Environ()
	cmd.Stdin = strings.NewReader(diffText)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message != "" {
			return "", false, fmt.Errorf("%w: %s", err, message)
		}
		return "", false, err
	}
	line := strings.TrimSpace(stdout.String())
	if line == "" {
		return "", false, nil
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", false, errors.New("empty patch-id output")
	}
	return fields[0], true, nil
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
			if !branchOK {
				return false, nil
			}
			if baseOK && baseEntry.hash == branchEntry.hash && baseEntry.mode == branchEntry.mode {
				continue
			}
			seen, err := c.treeEntrySeenInRange(repoPath, mergeBase, baseRef, entry.oldPath, branchEntry)
			if err != nil {
				return false, err
			}
			if !seen {
				return false, nil
			}
		case "D":
			baseEntry, baseOK, err := c.readTreeEntry(repoPath, baseRef, entry.oldPath)
			if err != nil {
				return false, err
			}
			if baseOK && baseEntry.hash != "" {
				deleted, err := c.pathDeletedInRange(repoPath, mergeBase, baseRef, entry.oldPath)
				if err != nil {
					return false, err
				}
				if !deleted {
					return false, nil
				}
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
			if !branchOK {
				return false, nil
			}
			if !baseOK || baseEntry.hash != branchEntry.hash || baseEntry.mode != branchEntry.mode {
				seen, err := c.treeEntrySeenInRange(repoPath, mergeBase, baseRef, entry.newPath, branchEntry)
				if err != nil {
					return false, err
				}
				if !seen {
					return false, nil
				}
			}
			if entry.status != "R" {
				continue
			}
			oldEntry, oldOK, err := c.readTreeEntry(repoPath, baseRef, entry.oldPath)
			if err != nil {
				return false, err
			}
			if oldOK && oldEntry.hash != "" {
				deleted, err := c.pathDeletedInRange(repoPath, mergeBase, baseRef, entry.oldPath)
				if err != nil {
					return false, err
				}
				if !deleted {
					return false, nil
				}
			}
		default:
			return false, fmt.Errorf("unsupported diff status %s", entry.status)
		}
	}
	return true, nil
}

func (c CLIClient) treeEntrySeenInRange(repoPath, startRef, endRef, path string, expected treeEntry) (bool, error) {
	if expected.hash == "" || expected.mode == "" {
		return false, errors.New("tree entry mode and hash required")
	}
	result, err := c.run(context.Background(), repoPath, "log", "--format=%H", startRef+".."+endRef, "--find-object="+expected.hash, "--", path)
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(result.stdout, "\n") {
		commit := strings.TrimSpace(line)
		if commit == "" {
			continue
		}
		entry, ok, err := c.readTreeEntry(repoPath, commit, path)
		if err != nil {
			return false, err
		}
		if ok && entry.hash == expected.hash && entry.mode == expected.mode {
			return true, nil
		}
	}
	return false, nil
}

func (c CLIClient) pathDeletedInRange(repoPath, startRef, endRef, path string) (bool, error) {
	result, err := c.run(context.Background(), repoPath, "log", "--format=%H", "--diff-filter=D", startRef+".."+endRef, "--", path)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(result.stdout) != "", nil
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
	for line := range strings.SplitSeq(logRes.stdout, "\n") {
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

func shouldRetryWorktreeRemoveWithForce(stderr string) bool {
	msg := strings.ToLower(stderr)
	return strings.Contains(msg, "contains modified or untracked files") ||
		strings.Contains(msg, "directory not empty")
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
	if filepath.IsAbs(gitPath) {
		return filepath.Clean(gitPath), nil
	}
	workDir, _, normErr := normalizeRepoPath(repoPath)
	if normErr == nil && workDir != "" {
		return filepath.Clean(filepath.Join(workDir, gitPath)), nil
	}
	return filepath.Clean(filepath.Join(repoPath, gitPath)), nil
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
