package main

import (
	"context"
	"os/exec"
	"strings"
	"syscall"

	"github.com/strantalis/workset/pkg/worksetapi"
	"github.com/urfave/cli/v3"
)

func execCommand() *cli.Command {
	return &cli.Command{
		Name:      "exec",
		Usage:     "Run a command in a workspace",
		ArgsUsage: "[<workspace>] [-- <command> [args...]]",
		Description: "If defaults.workspace is set, use `workset exec -- <cmd>` to run a command " +
			"without specifying a workspace argument.",
		Flags: []cli.Flag{
			workspaceFlag(false),
		},
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if cmd.NArg() == 0 && strings.TrimSpace(cmd.String("workspace")) == "" {
				completeWorkspaceNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, _, err := loadGlobal(cmd)
			if err != nil {
				return err
			}

			workspaceArg, commandArgs := parseWorkspaceAndCommand(cmd, &cfg)
			if workspaceArg == "" && cfg.Defaults.Workspace == "" {
				return usageError(ctx, cmd, "workspace required: pass -w <name|path> or set defaults.workspace (example: workset exec -w demo -- ls)")
			}

			svc := apiService(cmd)
			if err := svc.Exec(ctx, worksetapi.ExecInput{
				Workspace: worksetapi.WorkspaceSelector{Value: workspaceArg},
				Command:   commandArgs,
			}); err != nil {
				return exitWithStatus(err)
			}
			return nil
		},
	}
}

func exitWithStatus(err error) error {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return err
	}
	code := exitErr.ExitCode()
	if code >= 0 {
		return cli.Exit("", code)
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if ok && status.Signaled() {
		return cli.Exit("", 128+int(status.Signal()))
	}
	return cli.Exit("", 1)
}
