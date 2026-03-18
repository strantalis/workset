package worksetapi

import (
	"context"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

func (s *Service) rebuildWorksetRepoModel(ctx context.Context, cfg *config.GlobalConfig) bool {
	if cfg == nil {
		return false
	}
	changed := false
	if cfg.WorksetRepos == nil {
		cfg.WorksetRepos = map[string][]string{}
		changed = true
	}
	worksetThreads := map[string][]string{}
	threadRepos := map[string][]string{}
	for threadName, ref := range cfg.Workspaces {
		worksetName := strings.TrimSpace(workspaceRefWorkset(ref))
		if worksetName == "" {
			worksetName = strings.TrimSpace(threadName)
		}
		worksetThreads[worksetName] = append(worksetThreads[worksetName], threadName)
		repos, ok := s.loadThreadRepoNames(ctx, ref.Path)
		if !ok {
			continue
		}
		threadRepos[threadName] = repos
	}

	newWorksetRepos := map[string][]string{}
	for worksetName, repos := range cfg.WorksetRepos {
		normalizedWorkset := strings.TrimSpace(worksetName)
		if normalizedWorkset == "" {
			continue
		}
		if _, hasThreads := worksetThreads[normalizedWorkset]; hasThreads {
			continue
		}
		if orphanWorksetEntryStale(normalizedWorkset, cfg.Workspaces) {
			changed = true
			continue
		}
		newWorksetRepos[normalizedWorkset] = normalizeRepoNames(repos)
	}
	for worksetName, threads := range worksetThreads {
		base, hasBase := intersectThreadRepos(threads, threadRepos)
		if !hasBase {
			base = nil
		}
		newWorksetRepos[worksetName] = base
		for _, threadName := range threads {
			ref := cfg.Workspaces[threadName]
			current := normalizeRepoNames(ref.RepoOverrides)
			threadRepoSet, hasThreadRepos := threadRepos[threadName]
			next := current
			if hasThreadRepos {
				next = subtractRepoNames(threadRepoSet, base)
			} else if len(base) > 0 {
				next = subtractRepoNames(current, base)
			}
			if !sameRepoNames(current, next) {
				ref.RepoOverrides = next
				cfg.Workspaces[threadName] = ref
				changed = true
			}
		}
	}

	if !sameWorksetRepoMap(cfg.WorksetRepos, newWorksetRepos) {
		cfg.WorksetRepos = newWorksetRepos
		changed = true
	}
	return changed
}

func orphanWorksetEntryStale(worksetName string, workspaces map[string]config.WorkspaceRef) bool {
	if strings.TrimSpace(worksetName) == "" {
		return false
	}
	ref, ok := workspaces[worksetName]
	if !ok {
		return false
	}
	threadWorkset := strings.TrimSpace(workspaceRefWorkset(ref))
	if threadWorkset == "" {
		threadWorkset = strings.TrimSpace(worksetName)
	}
	return threadWorkset != strings.TrimSpace(worksetName)
}

func (s *Service) loadThreadRepoNames(ctx context.Context, root string) ([]string, bool) {
	if strings.TrimSpace(root) == "" {
		return nil, false
	}
	wsCfg, err := s.workspaces.LoadConfig(ctx, root)
	if err != nil {
		return nil, false
	}
	repos := make([]string, 0, len(wsCfg.Repos))
	for _, repo := range wsCfg.Repos {
		repoName := strings.TrimSpace(repo.Name)
		if repoName == "" {
			continue
		}
		repos = append(repos, repoName)
	}
	normalized := normalizeRepoNames(repos)
	return normalized, len(normalized) > 0
}

func intersectThreadRepos(threads []string, threadRepos map[string][]string) ([]string, bool) {
	base := []string(nil)
	initialized := false
	for _, threadName := range threads {
		repos, ok := threadRepos[threadName]
		if !ok {
			continue
		}
		if !initialized {
			base = append([]string(nil), repos...)
			initialized = true
			continue
		}
		base = intersectRepoNames(base, repos)
	}
	return base, initialized
}

func intersectRepoNames(base []string, other []string) []string {
	if len(base) == 0 || len(other) == 0 {
		return nil
	}
	otherSet := map[string]struct{}{}
	for _, repo := range normalizeRepoNames(other) {
		otherSet[strings.ToLower(repo)] = struct{}{}
	}
	out := make([]string, 0, len(base))
	for _, repo := range normalizeRepoNames(base) {
		if _, ok := otherSet[strings.ToLower(repo)]; ok {
			out = append(out, repo)
		}
	}
	return out
}

func subtractRepoNames(base []string, excluded []string) []string {
	base = normalizeRepoNames(base)
	if len(base) == 0 {
		return nil
	}
	excludedSet := map[string]struct{}{}
	for _, repo := range normalizeRepoNames(excluded) {
		excludedSet[strings.ToLower(repo)] = struct{}{}
	}
	out := make([]string, 0, len(base))
	for _, repo := range base {
		if _, ok := excludedSet[strings.ToLower(repo)]; ok {
			continue
		}
		out = append(out, repo)
	}
	return out
}

func normalizeRepoNames(repos []string) []string {
	if len(repos) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(repos))
	for _, repo := range repos {
		trimmed := strings.TrimSpace(repo)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func deriveWorksetLabelFromRepoNames(workspaceName string, repos []string) string {
	normalized := normalizeRepoNames(repos)
	if len(normalized) == 0 {
		return strings.TrimSpace(workspaceName)
	}
	return strings.Join(normalized, " + ")
}

func sameRepoNames(a []string, b []string) bool {
	a = normalizeRepoNames(a)
	b = normalizeRepoNames(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameWorksetRepoMap(a map[string][]string, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for worksetName, reposA := range a {
		reposB, ok := b[worksetName]
		if !ok {
			return false
		}
		if !sameRepoNames(reposA, reposB) {
			return false
		}
	}
	return true
}
