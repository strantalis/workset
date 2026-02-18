package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateFlowWithMultipleWorkspaces(t *testing.T) {
	runner := newRunner(t)
	repoA := setupRepo(t, filepath.Join(runner.root, "src", "repo-a"))
	repoB := setupRepo(t, filepath.Join(runner.root, "src", "repo-b"))

	if _, err := runner.run("repo", "registry", "add", "repo-a", repoA); err != nil {
		t.Fatalf("registry add repo-a: %v", err)
	}
	if _, err := runner.run("repo", "registry", "add", "repo-b", repoB); err != nil {
		t.Fatalf("registry add repo-b: %v", err)
	}
	if _, err := runner.run("repo", "registry", "set", "--default-branch", "main", "repo-a", repoA); err != nil {
		t.Fatalf("registry set default branch: %v", err)
	}
	out, err := runner.run("repo", "registry", "ls", "--json")
	if err != nil {
		t.Fatalf("registry ls: %v", err)
	}
	if !strings.Contains(out, "\"name\": \"repo-a\"") {
		t.Fatalf("registry ls missing repo-a: %s", out)
	}

	if _, err := runner.run("template", "create", "stack", "--description", "demo template"); err != nil {
		t.Fatalf("template create: %v", err)
	}
	if _, err := runner.run("template", "add", "stack", "repo-a"); err != nil {
		t.Fatalf("template add repo-a: %v", err)
	}
	if _, err := runner.run("template", "add", "stack", "repo-b"); err != nil {
		t.Fatalf("template add repo-b: %v", err)
	}
	out, err = runner.run("template", "ls", "--plain")
	if err != nil {
		t.Fatalf("template ls: %v", err)
	}
	if !strings.Contains(out, "stack") {
		t.Fatalf("template ls missing stack: %s", out)
	}
	out, err = runner.run("template", "show", "stack", "--plain")
	if err != nil {
		t.Fatalf("template show: %v", err)
	}
	if !strings.Contains(out, "repo-a") || !strings.Contains(out, "repo-b") {
		t.Fatalf("template show missing repos: %s", out)
	}

	if _, err := runner.run("new", "alpha"); err != nil {
		t.Fatalf("workset new alpha: %v", err)
	}
	if _, err := runner.run("new", "beta"); err != nil {
		t.Fatalf("workset new beta: %v", err)
	}
	if _, err := runner.run("template", "apply", "-w", "alpha", "stack"); err != nil {
		t.Fatalf("template apply alpha: %v", err)
	}
	if _, err := runner.run("template", "apply", "-w", "beta", "stack"); err != nil {
		t.Fatalf("template apply beta: %v", err)
	}
	out, err = runner.run("repo", "ls", "-w", "alpha", "--plain")
	if err != nil {
		t.Fatalf("repo ls alpha: %v", err)
	}
	if !strings.Contains(out, "repo-a") || !strings.Contains(out, "repo-b") {
		t.Fatalf("repo ls alpha missing repos: %s", out)
	}
	out, err = runner.run("repo", "ls", "-w", "beta", "--plain")
	if err != nil {
		t.Fatalf("repo ls beta: %v", err)
	}
	if !strings.Contains(out, "repo-a") || !strings.Contains(out, "repo-b") {
		t.Fatalf("repo ls beta missing repos: %s", out)
	}

	if _, err := runner.run("template", "remove", "stack", "repo-b"); err != nil {
		t.Fatalf("template remove: %v", err)
	}
	if _, err := runner.run("template", "rm", "stack"); err != nil {
		t.Fatalf("template rm: %v", err)
	}
	if _, err := runner.run("repo", "registry", "rm", "repo-b"); err != nil {
		t.Fatalf("registry rm repo-b: %v", err)
	}
}

func TestGroupRegistryCommands(t *testing.T) {
	runner := newRunner(t)
	repoA := setupRepo(t, filepath.Join(runner.root, "src", "group-repo-a"))

	if _, err := runner.run("repo", "registry", "add", "group-repo-a", repoA); err != nil {
		t.Fatalf("registry add: %v", err)
	}
	if _, err := runner.run("group", "create", "group-stack", "--description", "group demo"); err != nil {
		t.Fatalf("group create: %v", err)
	}
	if _, err := runner.run("group", "add", "group-stack", "group-repo-a"); err != nil {
		t.Fatalf("group add: %v", err)
	}
	out, err := runner.run("group", "ls", "--plain")
	if err != nil {
		t.Fatalf("group ls: %v", err)
	}
	if !strings.Contains(out, "group-stack") {
		t.Fatalf("group ls missing group-stack: %s", out)
	}
	out, err = runner.run("group", "show", "group-stack", "--plain")
	if err != nil {
		t.Fatalf("group show: %v", err)
	}
	if !strings.Contains(out, "group-repo-a") {
		t.Fatalf("group show missing repo: %s", out)
	}
	out, err = runner.run("group", "show", "group-stack", "--json")
	if err != nil {
		t.Fatalf("group show --json: %v", err)
	}
	if !strings.Contains(out, "\"repo\": \"group-repo-a\"") {
		t.Fatalf("group show missing repo in json: %s", out)
	}
	if _, err := runner.run("group", "rm", "group-stack"); err != nil {
		t.Fatalf("group rm: %v", err)
	}
	if _, err := runner.run("repo", "registry", "rm", "group-repo-a"); err != nil {
		t.Fatalf("registry rm: %v", err)
	}
}
