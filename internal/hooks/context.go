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
		"WORKSET_ROOT=" + c.WorkspaceRoot,
		"WORKSET_CONFIG=" + c.WorkspaceConfig,
		"WORKSET_WORKSPACE=" + c.WorkspaceName,
		"WORKSET_REPO=" + c.RepoName,
		"WORKSET_REPO_DIR=" + c.RepoDir,
		"WORKSET_REPO_PATH=" + c.RepoPath,
		"WORKSET_WORKTREE=" + c.WorktreePath,
		"WORKSET_BRANCH=" + c.Branch,
		fmt.Sprintf("WORKSET_EVENT=%s", c.Event),
		"WORKSET_REASON=" + c.Reason,
	}
	return env
}
