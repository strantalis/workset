package main

import (
	"context"
	"fmt"

	"github.com/strantalis/workset/pkg/worksetapi"
	"github.com/urfave/cli/v3"
)

func apiService(ctx context.Context, cmd *cli.Command) *worksetapi.Service {
	_, _ = worksetapi.EnsureLoginEnv(ctx)
	opts := worksetapi.Options{
		ConfigPath: cmd.String("config"),
	}
	if verboseEnabled(cmd) {
		opts.Logf = func(format string, args ...any) {
			_, _ = fmt.Fprintf(commandErrWriter(cmd), format+"\n", args...)
		}
	}
	return worksetapi.NewService(opts)
}

func printConfigInfo(cmd *cli.Command, info any) {
	if !verboseEnabled(cmd) {
		return
	}
	switch value := info.(type) {
	case worksetapi.WorkspaceListResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.WorkspaceCreateResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.WorkspaceDeleteResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.RepoListResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.RepoAddResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.RepoRemoveResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.WorkspaceStatusResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.GroupListResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.SessionListResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.RegisteredRepoListResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.SessionStartResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.SessionActionResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.HooksRunResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.PullRequestCreateResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.PullRequestStatusResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.PullRequestReviewCommentsResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.PullRequestGenerateResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	case worksetapi.PullRequestTrackedResult:
		printConfigLoadInfo(cmd, cmd.String("config"), value.Config)
	default:
		// no-op
	}
}
