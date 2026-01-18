package main

import (
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/urfave/cli/v3"
)

func parseWorkspaceAndCommand(cmd *cli.Command, cfg *config.GlobalConfig) (string, []string) {
	args := cmd.Args().Slice()
	workspaceArg := strings.TrimSpace(cmd.String("workspace"))

	if workspaceArg == "" && len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
		} else if _, _, err := resolveWorkspaceTarget(args[0], cfg); err == nil {
			workspaceArg = args[0]
			args = args[1:]
		}
	}

	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	return workspaceArg, args
}
