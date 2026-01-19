package worksetapi

import (
	"context"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
)

func TestGroupCRUDAndMembers(t *testing.T) {
	env := newTestEnv(t)

	_, _, err := env.svc.GetGroup(context.Background(), "missing")
	_ = requireErrorType[NotFoundError](t, err)

	_, _, err = env.svc.CreateGroup(context.Background(), GroupUpsertInput{})
	_ = requireErrorType[ValidationError](t, err)

	group, _, err := env.svc.CreateGroup(context.Background(), GroupUpsertInput{
		Name:        "core",
		Description: "core repos",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if group.Name != "core" {
		t.Fatalf("unexpected group name: %s", group.Name)
	}

	list, err := env.svc.ListGroups(context.Background())
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if len(list.Groups) != 1 || list.Groups[0].Name != "core" {
		t.Fatalf("unexpected groups: %+v", list.Groups)
	}

	group, _, err = env.svc.GetGroup(context.Background(), "core")
	if err != nil {
		t.Fatalf("get group: %v", err)
	}
	if group.Description != "core repos" {
		t.Fatalf("unexpected description: %s", group.Description)
	}

	group, _, err = env.svc.UpdateGroup(context.Background(), GroupUpsertInput{
		Name:        "core",
		Description: "updated",
	})
	if err != nil {
		t.Fatalf("update group: %v", err)
	}
	if group.Description != "updated" {
		t.Fatalf("expected updated description")
	}

	group, _, err = env.svc.AddGroupMember(context.Background(), GroupMemberInput{
		GroupName:  "core",
		RepoName:   "repo-a",
		BaseRemote: "origin",
	})
	if err != nil {
		t.Fatalf("add member: %v", err)
	}
	if len(group.Members) != 1 {
		t.Fatalf("expected 1 member")
	}

	_, _, err = env.svc.AddGroupMember(context.Background(), GroupMemberInput{})
	_ = requireErrorType[ValidationError](t, err)

	_, _, err = env.svc.RemoveGroupMember(context.Background(), GroupMemberInput{})
	_ = requireErrorType[ValidationError](t, err)

	group, _, err = env.svc.RemoveGroupMember(context.Background(), GroupMemberInput{
		GroupName: "core",
		RepoName:  "repo-a",
	})
	if err != nil {
		t.Fatalf("remove member: %v", err)
	}
	if len(group.Members) != 0 {
		t.Fatalf("expected no members")
	}

	_, _, err = env.svc.DeleteGroup(context.Background(), "core")
	if err != nil {
		t.Fatalf("delete group: %v", err)
	}

	_, _, err = env.svc.DeleteGroup(context.Background(), "missing")
	if err == nil {
		t.Fatalf("expected error for missing group")
	}
}

func TestApplyGroup(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	local := env.createLocalRepo("repo-a")

	cfg := env.loadConfig()
	cfg.Repos = map[string]config.RepoAlias{
		"repo-a": {Path: local, DefaultBranch: "main"},
	}
	cfg.Groups = map[string]config.Group{
		"core": {
			Members: []config.GroupMember{
				{
					Repo: "repo-a",
					Remotes: config.Remotes{
						Base: config.RemoteConfig{Name: "origin"},
					},
				},
			},
		},
	}
	env.saveConfig(cfg)

	result, _, err := env.svc.ApplyGroup(context.Background(), GroupApplyInput{
		Workspace: WorkspaceSelector{Value: root},
		Name:      "core",
	})
	if err != nil {
		t.Fatalf("apply group: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("unexpected status: %+v", result)
	}

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("load workspace config: %v", err)
	}
	if len(wsCfg.Repos) != 1 || wsCfg.Repos[0].Name != "repo-a" {
		t.Fatalf("expected repo added to workspace")
	}
}

func TestApplyGroupMissingGroup(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	_, _, err := env.svc.ApplyGroup(context.Background(), GroupApplyInput{
		Workspace: WorkspaceSelector{Value: root},
		Name:      "missing",
	})
	_ = requireErrorType[NotFoundError](t, err)
}
