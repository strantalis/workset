package groups

import (
	"testing"

	"github.com/strantalis/workset/internal/config"
)

func TestUpsertListDelete(t *testing.T) {
	cfg := config.DefaultConfig()
	if err := Upsert(&cfg, "alpha", "desc"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	names := List(cfg)
	if len(names) != 1 || names[0] != "alpha" {
		t.Fatalf("expected alpha, got %v", names)
	}
	if err := Delete(&cfg, "alpha"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(List(cfg)) != 0 {
		t.Fatalf("expected empty list")
	}
}

func TestAddRemoveMember(t *testing.T) {
	cfg := config.DefaultConfig()
	if err := Upsert(&cfg, "alpha", ""); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	member := config.GroupMember{Repo: "repo1"}
	if err := AddMember(&cfg, "alpha", member); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	group, ok := Get(cfg, "alpha")
	if !ok || len(group.Members) != 1 {
		t.Fatalf("expected member added")
	}
	if err := RemoveMember(&cfg, "alpha", "repo1"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	group, _ = Get(cfg, "alpha")
	if len(group.Members) != 0 {
		t.Fatalf("expected member removed")
	}
}

func TestFromWorkspace(t *testing.T) {
	cfg := config.DefaultConfig()
	ws := config.WorkspaceConfig{
		Name: "demo",
		Repos: []config.RepoConfig{
			{
				Name: "repo1",
				Remotes: config.Remotes{
					Base:  config.RemoteConfig{Name: "upstream", DefaultBranch: "main"},
					Write: config.RemoteConfig{Name: "origin", DefaultBranch: "main"},
				},
			},
		},
	}
	if err := FromWorkspace(&cfg, "alpha", ws); err != nil {
		t.Fatalf("FromWorkspace: %v", err)
	}
	group, ok := Get(cfg, "alpha")
	if !ok || len(group.Members) != 1 {
		t.Fatalf("expected members from workspace")
	}
	if group.Members[0].Repo != "repo1" {
		t.Fatalf("expected repo1, got %s", group.Members[0].Repo)
	}
}
