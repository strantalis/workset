package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const workspaceExtraRootPrefix = "::extra::"

type workspaceContentRoot struct {
	id          string
	name        string
	path        string
	isRepo      bool
	gitDetected bool
}

type WorkspaceExtraRoot struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	RelativePath string `json:"relativePath"`
	GitDetected  bool   `json:"gitDetected"`
}

func buildWorkspaceExtraRootID(workspaceID, relativePath string) string {
	return workspaceID + workspaceExtraRootPrefix + filepath.ToSlash(strings.TrimSpace(relativePath))
}

func isWorkspaceExtraRootID(workspaceID, rootID string) bool {
	return strings.HasPrefix(rootID, workspaceID+workspaceExtraRootPrefix)
}

func extraRootRelativePath(workspaceID, rootID string) string {
	if !isWorkspaceExtraRootID(workspaceID, rootID) {
		return ""
	}
	return strings.TrimPrefix(rootID, workspaceID+workspaceExtraRootPrefix)
}

func pathWithinRoot(rootPath, candidatePath string) bool {
	rel, err := filepath.Rel(rootPath, candidatePath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func isWorkspaceMetadataRoot(name string) bool {
	switch strings.TrimSpace(name) {
	case "", ".git", ".workset":
		return true
	default:
		return false
	}
}

func hasGitMetadata(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

func resolveDirectoryPath(rootPath, rawPath string) (string, string, error) {
	rootPath = filepath.Clean(strings.TrimSpace(rootPath))
	if rootPath == "" {
		return "", "", errors.New("root path is required")
	}

	cleanPath := filepath.Clean(strings.TrimSpace(rawPath))
	if cleanPath == "." {
		cleanPath = ""
	}
	if filepath.IsAbs(cleanPath) {
		return "", "", errors.New("directory path must be relative")
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", "", errors.New("directory path escapes root")
	}

	resolvedRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve root path: %w", err)
	}

	targetPath := rootPath
	if cleanPath != "" {
		targetPath = filepath.Join(rootPath, cleanPath)
	}
	resolvedTarget, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("directory %q not found", filepath.ToSlash(cleanPath))
		}
		return "", "", fmt.Errorf("resolve directory path: %w", err)
	}
	if !pathWithinRoot(resolvedRoot, resolvedTarget) {
		return "", "", errors.New("directory path escapes root")
	}
	if rel, err := filepath.Rel(resolvedRoot, resolvedTarget); err == nil && strings.HasPrefix(rel, ".git") {
		return "", "", errors.New("cannot access .git directory")
	}

	info, err := os.Stat(resolvedTarget)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", errors.New("directory path points to a file")
	}

	return resolvedTarget, filepath.ToSlash(cleanPath), nil
}

func listFileSystemDirectory(rootPath, dirPath string) ([]RepoDirectoryEntry, error) {
	resolvedDir, normalizedPath, err := resolveDirectoryPath(rootPath, dirPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(resolvedDir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	dirs := make([]RepoDirectoryEntry, 0)
	files := make([]RepoDirectoryEntry, 0)
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if isWorkspaceMetadataRoot(name) {
			continue
		}

		childPath := name
		if normalizedPath != "" {
			childPath = normalizedPath + "/" + name
		}

		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", name, err)
		}

		if info.IsDir() {
			dirs = append(dirs, RepoDirectoryEntry{
				Name:  name,
				Path:  childPath,
				IsDir: true,
			})
			continue
		}

		files = append(files, RepoDirectoryEntry{
			Name:       name,
			Path:       childPath,
			IsDir:      false,
			SizeBytes:  int(info.Size()),
			IsMarkdown: isMarkdownPath(name),
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name < dirs[j].Name
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	return append(dirs, files...), nil
}

func (a *App) resolveWorkspaceExtraRoots(
	ctx context.Context,
	workspaceID string,
) ([]workspaceContentRoot, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, errors.New("workspace is required")
	}

	workspacePath, err := a.resolveWorkspacePath(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	resolvedWorkspacePath, err := filepath.EvalSymlinks(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}

	configuredRepos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	claimedRoots := make(map[string]struct{}, len(configuredRepos))
	for _, repo := range configuredRepos {
		resolvedRepoPath, err := filepath.EvalSymlinks(repo.path)
		if err != nil {
			return nil, fmt.Errorf("resolve repo path %q: %w", repo.name, err)
		}
		claimedRoots[resolvedRepoPath] = struct{}{}
	}

	entries, err := os.ReadDir(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("read workspace directory: %w", err)
	}

	roots := make([]workspaceContentRoot, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if isWorkspaceMetadataRoot(name) {
			continue
		}

		childPath := filepath.Join(workspacePath, name)
		info, err := os.Stat(childPath)
		if err != nil || !info.IsDir() {
			continue
		}

		resolvedChildPath, err := filepath.EvalSymlinks(childPath)
		if err != nil || !pathWithinRoot(resolvedWorkspacePath, resolvedChildPath) {
			continue
		}
		if _, exists := claimedRoots[resolvedChildPath]; exists {
			continue
		}

		roots = append(roots, workspaceContentRoot{
			id:          buildWorkspaceExtraRootID(workspaceID, name),
			name:        name,
			path:        resolvedChildPath,
			isRepo:      false,
			gitDetected: hasGitMetadata(resolvedChildPath),
		})
	}

	sort.Slice(roots, func(i, j int) bool {
		return roots[i].name < roots[j].name
	})
	return roots, nil
}

func (a *App) resolveWorkspaceContentRoot(
	ctx context.Context,
	workspaceID string,
	rootID string,
) (workspaceContentRoot, error) {
	repos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return workspaceContentRoot{}, err
	}
	for _, repo := range repos {
		if repo.id != rootID {
			continue
		}
		return workspaceContentRoot{
			id:     repo.id,
			name:   repo.name,
			path:   repo.path,
			isRepo: true,
		}, nil
	}

	extraRoots, err := a.resolveWorkspaceExtraRoots(ctx, workspaceID)
	if err != nil {
		return workspaceContentRoot{}, err
	}
	for _, root := range extraRoots {
		if root.id == rootID {
			return root, nil
		}
	}

	return workspaceContentRoot{}, fmt.Errorf("root %q not found in workspace %q", rootID, workspaceID)
}

func (a *App) ListWorkspaceExtraRoots(workspaceID string) ([]WorkspaceExtraRoot, error) {
	ctx := a.appContext()

	roots, err := a.resolveWorkspaceExtraRoots(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}

	items := make([]WorkspaceExtraRoot, 0, len(roots))
	for _, root := range roots {
		items = append(items, WorkspaceExtraRoot{
			ID:           root.id,
			Label:        root.name,
			RelativePath: extraRootRelativePath(workspaceID, root.id),
			GitDetected:  root.gitDetected,
		})
	}
	return items, nil
}
