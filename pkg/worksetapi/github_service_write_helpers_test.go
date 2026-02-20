package worksetapi

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/strantalis/workset/internal/config"
)

func TestCommitPullRequestChangesRequiresConfiguredAgent(t *testing.T) {
	svc := &Service{}

	err := svc.commitPullRequestChanges(context.Background(), repoResolution{}, "feature/tests")
	if err == nil {
		t.Fatalf("expected validation error")
	}
	validationErr := requireErrorType[ValidationError](t, err)
	if validationErr.Message != "defaults.agent is not configured" {
		t.Fatalf("unexpected message: %q", validationErr.Message)
	}
}

func TestCommitPullRequestChangesReturnsEarlyWhenPatchIsEmpty(t *testing.T) {
	var commands [][]string
	runner := func(_ context.Context, _ string, command []string, _ []string, _ string) (CommandResult, error) {
		commands = append(commands, command)
		return CommandResult{ExitCode: 0}, nil
	}
	svc := &Service{commands: runner}

	err := svc.commitPullRequestChanges(context.Background(), repoResolution{
		RepoPath: t.TempDir(),
		Repo:     config.RepoConfig{Name: "repo-a"},
		Defaults: config.Defaults{Agent: "codex"},
	}, "feature/tests")
	if err != nil {
		t.Fatalf("commitPullRequestChanges: %v", err)
	}

	if len(commands) != 3 {
		t.Fatalf("expected 3 git commands for patch generation, got %d", len(commands))
	}
	if slices.ContainsFunc(commands, func(cmd []string) bool {
		return len(cmd) >= 3 && cmd[0] == "git" && cmd[1] == "add" && cmd[2] == "-A"
	}) {
		t.Fatalf("unexpected git add call when patch is empty: %+v", commands)
	}
}

func TestRecordPullRequestPersistsWorkspaceState(t *testing.T) {
	env, root, _ := setupGitHubServiceRepo(t)
	ctx := context.Background()
	resolution, err := env.svc.resolveRepo(ctx, RepoSelectionInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
	})
	if err != nil {
		t.Fatalf("resolveRepo: %v", err)
	}

	env.svc.recordPullRequest(ctx, resolution, PullRequestCreatedJSON{
		Repo:       "repo-a",
		Number:     73,
		URL:        "https://github.com/base-org/base-repo/pull/73",
		Title:      "Track this PR",
		Body:       "Body text",
		Draft:      false,
		State:      "open",
		Merged:     false,
		BaseRepo:   "base-org/base-repo",
		BaseBranch: "main",
		HeadRepo:   "head-org/head-repo",
		HeadBranch: "feature/track-pr",
	})

	state, err := env.svc.workspaces.LoadState(ctx, resolution.WorkspaceRoot)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	record, ok := state.PullRequests["repo-a"]
	if !ok {
		t.Fatalf("expected tracked PR entry for repo-a")
	}
	if record.Number != 73 || record.Title != "Track this PR" || record.HeadBranch != "feature/track-pr" {
		t.Fatalf("unexpected tracked PR record: %+v", record)
	}
	if record.Merged {
		t.Fatalf("expected tracked PR record to be unmerged")
	}
	if record.UpdatedAt != env.now.Format(time.RFC3339) {
		t.Fatalf("unexpected updated_at timestamp: %q", record.UpdatedAt)
	}
}
