package worksetapi

import (
	"testing"

	"github.com/strantalis/workset/internal/config"
)

func TestBuildNewWorkspaceRepoPlans(t *testing.T) {
	cfg := config.GlobalConfig{
		Defaults: config.Defaults{BaseBranch: "main"},
		Repos: map[string]config.RepoAlias{
			"app": {URL: "https://example.com/app.git", DefaultBranch: "dev"},
		},
		Groups: map[string]config.Group{
			"core": {
				Members: []config.GroupMember{
					{
						Repo: "app",
					},
				},
			},
		},
	}

	plans, err := buildNewWorkspaceRepoPlans(cfg, []string{"core"}, []string{"app"})
	if err != nil {
		t.Fatalf("build plans: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected one plan, got %d", len(plans))
	}
	if plans[0].Name != "app" || plans[0].Remotes.Base.DefaultBranch != "dev" {
		t.Fatalf("unexpected plan: %+v", plans[0])
	}
}

func TestBuildNewWorkspaceRepoPlansConflict(t *testing.T) {
	cfg := config.GlobalConfig{
		Defaults: config.Defaults{BaseBranch: "main"},
		Repos: map[string]config.RepoAlias{
			"app": {URL: "https://example.com/app.git"},
		},
		Groups: map[string]config.Group{
			"core": {
				Members: []config.GroupMember{
					{
						Repo: "app",
						Remotes: config.Remotes{
							Base:  config.RemoteConfig{Name: "origin"},
							Write: config.RemoteConfig{Name: "origin"},
						},
					},
				},
			},
		},
	}

	_, err := buildNewWorkspaceRepoPlans(cfg, []string{"core"}, []string{"app"})
	if err == nil {
		t.Fatalf("expected conflict error")
	}
}

func TestResolveAliasPlanErrors(t *testing.T) {
	cfg := config.GlobalConfig{}
	_, err := resolveAliasPlan(cfg, "missing")
	if err == nil {
		t.Fatalf("expected missing alias error")
	}

	cfg.Repos = map[string]config.RepoAlias{
		"empty": {},
	}
	_, err = resolveAliasPlan(cfg, "empty")
	if err == nil {
		t.Fatalf("expected missing source error")
	}
}

func TestResolveGroupMemberPlanErrors(t *testing.T) {
	cfg := config.GlobalConfig{}
	_, err := resolveGroupMemberPlan(cfg, config.GroupMember{Repo: "missing"})
	if err == nil {
		t.Fatalf("expected missing alias error")
	}

	cfg.Repos = map[string]config.RepoAlias{"empty": {}}
	_, err = resolveGroupMemberPlan(cfg, config.GroupMember{Repo: "empty"})
	if err == nil {
		t.Fatalf("expected missing source error")
	}
}
