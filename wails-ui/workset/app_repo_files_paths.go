package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/pkg/worksetapi"
)

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
