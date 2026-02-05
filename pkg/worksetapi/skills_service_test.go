package worksetapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeSkillFile(t *testing.T, base, dirName, content string) {
	t.Helper()
	dir := filepath.Join(base, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseSkillFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantN   string
		wantD   string
	}{
		{
			name:    "valid frontmatter",
			content: "---\nname: my-skill\ndescription: Does stuff\n---\n# Hello",
			wantN:   "my-skill",
			wantD:   "Does stuff",
		},
		{
			name:    "no frontmatter",
			content: "# Just markdown",
			wantN:   "",
			wantD:   "",
		},
		{
			name:    "quoted values",
			content: "---\nname: \"quoted-name\"\ndescription: 'quoted desc'\n---\n",
			wantN:   "quoted-name",
			wantD:   "quoted desc",
		},
		{
			name:    "missing closing fence",
			content: "---\nname: broken\n",
			wantN:   "",
			wantD:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, d := parseSkillFrontmatter(tt.content)
			if n != tt.wantN {
				t.Errorf("name = %q, want %q", n, tt.wantN)
			}
			if d != tt.wantD {
				t.Errorf("description = %q, want %q", d, tt.wantD)
			}
		})
	}
}

func TestValidateDirName(t *testing.T) {
	tests := []struct {
		name    string
		dirName string
		wantErr bool
	}{
		{"valid", "my-skill", false},
		{"valid underscores", "my_skill_v2", false},
		{"traversal dotdot", "../etc", true},
		{"traversal slash", "foo/bar", true},
		{"traversal backslash", "foo\\bar", true},
		{"deep traversal", "../../passwd", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDirName(tt.dirName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDirName(%q) error = %v, wantErr %v", tt.dirName, err, tt.wantErr)
			}
		})
	}
}

func TestListSkills(t *testing.T) {
	tmp := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create a global claude skill
	claudeSkills := filepath.Join(home, ".claude", "skills")
	writeSkillFile(t, claudeSkills, "test-skill", "---\nname: Test Skill\ndescription: A test\n---\n# Test")

	// Create same skill in codex global
	codexSkills := filepath.Join(home, ".codex", "skills")
	writeSkillFile(t, codexSkills, "test-skill", "---\nname: Test Skill\ndescription: A test\n---\n# Test")

	// Create a project skill
	projectSkills := filepath.Join(tmp, ".claude", "skills")
	writeSkillFile(t, projectSkills, "proj-skill", "---\nname: Project Skill\ndescription: Project only\n---\n")

	svc := &Service{}
	skills, err := svc.ListSkills(context.Background(), tmp)
	if err != nil {
		t.Fatal(err)
	}

	if len(skills) < 2 {
		t.Fatalf("expected at least 2 skills, got %d", len(skills))
	}

	// Find the global test-skill — it should list both claude and codex tools
	var globalSkill *SkillInfo
	for i := range skills {
		if skills[i].DirName == "test-skill" && skills[i].Scope == "global" {
			globalSkill = &skills[i]
			break
		}
	}
	if globalSkill == nil {
		t.Fatal("global test-skill not found")
	}
	if len(globalSkill.Tools) < 2 {
		t.Errorf("expected test-skill in at least 2 tools, got %v", globalSkill.Tools)
	}

	// Find the project skill
	var projSkill *SkillInfo
	for i := range skills {
		if skills[i].DirName == "proj-skill" && skills[i].Scope == "project" {
			projSkill = &skills[i]
			break
		}
	}
	if projSkill == nil {
		t.Fatal("project proj-skill not found")
	}
}

func TestSaveAndGetSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	svc := &Service{}
	content := "---\nname: new-skill\ndescription: Created by test\n---\n# Hello"

	err := svc.SaveSkill(context.Background(), "global", "new-skill", "claude", content)
	if err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	path := filepath.Join(home, ".claude", "skills", "new-skill", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("skill file not created: %v", err)
	}
	if string(data) != content {
		t.Errorf("content mismatch: got %q", string(data))
	}

	// Read it back via GetSkill
	got, err := svc.GetSkill(context.Background(), "global", "new-skill", "claude")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != content {
		t.Errorf("GetSkill content mismatch")
	}
	if got.Name != "new-skill" {
		t.Errorf("name = %q, want %q", got.Name, "new-skill")
	}
}

func TestDeleteSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	svc := &Service{}
	content := "---\nname: del-me\n---\n"
	if err := svc.SaveSkill(context.Background(), "global", "del-me", "claude", content); err != nil {
		t.Fatal(err)
	}

	// Delete it
	if err := svc.DeleteSkill(context.Background(), "global", "del-me", "claude"); err != nil {
		t.Fatal(err)
	}

	// Verify directory is gone
	dir := filepath.Join(home, ".claude", "skills", "del-me")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("skill directory should be deleted, but exists")
	}
}

func TestSyncSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	svc := &Service{}
	content := "---\nname: sync-test\ndescription: Sync me\n---\n# Sync"
	if err := svc.SaveSkill(context.Background(), "global", "sync-test", "claude", content); err != nil {
		t.Fatal(err)
	}

	// Sync from claude to codex and agents
	if err := svc.SyncSkill(context.Background(), "global", "sync-test", "claude", []string{"codex", "agents"}); err != nil {
		t.Fatal(err)
	}

	// Verify files exist in target dirs
	for _, tool := range []string{"codex", "agents"} {
		var dir string
		switch tool {
		case "codex":
			dir = ".codex"
		case "agents":
			dir = ".agents"
		}
		path := filepath.Join(home, dir, "skills", "sync-test", "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("synced file missing for %s: %v", tool, err)
		}
		if string(data) != content {
			t.Errorf("synced content mismatch for %s", tool)
		}
	}
}

func TestSaveSkillTraversalBlocked(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	svc := &Service{}
	err := svc.SaveSkill(context.Background(), "global", "../../../etc", "claude", "malicious")
	if err == nil {
		t.Fatal("expected error for directory traversal, got nil")
	}
}

func TestDeleteSkillTraversalBlocked(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	svc := &Service{}
	err := svc.DeleteSkill(context.Background(), "global", "../../../etc", "claude")
	if err == nil {
		t.Fatal("expected error for directory traversal, got nil")
	}
}

func TestListSkillsMissingDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	svc := &Service{}
	skills, err := svc.ListSkills(context.Background(), "/nonexistent/path")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills with no dirs, got %d", len(skills))
	}
}
