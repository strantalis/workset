package ops

import (
	"errors"
	"fmt"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
)

type UpdateRepoRemotesInput struct {
	WorkspaceRoot  string
	Name           string
	Defaults       config.Defaults
	BaseRemote     string
	WriteRemote    string
	BaseBranch     string
	WriteBranch    string
	BaseRemoteSet  bool
	WriteRemoteSet bool
	BaseBranchSet  bool
	WriteBranchSet bool
}

func UpdateRepoRemotes(input UpdateRepoRemotesInput) (config.WorkspaceConfig, error) {
	if input.WorkspaceRoot == "" {
		return config.WorkspaceConfig{}, errors.New("workspace root required")
	}
	if input.Name == "" {
		return config.WorkspaceConfig{}, errors.New("repo name required")
	}
	if !input.BaseRemoteSet && !input.WriteRemoteSet && !input.BaseBranchSet && !input.WriteBranchSet {
		return config.WorkspaceConfig{}, errors.New("at least one remote setting required")
	}

	ws, err := workspace.Load(input.WorkspaceRoot, input.Defaults)
	if err != nil {
		return config.WorkspaceConfig{}, err
	}

	repoIndex := -1
	var repo config.RepoConfig
	for i, cfg := range ws.Config.Repos {
		if cfg.Name == input.Name {
			repoIndex = i
			repo = cfg
			break
		}
	}
	if repoIndex == -1 {
		return config.WorkspaceConfig{}, fmt.Errorf("repo %q not found in workspace", input.Name)
	}

	if input.BaseRemoteSet {
		repo.Remotes.Base.Name = input.BaseRemote
	}
	if input.WriteRemoteSet {
		repo.Remotes.Write.Name = input.WriteRemote
	}
	if input.BaseBranchSet {
		repo.Remotes.Base.DefaultBranch = input.BaseBranch
	}
	if input.WriteBranchSet {
		repo.Remotes.Write.DefaultBranch = input.WriteBranch
	}

	ws.Config.Repos[repoIndex] = repo
	if err := config.SaveWorkspace(workspace.WorksetFile(input.WorkspaceRoot), ws.Config); err != nil {
		return config.WorkspaceConfig{}, err
	}
	return ws.Config, nil
}
