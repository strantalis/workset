package worksetapi

import (
	"context"
	"testing"
)

func TestAliasCRUD(t *testing.T) {
	env := newTestEnv(t)

	_, _, err := env.svc.GetAlias(context.Background(), "missing")
	_ = requireErrorType[NotFoundError](t, err)

	_, _, err = env.svc.CreateAlias(context.Background(), AliasUpsertInput{
		Name: "empty",
	})
	_ = requireErrorType[ValidationError](t, err)

	created, _, err := env.svc.CreateAlias(context.Background(), AliasUpsertInput{
		Name:      "repo-a",
		Source:    "https://example.com/repo-a.git",
		SourceSet: true,
	})
	if err != nil {
		t.Fatalf("create alias: %v", err)
	}
	if created.Status != "ok" {
		t.Fatalf("unexpected status: %s", created.Status)
	}

	_, _, err = env.svc.CreateAlias(context.Background(), AliasUpsertInput{
		Name:      "repo-a",
		Source:    "https://example.com/repo-a.git",
		SourceSet: true,
	})
	_ = requireErrorType[ConflictError](t, err)

	alias, _, err := env.svc.GetAlias(context.Background(), "repo-a")
	if err != nil {
		t.Fatalf("get alias: %v", err)
	}
	if alias.URL == "" {
		t.Fatalf("expected url set")
	}

	updated, _, err := env.svc.UpdateAlias(context.Background(), AliasUpsertInput{
		Name:             "repo-a",
		DefaultBranch:    "dev",
		DefaultBranchSet: true,
	})
	if err != nil {
		t.Fatalf("update alias: %v", err)
	}
	if updated.Status != "ok" {
		t.Fatalf("unexpected update status")
	}

	_, _, err = env.svc.UpdateAlias(context.Background(), AliasUpsertInput{
		Name:             "repo-a",
		DefaultBranch:    "",
		DefaultBranchSet: true,
	})
	_ = requireErrorType[ValidationError](t, err)

	_, _, err = env.svc.UpdateAlias(context.Background(), AliasUpsertInput{
		Name:      "repo-a",
		SourceSet: true,
	})
	_ = requireErrorType[ValidationError](t, err)

	local := env.createLocalRepo("repo-path")
	updated, _, err = env.svc.UpdateAlias(context.Background(), AliasUpsertInput{
		Name:      "repo-a",
		Source:    local,
		SourceSet: true,
	})
	if err != nil {
		t.Fatalf("update alias source: %v", err)
	}
	if updated.Status != "ok" {
		t.Fatalf("unexpected update status for source")
	}

	updated, _, err = env.svc.UpdateAlias(context.Background(), AliasUpsertInput{
		Name:      "repo-a",
		Source:    "https://example.com/repo-a.git",
		SourceSet: true,
	})
	if err != nil {
		t.Fatalf("update alias url: %v", err)
	}

	deleted, _, err := env.svc.DeleteAlias(context.Background(), "repo-a")
	if err != nil {
		t.Fatalf("delete alias: %v", err)
	}
	if deleted.Status != "ok" {
		t.Fatalf("unexpected delete status")
	}

	_, _, err = env.svc.UpdateAlias(context.Background(), AliasUpsertInput{
		Name: "missing",
	})
	_ = requireErrorType[NotFoundError](t, err)

	_, _, err = env.svc.DeleteAlias(context.Background(), "missing")
	_ = requireErrorType[NotFoundError](t, err)
}

func TestListAliasesSorted(t *testing.T) {
	env := newTestEnv(t)
	_, _, _ = env.svc.CreateAlias(context.Background(), AliasUpsertInput{
		Name:      "zeta",
		Source:    "https://example.com/zeta.git",
		SourceSet: true,
	})
	_, _, _ = env.svc.CreateAlias(context.Background(), AliasUpsertInput{
		Name:      "alpha",
		Source:    "https://example.com/alpha.git",
		SourceSet: true,
	})

	result, err := env.svc.ListAliases(context.Background())
	if err != nil {
		t.Fatalf("list aliases: %v", err)
	}
	if len(result.Aliases) != 2 {
		t.Fatalf("expected 2 aliases")
	}
	if result.Aliases[0].Name != "alpha" || result.Aliases[1].Name != "zeta" {
		t.Fatalf("unexpected order: %+v", result.Aliases)
	}
}
