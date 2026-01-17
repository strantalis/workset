package main

import (
	"fmt"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

type repoPlan struct {
	Name       string
	URL        string
	SourcePath string
	Remotes    config.Remotes
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
				return fmt.Errorf("conflicting repo %q: %s vs %s", plan.Name, describeRemotes(existing.Remotes), describeRemotes(plan.Remotes))
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
		plan, err := resolveAliasPlan(cfg, repoName)
		if err != nil {
			return nil, err
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

	baseBranch := cfg.Defaults.BaseBranch
	if member.Remotes.Base.DefaultBranch != "" {
		baseBranch = member.Remotes.Base.DefaultBranch
	} else if alias.DefaultBranch != "" {
		baseBranch = alias.DefaultBranch
	}

	baseRemote := member.Remotes.Base.Name
	if baseRemote == "" {
		baseRemote = cfg.Defaults.Remotes.Base
	}
	writeRemote := member.Remotes.Write.Name
	if writeRemote == "" {
		writeRemote = cfg.Defaults.Remotes.Write
	}

	plan := repoPlan{
		Name:       member.Repo,
		URL:        alias.URL,
		SourcePath: alias.Path,
		Remotes: config.Remotes{
			Base: config.RemoteConfig{
				Name:          baseRemote,
				DefaultBranch: baseBranch,
			},
			Write: config.RemoteConfig{
				Name:          writeRemote,
				DefaultBranch: baseBranch,
			},
		},
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

	baseBranch := cfg.Defaults.BaseBranch
	if alias.DefaultBranch != "" {
		baseBranch = alias.DefaultBranch
	}

	plan := repoPlan{
		Name:       name,
		URL:        alias.URL,
		SourcePath: alias.Path,
		Remotes: config.Remotes{
			Base: config.RemoteConfig{
				Name:          cfg.Defaults.Remotes.Base,
				DefaultBranch: baseBranch,
			},
			Write: config.RemoteConfig{
				Name:          cfg.Defaults.Remotes.Write,
				DefaultBranch: baseBranch,
			},
		},
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
		remoteEqual(a.Remotes.Base, b.Remotes.Base) &&
		remoteEqual(a.Remotes.Write, b.Remotes.Write)
}

func remoteEqual(a, b config.RemoteConfig) bool {
	return a.Name == b.Name && a.DefaultBranch == b.DefaultBranch
}

func describeRemotes(remotes config.Remotes) string {
	base := remotes.Base.Name
	if remotes.Base.DefaultBranch != "" {
		base = fmt.Sprintf("%s/%s", base, remotes.Base.DefaultBranch)
	}
	write := remotes.Write.Name
	if remotes.Write.DefaultBranch != "" {
		write = fmt.Sprintf("%s/%s", write, remotes.Write.DefaultBranch)
	}
	return fmt.Sprintf("base=%s write=%s", base, write)
}
