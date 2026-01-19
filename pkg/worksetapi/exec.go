package worksetapi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/session"
	"github.com/strantalis/workset/internal/workspace"
)

// Exec runs a command inside the workspace root with standard env variables set.
func (s *Service) Exec(ctx context.Context, input ExecInput) error {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return err
	}

	workspaceArg := strings.TrimSpace(input.Workspace.Value)
	if workspaceArg == "" {
		workspaceArg = strings.TrimSpace(cfg.Defaults.Workspace)
	}
	if workspaceArg == "" {
		return ValidationError{Message: "workspace required"}
	}

	name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
	if err != nil {
		return err
	}

	wsConfig, err := s.workspaces.LoadConfig(ctx, root)
	if err != nil {
		if os.IsNotExist(err) {
			return NotFoundError{Message: fmt.Sprintf("workset.yaml not found at %s", workspace.WorksetFile(root))}
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
		registerWorkspace(&cfg, wsName, root, s.clock())
		if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
			return err
		}
	}

	env := append(os.Environ(),
		fmt.Sprintf("WORKSET_ROOT=%s", root),
		fmt.Sprintf("WORKSET_CONFIG=%s", workspace.WorksetFile(root)),
	)
	if wsName != "" {
		env = append(env, fmt.Sprintf("WORKSET_WORKSPACE=%s", wsName))
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
