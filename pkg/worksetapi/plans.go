package worksetapi

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/ops"
)

type repoPlan struct {
	Name          string
	URL           string
	SourcePath    string
	Remote        string
	DefaultBranch string
}

func buildNewWorkspaceRepoPlans(cfg config.GlobalConfig, groupNames, repoNames []string) ([]repoPlan, error) {
	if len(groupNames) == 0 && len(repoNames) == 0 {
		return nil, nil
	}

	plans := make([]repoPlan, 0, len(groupNames)+len(repoNames))
	seen := make(map[string]repoPlan)

	addPlan := func(plan repoPlan) error {
		if existing, ok := seen[plan.Name]; ok {
			if !repoPlanEqual(existing, plan) {
				return fmt.Errorf("conflicting repo %q: %s vs %s", plan.Name, describeRepoDefaults(existing), describeRepoDefaults(plan))
			}
			return nil
		}
		seen[plan.Name] = plan
		plans = append(plans, plan)
		return nil
	}

	for _, groupName := range groupNames {
		groupName = strings.TrimSpace(groupName)
		if groupName == "" {
			continue
		}
		group, ok := cfg.Groups[groupName]
		if !ok {
			return nil, fmt.Errorf("group %q not found", groupName)
		}
		for _, member := range group.Members {
			plan, err := resolveGroupMemberPlan(cfg, member)
			if err != nil {
				return nil, err
			}
			if err := addPlan(plan); err != nil {
				return nil, err
			}
		}
	}

	for _, repoName := range repoNames {
		repoName = strings.TrimSpace(repoName)
		if repoName == "" {
			continue
		}

		// Try alias first
		if _, ok := cfg.Repos[repoName]; ok {
			plan, err := resolveAliasPlan(cfg, repoName)
			if err != nil {
				return nil, err
			}
			if err := addPlan(plan); err != nil {
				return nil, err
			}
			continue
		}

		// Handle URL or path directly
		plan := repoPlan{
			DefaultBranch: cfg.Defaults.BaseBranch,
			Remote:        cfg.Defaults.Remote,
		}
		if looksLikeURL(repoName) {
			plan.Name = ops.DeriveRepoNameFromURL(repoName)
			plan.URL = repoName
		} else if looksLikeLocalPath(repoName) {
			resolved, err := resolveLocalPathInput(repoName)
			if err != nil {
				return nil, err
			}
			plan.Name = filepath.Base(resolved)
			plan.SourcePath = resolved
		} else {
			return nil, fmt.Errorf("repo %q not found", repoName)
		}
		if err := addPlan(plan); err != nil {
			return nil, err
		}
	}

	return plans, nil
}

func resolveGroupMemberPlan(cfg config.GlobalConfig, member config.GroupMember) (repoPlan, error) {
	alias, ok := cfg.Repos[member.Repo]
	if !ok {
		return repoPlan{}, fmt.Errorf("repo alias %q not found in config", member.Repo)
	}

	defaultBranch := cfg.Defaults.BaseBranch
	if alias.DefaultBranch != "" {
		defaultBranch = alias.DefaultBranch
	}
	remote := strings.TrimSpace(alias.Remote)
	if remote == "" {
		remote = cfg.Defaults.Remote
	}

	plan := repoPlan{
		Name:          member.Repo,
		URL:           alias.URL,
		SourcePath:    alias.Path,
		Remote:        remote,
		DefaultBranch: defaultBranch,
	}
	if plan.URL == "" && plan.SourcePath == "" {
		return repoPlan{}, fmt.Errorf("repo alias %q has no source", member.Repo)
	}
	return plan, nil
}

func resolveAliasPlan(cfg config.GlobalConfig, name string) (repoPlan, error) {
	alias, ok := cfg.Repos[name]
	if !ok {
		return repoPlan{}, fmt.Errorf("repo alias %q not found in config", name)
	}

	defaultBranch := cfg.Defaults.BaseBranch
	if alias.DefaultBranch != "" {
		defaultBranch = alias.DefaultBranch
	}
	remote := strings.TrimSpace(alias.Remote)
	if remote == "" {
		remote = cfg.Defaults.Remote
	}

	plan := repoPlan{
		Name:          name,
		URL:           alias.URL,
		SourcePath:    alias.Path,
		Remote:        remote,
		DefaultBranch: defaultBranch,
	}
	if plan.URL == "" && plan.SourcePath == "" {
		return repoPlan{}, fmt.Errorf("repo alias %q has no source", name)
	}
	return plan, nil
}

func repoPlanEqual(a, b repoPlan) bool {
	return a.Name == b.Name &&
		a.URL == b.URL &&
		a.SourcePath == b.SourcePath &&
		a.Remote == b.Remote &&
		a.DefaultBranch == b.DefaultBranch
}

func describeRepoDefaults(plan repoPlan) string {
	if plan.Remote == "" && plan.DefaultBranch == "" {
		return "remote=<empty> branch=<empty>"
	}
	return fmt.Sprintf("remote=%s branch=%s", plan.Remote, plan.DefaultBranch)
}
