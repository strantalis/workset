package hooks

import (
	"fmt"
)

type Context struct {
	WorkspaceRoot   string
	WorkspaceName   string
	WorkspaceConfig string
	RepoName        string
	RepoDir         string
	RepoPath        string
	WorktreePath    string
	Branch          string
	Event           Event
	Reason          string
}

func (c Context) TokenMap() map[string]string {
	values := map[string]string{
		"{workspace.root}":   c.WorkspaceRoot,
		"{workspace.name}":   c.WorkspaceName,
		"{workspace.config}": c.WorkspaceConfig,
		"{repo.name}":        c.RepoName,
		"{repo.dir}":         c.RepoDir,
		"{repo.path}":        c.RepoPath,
		"{worktree.path}":    c.WorktreePath,
		"{branch}":           c.Branch,
		"{event}":            string(c.Event),
		"{reason}":           c.Reason,
	}
	for key, value := range values {
		if value == "" {
			delete(values, key)
		}
	}
	return values
}

func (c Context) Env() []string {
	env := []string{
		fmt.Sprintf("WORKSET_ROOT=%s", c.WorkspaceRoot),
		fmt.Sprintf("WORKSET_CONFIG=%s", c.WorkspaceConfig),
		fmt.Sprintf("WORKSET_WORKSPACE=%s", c.WorkspaceName),
		fmt.Sprintf("WORKSET_REPO=%s", c.RepoName),
		fmt.Sprintf("WORKSET_REPO_DIR=%s", c.RepoDir),
		fmt.Sprintf("WORKSET_REPO_PATH=%s", c.RepoPath),
		fmt.Sprintf("WORKSET_WORKTREE=%s", c.WorktreePath),
		fmt.Sprintf("WORKSET_BRANCH=%s", c.Branch),
		fmt.Sprintf("WORKSET_EVENT=%s", c.Event),
		fmt.Sprintf("WORKSET_REASON=%s", c.Reason),
	}
	return env
}
