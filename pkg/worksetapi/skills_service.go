package worksetapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillInfo describes a discovered SKILL.md file.
type SkillInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	DirName     string   `json:"dirName"`
	Scope       string   `json:"scope"` // "global" or "project"
	Tools       []string `json:"tools"` // e.g. ["claude","codex","copilot","agents"]
	Path        string   `json:"path"`  // primary SKILL.md path (first found)
}

// SkillContent is a SkillInfo plus the raw SKILL.md content.
type SkillContent struct {
	SkillInfo

	Content string `json:"content"`
}

// skillToolDir describes a tool's skill directory patterns.
type skillToolDir struct {
	name      string // tool identifier: "claude", "codex", "copilot", "agents"
	globalDir string // glob-expanded dir under $HOME (empty = no global)
	localDir  string // dir relative to project root
}

var skillToolDirs = []skillToolDir{
	{name: "claude", globalDir: ".claude/skills", localDir: ".claude/skills"},
	{name: "codex", globalDir: ".codex/skills", localDir: ".codex/skills"},
	{name: "copilot", globalDir: "", localDir: ".github/skills"},
	{name: "agents", globalDir: ".agents/skills", localDir: ".agents/skills"},
}

// ListSkills scans all tool directories for SKILL.md files and returns a
// deduplicated list grouped by skill directory name and scope.
func (s *Service) ListSkills(_ context.Context, projectRoot string) ([]SkillInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	index := map[skillKey]*SkillInfo{}

	for _, td := range skillToolDirs {
		// Global skills
		if td.globalDir != "" {
			base := filepath.Join(home, td.globalDir)
			s.scanSkillDir(base, "global", td.name, index)
		}
		// Project skills
		if projectRoot != "" && td.localDir != "" {
			base := filepath.Join(projectRoot, td.localDir)
			s.scanSkillDir(base, "project", td.name, index)
		}
	}

	skills := make([]SkillInfo, 0, len(index))
	for _, info := range index {
		skills = append(skills, *info)
	}
	sort.Slice(skills, func(i, j int) bool {
		if skills[i].Scope != skills[j].Scope {
			return skills[i].Scope == "global"
		}
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

type skillKey struct {
	dirName string
	scope   string
}

func (s *Service) scanSkillDir(base, scope, tool string, index map[skillKey]*SkillInfo) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return // directory doesn't exist or unreadable
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		skillPath := filepath.Join(base, dirName, "SKILL.md")
		content, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		key := skillKey{dirName: dirName, scope: scope}

		if existing, ok := index[key]; ok {
			// Add tool if not already present
			hasTool := false
			for _, t := range existing.Tools {
				if t == tool {
					hasTool = true
					break
				}
			}
			if !hasTool {
				existing.Tools = append(existing.Tools, tool)
			}
		} else {
			name, desc := parseSkillFrontmatter(string(content))
			if name == "" {
				name = dirName
			}
			index[key] = &SkillInfo{
				Name:        name,
				Description: desc,
				DirName:     dirName,
				Scope:       scope,
				Tools:       []string{tool},
				Path:        skillPath,
			}
		}
	}
}

// GetSkill reads the SKILL.md content for a specific skill.
func (s *Service) GetSkill(_ context.Context, scope, dirName, tool string) (SkillContent, error) {
	return getSkillFromPath(scope, dirName, tool, "")
}

// GetSkillWithRoot reads the SKILL.md content using an explicit project root.
func (s *Service) GetSkillWithRoot(_ context.Context, scope, dirName, tool, projectRoot string) (SkillContent, error) {
	return getSkillFromPath(scope, dirName, tool, projectRoot)
}

func getSkillFromPath(scope, dirName, tool, projectRoot string) (SkillContent, error) {
	path, err := resolveSkillPathWithRoot(scope, dirName, tool, projectRoot)
	if err != nil {
		return SkillContent{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SkillContent{}, fmt.Errorf("cannot read skill: %w", err)
	}
	name, desc := parseSkillFrontmatter(string(data))
	if name == "" {
		name = dirName
	}
	return SkillContent{
		SkillInfo: SkillInfo{
			Name:        name,
			Description: desc,
			DirName:     dirName,
			Scope:       scope,
			Tools:       []string{tool},
			Path:        path,
		},
		Content: string(data),
	}, nil
}

// SaveSkill writes SKILL.md content for a specific skill, creating the
// directory if needed.
func (s *Service) SaveSkill(_ context.Context, scope, dirName, tool, content string) error {
	return saveSkillToPath(scope, dirName, tool, content, "")
}

// SaveSkillWithRoot writes SKILL.md using an explicit project root.
func (s *Service) SaveSkillWithRoot(_ context.Context, scope, dirName, tool, content, projectRoot string) error {
	return saveSkillToPath(scope, dirName, tool, content, projectRoot)
}

func saveSkillToPath(scope, dirName, tool, content, projectRoot string) error {
	if err := validateDirName(dirName); err != nil {
		return err
	}
	path, err := resolveSkillPathWithRoot(scope, dirName, tool, projectRoot)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create skill directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("cannot write skill: %w", err)
	}
	return nil
}

// DeleteSkill removes a skill directory for a specific tool.
func (s *Service) DeleteSkill(_ context.Context, scope, dirName, tool string) error {
	return deleteSkillAtPath(scope, dirName, tool, "")
}

// DeleteSkillWithRoot removes a skill directory using an explicit project root.
func (s *Service) DeleteSkillWithRoot(_ context.Context, scope, dirName, tool, projectRoot string) error {
	return deleteSkillAtPath(scope, dirName, tool, projectRoot)
}

func deleteSkillAtPath(scope, dirName, tool, projectRoot string) error {
	if err := validateDirName(dirName); err != nil {
		return err
	}
	path, err := resolveSkillPathWithRoot(scope, dirName, tool, projectRoot)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("cannot delete skill: %w", err)
	}
	return nil
}

// SyncSkill copies a skill directory from one tool to others.
func (s *Service) SyncSkill(_ context.Context, scope, dirName, fromTool string, toTools []string) error {
	return syncSkillAtPath(scope, dirName, fromTool, toTools, "")
}

// SyncSkillWithRoot copies a skill directory using an explicit project root.
func (s *Service) SyncSkillWithRoot(_ context.Context, scope, dirName, fromTool string, toTools []string, projectRoot string) error {
	return syncSkillAtPath(scope, dirName, fromTool, toTools, projectRoot)
}

func syncSkillAtPath(scope, dirName, fromTool string, toTools []string, projectRoot string) error {
	src, err := resolveSkillPathWithRoot(scope, dirName, fromTool, projectRoot)
	if err != nil {
		return err
	}
	srcDir := filepath.Dir(src)

	for _, tool := range toTools {
		dst, err := resolveSkillPathWithRoot(scope, dirName, tool, projectRoot)
		if err != nil {
			return err
		}
		dstDir := filepath.Dir(dst)
		if err := copyDir(srcDir, dstDir); err != nil {
			return fmt.Errorf("cannot sync skill to %s: %w", tool, err)
		}
	}
	return nil
}

// resolveSkillPathWithRoot resolves a skill path using an explicit project root.
// If projectRoot is empty for project scope, CWD is used as fallback.
func resolveSkillPathWithRoot(scope, dirName, tool, projectRoot string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	for _, td := range skillToolDirs {
		if td.name != tool {
			continue
		}
		switch scope {
		case "global":
			if td.globalDir == "" {
				return "", fmt.Errorf("tool %q has no global skill directory", tool)
			}
			return filepath.Join(home, td.globalDir, dirName, "SKILL.md"), nil
		case "project":
			root := projectRoot
			if root == "" {
				root, _ = os.Getwd()
			}
			if root == "" {
				return "", errors.New("project root required for project-scoped skills")
			}
			return filepath.Join(root, td.localDir, dirName, "SKILL.md"), nil
		default:
			return "", fmt.Errorf("invalid scope %q", scope)
		}
	}
	return "", fmt.Errorf("unknown tool %q", tool)
}

// validateDirName checks that the skill directory name does not contain path
// separators or traversal sequences.
func validateDirName(dirName string) error {
	if strings.Contains(dirName, "..") || strings.Contains(dirName, "/") || strings.Contains(dirName, "\\") {
		return errors.New("invalid dirName: must not contain path separators or ..")
	}
	return nil
}

// parseSkillFrontmatter extracts name and description from YAML frontmatter.
func parseSkillFrontmatter(content string) (name, description string) {
	if !strings.HasPrefix(content, "---") {
		return "", ""
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return "", ""
	}
	frontmatter := content[3 : 3+end]
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			name = strings.Trim(name, `"'`)
		} else if strings.HasPrefix(line, "description:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			description = strings.Trim(description, `"'`)
		}
	}
	return name, description
}

// copyDir copies all files from src to dst directory.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		srcInfo, err := os.Stat(srcPath)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, srcInfo.Mode()); err != nil {
			return err
		}
	}
	return nil
}
