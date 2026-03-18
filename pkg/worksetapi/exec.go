package worksetapi

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/session"
	"github.com/strantalis/workset/internal/workspace"
)

// Exec runs a command inside the workspace root with standard env variables set.
func (s *Service) Exec(ctx context.Context, input ExecInput) error {
	cfg, _, err := s.loadGlobal(ctx)
	if err != nil {
		return err
	}

	threadArg := strings.TrimSpace(input.Workspace.Value)
	if threadArg == "" {
		threadArg = strings.TrimSpace(cfg.Defaults.Thread)
	}
	if threadArg == "" {
		return ValidationError{Message: "thread required"}
	}

	name, root, err := resolveThreadTarget(threadArg, &cfg)
	if err != nil {
		return err
	}

	wsConfig, err := s.workspaces.LoadConfig(ctx, root)
	if err != nil {
		if os.IsNotExist(err) {
			return NotFoundError{Message: "workset.yaml not found at " + workspace.WorksetFile(root)}
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
		if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, _ config.GlobalConfigLoadInfo) error {
			registerWorkspace(cfg, wsName, root, s.clock(), "")
			return nil
		}); err != nil {
			return err
		}
	}

	env := append(os.Environ(),
		"WORKSET_ROOT="+root,
		"WORKSET_CONFIG="+workspace.WorksetFile(root),
	)
	if wsName != "" {
		env = append(env, "WORKSET_WORKSPACE="+wsName)
	}

	return s.exec(ctx, root, input.Command, env)
}

func runExecCommand(ctx context.Context, root string, command []string, env []string) error {
	execName, execArgs := session.ResolveExecCommand(command)
	execCmd := exec.CommandContext(ctx, execName, execArgs...)
	execCmd.Dir = root
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Env = env
	return execCmd.Run()
}
