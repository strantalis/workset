package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

var version = "dev"

func main() {
	root := &cli.Command{
		Name:        "workset",
		Usage:       "Manage multi-repo workspaces with predictable defaults",
		Description: "Workspace commands require -w/--workspace (or defaults.workspace) to target a workspace.",
		Version:     version,
		Flags: []cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "config",
				Usage: "Override global config path",
			},
		},
		Commands: []*cli.Command{
			newCommand(),
			listCommand(),
			versionCommand(),
			removeWorkspaceCommand(),
			configCommand(),
			groupCommand(),
			repoCommand(),
			statusCommand(),
		},
	}

	args := normalizeArgs(root, os.Args)
	if err := root.Run(context.Background(), args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
