package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusJSONOutput(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "status-repo"))
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}
	out, err := runner.run("status", "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}
	if !strings.Contains(out, "\"name\": \"status-repo\"") {
		t.Fatalf("status json missing repo: %s", out)
	}
	if !strings.Contains(out, "\"path\":") {
		t.Fatalf("status json missing path: %s", out)
	}
}

func TestStatusPlainOutput(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "status-plain-repo"))
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}
	out, err := runner.run("status", "-w", "demo", "--plain")
	if err != nil {
		t.Fatalf("status --plain: %v", err)
	}
	if !strings.Contains(out, "status-plain-repo") {
		t.Fatalf("status plain missing repo: %s", out)
	}
}
