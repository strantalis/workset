package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RepoFileDefinitionRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Path        string `json:"path"`
	Content     string `json:"content"`
	Line        int    `json:"line"`
	Character   int    `json:"character"`
}

type RepoFileDefinitionTarget struct {
	RepoID       string `json:"repoId"`
	Path         string `json:"path"`
	Line         int    `json:"line"`
	Character    int    `json:"character"`
	EndLine      int    `json:"endLine"`
	EndCharacter int    `json:"endCharacter"`
}

type RepoFileDefinitionResponse struct {
	Supported         bool                       `json:"supported"`
	Available         bool                       `json:"available"`
	Found             bool                       `json:"found"`
	Language          string                     `json:"language,omitempty"`
	Provider          string                     `json:"provider,omitempty"`
	Targets           []RepoFileDefinitionTarget `json:"targets,omitempty"`
	UnavailableReason string                     `json:"unavailableReason,omitempty"`
	InstallHint       string                     `json:"installHint,omitempty"`
}

func (a *App) GetRepoFileDefinition(input RepoFileDefinitionRequest) (RepoFileDefinitionResponse, error) {
	ctx := a.appContext()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	repoID := strings.TrimSpace(input.RepoID)
	if workspaceID == "" || repoID == "" {
		return RepoFileDefinitionResponse{}, errors.New("workspace and repo are required")
	}
	if strings.TrimSpace(input.Path) == "" {
		return RepoFileDefinitionResponse{}, errors.New("path is required")
	}
	if input.Line < 0 || input.Character < 0 {
		return RepoFileDefinitionResponse{}, errors.New("line and character must be non-negative")
	}

	root, err := a.resolveWorkspaceContentRoot(ctx, workspaceID, repoID)
	if err != nil {
		return RepoFileDefinitionResponse{}, err
	}

	resolvedPath, normalizedPath, err := resolveRepoFilePath(root.path, input.Path)
	if err != nil {
		return RepoFileDefinitionResponse{}, err
	}

	hoverRuntime, supported, err := resolveRepoHoverRuntime(root.path, resolvedPath)
	if err != nil {
		return RepoFileDefinitionResponse{}, err
	}
	if !supported {
		return RepoFileDefinitionResponse{}, nil
	}

	response := RepoFileDefinitionResponse{
		Supported: true,
		Language:  hoverRuntime.language,
		Provider:  hoverRuntime.provider,
	}
	if hoverRuntime.command == "" {
		response.InstallHint = hoverRuntime.installHint
		response.UnavailableReason = fmt.Sprintf("%s is not installed or not available in this repo.", hoverRuntime.provider)
		return response, nil
	}

	client, err := a.repoHoverClient(ctx, hoverRuntime)
	if err != nil {
		response.InstallHint = hoverRuntime.installHint
		response.UnavailableReason = err.Error()
		return response, nil
	}

	rawTargets, err := client.Definition(ctx, repoHoverLSPRequest{
		filePath:   resolvedPath,
		path:       normalizedPath,
		content:    input.Content,
		line:       input.Line,
		character:  input.Character,
		languageID: hoverRuntime.languageID,
		language:   hoverRuntime.language,
		provider:   hoverRuntime.provider,
	})
	if err != nil {
		a.invalidateRepoHoverClient(hoverRuntime)
		response.Available = true
		return response, nil
	}

	targets, err := a.resolveRepoFileDefinitionTargets(ctx, workspaceID, repoID, rawTargets)
	if err != nil {
		return RepoFileDefinitionResponse{}, err
	}
	response.Available = true
	response.Found = len(targets) > 0
	response.Targets = targets
	return response, nil
}

func (a *App) resolveRepoFileDefinitionTargets(
	ctx context.Context,
	workspaceID string,
	currentRootID string,
	rawTargets []repoFileDefinitionLocation,
) ([]RepoFileDefinitionTarget, error) {
	if len(rawTargets) == 0 {
		return nil, nil
	}

	roots, err := a.resolveWorkspaceDefinitionRoots(ctx, workspaceID, currentRootID)
	if err != nil {
		return nil, err
	}

	targets := make([]RepoFileDefinitionTarget, 0, len(rawTargets))
	seen := make(map[string]struct{}, len(rawTargets))
	for _, rawTarget := range rawTargets {
		target, ok := resolveRepoFileDefinitionTarget(roots, rawTarget)
		if !ok {
			continue
		}
		key := fmt.Sprintf(
			"%s|%s|%d|%d|%d|%d",
			target.RepoID,
			target.Path,
			target.Line,
			target.Character,
			target.EndLine,
			target.EndCharacter,
		)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, target)
	}
	return targets, nil
}

func (a *App) resolveWorkspaceDefinitionRoots(
	ctx context.Context,
	workspaceID string,
	currentRootID string,
) ([]workspaceContentRoot, error) {
	currentRoot, err := a.resolveWorkspaceContentRoot(ctx, workspaceID, currentRootID)
	if err != nil {
		return nil, err
	}

	roots := []workspaceContentRoot{currentRoot}
	repos, err := a.resolveWorkspaceRepos(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, repo := range repos {
		if repo.id == currentRootID {
			continue
		}
		roots = append(roots, workspaceContentRoot{
			id:     repo.id,
			name:   repo.name,
			path:   repo.path,
			isRepo: true,
		})
	}

	extraRoots, err := a.resolveWorkspaceExtraRoots(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, root := range extraRoots {
		if root.id == currentRootID {
			continue
		}
		roots = append(roots, root)
	}
	return roots, nil
}

func resolveRepoFileDefinitionTarget(
	roots []workspaceContentRoot,
	rawTarget repoFileDefinitionLocation,
) (RepoFileDefinitionTarget, bool) {
	targetPath := strings.TrimSpace(rawTarget.filePath)
	if targetPath == "" {
		return RepoFileDefinitionTarget{}, false
	}
	if resolvedTarget, err := filepath.EvalSymlinks(targetPath); err == nil {
		targetPath = resolvedTarget
	} else if !errors.Is(err, os.ErrNotExist) {
		return RepoFileDefinitionTarget{}, false
	}
	targetPath = filepath.Clean(targetPath)

	for _, root := range roots {
		rootPath := root.path
		if resolvedRoot, err := filepath.EvalSymlinks(rootPath); err == nil {
			rootPath = resolvedRoot
		}
		if !pathWithinRoot(rootPath, targetPath) {
			continue
		}
		relPath, err := filepath.Rel(rootPath, targetPath)
		if err != nil {
			continue
		}
		relPath = filepath.ToSlash(relPath)
		if relPath == "." || relPath == "" {
			continue
		}
		return RepoFileDefinitionTarget{
			RepoID:       root.id,
			Path:         relPath,
			Line:         max(0, rawTarget.startLine),
			Character:    max(0, rawTarget.startCharacter),
			EndLine:      max(0, rawTarget.endLine),
			EndCharacter: max(0, rawTarget.endCharacter),
		}, true
	}

	return RepoFileDefinitionTarget{}, false
}
