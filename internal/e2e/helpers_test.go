package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type runner struct {
	t      *testing.T
	root   string
	home   string
	config string
}

func newRunner(t *testing.T) *runner {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	config := filepath.Join(home, ".workset", "config.yaml")
	return &runner{
		t:      t,
		root:   root,
		home:   home,
		config: config,
	}
}

func (r *runner) env() []string {
	return append(os.Environ(),
		"HOME="+r.home,
	)
}

func (r *runner) workspaceRoot() string {
	return filepath.Join(r.home, ".workset", "workspaces")
}

func (r *runner) run(args ...string) (string, error) {
	return r.runDir(r.root, args...)
}

func (r *runner) runDir(dir string, args ...string) (string, error) {
	cmd := exec.Command(worksetBin, args...)
	cmd.Dir = dir
	cmd.Env = r.env()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &execError{err: err, stderr: stderr.String(), stdout: stdout.String()}
	}
	if stderr.Len() > 0 {
		return stdout.String() + stderr.String(), nil
	}
	return stdout.String(), nil
}

type execError struct {
	err    error
	stdout string
	stderr string
}

func (e *execError) Error() string {
	msg := e.err.Error()
	if e.stderr != "" {
		msg += ": " + e.stderr
	}
	if e.stdout != "" {
		msg += ": " + e.stdout
	}
	return msg
}

func setupRepo(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	runGit(t, path, "init", "-b", "main")
	runGit(t, path, "config", "user.name", "Tester")
	runGit(t, path, "config", "user.email", "tester@example.com")
	runGit(t, path, "config", "commit.gpgsign", "false")
	runGit(t, path, "config", "tag.gpgsign", "false")
	runGit(t, path, "remote", "add", "origin", path)
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGit(t, path, "add", "README.md")
	runGit(t, path, "commit", "-m", "initial")
	return path
}

func commitFile(t *testing.T, repoPath, branch, filename, contents, message string) {
	t.Helper()
	if branch != "" {
		runGit(t, repoPath, "checkout", "-B", branch)
	}

	if err := os.MkdirAll(filepath.Dir(filepath.Join(repoPath, filename)), 0o755); err != nil {
		t.Fatalf("mkdir file dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, filename), []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, repoPath, "add", filename)
	runGit(t, repoPath, "commit", "-m", message)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
}

func fileURL(path string) string {
	return "file://" + filepath.ToSlash(path)
}
