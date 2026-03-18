package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
	"github.com/urfave/cli/v3"
)

func completeThreadNames(cmd *cli.Command) {
	cfg, _, err := loadGlobal(cmd)
	if err != nil {
		return
	}
	names := make([]string, 0, len(cfg.Workspaces))
	for name := range cfg.Workspaces {
		names = append(names, name)
	}
	sort.Strings(names)
	writeCompletion(cmd, names)
}

func completeRegisteredRepos(cmd *cli.Command) {
	cfg, _, err := loadGlobal(cmd)
	if err != nil {
		return
	}
	names := make([]string, 0, len(cfg.Repos))
	for name := range cfg.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	writeCompletion(cmd, names)
}

func completeThreadRepoNames(cmd *cli.Command) {
	cfg, _, err := loadGlobal(cmd)
	if err != nil {
		return
	}
	arg := strings.TrimSpace(cmd.String("thread"))
	if arg == "" {
		arg = strings.TrimSpace(threadFromArgs(cmd))
	}
	_, root, err := resolveThreadTarget(arg, &cfg)
	if err != nil {
		return
	}
	wsConfig, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		return
	}
	names := make([]string, 0, len(wsConfig.Repos))
	for _, repo := range wsConfig.Repos {
		names = append(names, repo.Name)
	}
	sort.Strings(names)
	writeCompletion(cmd, names)
}

func writeCompletion(cmd *cli.Command, options []string) {
	if len(options) == 0 {
		return
	}
	w := commandWriter(cmd)
	for _, option := range options {
		option = strings.TrimSpace(option)
		if option == "" {
			continue
		}
		_, _ = fmt.Fprintln(w, option)
	}
}
