package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
	"github.com/urfave/cli/v3"
)

func completeWorkspaceNames(cmd *cli.Command) {
	cfg, _, err := loadGlobal(cmd.String("config"))
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

func completeGroupNames(cmd *cli.Command) {
	cfg, _, err := loadGlobal(cmd.String("config"))
	if err != nil {
		return
	}
	names := make([]string, 0, len(cfg.Groups))
	for name := range cfg.Groups {
		names = append(names, name)
	}
	sort.Strings(names)
	writeCompletion(cmd, names)
}

func completeRepoAliases(cmd *cli.Command) {
	cfg, _, err := loadGlobal(cmd.String("config"))
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

func completeWorkspaceRepoNames(cmd *cli.Command) {
	cfg, _, err := loadGlobal(cmd.String("config"))
	if err != nil {
		return
	}
	arg := strings.TrimSpace(cmd.String("workspace"))
	if arg == "" {
		arg = strings.TrimSpace(workspaceFromArgs(cmd))
	}
	_, root, err := resolveWorkspaceTarget(arg, &cfg)
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

func completeSessionNames(cmd *cli.Command) {
	cfg, _, err := loadGlobal(cmd.String("config"))
	if err != nil {
		return
	}
	workspaceArg := strings.TrimSpace(cmd.String("workspace"))
	if workspaceArg == "" {
		if cmd.NArg() > 0 {
			workspaceArg = strings.TrimSpace(cmd.Args().Get(0))
		}
	}
	if workspaceArg == "" {
		workspaceArg = strings.TrimSpace(cfg.Defaults.Workspace)
	}
	if workspaceArg == "" {
		return
	}
	_, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
	if err != nil {
		return
	}
	state, err := workspace.LoadState(root)
	if err != nil {
		return
	}
	workspace.EnsureSessionState(&state)
	if len(state.Sessions) == 0 {
		return
	}
	names := make([]string, 0, len(state.Sessions))
	for name := range state.Sessions {
		names = append(names, name)
	}
	sort.Strings(names)
	writeCompletion(cmd, names)
}

func completeSessionBackends(cmd *cli.Command, includeExec bool) {
	if !completionFlagRequested(cmd, "backend") {
		return
	}
	backends := []string{"auto", "tmux", "screen"}
	if includeExec {
		backends = append(backends, "exec")
	}
	writeCompletion(cmd, backends)
}

func completionFlagRequested(cmd *cli.Command, name string) bool {
	args := cmd.Args().Slice()
	long := "--" + name
	for _, arg := range args {
		if arg == long {
			return true
		}
		if strings.HasPrefix(arg, long+"=") {
			return true
		}
	}
	return false
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
