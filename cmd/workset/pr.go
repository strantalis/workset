package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/strantalis/workset/internal/output"
	"github.com/strantalis/workset/pkg/worksetapi"
	"github.com/urfave/cli/v3"
)

func prCommand() *cli.Command {
	return &cli.Command{
		Name:  "pr",
		Usage: "Manage pull requests (requires -w)",
		Commands: []*cli.Command{
			prCreateCommand(),
			prStatusCommand(),
			prChecksCommand(),
			prReviewsCommand(),
			prGenerateCommand(),
			prTrackedCommand(),
			prReplyCommand(),
			prEditCommand(),
			prDeleteCommand(),
			prResolveCommand(),
		},
	}
}

func prCreateCommand() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create a pull request",
		ArgsUsage: "-w <workspace> [--repo <alias>]",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "repo",
				Usage: "Workspace repo alias",
			},
			&cli.StringFlag{
				Name:  "base",
				Usage: "Base branch (defaults to upstream default branch)",
			},
			&cli.StringFlag{
				Name:  "head",
				Usage: "Head branch (defaults to current branch)",
			},
			&cli.StringFlag{
				Name:  "title",
				Usage: "Pull request title",
			},
			&cli.StringFlag{
				Name:  "body",
				Usage: "Pull request body",
			},
			&cli.BoolFlag{
				Name:  "draft",
				Usage: "Create as draft",
			},
			&cli.BoolFlag{
				Name:  "commit",
				Usage: "Stage and commit changes before creating the PR (uses defaults.agent)",
			},
			&cli.BoolFlag{
				Name:  "push",
				Usage: "Push the head branch before creating the PR",
			},
			&cli.BoolFlag{
				Name:  "ai",
				Usage: "Generate title/body with the configured agent",
			},
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Create PRs for all dirty repos in the workspace",
			},
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Skip confirmation prompts",
			},
		}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "repo") {
				completeWorkspaceRepoNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			svc := apiService(ctx, cmd)
			mode := outputModeFromContext(cmd)
			workspace := cmd.String("workspace")
			repo := cmd.String("repo")
			title := strings.TrimSpace(cmd.String("title"))
			body := strings.TrimSpace(cmd.String("body"))
			base := strings.TrimSpace(cmd.String("base"))
			head := strings.TrimSpace(cmd.String("head"))
			useAI := cmd.Bool("ai")

			if cmd.Bool("all") {
				status, err := svc.StatusWorkspace(ctx, worksetapi.WorkspaceSelector{Value: workspace})
				if err != nil {
					return err
				}
				printConfigInfo(cmd, status)
				names := make([]string, 0, len(status.Statuses))
				for _, entry := range status.Statuses {
					if entry.Dirty && !entry.Missing {
						names = append(names, entry.Name)
					}
				}
				sort.Strings(names)
				if len(names) == 0 {
					return errors.New("no dirty repos found for workspace")
				}
				if !cmd.Bool("yes") {
					ok, err := confirmPrompt(os.Stdin, commandWriter(cmd), fmt.Sprintf("Create PRs for %d repos? [y/N] ", len(names)))
					if err != nil {
						return err
					}
					if !ok {
						return nil
					}
				}
				results := make([]worksetapi.PullRequestCreatedJSON, 0, len(names))
				for _, name := range names {
					prTitle, prBody := title, body
					if useAI && (prTitle == "" || prBody == "") {
						generated, err := svc.GeneratePullRequestText(ctx, worksetapi.PullRequestGenerateInput{
							Workspace: worksetapi.WorkspaceSelector{Value: workspace},
							Repo:      name,
						})
						if err != nil {
							return err
						}
						if prTitle == "" {
							prTitle = generated.Payload.Title
						}
						if prBody == "" {
							prBody = generated.Payload.Body
						}
					}
					result, err := svc.CreatePullRequest(ctx, worksetapi.PullRequestCreateInput{
						Workspace:  worksetapi.WorkspaceSelector{Value: workspace},
						Repo:       name,
						Base:       base,
						Head:       head,
						Title:      prTitle,
						Body:       prBody,
						Draft:      cmd.Bool("draft"),
						AutoCommit: cmd.Bool("commit"),
						AutoPush:   cmd.Bool("push"),
					})
					if err != nil {
						return err
					}
					printConfigInfo(cmd, result)
					results = append(results, result.Payload)
				}
				if mode.JSON {
					return output.WriteJSON(commandWriter(cmd), results)
				}
				for _, created := range results {
					if _, err := fmt.Fprintf(commandWriter(cmd), "Created PR #%d (%s) for %s\n", created.Number, created.URL, created.Repo); err != nil {
						return err
					}
				}
				return nil
			}

			if useAI && (title == "" || body == "") {
				generated, err := svc.GeneratePullRequestText(ctx, worksetapi.PullRequestGenerateInput{
					Workspace: worksetapi.WorkspaceSelector{Value: workspace},
					Repo:      repo,
				})
				if err != nil {
					return err
				}
				printConfigInfo(cmd, generated)
				if title == "" {
					title = generated.Payload.Title
				}
				if body == "" {
					body = generated.Payload.Body
				}
			}

			result, err := svc.CreatePullRequest(ctx, worksetapi.PullRequestCreateInput{
				Workspace:  worksetapi.WorkspaceSelector{Value: workspace},
				Repo:       repo,
				Base:       base,
				Head:       head,
				Title:      title,
				Body:       body,
				Draft:      cmd.Bool("draft"),
				AutoCommit: cmd.Bool("commit"),
				AutoPush:   cmd.Bool("push"),
			})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), result.Payload)
			}
			if _, err := fmt.Fprintf(commandWriter(cmd), "Created PR #%d (%s) for %s\n", result.Payload.Number, result.Payload.URL, result.Payload.Repo); err != nil {
				return err
			}
			return nil
		},
	}
}

func prStatusCommand() *cli.Command {
	return &cli.Command{
		Name:      "status",
		Usage:     "Show PR status for a repo",
		ArgsUsage: "-w <workspace> [--repo <alias>] [--number <id>]",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "repo",
				Usage: "Workspace repo alias",
			},
			&cli.IntFlag{
				Name:  "number",
				Usage: "Pull request number",
			},
			&cli.StringFlag{
				Name:  "branch",
				Usage: "Branch name to resolve PR",
			},
		}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "repo") {
				completeWorkspaceRepoNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			svc := apiService(ctx, cmd)
			result, err := svc.GetPullRequestStatus(ctx, worksetapi.PullRequestStatusInput{
				Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
				Repo:      cmd.String("repo"),
				Number:    cmd.Int("number"),
				Branch:    cmd.String("branch"),
			})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), result)
			}
			if _, err := fmt.Fprintf(commandWriter(cmd), "PR #%d %s (%s)\n", result.PullRequest.Number, result.PullRequest.Title, result.PullRequest.State); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(commandWriter(cmd), "URL: %s\n", result.PullRequest.URL); err != nil {
				return err
			}
			if result.PullRequest.Mergeable != "" {
				if _, err := fmt.Fprintf(commandWriter(cmd), "Mergeable: %s\n", result.PullRequest.Mergeable); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func prTrackedCommand() *cli.Command {
	return &cli.Command{
		Name:      "last",
		Aliases:   []string{"tracked"},
		Usage:     "Show the last tracked PR for a repo",
		ArgsUsage: "-w <workspace> [--repo <alias>]",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "repo",
				Usage: "Workspace repo alias",
			},
		}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "repo") {
				completeWorkspaceRepoNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			svc := apiService(ctx, cmd)
			result, err := svc.GetTrackedPullRequest(ctx, worksetapi.PullRequestTrackedInput{
				Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
				Repo:      cmd.String("repo"),
			})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), result.Payload)
			}
			if !result.Payload.Found {
				_, err := fmt.Fprintln(commandWriter(cmd), "no tracked pull request for repo")
				return err
			}
			pr := result.Payload.PullRequest
			_, err = fmt.Fprintf(commandWriter(cmd), "Last PR #%d (%s)\n", pr.Number, pr.URL)
			return err
		},
	}
}

func prChecksCommand() *cli.Command {
	return &cli.Command{
		Name:      "checks",
		Usage:     "Show PR checks",
		ArgsUsage: "-w <workspace> [--repo <alias>] [--number <id>]",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "repo",
				Usage: "Workspace repo alias",
			},
			&cli.IntFlag{
				Name:  "number",
				Usage: "Pull request number",
			},
			&cli.StringFlag{
				Name:  "branch",
				Usage: "Branch name to resolve PR",
			},
		}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "repo") {
				completeWorkspaceRepoNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			svc := apiService(ctx, cmd)
			result, err := svc.GetPullRequestStatus(ctx, worksetapi.PullRequestStatusInput{
				Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
				Repo:      cmd.String("repo"),
				Number:    cmd.Int("number"),
				Branch:    cmd.String("branch"),
			})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), result.Checks)
			}
			styles := output.NewStyles(commandWriter(cmd), mode.Plain)
			rows := make([][]string, 0, len(result.Checks))
			for _, check := range result.Checks {
				rows = append(rows, []string{check.Name, check.Status, check.Conclusion, check.DetailsURL})
			}
			if len(rows) == 0 {
				_, err := fmt.Fprintln(commandWriter(cmd), "No checks found.")
				return err
			}
			rendered := output.RenderTable(styles, []string{"NAME", "STATUS", "CONCLUSION", "DETAILS"}, rows)
			_, err = fmt.Fprint(commandWriter(cmd), rendered)
			return err
		},
	}
}

func prReviewsCommand() *cli.Command {
	return &cli.Command{
		Name:      "reviews",
		Usage:     "List review comments for a PR",
		ArgsUsage: "-w <workspace> [--repo <alias>] [--number <id>]",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "repo",
				Usage: "Workspace repo alias",
			},
			&cli.IntFlag{
				Name:  "number",
				Usage: "Pull request number",
			},
			&cli.StringFlag{
				Name:  "branch",
				Usage: "Branch name to resolve PR",
			},
		}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "repo") {
				completeWorkspaceRepoNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			svc := apiService(ctx, cmd)
			result, err := svc.ListPullRequestReviewComments(ctx, worksetapi.PullRequestReviewsInput{
				Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
				Repo:      cmd.String("repo"),
				Number:    cmd.Int("number"),
				Branch:    cmd.String("branch"),
			})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), result.Comments)
			}
			for _, comment := range result.Comments {
				location := comment.Path
				if comment.Line > 0 {
					location = fmt.Sprintf("%s:%d", location, comment.Line)
				}
				if _, err := fmt.Fprintf(commandWriter(cmd), "- %s (%s): %s\n", location, comment.Author, strings.TrimSpace(comment.Body)); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func prGenerateCommand() *cli.Command {
	return &cli.Command{
		Name:      "generate",
		Usage:     "Generate PR title/body with the configured agent",
		ArgsUsage: "-w <workspace> [--repo <alias>]",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "repo",
				Usage: "Workspace repo alias",
			},
		}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "repo") {
				completeWorkspaceRepoNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			svc := apiService(ctx, cmd)
			result, err := svc.GeneratePullRequestText(ctx, worksetapi.PullRequestGenerateInput{
				Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
				Repo:      cmd.String("repo"),
			})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), result.Payload)
			}
			if _, err := fmt.Fprintf(commandWriter(cmd), "Title: %s\n\n%s\n", result.Payload.Title, result.Payload.Body); err != nil {
				return err
			}
			return nil
		},
	}
}

func prReplyCommand() *cli.Command {
	return &cli.Command{
		Name:      "reply",
		Usage:     "Reply to a review comment",
		ArgsUsage: "-w <workspace> --repo <alias> --comment <id> --body <text>",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "repo",
				Usage: "Workspace repo alias",
			},
			&cli.IntFlag{
				Name:     "comment",
				Usage:    "Comment ID to reply to",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "body",
				Usage:    "Reply body text",
				Required: true,
			},
			&cli.IntFlag{
				Name:  "number",
				Usage: "Pull request number",
			},
			&cli.StringFlag{
				Name:  "branch",
				Usage: "Branch name to resolve PR",
			},
		}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "repo") {
				completeWorkspaceRepoNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			svc := apiService(ctx, cmd)
			result, err := svc.ReplyToReviewComment(ctx, worksetapi.ReplyToReviewCommentInput{
				Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
				Repo:      cmd.String("repo"),
				Number:    cmd.Int("number"),
				Branch:    cmd.String("branch"),
				CommentID: int64(cmd.Int("comment")),
				Body:      cmd.String("body"),
			})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), result.Comment)
			}
			_, err = fmt.Fprintf(commandWriter(cmd), "Reply posted (ID: %d)\n", result.Comment.ID)
			return err
		},
	}
}

func prEditCommand() *cli.Command {
	return &cli.Command{
		Name:      "edit",
		Usage:     "Edit a review comment",
		ArgsUsage: "-w <workspace> --repo <alias> --comment <id> --body <text>",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "repo",
				Usage: "Workspace repo alias",
			},
			&cli.IntFlag{
				Name:     "comment",
				Usage:    "Comment ID to edit",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "body",
				Usage:    "New comment body text",
				Required: true,
			},
		}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "repo") {
				completeWorkspaceRepoNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			svc := apiService(ctx, cmd)
			result, err := svc.EditReviewComment(ctx, worksetapi.EditReviewCommentInput{
				Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
				Repo:      cmd.String("repo"),
				CommentID: int64(cmd.Int("comment")),
				Body:      cmd.String("body"),
			})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), result.Comment)
			}
			_, err = fmt.Fprintf(commandWriter(cmd), "Comment updated (ID: %d)\n", result.Comment.ID)
			return err
		},
	}
}

func prDeleteCommand() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a review comment",
		ArgsUsage: "-w <workspace> --repo <alias> --comment <id>",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "repo",
				Usage: "Workspace repo alias",
			},
			&cli.IntFlag{
				Name:     "comment",
				Usage:    "Comment ID to delete",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "confirm",
				Usage: "Skip confirmation prompt",
			},
		}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "repo") {
				completeWorkspaceRepoNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if !cmd.Bool("confirm") {
				_, err := fmt.Fprintln(commandWriter(cmd), "Use --confirm to delete the comment")
				return err
			}
			svc := apiService(ctx, cmd)
			result, err := svc.DeleteReviewComment(ctx, worksetapi.DeleteReviewCommentInput{
				Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
				Repo:      cmd.String("repo"),
				CommentID: int64(cmd.Int("comment")),
			})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), map[string]bool{"success": result.Success})
			}
			_, err = fmt.Fprintln(commandWriter(cmd), "Comment deleted")
			return err
		},
	}
}

func prResolveCommand() *cli.Command {
	return &cli.Command{
		Name:      "resolve",
		Usage:     "Resolve or unresolve a review thread",
		ArgsUsage: "-w <workspace> --repo <alias> --thread <id>",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "repo",
				Usage: "Workspace repo alias",
			},
			&cli.StringFlag{
				Name:     "thread",
				Usage:    "Thread node ID (GraphQL ID)",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "unresolve",
				Usage: "Unresolve instead of resolve",
			},
		}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "repo") {
				completeWorkspaceRepoNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			svc := apiService(ctx, cmd)
			resolve := !cmd.Bool("unresolve")
			result, err := svc.ResolveReviewThread(ctx, worksetapi.ResolveReviewThreadInput{
				Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
				Repo:      cmd.String("repo"),
				ThreadID:  cmd.String("thread"),
				Resolve:   resolve,
			})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), map[string]bool{"resolved": result.Resolved})
			}
			if result.Resolved {
				_, err = fmt.Fprintln(commandWriter(cmd), "Thread resolved")
			} else {
				_, err = fmt.Fprintln(commandWriter(cmd), "Thread unresolved")
			}
			return err
		},
	}
}
