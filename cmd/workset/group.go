package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/strantalis/workset/internal/output"
	"github.com/strantalis/workset/pkg/worksetapi"
	"github.com/urfave/cli/v3"
)

func groupCommand() *cli.Command {
	return &cli.Command{
		Name:    "group",
		Aliases: []string{"template"},
		Usage:   "Manage repo groups (aka templates; apply requires -w)",
		Commands: []*cli.Command{
			{
				Name:  "ls",
				Usage: "List groups",
				Flags: outputFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					svc := apiService(cmd)
					result, err := svc.ListGroups(ctx)
					if err != nil {
						return err
					}
					printConfigInfo(cmd, result)
					mode := outputModeFromContext(cmd)
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					if len(result.Groups) == 0 {
						if mode.JSON {
							return output.WriteJSON(commandWriter(cmd), []any{})
						}
						msg := "no groups defined"
						if styles.Enabled {
							msg = styles.Render(styles.Muted, msg)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
							return err
						}
						return nil
					}
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), result.Groups)
					}
					rows := make([][]string, 0, len(result.Groups))
					for _, group := range result.Groups {
						desc := group.Description
						if desc == "" {
							desc = "-"
						}
						rows = append(rows, []string{group.Name, desc, fmt.Sprintf("%d", group.RepoCount)})
					}
					rendered := output.RenderTable(styles, []string{"NAME", "DESCRIPTION", "REPOS"}, rows)
					_, err = fmt.Fprint(commandWriter(cmd), rendered)
					return err
				},
			},
			{
				Name:      "show",
				Usage:     "Show a group",
				ArgsUsage: "<name>",
				Flags:     outputFlags(),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					if cmd.NArg() == 0 {
						completeGroupNames(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().First())
					if name == "" {
						return usageError(ctx, cmd, "group name required")
					}
					svc := apiService(cmd)
					group, info, err := svc.GetGroup(ctx, name)
					if err != nil {
						return err
					}
					if verboseEnabled(cmd) {
						printConfigLoadInfo(cmd, cmd.String("config"), info)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						payload := map[string]any{
							"name":        group.Name,
							"description": group.Description,
							"members":     group.Members,
						}
						return output.WriteJSON(commandWriter(cmd), payload)
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					header := group.Name
					if styles.Enabled {
						header = styles.Render(styles.Title, header)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), header); err != nil {
						return err
					}
					if group.Description != "" {
						desc := group.Description
						if styles.Enabled {
							desc = styles.Render(styles.Muted, desc)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), desc); err != nil {
							return err
						}
					}
					rows := make([][]string, 0, len(group.Members))
					for _, member := range group.Members {
						rows = append(rows, []string{member.Repo})
					}
					if len(rows) == 0 {
						msg := "no repos in group"
						if styles.Enabled {
							msg = styles.Render(styles.Muted, msg)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
							return err
						}
						return nil
					}
					rendered := output.RenderTable(styles, []string{"REPO"}, rows)
					_, err = fmt.Fprint(commandWriter(cmd), rendered)
					return err
				},
			},
			{
				Name:      "create",
				Usage:     "Create or update a group",
				ArgsUsage: "<name>",
				Flags: appendOutputFlags([]cli.Flag{
					&cli.StringFlag{
						Name:  "description",
						Usage: "Group description",
					},
				}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().First())
					if name == "" {
						return usageError(ctx, cmd, "group name required")
					}
					svc := apiService(cmd)
					result, info, err := svc.CreateGroup(ctx, worksetapi.GroupUpsertInput{
						Name:        name,
						Description: cmd.String("description"),
					})
					if err != nil {
						return err
					}
					if verboseEnabled(cmd) {
						printConfigLoadInfo(cmd, cmd.String("config"), info)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status": "ok",
							"name":   result.Name,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("group %s saved", result.Name)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:      "rm",
				Usage:     "Remove a group",
				ArgsUsage: "<name>",
				Flags:     outputFlags(),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					if cmd.NArg() == 0 {
						completeGroupNames(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().First())
					if name == "" {
						return usageError(ctx, cmd, "group name required")
					}
					svc := apiService(cmd)
					result, info, err := svc.DeleteGroup(ctx, name)
					if err != nil {
						return err
					}
					if verboseEnabled(cmd) {
						printConfigLoadInfo(cmd, cmd.String("config"), info)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), result)
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("group %s removed", name)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:      "add",
				Usage:     "Add a repo to a group",
				ArgsUsage: "<group> <repo>",
				Flags:     outputFlags(),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					switch cmd.NArg() {
					case 0:
						completeGroupNames(cmd)
					case 1:
						completeRepoAliases(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					groupName := strings.TrimSpace(cmd.Args().Get(0))
					repoName := strings.TrimSpace(cmd.Args().Get(1))
					if groupName == "" || repoName == "" {
						return usageError(ctx, cmd, "group and repo name required")
					}
					svc := apiService(cmd)
					result, info, err := svc.AddGroupMember(ctx, worksetapi.GroupMemberInput{
						GroupName: groupName,
						RepoName:  repoName,
					})
					if err != nil {
						return err
					}
					if verboseEnabled(cmd) {
						printConfigLoadInfo(cmd, cmd.String("config"), info)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status":   "ok",
							"template": result.Name,
							"repo":     repoName,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("added %s to %s", repoName, groupName)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:      "remove",
				Usage:     "Remove a repo from a group",
				ArgsUsage: "<group> <repo>",
				Flags:     outputFlags(),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					switch cmd.NArg() {
					case 0:
						completeGroupNames(cmd)
					case 1:
						completeRepoAliases(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					groupName := strings.TrimSpace(cmd.Args().Get(0))
					repoName := strings.TrimSpace(cmd.Args().Get(1))
					if groupName == "" || repoName == "" {
						return usageError(ctx, cmd, "group and repo name required")
					}
					svc := apiService(cmd)
					result, info, err := svc.RemoveGroupMember(ctx, worksetapi.GroupMemberInput{
						GroupName: groupName,
						RepoName:  repoName,
					})
					if err != nil {
						return err
					}
					if verboseEnabled(cmd) {
						printConfigLoadInfo(cmd, cmd.String("config"), info)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status":   "ok",
							"template": result.Name,
							"repo":     repoName,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("removed %s from %s", repoName, groupName)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:      "apply",
				Usage:     "Apply a group to a workspace (requires -w)",
				ArgsUsage: "-w <workspace> <name>",
				Flags:     appendOutputFlags([]cli.Flag{workspaceFlag(true)}),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					if cmd.NArg() == 0 {
						completeGroupNames(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					groupName := strings.TrimSpace(cmd.Args().First())
					if groupName == "" {
						return usageError(ctx, cmd, "group name required")
					}
					svc := apiService(cmd)
					payload, info, err := svc.ApplyGroup(ctx, worksetapi.GroupApplyInput{
						Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
						Name:      groupName,
					})
					if err != nil {
						return err
					}
					if verboseEnabled(cmd) {
						printConfigLoadInfo(cmd, cmd.String("config"), info)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), payload)
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("group %s applied to %s", payload.Template, payload.Workspace)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					return nil
				},
			},
		},
	}
}
