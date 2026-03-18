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
		Name:                  "workset",
		Usage:                 "Manage multi-repo threads with predictable defaults",
		Description:           "Thread commands require -t/--thread (or defaults.thread) to target a thread.",
		Version:               version,
		EnableShellCompletion: true,
		ConfigureShellCompletionCommand: func(cmd *cli.Command) {
			cmd.Hidden = false
			cmd.Usage = "Generate shell completion script"
			cmd.Description = "Generate shell completion for bash, zsh, fish, or powershell."
		},
		Flags: []cli.Flag{
			threadFlag(false),
			&cli.StringFlag{
				Name:  "config",
				Usage: "Override global config path",
			},
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "Verbose debug output",
			},
		},
		Commands: []*cli.Command{
			newCommand(),
			listCommand(),
			hooksCommand(),
			versionCommand(),
			removeWorkspaceCommand(),
			configCommand(),
			repoCommand(),
			statusCommand(),
		},
	}
	enableSuggestions(root)

	args := normalizeArgs(root, os.Args)
	if err := root.Run(context.Background(), args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
