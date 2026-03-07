package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/strantalis/workset/pkg/worksetapi"
)

const (
	defaultRepoFileSearchLimit = 250
	maxRepoFileSearchLimit     = 5000
	maxRepoFileReadBytes       = 256 * 1024
	repoFileBinarySniffBytes   = 8 * 1024
	repoFileIndexCacheTTL      = 10 * time.Second
)

type RepoFileSearchRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId,omitempty"`
	Query       string `json:"query"`
	Limit       int    `json:"limit,omitempty"`
}

type RepoFileSearchResult struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	RepoName    string `json:"repoName"`
	Path        string `json:"path"`
	IsMarkdown  bool   `json:"isMarkdown"`
	SizeBytes   int    `json:"sizeBytes"`
	Score       int    `json:"score"`
}

type RepoFileReadRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Path        string `json:"path"`
}

type RepoFileReadResponse struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	RepoName    string `json:"repoName"`
	Path        string `json:"path"`
	Content     string `json:"content,omitempty"`
	SizeBytes   int    `json:"sizeBytes"`
	IsBinary    bool   `json:"isBinary"`
	IsTruncated bool   `json:"isTruncated"`
	IsMarkdown  bool   `json:"isMarkdown"`
}

type workspaceRepoRef struct {
	id   string
	name string
	path string
}

type repoFileContent struct {
	content   string
	sizeBytes int
	binary    bool
	truncated bool
}

type repoFileIndexItem struct {
	path           string
	pathFolded     string
	repoPathFolded string
	isMarkdown     bool
	sizeBytes      int
}

type repoFileIndexCacheEntry struct {
	loadedAt time.Time
	items    []repoFileIndexItem
}

func (a *App) SearchWorkspaceRepoFiles(input RepoFileSearchRequest) ([]RepoFileSearchResult, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if workspaceID == "" {
		return nil, errors.New("workspace is required")
	}
	repoIDFilter := strings.TrimSpace(input.RepoID)
	query := strings.TrimSpace(input.Query)

	repos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	limit := normalizeRepoFileSearchLimit(input.Limit)
	results := make([]RepoFileSearchResult, 0, limit)
	queryFolded := strings.ToLower(query)

	for _, repo := range repos {
		if repoIDFilter != "" && repo.id != repoIDFilter {
			continue
		}
		indexItems, err := a.loadRepoFileIndex(ctx, repo)
		if err != nil {
			return nil, err
		}
		for _, item := range indexItems {
			if !matchesRepoFileQuery(item.pathFolded, item.repoPathFolded, queryFolded) {
				continue
			}
			results = append(results, RepoFileSearchResult{
				WorkspaceID: workspaceID,
				RepoID:      repo.id,
				RepoName:    repo.name,
				Path:        item.path,
				IsMarkdown:  item.isMarkdown,
				SizeBytes:   item.sizeBytes,
				Score:       repoFileMatchScore(item.pathFolded, item.repoPathFolded, queryFolded),
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		left := results[i]
		right := results[j]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if left.RepoName != right.RepoName {
			return left.RepoName < right.RepoName
		}
		return left.Path < right.Path
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (a *App) ReadWorkspaceRepoFile(input RepoFileReadRequest) (RepoFileReadResponse, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	repoID := strings.TrimSpace(input.RepoID)
	if workspaceID == "" || repoID == "" {
		return RepoFileReadResponse{}, errors.New("workspace and repo are required")
	}

	repos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return RepoFileReadResponse{}, err
	}
	var repo workspaceRepoRef
	found := false
	for _, candidate := range repos {
		if candidate.id != repoID {
			continue
		}
		repo = candidate
		found = true
		break
	}
	if !found {
		return RepoFileReadResponse{}, fmt.Errorf("repo %q not found in workspace %q", repoID, workspaceID)
	}

	resolvedPath, normalizedPath, err := resolveRepoFilePath(repo.path, input.Path)
	if err != nil {
		return RepoFileReadResponse{}, err
	}
	content, err := readRepoFileContent(resolvedPath, maxRepoFileReadBytes)
	if err != nil {
		return RepoFileReadResponse{}, err
	}

	return RepoFileReadResponse{
		WorkspaceID: workspaceID,
		RepoID:      repo.id,
		RepoName:    repo.name,
		Path:        normalizedPath,
		Content:     content.content,
		SizeBytes:   content.sizeBytes,
		IsBinary:    content.binary,
		IsTruncated: content.truncated,
		IsMarkdown:  isMarkdownPath(normalizedPath),
	}, nil
}

func (a *App) resolveWorkspaceRepos(ctx context.Context, workspaceID string) ([]workspaceRepoRef, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, errors.New("workspace is required")
	}
	workspacePath, err := a.resolveWorkspacePath(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	repos, err := a.service.ListRepos(ctx, worksetapi.WorkspaceSelector{Value: workspaceID})
	if err != nil {
		return nil, err
	}

	entries := make([]workspaceRepoRef, 0, len(repos.Repos))
	for _, repo := range repos.Repos {
		repoPath, err := resolveWorkspaceRepoRoot(workspacePath, repo.RepoDir, repo.LocalPath)
		if err != nil {
			return nil, fmt.Errorf("resolve repo %q: %w", repo.Name, err)
		}
		entries = append(entries, workspaceRepoRef{
			id:   workspaceID + "::" + repo.Name,
			name: repo.Name,
			path: repoPath,
		})
	}
	return entries, nil
}

func resolveWorkspaceRepoRoot(workspacePath, repoDir, localPath string) (string, error) {
	if repoDir != "" && workspacePath != "" {
		worktreePath := filepath.Join(workspacePath, repoDir)
		if stat, err := os.Stat(worktreePath); err == nil && stat.IsDir() {
			return filepath.Clean(worktreePath), nil
		}
	}
	if localPath != "" {
		if stat, err := os.Stat(localPath); err == nil && stat.IsDir() {
			return filepath.Clean(localPath), nil
		}
		return "", fmt.Errorf("repo path unavailable: %s", localPath)
	}
	return "", errors.New("repo path not found")
}

func normalizeRepoFileSearchLimit(limit int) int {
	if limit <= 0 {
		return defaultRepoFileSearchLimit
	}
	if limit > maxRepoFileSearchLimit {
		return maxRepoFileSearchLimit
	}
	return limit
}

func (a *App) loadRepoFileIndex(
	ctx context.Context,
	repo workspaceRepoRef,
) ([]repoFileIndexItem, error) {
	cacheKey := repo.id
	now := time.Now()

	a.repoFileIndexMu.Lock()
	cached, ok := a.repoFileIndexes[cacheKey]
	if ok && now.Sub(cached.loadedAt) < repoFileIndexCacheTTL {
		items := cached.items
		a.repoFileIndexMu.Unlock()
		return items, nil
	}
	a.repoFileIndexMu.Unlock()

	items, err := buildRepoFileIndex(ctx, repo)
	if err != nil {
		return nil, err
	}

	a.repoFileIndexMu.Lock()
	a.repoFileIndexes[cacheKey] = repoFileIndexCacheEntry{
		loadedAt: now,
		items:    items,
	}
	a.repoFileIndexMu.Unlock()
	return items, nil
}

var buildRepoFileIndex = func(ctx context.Context, repo workspaceRepoRef) ([]repoFileIndexItem, error) {
	paths, err := listRepoFiles(ctx, repo.path)
	if err != nil {
		return nil, err
	}
	items := make([]repoFileIndexItem, 0, len(paths))
	repoNameFolded := strings.ToLower(repo.name)
	for _, path := range paths {
		pathFolded := strings.ToLower(path)
		items = append(items, repoFileIndexItem{
			path:           path,
			pathFolded:     pathFolded,
			repoPathFolded: repoNameFolded + "/" + pathFolded,
			isMarkdown:     isMarkdownPath(path),
			sizeBytes:      repoFileSize(repo.path, path),
		})
	}
	return items, nil
}

func matchesRepoFileQuery(pathFolded, repoPathFolded, queryFolded string) bool {
	if queryFolded == "" {
		return true
	}
	return strings.Contains(pathFolded, queryFolded) ||
		strings.Contains(repoPathFolded, queryFolded)
}

func repoFileMatchScore(pathFolded, repoPathFolded, queryFolded string) int {
	switch {
	case strings.HasPrefix(pathFolded, queryFolded):
		return 3
	case strings.Contains(pathFolded, queryFolded):
		return 2
	case strings.Contains(repoPathFolded, queryFolded):
		return 1
	default:
		return 0
	}
}

func repoFileSize(repoRoot, relPath string) int {
	targetPath := filepath.Join(repoRoot, filepath.FromSlash(relPath))
	info, err := os.Lstat(targetPath)
	if err != nil {
		return 0
	}
	if info.IsDir() {
		return 0
	}
	return int(info.Size())
}

func listRepoFiles(ctx context.Context, repoPath string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "ls-files", "-z", "-c", "-o", "--exclude-standard")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("git ls-files failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git ls-files failed: %w", err)
	}
	if len(output) == 0 {
		return []string{}, nil
	}

	parts := bytes.Split(output, []byte{0})
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		paths = append(paths, filepath.ToSlash(string(part)))
	}
	sort.Strings(paths)
	return paths, nil
}

func resolveRepoFilePath(repoRoot, rawPath string) (string, string, error) {
	repoRoot = filepath.Clean(strings.TrimSpace(repoRoot))
	if repoRoot == "" {
		return "", "", errors.New("repo root is required")
	}
	cleanPath := filepath.Clean(strings.TrimSpace(rawPath))
	if cleanPath == "." || cleanPath == "" {
		return "", "", errors.New("file path is required")
	}
	if filepath.IsAbs(cleanPath) {
		return "", "", errors.New("file path must be relative")
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", "", errors.New("file path escapes repo root")
	}

	resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve repo root: %w", err)
	}
	targetPath := filepath.Join(repoRoot, cleanPath)
	resolvedTarget, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("file %q not found", filepath.ToSlash(cleanPath))
		}
		return "", "", fmt.Errorf("resolve file path: %w", err)
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil {
		return "", "", fmt.Errorf("resolve file path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", errors.New("file path escapes repo root")
	}

	info, err := os.Stat(resolvedTarget)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "", "", errors.New("file path points to a directory")
	}

	return resolvedTarget, filepath.ToSlash(cleanPath), nil
}

func readRepoFileContent(path string, maxBytes int) (repoFileContent, error) {
	file, err := os.Open(path)
	if err != nil {
		return repoFileContent{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return repoFileContent{}, err
	}

	limit := maxBytes
	if limit <= 0 {
		limit = maxRepoFileReadBytes
	}
	buffer := make([]byte, limit+1)
	n, readErr := file.Read(buffer)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return repoFileContent{}, readErr
	}
	data := buffer[:n]
	truncated := false
	if len(data) > limit {
		data = data[:limit]
		truncated = true
	} else if info.Size() > int64(limit) {
		truncated = true
	}

	if isBinaryContent(data) {
		return repoFileContent{
			sizeBytes: int(info.Size()),
			binary:    true,
			truncated: truncated,
		}, nil
	}

	return repoFileContent{
		content:   string(data),
		sizeBytes: int(info.Size()),
		binary:    false,
		truncated: truncated,
	}, nil
}

func isBinaryContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if len(data) > repoFileBinarySniffBytes {
		data = data[:repoFileBinarySniffBytes]
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}
	return !utf8.Valid(data)
}

func isMarkdownPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".markdown", ".mdown", ".mkd", ".mkdn", ".mdx":
		return true
	default:
		return false
	}
}
