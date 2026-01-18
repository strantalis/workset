package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
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
			cfg, cfgPath, err := loadGlobal(cmd.String("config"))
			if err != nil {
				return err
			}

			workspaceArg, commandArgs := parseWorkspaceAndCommand(cmd, &cfg)
			if workspaceArg == "" && cfg.Defaults.Workspace == "" {
				return usageError(ctx, cmd, "workspace required: pass -w <name|path> or set defaults.workspace (example: workset exec -w demo -- ls)")
			}

			name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
			if err != nil {
				return err
			}

			wsConfig, err := config.LoadWorkspace(workspace.WorksetFile(root))
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("workset.yaml not found at %s\nhint: check -w or register the workspace with workset ls", workspace.WorksetFile(root))
				}
				return err
			}

			wsName := wsConfig.Name
			if wsName == "" {
				wsName = name
			}
			if wsName == "" {
				wsName = filepath.Base(root)
			}

			if wsName != "" {
				registerWorkspace(&cfg, wsName, root, time.Now())
				if err := config.SaveGlobal(cfgPath, cfg); err != nil {
					return err
				}
			}

			command, args := resolveExecCommand(commandArgs)
			execCmd := exec.CommandContext(ctx, command, args...)
			execCmd.Dir = root
			execCmd.Stdin = os.Stdin
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			execCmd.Env = append(os.Environ(),
				fmt.Sprintf("WORKSET_ROOT=%s", root),
				fmt.Sprintf("WORKSET_CONFIG=%s", workspace.WorksetFile(root)),
			)
			if wsName != "" {
				execCmd.Env = append(execCmd.Env, fmt.Sprintf("WORKSET_WORKSPACE=%s", wsName))
			}

			if err := execCmd.Run(); err != nil {
				return exitWithStatus(err)
			}
			return nil
		},
	}
}

func resolveExecCommand(args []string) (string, []string) {
	if len(args) > 0 {
		return args[0], args[1:]
	}
	return defaultShell(), nil
}

func defaultShell() string {
	if runtime.GOOS == "windows" {
		if shell := os.Getenv("COMSPEC"); shell != "" {
			return shell
		}
		return "cmd.exe"
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
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
