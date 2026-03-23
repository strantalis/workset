package main

import (
	"bytes"
	"context"
	"encoding/base64"
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
	maxRepoImageReadBytes      = 10 * 1024 * 1024
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
	ctx := a.appContext()

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
	ctx := a.appContext()

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

	_, svc := a.serviceContext()
	repos, err := svc.ListRepos(ctx, worksetapi.WorkspaceSelector{Value: workspaceID})
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
		sizeBytes, ok := repoFileSize(repo.path, path)
		if !ok {
			continue
		}
		pathFolded := strings.ToLower(path)
		items = append(items, repoFileIndexItem{
			path:           path,
			pathFolded:     pathFolded,
			repoPathFolded: repoNameFolded + "/" + pathFolded,
			isMarkdown:     isMarkdownPath(path),
			sizeBytes:      sizeBytes,
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

func repoFileSize(repoRoot, relPath string) (int, bool) {
	targetPath := filepath.Join(repoRoot, filepath.FromSlash(relPath))
	info, err := os.Lstat(targetPath)
	if err != nil {
		return 0, false
	}
	if info.IsDir() {
		return 0, false
	}
	return int(info.Size()), true
}

func listRepoFiles(ctx context.Context, repoPath string) ([]string, error) {
	cmd := newGitCommandContext(ctx, "-C", repoPath, "ls-files", "-z", "-c", "-o", "--exclude-standard")
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

const maxRepoFileWriteBytes = 256 * 1024

type RepoFileWriteRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Path        string `json:"path"`
	Content     string `json:"content"`
}

type RepoFileWriteResponse struct {
	Written bool   `json:"written"`
	Error   string `json:"error,omitempty"`
}

func (a *App) WriteWorkspaceRepoFile(input RepoFileWriteRequest) (RepoFileWriteResponse, error) {
	ctx := a.appContext()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	repoID := strings.TrimSpace(input.RepoID)
	if workspaceID == "" || repoID == "" {
		return RepoFileWriteResponse{}, errors.New("workspace and repo are required")
	}
	if len(input.Content) > maxRepoFileWriteBytes {
		return RepoFileWriteResponse{}, fmt.Errorf("content too large (%d bytes, limit %d)", len(input.Content), maxRepoFileWriteBytes)
	}

	repos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return RepoFileWriteResponse{}, err
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
		return RepoFileWriteResponse{}, fmt.Errorf("repo %q not found in workspace %q", repoID, workspaceID)
	}

	resolvedPath, _, err := resolveRepoFilePath(repo.path, input.Path)
	if err != nil {
		return RepoFileWriteResponse{}, err
	}

	if err := os.WriteFile(resolvedPath, []byte(input.Content), 0644); err != nil {
		return RepoFileWriteResponse{}, fmt.Errorf("write file: %w", err)
	}

	return RepoFileWriteResponse{Written: true}, nil
}

type RepoFileAtRefRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Path        string `json:"path"`
	Ref         string `json:"ref"`
}

type RepoFileAtRefResponse struct {
	Content string `json:"content,omitempty"`
	Found   bool   `json:"found"`
	Binary  bool   `json:"binary"`
}

func (a *App) ReadWorkspaceRepoFileAtRef(input RepoFileAtRefRequest) (RepoFileAtRefResponse, error) {
	ctx := a.appContext()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	repoID := strings.TrimSpace(input.RepoID)
	ref := strings.TrimSpace(input.Ref)
	path := strings.TrimSpace(input.Path)
	if workspaceID == "" || repoID == "" || path == "" {
		return RepoFileAtRefResponse{}, errors.New("workspace, repo, and path are required")
	}
	if ref == "" {
		ref = "HEAD"
	}

	repos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return RepoFileAtRefResponse{}, err
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
		return RepoFileAtRefResponse{}, fmt.Errorf("repo %q not found in workspace %q", repoID, workspaceID)
	}

	cmd := newGitCommandContext(ctx, "-C", repo.path, "show", ref+":"+path)
	output, err := cmd.Output()
	if err != nil {
		// File may not exist at that ref (new file)
		return RepoFileAtRefResponse{Found: false}, nil
	}

	if isBinaryContent(output) {
		return RepoFileAtRefResponse{Found: true, Binary: true}, nil
	}

	// Enforce size limit
	if len(output) > maxRepoFileReadBytes {
		output = output[:maxRepoFileReadBytes]
	}

	return RepoFileAtRefResponse{
		Content: string(output),
		Found:   true,
		Binary:  false,
	}, nil
}

type RepoImageReadRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Path        string `json:"path"`
}

type RepoImageReadResponse struct {
	Base64   string `json:"base64,omitempty"`
	MimeType string `json:"mimeType"`
	Error    string `json:"error,omitempty"`
}

func (a *App) ReadWorkspaceRepoImageBase64(input RepoImageReadRequest) (RepoImageReadResponse, error) {
	ctx := a.appContext()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	repoID := strings.TrimSpace(input.RepoID)
	if workspaceID == "" || repoID == "" {
		return RepoImageReadResponse{}, errors.New("workspace and repo are required")
	}

	repos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return RepoImageReadResponse{}, err
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
		return RepoImageReadResponse{}, fmt.Errorf("repo %q not found in workspace %q", repoID, workspaceID)
	}

	mimeType := mimeTypeFromImageExt(input.Path)
	if mimeType == "" {
		return RepoImageReadResponse{Error: "unsupported image type"}, nil
	}

	resolvedPath, _, err := resolveRepoFilePath(repo.path, input.Path)
	if err != nil {
		return RepoImageReadResponse{Error: err.Error()}, nil
	}

	data, err := readRepoFileBytes(resolvedPath, maxRepoImageReadBytes)
	if err != nil {
		return RepoImageReadResponse{Error: err.Error()}, nil
	}

	return RepoImageReadResponse{
		Base64:   base64.StdEncoding.EncodeToString(data),
		MimeType: mimeType,
	}, nil
}

func readRepoFileBytes(path string, maxBytes int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > int64(maxBytes) {
		return nil, fmt.Errorf("file too large (%d bytes, limit %d)", info.Size(), maxBytes)
	}

	data, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func mimeTypeFromImageExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".bmp":
		return "image/bmp"
	case ".avif":
		return "image/avif"
	default:
		return ""
	}
}

// ── Lazy directory listing ───────────────────────────────

// RepoDirectoryListRequest describes which directory to list.
type RepoDirectoryListRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	DirPath     string `json:"dirPath"` // "" for root
}

// RepoDirectoryEntry is one child of a directory (file or subdirectory).
type RepoDirectoryEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	IsDir      bool   `json:"isDir"`
	SizeBytes  int    `json:"sizeBytes,omitempty"`
	IsMarkdown bool   `json:"isMarkdown,omitempty"`
	ChildCount int    `json:"childCount,omitempty"`
}

// ListRepoDirectory returns the immediate children of a directory within a
// workspace repo.  It projects over the cached file index (same data as
// SearchWorkspaceRepoFiles) so no extra git commands are issued.
func (a *App) ListRepoDirectory(input RepoDirectoryListRequest) ([]RepoDirectoryEntry, error) {
	ctx := a.appContext()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if workspaceID == "" {
		return nil, errors.New("workspace is required")
	}
	repoID := strings.TrimSpace(input.RepoID)
	if repoID == "" {
		return nil, errors.New("repo is required")
	}
	dirPath := strings.TrimSpace(input.DirPath)
	// Normalise: strip trailing slash.
	dirPath = strings.TrimRight(dirPath, "/")

	repos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	var repo *workspaceRepoRef
	for i := range repos {
		if repos[i].id == repoID {
			repo = &repos[i]
			break
		}
	}
	if repo == nil {
		return nil, fmt.Errorf("repo %q not found in workspace", repoID)
	}

	items, err := a.loadRepoFileIndex(ctx, *repo)
	if err != nil {
		return nil, err
	}

	prefix := ""
	depth := 0 // depth of the target directory
	if dirPath != "" {
		prefix = dirPath + "/"
		depth = strings.Count(dirPath, "/") + 1
	}

	type dirInfo struct {
		childCount int
	}
	dirs := make(map[string]*dirInfo)
	var files []RepoDirectoryEntry

	for _, item := range items {
		if dirPath != "" && !strings.HasPrefix(item.path, prefix) {
			continue
		}
		if dirPath == "" && !strings.Contains(item.path, "/") {
			// Root-level file.
			files = append(files, RepoDirectoryEntry{
				Name:       item.path,
				Path:       item.path,
				IsDir:      false,
				SizeBytes:  item.sizeBytes,
				IsMarkdown: item.isMarkdown,
			})
			continue
		}

		rest := item.path
		if prefix != "" {
			rest = strings.TrimPrefix(item.path, prefix)
		}
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 1 && dirPath != "" {
			// Direct file child of the target directory.
			files = append(files, RepoDirectoryEntry{
				Name:       parts[0],
				Path:       item.path,
				IsDir:      false,
				SizeBytes:  item.sizeBytes,
				IsMarkdown: item.isMarkdown,
			})
		} else if len(parts) >= 2 || (len(parts) == 1 && dirPath == "") {
			// File is deeper — contribute to subdirectory count.
			childDirName := parts[0]
			if dirPath == "" && strings.Contains(item.path, "/") {
				childDirName = strings.SplitN(item.path, "/", 2)[0]
			}
			d, ok := dirs[childDirName]
			if !ok {
				d = &dirInfo{}
				dirs[childDirName] = d
			}
			// Count unique immediate children of the subdirectory.
			// For simplicity, count all files under it.
			_ = depth // suppress unused
			d.childCount++
		}
	}

	entries := make([]RepoDirectoryEntry, 0, len(dirs)+len(files))
	// Collect directories.
	dirNames := make([]string, 0, len(dirs))
	for name := range dirs {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)
	for _, name := range dirNames {
		d := dirs[name]
		path := name
		if dirPath != "" {
			path = dirPath + "/" + name
		}
		entries = append(entries, RepoDirectoryEntry{
			Name:       name,
			Path:       path,
			IsDir:      true,
			ChildCount: d.childCount,
		})
	}
	// Collect files (sorted).
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})
	entries = append(entries, files...)

	return entries, nil
}

// ── Git Blame ──────────────────────────────────────────────

type RepoBlameRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Path        string `json:"path"`
	Ref         string `json:"ref,omitempty"`
}

type RepoBlameEntry struct {
	StartLine  int    `json:"startLine"`
	EndLine    int    `json:"endLine"`
	CommitHash string `json:"commitHash"`
	Author     string `json:"author"`
	AuthorDate string `json:"authorDate"`
	Summary    string `json:"summary"`
}

func (a *App) GetRepoBlame(input RepoBlameRequest) ([]RepoBlameEntry, error) {
	ctx := a.appContext()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	repoID := strings.TrimSpace(input.RepoID)
	path := strings.TrimSpace(input.Path)
	ref := strings.TrimSpace(input.Ref)
	if workspaceID == "" || repoID == "" || path == "" {
		return nil, errors.New("workspace, repo, and path are required")
	}
	if ref == "" {
		ref = "HEAD"
	}

	repos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	var repo workspaceRepoRef
	found := false
	for _, candidate := range repos {
		if candidate.id == repoID {
			repo = candidate
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("repo %q not found in workspace %q", repoID, workspaceID)
	}

	_, normalizedPath, err := resolveRepoFilePath(repo.path, path)
	if err != nil {
		return nil, err
	}

	cmd := newGitCommandContext(ctx, "blame", "--line-porcelain", ref, "--", normalizedPath)
	cmd.Dir = repo.path
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame: %w", err)
	}

	return parseBlameOutput(output), nil
}

func parseBlameOutput(output []byte) []RepoBlameEntry {
	var entries []RepoBlameEntry
	lines := strings.Split(string(output), "\n")

	var currentHash, author, authorDate, summary string
	var lineNum int

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if len(line) >= 40 && isHexString(line[:40]) {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				currentHash = parts[0]
				if n, err := fmt.Sscanf(parts[2], "%d", &lineNum); n == 1 && err == nil {
					// valid line number
				}
			}
			continue
		}
		if strings.HasPrefix(line, "author ") {
			author = strings.TrimPrefix(line, "author ")
			continue
		}
		if strings.HasPrefix(line, "author-time ") {
			ts := strings.TrimPrefix(line, "author-time ")
			if t, err := fmt.Sscanf(ts, "%d", new(int64)); t == 1 && err == nil {
				var epoch int64
				fmt.Sscanf(ts, "%d", &epoch) //nolint:errcheck
				authorDate = time.Unix(epoch, 0).Format(time.RFC3339)
			}
			continue
		}
		if strings.HasPrefix(line, "summary ") {
			summary = strings.TrimPrefix(line, "summary ")
			continue
		}
		if strings.HasPrefix(line, "\t") {
			// This is the content line — end of this blame entry
			if currentHash != "" && lineNum > 0 {
				// Try to merge with the last entry if same commit and adjacent
				if len(entries) > 0 {
					last := &entries[len(entries)-1]
					if last.CommitHash == currentHash && last.EndLine == lineNum-1 {
						last.EndLine = lineNum
						continue
					}
				}
				entries = append(entries, RepoBlameEntry{
					StartLine:  lineNum,
					EndLine:    lineNum,
					CommitHash: currentHash,
					Author:     author,
					AuthorDate: authorDate,
					Summary:    summary,
				})
			}
		}
	}

	return entries
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// ── File Creation ──────────────────────────────────────────

type RepoFileCreateRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Path        string `json:"path"`
	Content     string `json:"content"`
}

func resolveRepoFilePathForCreate(repoRoot, rawPath string) (string, string, error) {
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
	targetPath := filepath.Join(resolvedRoot, cleanPath)

	// Verify the target stays within the repo root
	rel, err := filepath.Rel(resolvedRoot, targetPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve file path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", errors.New("file path escapes repo root")
	}

	// Refuse paths inside .git
	if strings.HasPrefix(rel, ".git") {
		return "", "", errors.New("cannot create files inside .git directory")
	}

	return targetPath, filepath.ToSlash(cleanPath), nil
}

func (a *App) CreateWorkspaceRepoFile(input RepoFileCreateRequest) (RepoFileWriteResponse, error) {
	ctx := a.appContext()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	repoID := strings.TrimSpace(input.RepoID)
	if workspaceID == "" || repoID == "" {
		return RepoFileWriteResponse{}, errors.New("workspace and repo are required")
	}
	if len(input.Content) > maxRepoFileWriteBytes {
		return RepoFileWriteResponse{}, fmt.Errorf("content too large (%d bytes, limit %d)", len(input.Content), maxRepoFileWriteBytes)
	}

	repos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return RepoFileWriteResponse{}, err
	}
	var repo workspaceRepoRef
	found := false
	for _, candidate := range repos {
		if candidate.id == repoID {
			repo = candidate
			found = true
			break
		}
	}
	if !found {
		return RepoFileWriteResponse{}, fmt.Errorf("repo %q not found in workspace %q", repoID, workspaceID)
	}

	targetPath, _, err := resolveRepoFilePathForCreate(repo.path, input.Path)
	if err != nil {
		return RepoFileWriteResponse{}, err
	}

	// Check file doesn't already exist
	if _, err := os.Stat(targetPath); err == nil {
		return RepoFileWriteResponse{}, fmt.Errorf("file already exists: %s", input.Path)
	}

	// Create intermediate directories
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return RepoFileWriteResponse{}, fmt.Errorf("create directories: %w", err)
	}

	// Create file exclusively
	f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return RepoFileWriteResponse{}, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(input.Content); err != nil {
		return RepoFileWriteResponse{}, fmt.Errorf("write file: %w", err)
	}

	return RepoFileWriteResponse{Written: true}, nil
}

// ── File Deletion ──────────────────────────────────────────

type RepoFileDeleteRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Path        string `json:"path"`
}

type RepoFileDeleteResponse struct {
	Deleted bool   `json:"deleted"`
	Error   string `json:"error,omitempty"`
}

func (a *App) DeleteWorkspaceRepoFile(input RepoFileDeleteRequest) (RepoFileDeleteResponse, error) {
	ctx := a.appContext()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	repoID := strings.TrimSpace(input.RepoID)
	if workspaceID == "" || repoID == "" {
		return RepoFileDeleteResponse{}, errors.New("workspace and repo are required")
	}

	repos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return RepoFileDeleteResponse{}, err
	}
	var repo workspaceRepoRef
	found := false
	for _, candidate := range repos {
		if candidate.id == repoID {
			repo = candidate
			found = true
			break
		}
	}
	if !found {
		return RepoFileDeleteResponse{}, fmt.Errorf("repo %q not found in workspace %q", repoID, workspaceID)
	}

	resolvedPath, normalizedPath, err := resolveRepoFilePath(repo.path, input.Path)
	if err != nil {
		return RepoFileDeleteResponse{}, err
	}

	// Refuse to delete anything inside .git
	if strings.HasPrefix(normalizedPath, ".git") {
		return RepoFileDeleteResponse{}, errors.New("cannot delete files inside .git directory")
	}

	// Only delete files, not directories
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return RepoFileDeleteResponse{}, err
	}
	if info.IsDir() {
		return RepoFileDeleteResponse{}, errors.New("cannot delete directories")
	}

	if err := os.Remove(resolvedPath); err != nil {
		return RepoFileDeleteResponse{}, fmt.Errorf("delete file: %w", err)
	}

	return RepoFileDeleteResponse{Deleted: true}, nil
}
