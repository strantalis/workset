package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

const (
	agentsGeneratedStart = "<!-- workset:generated:start -->"
	agentsGeneratedEnd   = "<!-- workset:generated:end -->"
)

func UpdateAgentsFile(root string, cfg config.WorkspaceConfig, state State) error {
	if root == "" {
		return errors.New("workspace root required")
	}
	generated, err := buildAgentsGeneratedBlock(root, cfg, state)
	if err != nil {
		return err
	}
	section := formatGeneratedSection(generated)
	path := AgentsFile(root)
	current, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.WriteFile(path, []byte(buildAgentsTemplate(section)), 0o644)
		}
		return err
	}

	content := string(current)
	if updated, ok := replaceGeneratedSection(content, section); ok {
		return os.WriteFile(path, []byte(updated), 0o644)
	}
	if strings.TrimSpace(content) == "" {
		return os.WriteFile(path, []byte(buildAgentsTemplate(section)), 0o644)
	}
	return os.WriteFile(path, []byte(appendGeneratedSection(content, section)), 0o644)
}

func buildAgentsTemplate(section string) string {
	return strings.TrimSpace(`# Workspace Guidance

READ FIRST: This is a Workset workspace root, not necessarily a git repo.
If you enter a repo within this workspace, read that repo's AGENTS.md after this file.

Common workset commands:
- workset repo add <source> -w <workspace>
- workset status -w <workspace>
- workset session start <workspace> -- <cmd>
- workset session attach <workspace>
- workset session show <workspace>
- workset session stop <workspace>
`) + "\n\n" + section
}

func buildAgentsGeneratedBlock(root string, cfg config.WorkspaceConfig, state State) (string, error) {
	tree, err := buildTopLevelTree(root)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("## Workspace Layout (generated)\n")
	b.WriteString("Tree (top-level only):\n")
	b.WriteString("```text\n")
	b.WriteString(tree)
	if !strings.HasSuffix(tree, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```\n\n")
	b.WriteString("## Configured Repos (from workset.yaml)\n")
	if len(cfg.Repos) == 0 {
		b.WriteString("- (none configured)\n")
	} else {
		repos := append([]config.RepoConfig{}, cfg.Repos...)
		sort.Slice(repos, func(i, j int) bool {
			return repos[i].Name < repos[j].Name
		})
		for _, repo := range repos {
			repoDir := repo.RepoDir
			if repoDir == "" {
				repoDir = repo.Name
			}
			worktreePath := RepoWorktreePath(root, state.CurrentBranch, repoDir)
			relWorktree, err := filepath.Rel(root, worktreePath)
			if err != nil {
				relWorktree = worktreePath
			}
			b.WriteString("- name: " + repo.Name + "\n")
			b.WriteString("  repo_dir: " + repoDir + "\n")
			b.WriteString("  worktree_path: " + relWorktree + "\n")
			if repo.LocalPath != "" {
				b.WriteString("  local_path: " + repo.LocalPath + "\n")
			}
		}
	}
	b.WriteString("\nNote: Read each repo's AGENTS.md after entering its worktree.\n")
	return b.String(), nil
}

func buildTopLevelTree(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString(".\n")
	for i, name := range names {
		branch := "├── "
		if i == len(names)-1 {
			branch = "└── "
		}
		b.WriteString(branch + name + "\n")
	}
	return b.String(), nil
}

func formatGeneratedSection(generated string) string {
	trimmed := strings.TrimSpace(generated)
	return agentsGeneratedStart + "\n" + trimmed + "\n" + agentsGeneratedEnd + "\n"
}

func replaceGeneratedSection(content, section string) (string, bool) {
	start := strings.Index(content, agentsGeneratedStart)
	end := strings.Index(content, agentsGeneratedEnd)
	if start == -1 || end == -1 || end < start {
		return content, false
	}
	end += len(agentsGeneratedEnd)
	return content[:start] + section + content[end:], true
}

func appendGeneratedSection(content, section string) string {
	trimmed := strings.TrimRight(content, "\n")
	return trimmed + "\n\n" + section
}
