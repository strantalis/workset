package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/strantalis/workset/pkg/worksetapi"
)

type RepoDiffSnapshot struct {
	Patch string `json:"patch"`
}

type RepoDiffFile struct {
	Path     string `json:"path"`
	PrevPath string `json:"prevPath,omitempty"`
	Added    int    `json:"added"`
	Removed  int    `json:"removed"`
	Status   string `json:"status"`
	Binary   bool   `json:"binary,omitempty"`
}

type RepoDiffSummary struct {
	Files        []RepoDiffFile `json:"files"`
	TotalAdded   int            `json:"totalAdded"`
	TotalRemoved int            `json:"totalRemoved"`
}

type RepoFileDiffSnapshot struct {
	Patch      string `json:"patch"`
	Truncated  bool   `json:"truncated"`
	TotalLines int    `json:"totalLines"`
	TotalBytes int    `json:"totalBytes"`
	Binary     bool   `json:"binary,omitempty"`
}

// GetRepoDiff returns a unified patch for the selected repo.
func (a *App) GetRepoDiff(workspaceID, repoID string) (RepoDiffSnapshot, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	repoPath, err := a.resolveRepoPath(ctx, workspaceID, repoID)
	if err != nil {
		return RepoDiffSnapshot{}, err
	}
	patch, err := buildRepoPatch(ctx, repoPath)
	if err != nil {
		return RepoDiffSnapshot{}, err
	}
	return RepoDiffSnapshot{Patch: patch}, nil
}

// GetRepoDiffSummary returns a list of changed files with stats.
func (a *App) GetRepoDiffSummary(workspaceID, repoID string) (RepoDiffSummary, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	repoPath, err := a.resolveRepoPath(ctx, workspaceID, repoID)
	if err != nil {
		return RepoDiffSummary{}, err
	}

	files, err := collectRepoDiffSummary(ctx, repoPath)
	if err != nil {
		return RepoDiffSummary{}, err
	}

	summary := RepoDiffSummary{Files: files}
	for _, file := range files {
		summary.TotalAdded += file.Added
		summary.TotalRemoved += file.Removed
	}
	return summary, nil
}

// GetRepoFileDiff returns a patch for a single file.
func (a *App) GetRepoFileDiff(workspaceID, repoID, path, prevPath, status string) (RepoFileDiffSnapshot, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	repoPath, err := a.resolveRepoPath(ctx, workspaceID, repoID)
	if err != nil {
		return RepoFileDiffSnapshot{}, err
	}
	if path == "" {
		return RepoFileDiffSnapshot{}, errors.New("file path is required")
	}

	var patch string
	switch status {
	case "untracked":
		patch, err = gitDiffNoIndex(ctx, repoPath, path)
	default:
		patch, err = buildRepoFilePatch(ctx, repoPath, path, prevPath)
	}
	if err != nil {
		return RepoFileDiffSnapshot{}, err
	}

	return finalizePatch(patch), nil
}

func (a *App) resolveRepoPath(ctx context.Context, workspaceID, repoID string) (string, error) {
	if workspaceID == "" || repoID == "" {
		return "", errors.New("workspace and repo are required")
	}
	workspacePath, err := a.resolveWorkspacePath(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	repos, err := a.service.ListRepos(ctx, worksetapi.WorkspaceSelector{Value: workspaceID})
	if err != nil {
		return "", err
	}
	for _, repo := range repos.Repos {
		if workspaceID+"::"+repo.Name == repoID {
			if repo.RepoDir != "" && workspacePath != "" {
				worktreePath := filepath.Join(workspacePath, repo.RepoDir)
				if stat, err := os.Stat(worktreePath); err == nil && stat.IsDir() {
					return worktreePath, nil
				}
			}
			if repo.LocalPath != "" {
				if stat, err := os.Stat(repo.LocalPath); err == nil && stat.IsDir() {
					return repo.LocalPath, nil
				}
				return "", fmt.Errorf("repo path unavailable: %s", repo.LocalPath)
			}
			return "", errors.New("repo path not found")
		}
	}
	return "", fmt.Errorf("repo %q not found in workspace %q", repoID, workspaceID)
}

func (a *App) resolveWorkspacePath(ctx context.Context, workspaceID string) (string, error) {
	result, err := a.service.ListWorkspaces(ctx)
	if err != nil {
		return "", err
	}
	for _, workspace := range result.Workspaces {
		if workspace.Name == workspaceID || workspace.Path == workspaceID {
			return workspace.Path, nil
		}
	}
	return "", fmt.Errorf("workspace %q not found", workspaceID)
}

func buildRepoPatch(ctx context.Context, repoPath string) (string, error) {
	unstaged, err := gitDiff(ctx, repoPath, false)
	if err != nil {
		return "", err
	}
	staged, err := gitDiff(ctx, repoPath, true)
	if err != nil {
		return "", err
	}
	untracked, err := gitUntracked(ctx, repoPath)
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, 2+len(untracked))
	if trimmed := strings.TrimSpace(staged); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(unstaged); trimmed != "" {
		parts = append(parts, trimmed)
	}
	for _, path := range untracked {
		diff, err := gitDiffNoIndex(ctx, repoPath, path)
		if err != nil {
			return "", err
		}
		if trimmed := strings.TrimSpace(diff); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, "\n") + "\n", nil
}

func buildRepoFilePatch(ctx context.Context, repoPath, path, prevPath string) (string, error) {
	unstaged, err := gitFileDiff(ctx, repoPath, path, prevPath, false)
	if err != nil {
		return "", err
	}
	staged, err := gitFileDiff(ctx, repoPath, path, prevPath, true)
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(staged); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(unstaged); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, "\n") + "\n", nil
}

func gitDiff(ctx context.Context, repoPath string, cached bool) (string, error) {
	args := []string{
		"-c", "color.ui=false",
		"-C", repoPath,
		"diff",
		"--no-ext-diff",
		"--unified=3",
	}
	if cached {
		args = append(args, "--cached")
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr := (&exec.ExitError{}); errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				return string(output), nil
			}
		}
		return "", fmt.Errorf("git diff failed: %s", strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func gitFileDiff(ctx context.Context, repoPath, path, prevPath string, cached bool) (string, error) {
	args := []string{
		"-c", "color.ui=false",
		"-C", repoPath,
		"diff",
		"--no-ext-diff",
		"--unified=3",
		"--find-renames",
	}
	if cached {
		args = append(args, "--cached")
	}
	args = append(args, "--", path)
	if prevPath != "" && prevPath != path {
		args = append(args, prevPath)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr := (&exec.ExitError{}); errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				return string(output), nil
			}
		}
		return "", fmt.Errorf("git diff failed: %s", strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func gitUntracked(ctx context.Context, repoPath string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "ls-files", "--others", "--exclude-standard", "-z")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files failed: %w", err)
	}
	if len(output) == 0 {
		return nil, nil
	}
	entries := strings.Split(string(output), "\x00")
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		path := strings.TrimSpace(entry)
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func gitDiffNoIndex(ctx context.Context, repoPath, relativePath string) (string, error) {
	absolutePath := filepath.Join(repoPath, relativePath)
	info, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("untracked path unavailable: %w", err)
	}
	if info.IsDir() {
		return "", nil
	}
	args := []string{
		"-C", repoPath,
		"diff",
		"--no-index",
		"--no-ext-diff",
		"--unified=3",
		"--",
		os.DevNull,
		relativePath,
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr := (&exec.ExitError{}); errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				return string(output), nil
			}
		}
		return "", fmt.Errorf("git diff for untracked file failed: %s", strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

type nameStatusEntry struct {
	status   string
	path     string
	prevPath string
}

type numstatEntry struct {
	path     string
	prevPath string
	added    int
	removed  int
	binary   bool
}

func collectRepoDiffSummary(ctx context.Context, repoPath string) ([]RepoDiffFile, error) {
	statusEntries, err := collectNameStatus(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	statsEntries, err := collectNumstat(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	fileMap := map[string]RepoDiffFile{}
	order := []string{}
	upsert := func(file RepoDiffFile) {
		key := fileKey(file.Path, file.PrevPath)
		if _, exists := fileMap[key]; !exists {
			order = append(order, key)
		}
		fileMap[key] = file
	}

	for _, entry := range statusEntries {
		file := RepoDiffFile{
			Path:     entry.path,
			PrevPath: entry.prevPath,
			Status:   statusLabel(entry.status),
		}
		upsert(file)
	}

	for _, entry := range statsEntries {
		key := fileKey(entry.path, entry.prevPath)
		file := fileMap[key]
		file.Path = entry.path
		file.PrevPath = entry.prevPath
		file.Added += entry.added
		file.Removed += entry.removed
		file.Binary = file.Binary || entry.binary
		if file.Status == "" {
			file.Status = "modified"
		}
		if entry.binary {
			file.Status = "binary"
		}
		upsert(file)
	}

	untracked, err := gitUntracked(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	for _, path := range untracked {
		added, removed, binary, err := untrackedNumstat(ctx, repoPath, path)
		if err != nil {
			return nil, err
		}
		file := RepoDiffFile{
			Path:    path,
			Added:   added,
			Removed: removed,
			Status:  "untracked",
			Binary:  binary,
		}
		upsert(file)
	}

	files := make([]RepoDiffFile, 0, len(order))
	for _, key := range order {
		files = append(files, fileMap[key])
	}
	return files, nil
}

func collectNameStatus(ctx context.Context, repoPath string) ([]nameStatusEntry, error) {
	entries := []nameStatusEntry{}
	for _, cached := range []bool{false, true} {
		data, err := gitNameStatus(ctx, repoPath, cached)
		if err != nil {
			return nil, err
		}
		entries = append(entries, parseNameStatusZ(data)...)
	}
	return entries, nil
}

func collectNumstat(ctx context.Context, repoPath string) ([]numstatEntry, error) {
	entries := []numstatEntry{}
	for _, cached := range []bool{false, true} {
		data, err := gitNumstat(ctx, repoPath, cached)
		if err != nil {
			return nil, err
		}
		entries = append(entries, parseNumstatZ(data)...)
	}
	return entries, nil
}

func gitNameStatus(ctx context.Context, repoPath string, cached bool) ([]byte, error) {
	args := []string{"-C", repoPath, "diff", "--name-status", "--find-renames", "-z"}
	if cached {
		args = append(args, "--cached")
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git name-status failed: %w", err)
	}
	return output, nil
}

func gitNumstat(ctx context.Context, repoPath string, cached bool) ([]byte, error) {
	args := []string{"-C", repoPath, "diff", "--numstat", "--find-renames", "-z"}
	if cached {
		args = append(args, "--cached")
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git numstat failed: %w", err)
	}
	return output, nil
}

func parseNameStatusZ(output []byte) []nameStatusEntry {
	tokens := strings.Split(string(output), "\x00")
	entries := []nameStatusEntry{}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token == "" {
			continue
		}
		status := token
		if i+1 >= len(tokens) {
			break
		}
		path := tokens[i+1]
		i++
		entry := nameStatusEntry{status: status, path: path}
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			if i+1 < len(tokens) && tokens[i+1] != "" {
				entry.prevPath = path
				entry.path = tokens[i+1]
				i++
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

func parseNumstatZ(output []byte) []numstatEntry {
	tokens := strings.Split(string(output), "\x00")
	entries := []numstatEntry{}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token == "" {
			continue
		}
		parts := strings.Split(token, "\t")
		if len(parts) < 3 {
			continue
		}
		added, removed, binary := parseNumstatCounts(parts[0], parts[1])
		entry := numstatEntry{
			path:    parts[2],
			added:   added,
			removed: removed,
			binary:  binary,
		}
		if i+1 < len(tokens) && tokens[i+1] != "" && !strings.Contains(tokens[i+1], "\t") {
			entry.prevPath = entry.path
			entry.path = tokens[i+1]
			i++
		}
		entries = append(entries, entry)
	}
	return entries
}

func parseNumstatCounts(addedRaw, removedRaw string) (int, int, bool) {
	if addedRaw == "-" || removedRaw == "-" {
		return 0, 0, true
	}
	added, _ := strconv.Atoi(addedRaw)
	removed, _ := strconv.Atoi(removedRaw)
	return added, removed, false
}

func statusLabel(status string) string {
	if status == "" {
		return "modified"
	}
	switch status[0] {
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R', 'C':
		return "renamed"
	case 'T':
		return "modified"
	default:
		return "modified"
	}
}

func fileKey(path, prevPath string) string {
	if prevPath == "" {
		return path
	}
	return prevPath + "->" + path
}

func untrackedNumstat(ctx context.Context, repoPath, relativePath string) (int, int, bool, error) {
	diff, err := gitDiffNoIndexNumstat(ctx, repoPath, relativePath)
	if err != nil {
		return 0, 0, false, err
	}
	entries := parseNumstatZ([]byte(diff))
	if len(entries) == 0 {
		return 0, 0, false, nil
	}
	return entries[0].added, entries[0].removed, entries[0].binary, nil
}

func gitDiffNoIndexNumstat(ctx context.Context, repoPath, relativePath string) (string, error) {
	absolutePath := filepath.Join(repoPath, relativePath)
	info, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("untracked path unavailable: %w", err)
	}
	if info.IsDir() {
		return "", nil
	}
	args := []string{
		"-C", repoPath,
		"diff",
		"--no-index",
		"--no-ext-diff",
		"--numstat",
		"-z",
		"--",
		os.DevNull,
		relativePath,
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr := (&exec.ExitError{}); errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				return string(output), nil
			}
		}
		return "", fmt.Errorf("git numstat for untracked file failed: %s", strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

const (
	maxDiffBytes = 2_000_000
	maxDiffLines = 20_000
)

// GetBranchDiffSummary returns a list of changed files between two refs (for PR view).
func (a *App) GetBranchDiffSummary(workspaceID, repoID, base, head string) (RepoDiffSummary, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	repoPath, err := a.resolveRepoPath(ctx, workspaceID, repoID)
	if err != nil {
		return RepoDiffSummary{}, err
	}

	if base == "" {
		base = "HEAD~1"
	}
	if head == "" {
		head = "HEAD"
	}

	files, err := collectBranchDiffSummary(ctx, repoPath, base, head)
	if err != nil {
		return RepoDiffSummary{}, err
	}

	summary := RepoDiffSummary{Files: files}
	for _, file := range files {
		summary.TotalAdded += file.Added
		summary.TotalRemoved += file.Removed
	}
	return summary, nil
}

// GetBranchFileDiff returns a patch for a single file between two refs (for PR view).
func (a *App) GetBranchFileDiff(workspaceID, repoID, base, head, path, prevPath string) (RepoFileDiffSnapshot, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	repoPath, err := a.resolveRepoPath(ctx, workspaceID, repoID)
	if err != nil {
		return RepoFileDiffSnapshot{}, err
	}
	if path == "" {
		return RepoFileDiffSnapshot{}, errors.New("file path is required")
	}

	if base == "" {
		base = "HEAD~1"
	}
	if head == "" {
		head = "HEAD"
	}

	patch, err := gitBranchFileDiff(ctx, repoPath, base, head, path, prevPath)
	if err != nil {
		return RepoFileDiffSnapshot{}, err
	}

	return finalizePatch(patch), nil
}

func collectBranchDiffSummary(ctx context.Context, repoPath, base, head string) ([]RepoDiffFile, error) {
	// Use merge-base to get the common ancestor for proper three-dot diff
	mergeBase, err := gitMergeBase(ctx, repoPath, base, head)
	if err != nil {
		// Fall back to direct diff if merge-base fails
		mergeBase = base
	}

	statusEntries, err := gitBranchNameStatus(ctx, repoPath, mergeBase, head)
	if err != nil {
		return nil, err
	}
	statsEntries, err := gitBranchNumstat(ctx, repoPath, mergeBase, head)
	if err != nil {
		return nil, err
	}

	fileMap := map[string]RepoDiffFile{}
	order := []string{}
	upsert := func(file RepoDiffFile) {
		key := fileKey(file.Path, file.PrevPath)
		if _, exists := fileMap[key]; !exists {
			order = append(order, key)
		}
		fileMap[key] = file
	}

	for _, entry := range statusEntries {
		file := RepoDiffFile{
			Path:     entry.path,
			PrevPath: entry.prevPath,
			Status:   statusLabel(entry.status),
		}
		upsert(file)
	}

	for _, entry := range statsEntries {
		key := fileKey(entry.path, entry.prevPath)
		file := fileMap[key]
		file.Path = entry.path
		file.PrevPath = entry.prevPath
		file.Added += entry.added
		file.Removed += entry.removed
		file.Binary = file.Binary || entry.binary
		if file.Status == "" {
			file.Status = "modified"
		}
		if entry.binary {
			file.Status = "binary"
		}
		upsert(file)
	}

	files := make([]RepoDiffFile, 0, len(order))
	for _, key := range order {
		files = append(files, fileMap[key])
	}
	return files, nil
}

func gitMergeBase(ctx context.Context, repoPath, ref1, ref2 string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "merge-base", ref1, ref2)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git merge-base failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func gitBranchNameStatus(ctx context.Context, repoPath, base, head string) ([]nameStatusEntry, error) {
	args := []string{"-C", repoPath, "diff", "--name-status", "--find-renames", "-z", base, head}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff name-status failed: %w", err)
	}
	return parseNameStatusZ(output), nil
}

func gitBranchNumstat(ctx context.Context, repoPath, base, head string) ([]numstatEntry, error) {
	args := []string{"-C", repoPath, "diff", "--numstat", "--find-renames", "-z", base, head}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff numstat failed: %w", err)
	}
	return parseNumstatZ(output), nil
}

func gitBranchFileDiff(ctx context.Context, repoPath, base, head, path, prevPath string) (string, error) {
	// Use merge-base for proper three-dot diff
	mergeBase, err := gitMergeBase(ctx, repoPath, base, head)
	if err != nil {
		mergeBase = base
	}

	args := []string{
		"-c", "color.ui=false",
		"-C", repoPath,
		"diff",
		"--no-ext-diff",
		"--unified=3",
		"--find-renames",
		mergeBase,
		head,
		"--",
		path,
	}
	if prevPath != "" && prevPath != path {
		args = append(args, prevPath)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr := (&exec.ExitError{}); errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				return string(output), nil
			}
		}
		return "", fmt.Errorf("git diff failed: %s", strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func finalizePatch(patch string) RepoFileDiffSnapshot {
	totalBytes := len(patch)
	totalLines := 0
	if patch != "" {
		totalLines = strings.Count(patch, "\n")
	}
	truncated := totalBytes > maxDiffBytes || totalLines > maxDiffLines
	if truncated {
		return RepoFileDiffSnapshot{
			Truncated:  true,
			TotalBytes: totalBytes,
			TotalLines: totalLines,
		}
	}
	return RepoFileDiffSnapshot{
		Patch:      patch,
		TotalBytes: totalBytes,
		TotalLines: totalLines,
	}
}
