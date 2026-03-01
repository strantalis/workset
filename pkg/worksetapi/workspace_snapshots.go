package worksetapi

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/ops"
	"github.com/strantalis/workset/internal/workspace"
)

// ListWorkspaceSnapshots returns workspace snapshots with optional repo status.
func (s *Service) ListWorkspaceSnapshots(ctx context.Context, opts WorkspaceSnapshotOptions) (WorkspaceSnapshotResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return WorkspaceSnapshotResult{}, err
	}
	if len(cfg.Workspaces) == 0 {
		return WorkspaceSnapshotResult{Workspaces: []WorkspaceSnapshotJSON{}, Config: info}, nil
	}

	names := make([]string, 0, len(cfg.Workspaces))
	for name := range cfg.Workspaces {
		names = append(names, name)
	}
	sort.Strings(names)

	snapshots := make([]WorkspaceSnapshotJSON, 0, len(names))
	for _, name := range names {
		ref := cfg.Workspaces[name]
		if !opts.IncludeArchived && ref.ArchivedAt != "" {
			continue
		}
		root := ref.Path
		if root == "" {
			if s.logf != nil {
				s.logf("workspace snapshots: skipping %q (path missing)", name)
			}
			continue
		}

		wsConfig, err := s.workspaces.LoadConfig(ctx, root)
		hasWorkspaceConfig := true
		if err != nil {
			if os.IsNotExist(err) {
				if s.logf != nil {
					s.logf("workspace snapshots: workspace config missing for %q at %s", name, worksetFilePath(root))
				}
				hasWorkspaceConfig = false
				wsConfig = config.WorkspaceConfig{}
			} else {
				if s.logf != nil {
					s.logf("workspace snapshots: skipping %q (load config: %v)", name, err)
				}
				continue
			}
		}
		if hasWorkspaceConfig {
			if err := s.migrateLegacyWorkspaceRemotes(ctx, &cfg, info.Path, root, &wsConfig); err != nil {
				return WorkspaceSnapshotResult{}, err
			}
		}
		var state workspace.State
		if hasWorkspaceConfig {
			state, err = s.workspaces.LoadState(ctx, root)
			if err != nil && !os.IsNotExist(err) {
				if s.logf != nil {
					s.logf("workspace snapshots: state unavailable for %q: %v", name, err)
				}
				state = workspace.State{}
			}
		}

		repos := make([]RepoSnapshotJSON, 0, len(wsConfig.Repos))
		repoDefaults := make(map[string]string, len(wsConfig.Repos))
		for _, repo := range wsConfig.Repos {
			config.ApplyRepoDefaults(&repo, cfg.Defaults)
			defaults := resolveRepoDefaults(cfg, repo.Name)
			repoDefaults[repo.Name] = defaults.DefaultBranch
			var trackedPR *TrackedPullRequestSnapshotJSON
			if pr, ok := state.PullRequests[repo.Name]; ok {
				trackedPR = &TrackedPullRequestSnapshotJSON{
					Repo:          pr.Repo,
					Number:        pr.Number,
					URL:           pr.URL,
					Title:         pr.Title,
					Body:          pr.Body,
					State:         pr.State,
					Draft:         pr.Draft,
					Merged:        pr.Merged,
					BaseRepo:      pr.BaseRepo,
					BaseBranch:    pr.BaseBranch,
					HeadRepo:      pr.HeadRepo,
					HeadBranch:    pr.HeadBranch,
					UpdatedAt:     pr.UpdatedAt,
					Author:        pr.Author,
					CommentsCount: pr.CommentsCount,
				}
			}
			repos = append(repos, RepoSnapshotJSON{
				Name:               repo.Name,
				LocalPath:          repo.LocalPath,
				Managed:            repo.Managed,
				RepoDir:            repo.RepoDir,
				Remote:             defaults.Remote,
				DefaultBranch:      defaults.DefaultBranch,
				Dirty:              false,
				Missing:            false,
				StatusKnown:        false,
				TrackedPullRequest: trackedPR,
			})
		}

		if opts.IncludeStatus && hasWorkspaceConfig {
			statuses, err := ops.Status(ctx, ops.StatusInput{
				WorkspaceRoot:       root,
				Defaults:            cfg.Defaults,
				RepoDefaultBranches: repoDefaults,
				Git:                 s.git,
			})
			if err != nil {
				if s.logf != nil {
					s.logf("workspace snapshots: status unavailable for %q: %v", name, err)
				}
			} else {
				byName := map[string]ops.RepoStatus{}
				for _, status := range statuses {
					byName[status.Name] = status
				}
				for i := range repos {
					if status, ok := byName[repos[i].Name]; ok && status.Err == nil {
						repos[i].Dirty = status.Dirty
						repos[i].Missing = status.Missing
						repos[i].StatusKnown = true
					}
				}
			}
		}
		workset := workspaceRefWorkset(ref)
		worksetKey, worksetLabel := deriveWorksetIdentity(name, workset, repos)

		snapshots = append(snapshots, WorkspaceSnapshotJSON{
			Name:           name,
			Path:           ref.Path,
			Workset:        workset,
			Template:       workset,
			WorksetKey:     worksetKey,
			WorksetLabel:   worksetLabel,
			CreatedAt:      ref.CreatedAt,
			LastUsed:       ref.LastUsed,
			ArchivedAt:     ref.ArchivedAt,
			ArchivedReason: ref.ArchivedReason,
			Archived:       ref.ArchivedAt != "",
			Pinned:         ref.Pinned,
			PinOrder:       ref.PinOrder,
			Color:          ref.Color,
			Description:    ref.Description,
			Expanded:       ref.Expanded,
			Repos:          repos,
		})
	}

	return WorkspaceSnapshotResult{Workspaces: snapshots, Config: info}, nil
}

func deriveWorksetIdentity(workspaceName, explicitWorkset string, repos []RepoSnapshotJSON) (string, string) {
	workset := strings.TrimSpace(explicitWorkset)
	if workset != "" {
		return "workset:" + normalizeWorksetIdentity(workset), workset
	}

	repoByKey := map[string]string{}
	for _, repo := range repos {
		name := strings.TrimSpace(repo.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := repoByKey[key]; exists {
			continue
		}
		repoByKey[key] = name
	}
	if len(repoByKey) == 0 {
		trimmedWorkspaceName := strings.TrimSpace(workspaceName)
		if trimmedWorkspaceName == "" {
			trimmedWorkspaceName = "Workset"
		}
		return "workspace:" + strings.ToLower(trimmedWorkspaceName), trimmedWorkspaceName
	}

	keys := make([]string, 0, len(repoByKey))
	for key := range repoByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	sortedRepoNames := make([]string, 0, len(keys))
	for _, key := range keys {
		sortedRepoNames = append(sortedRepoNames, repoByKey[key])
	}

	if len(sortedRepoNames) == 1 {
		return "repo:" + keys[0], sortedRepoNames[0]
	}
	if len(sortedRepoNames) <= 2 {
		return "repos:" + strings.Join(keys, "|"), strings.Join(sortedRepoNames, " + ")
	}
	return "repos:" + strings.Join(keys, "|"), fmt.Sprintf("%s + %d repos", sortedRepoNames[0], len(sortedRepoNames)-1)
}

func normalizeWorksetIdentity(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return "unnamed"
	}

	var b strings.Builder
	lastDash := false
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if lastDash {
			continue
		}
		b.WriteRune('-')
		lastDash = true
	}

	normalized := strings.Trim(b.String(), "-")
	if normalized == "" {
		return "unnamed"
	}
	return normalized
}
