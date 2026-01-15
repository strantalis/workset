package groups

import (
	"errors"
	"fmt"
	"sort"

	"github.com/strantalis/workset/internal/config"
)

// List returns sorted group names.
func List(cfg config.GlobalConfig) []string {
	if len(cfg.Groups) == 0 {
		return []string{}
	}
	names := make([]string, 0, len(cfg.Groups))
	for name := range cfg.Groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Get returns a group by name.
func Get(cfg config.GlobalConfig, name string) (config.Group, bool) {
	group, ok := cfg.Groups[name]
	return group, ok
}

// Upsert creates or updates a group with the provided description.
func Upsert(cfg *config.GlobalConfig, name, description string) error {
	if name == "" {
		return errors.New("group name required")
	}
	if cfg.Groups == nil {
		cfg.Groups = map[string]config.Group{}
	}
	group := cfg.Groups[name]
	if description != "" {
		group.Description = description
	}
	cfg.Groups[name] = group
	return nil
}

// Delete removes a group.
func Delete(cfg *config.GlobalConfig, name string) error {
	if name == "" {
		return errors.New("group name required")
	}
	if _, ok := cfg.Groups[name]; !ok {
		return fmt.Errorf("group %q not found", name)
	}
	delete(cfg.Groups, name)
	return nil
}

// AddMember adds or updates a member in a group.
func AddMember(cfg *config.GlobalConfig, groupName string, member config.GroupMember) error {
	if groupName == "" {
		return errors.New("group name required")
	}
	if member.Repo == "" {
		return errors.New("repo name required")
	}
	if cfg.Groups == nil {
		cfg.Groups = map[string]config.Group{}
	}
	group := cfg.Groups[groupName]
	for i, existing := range group.Members {
		if existing.Repo == member.Repo {
			group.Members[i] = member
			cfg.Groups[groupName] = group
			return nil
		}
	}
	group.Members = append(group.Members, member)
	cfg.Groups[groupName] = group
	return nil
}

// RemoveMember removes a member from a group.
func RemoveMember(cfg *config.GlobalConfig, groupName, repoName string) error {
	if groupName == "" {
		return errors.New("group name required")
	}
	if repoName == "" {
		return errors.New("repo name required")
	}
	group, ok := cfg.Groups[groupName]
	if !ok {
		return fmt.Errorf("group %q not found", groupName)
	}
	next := group.Members[:0]
	found := false
	for _, member := range group.Members {
		if member.Repo == repoName {
			found = true
			continue
		}
		next = append(next, member)
	}
	if !found {
		return fmt.Errorf("repo %q not found in group %q", repoName, groupName)
	}
	group.Members = next
	cfg.Groups[groupName] = group
	return nil
}

// FromWorkspace snapshots a workspace config into a group definition.
func FromWorkspace(cfg *config.GlobalConfig, groupName string, ws config.WorkspaceConfig) error {
	if groupName == "" {
		return errors.New("group name required")
	}
	members := make([]config.GroupMember, 0, len(ws.Repos))
	for _, repo := range ws.Repos {
		members = append(members, config.GroupMember{
			Repo:    repo.Name,
			Remotes: repo.Remotes,
		})
	}
	if cfg.Groups == nil {
		cfg.Groups = map[string]config.Group{}
	}
	group := cfg.Groups[groupName]
	group.Members = members
	cfg.Groups[groupName] = group
	return nil
}
