package worksetapi

import (
	"context"
	"testing"
)

func TestRegisteredRepoCRUD(t *testing.T) {
	env := newTestEnv(t)

	_, _, err := env.svc.GetRegisteredRepo(context.Background(), "missing")
	_ = requireErrorType[NotFoundError](t, err)

	_, _, err = env.svc.RegisterRepo(context.Background(), RepoRegistryInput{
		Name: "empty",
	})
	_ = requireErrorType[ValidationError](t, err)

	created, _, err := env.svc.RegisterRepo(context.Background(), RepoRegistryInput{
		Name:      "repo-a",
		Source:    "https://example.com/repo-a.git",
		SourceSet: true,
	})
	if err != nil {
		t.Fatalf("register repo: %v", err)
	}
	if created.Status != "ok" {
		t.Fatalf("unexpected status: %s", created.Status)
	}

	_, _, err = env.svc.RegisterRepo(context.Background(), RepoRegistryInput{
		Name:      "repo-a",
		Source:    "https://example.com/repo-a.git",
		SourceSet: true,
	})
	_ = requireErrorType[ConflictError](t, err)

	repo, _, err := env.svc.GetRegisteredRepo(context.Background(), "repo-a")
	if err != nil {
		t.Fatalf("get registered repo: %v", err)
	}
	if repo.URL == "" {
		t.Fatalf("expected url set")
	}

	updated, _, err := env.svc.UpdateRegisteredRepo(context.Background(), RepoRegistryInput{
		Name:             "repo-a",
		DefaultBranch:    "dev",
		DefaultBranchSet: true,
	})
	if err != nil {
		t.Fatalf("update registered repo: %v", err)
	}
	if updated.Status != "ok" {
		t.Fatalf("unexpected update status")
	}

	_, _, err = env.svc.UpdateRegisteredRepo(context.Background(), RepoRegistryInput{
		Name:             "repo-a",
		DefaultBranch:    "",
		DefaultBranchSet: true,
	})
	_ = requireErrorType[ValidationError](t, err)

	_, _, err = env.svc.UpdateRegisteredRepo(context.Background(), RepoRegistryInput{
		Name:      "repo-a",
		SourceSet: true,
	})
	_ = requireErrorType[ValidationError](t, err)

	local := env.createLocalRepo("repo-path")
	updated, _, err = env.svc.UpdateRegisteredRepo(context.Background(), RepoRegistryInput{
		Name:      "repo-a",
		Source:    local,
		SourceSet: true,
	})
	if err != nil {
		t.Fatalf("update registered repo source: %v", err)
	}
	if updated.Status != "ok" {
		t.Fatalf("unexpected update status for source")
	}

	updated, _, err = env.svc.UpdateRegisteredRepo(context.Background(), RepoRegistryInput{
		Name:      "repo-a",
		Source:    "https://example.com/repo-a.git",
		SourceSet: true,
	})
	if err != nil {
		t.Fatalf("update registered repo url: %v", err)
	}

	deleted, _, err := env.svc.UnregisterRepo(context.Background(), "repo-a")
	if err != nil {
		t.Fatalf("unregister repo: %v", err)
	}
	if deleted.Status != "ok" {
		t.Fatalf("unexpected delete status")
	}

	_, _, err = env.svc.UpdateRegisteredRepo(context.Background(), RepoRegistryInput{
		Name: "missing",
	})
	_ = requireErrorType[NotFoundError](t, err)

	_, _, err = env.svc.UnregisterRepo(context.Background(), "missing")
	_ = requireErrorType[NotFoundError](t, err)
}

func TestListRegisteredReposSorted(t *testing.T) {
	env := newTestEnv(t)
	_, _, _ = env.svc.RegisterRepo(context.Background(), RepoRegistryInput{
		Name:      "zeta",
		Source:    "https://example.com/zeta.git",
		SourceSet: true,
	})
	_, _, _ = env.svc.RegisterRepo(context.Background(), RepoRegistryInput{
		Name:      "alpha",
		Source:    "https://example.com/alpha.git",
		SourceSet: true,
	})

	result, err := env.svc.ListRegisteredRepos(context.Background())
	if err != nil {
		t.Fatalf("list registered repos: %v", err)
	}
	if len(result.Repos) != 2 {
		t.Fatalf("expected 2 repos")
	}
	if result.Repos[0].Name != "alpha" || result.Repos[1].Name != "zeta" {
		t.Fatalf("unexpected order: %+v", result.Repos)
	}
}
