package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/groups"
	"github.com/strantalis/workset/internal/ops"
	"github.com/strantalis/workset/internal/output"
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
					cfg, _, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					names := groups.List(cfg)
					mode := outputModeFromContext(cmd)
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					if len(names) == 0 {
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
						type row struct {
							Name        string `json:"name"`
							Description string `json:"description,omitempty"`
							RepoCount   int    `json:"repo_count"`
						}
						rows := make([]row, 0, len(names))
						for _, name := range names {
							group, _ := groups.Get(cfg, name)
							rows = append(rows, row{
								Name:        name,
								Description: group.Description,
								RepoCount:   len(group.Members),
							})
						}
						return output.WriteJSON(commandWriter(cmd), rows)
					}
					rows := make([][]string, 0, len(names))
					for _, name := range names {
						group, _ := groups.Get(cfg, name)
						desc := group.Description
						if desc == "" {
							desc = "-"
						}
						rows = append(rows, []string{name, desc, fmt.Sprintf("%d", len(group.Members))})
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
					cfg, _, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					group, ok := groups.Get(cfg, name)
					if !ok {
						return cli.Exit("group not found", 1)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						payload := map[string]any{
							"name":        name,
							"description": group.Description,
							"members":     group.Members,
						}
						return output.WriteJSON(commandWriter(cmd), payload)
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					header := name
					if styles.Enabled {
						header = styles.Render(styles.Title, name)
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
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					if err := groups.Upsert(&cfg, name, cmd.String("description")); err != nil {
						return err
					}
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status": "ok",
							"name":   name,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("group %s saved", name)
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
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					if err := groups.Delete(&cfg, name); err != nil {
						return err
					}
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status": "ok",
							"name":   name,
						})
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
				Flags: appendOutputFlags([]cli.Flag{
					&cli.StringFlag{
						Name:  "base-remote",
						Usage: "Base remote name",
					},
					&cli.StringFlag{
						Name:  "write-remote",
						Usage: "Write remote name",
					},
					&cli.StringFlag{
						Name:  "base-branch",
						Usage: "Base branch name",
					},
				}),
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
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					baseRemote := strings.TrimSpace(cmd.String("base-remote"))
					if baseRemote == "" {
						baseRemote = cfg.Defaults.Remotes.Base
					}
					writeRemote := strings.TrimSpace(cmd.String("write-remote"))
					if writeRemote == "" {
						writeRemote = cfg.Defaults.Remotes.Write
					}
					baseBranch := strings.TrimSpace(cmd.String("base-branch"))
					if baseBranch == "" {
						if alias, ok := cfg.Repos[repoName]; ok && alias.DefaultBranch != "" {
							baseBranch = alias.DefaultBranch
						} else {
							baseBranch = cfg.Defaults.BaseBranch
						}
					}
					member := config.GroupMember{
						Repo: repoName,
						Remotes: config.Remotes{
							Base: config.RemoteConfig{
								Name:          baseRemote,
								DefaultBranch: baseBranch,
							},
							Write: config.RemoteConfig{
								Name:          writeRemote,
								DefaultBranch: baseBranch,
							},
						},
					}
					if err := groups.AddMember(&cfg, groupName, member); err != nil {
						return err
					}
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status":   "ok",
							"template": groupName,
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
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					if err := groups.RemoveMember(&cfg, groupName, repoName); err != nil {
						return err
					}
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status":   "ok",
							"template": groupName,
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
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					wsRoot, wsConfig, err := resolveWorkspace(cmd, &cfg, cfgPath)
					if err != nil {
						return err
					}
					group, ok := groups.Get(cfg, groupName)
					if !ok {
						return cli.Exit("group not found", 1)
					}
					for _, member := range group.Members {
						alias, ok := cfg.Repos[member.Repo]
						if !ok {
							return fmt.Errorf("repo alias %q not found in config", member.Repo)
						}
						baseBranch := cfg.Defaults.BaseBranch
						if member.Remotes.Base.DefaultBranch != "" {
							baseBranch = member.Remotes.Base.DefaultBranch
						} else if alias.DefaultBranch != "" {
							baseBranch = alias.DefaultBranch
						}
						remotes := config.Remotes{
							Base: config.RemoteConfig{
								Name:          member.Remotes.Base.Name,
								DefaultBranch: baseBranch,
							},
							Write: config.RemoteConfig{
								Name:          member.Remotes.Write.Name,
								DefaultBranch: baseBranch,
							},
						}
						if remotes.Base.Name == "" {
							remotes.Base.Name = cfg.Defaults.Remotes.Base
						}
						if remotes.Write.Name == "" {
							remotes.Write.Name = cfg.Defaults.Remotes.Write
						}

						if _, err := ops.AddRepo(ctx, ops.AddRepoInput{
							WorkspaceRoot: wsRoot,
							Name:          member.Repo,
							URL:           alias.URL,
							SourcePath:    alias.Path,
							Defaults:      cfg.Defaults,
							Remotes:       remotes,
							Git:           git.NewGoGitClient(),
						}); err != nil {
							return err
						}
					}
					registerWorkspace(&cfg, wsConfig.Name, wsRoot, time.Now())
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status":    "ok",
							"template":  groupName,
							"workspace": wsConfig.Name,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("group %s applied to %s", groupName, wsConfig.Name)
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
