package worksetapi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/hooks"
)

func TestPreviewRepoHooksLocalPath(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	repoPath := env.createLocalRepo("local-hooks")

	hookDir := filepath.Join(repoPath, ".workset")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatalf("mkdir hooks dir: %v", err)
	}
	data := []byte("hooks:\n  - id: bootstrap\n    on: [worktree.created]\n    run: [\"npm\", \"ci\"]\n")
	if err := os.WriteFile(filepath.Join(hookDir, "hooks.yaml"), data, 0o644); err != nil {
		t.Fatalf("write hooks file: %v", err)
	}

	result, err := env.svc.PreviewRepoHooks(ctx, RepoHooksPreviewInput{Source: repoPath})
	if err != nil {
		t.Fatalf("PreviewRepoHooks: %v", err)
	}
	if !result.Payload.Exists {
		t.Fatalf("expected hooks to exist")
	}
	resolvedRepoPath, err := resolveLocalPathInput(repoPath)
	if err != nil {
		t.Fatalf("resolve local path: %v", err)
	}
	if result.Payload.ResolvedSource != resolvedRepoPath {
		t.Fatalf("unexpected resolved source: %s", result.Payload.ResolvedSource)
	}
	if len(result.Payload.Hooks) != 1 || result.Payload.Hooks[0].ID != "bootstrap" {
		t.Fatalf("unexpected hooks: %+v", result.Payload.Hooks)
	}
}

func TestPreviewRepoHooksAliasURLUsesGitHubClient(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)

	cfg := env.loadConfig()
	cfg.Repos["widgets"] = config.RegisteredRepo{
		URL: "git@github.com:acme/widgets.git",
	}
	env.saveConfig(cfg)

	client := &readHelpersGitHubClient{
		getFileContentFunc: func(_ context.Context, owner, repo, path, ref string) ([]byte, bool, error) {
			if owner != "acme" || repo != "widgets" {
				t.Fatalf("unexpected repo lookup: %s/%s", owner, repo)
			}
			if path != hooks.RepoHooksPath {
				t.Fatalf("unexpected hook path: %s", path)
			}
			if ref != "main" {
				t.Fatalf("unexpected ref: %s", ref)
			}
			return []byte("hooks:\n  - id: bootstrap\n    on: [worktree.created]\n    run: [\"npm\", \"ci\"]\n"), true, nil
		},
	}
	provider := &readHelpersGitHubProvider{client: client}
	env.svc.github = provider

	result, err := env.svc.PreviewRepoHooks(ctx, RepoHooksPreviewInput{
		Source: "widgets",
		Ref:    "main",
	})
	if err != nil {
		t.Fatalf("PreviewRepoHooks: %v", err)
	}
	if provider.importCalls != 1 {
		t.Fatalf("expected one auth import call, got %d", provider.importCalls)
	}
	if len(provider.clientHosts) != 1 || provider.clientHosts[0] != defaultGitHubHost {
		t.Fatalf("unexpected client hosts: %+v", provider.clientHosts)
	}
	if !result.Payload.Exists {
		t.Fatalf("expected hooks to exist")
	}
	if result.Payload.Owner != "acme" || result.Payload.Repo != "widgets" {
		t.Fatalf("unexpected remote metadata: %+v", result.Payload)
	}
	if len(result.Payload.Hooks) != 1 || result.Payload.Hooks[0].ID != "bootstrap" {
		t.Fatalf("unexpected hooks: %+v", result.Payload.Hooks)
	}
}

func TestPreviewRepoHooksAliasURLMissingFile(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)

	cfg := env.loadConfig()
	cfg.Repos["widgets"] = config.RegisteredRepo{
		URL: "https://github.com/acme/widgets.git",
	}
	env.saveConfig(cfg)

	client := &readHelpersGitHubClient{
		getFileContentFunc: func(_ context.Context, _ string, _ string, _ string, _ string) ([]byte, bool, error) {
			return nil, false, nil
		},
	}
	env.svc.github = &readHelpersGitHubProvider{client: client}

	result, err := env.svc.PreviewRepoHooks(ctx, RepoHooksPreviewInput{Source: "widgets"})
	if err != nil {
		t.Fatalf("PreviewRepoHooks: %v", err)
	}
	if result.Payload.Exists {
		t.Fatalf("expected missing hooks file")
	}
}

func TestPreviewRepoHooksAliasPathMissingFile(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	repoPath := env.createLocalRepo("local-hooks")

	cfg := env.loadConfig()
	cfg.Repos["widgets"] = config.RegisteredRepo{Path: repoPath}
	env.saveConfig(cfg)

	result, err := env.svc.PreviewRepoHooks(ctx, RepoHooksPreviewInput{Source: "widgets"})
	if err != nil {
		t.Fatalf("PreviewRepoHooks: %v", err)
	}
	if result.Payload.Exists {
		t.Fatalf("expected missing hooks file")
	}
}

func TestPreviewRepoHooksRejectsUnknownShorthand(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)

	_, err := env.svc.PreviewRepoHooks(ctx, RepoHooksPreviewInput{Source: "widgets"})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	validationErr := requireErrorType[ValidationError](t, err)
	if validationErr.Message != "repo source must be a registered alias, local path, or git URL" {
		t.Fatalf("unexpected message: %q", validationErr.Message)
	}
}

func TestPreviewRepoHooksPropagatesGitHubClientErrors(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)

	cfg := env.loadConfig()
	cfg.Repos["widgets"] = config.RegisteredRepo{
		URL: "https://github.com/acme/widgets.git",
	}
	env.saveConfig(cfg)

	wantErr := errors.New("api down")
	client := &readHelpersGitHubClient{
		getFileContentFunc: func(_ context.Context, _ string, _ string, _ string, _ string) ([]byte, bool, error) {
			return nil, false, wantErr
		},
	}
	env.svc.github = &readHelpersGitHubProvider{client: client}

	_, err := env.svc.PreviewRepoHooks(ctx, RepoHooksPreviewInput{Source: "widgets"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}
