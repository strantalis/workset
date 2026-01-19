package hooks

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type stubRunner struct {
	last RunRequest
}

func (s *stubRunner) Run(_ context.Context, req RunRequest) error {
	s.last = req
	_, _ = io.WriteString(req.Stdout, "hello from hook\n")
	_, _ = io.WriteString(req.Stderr, "hook stderr\n")
	return nil
}

func TestEngineRunsHookAndLogs(t *testing.T) {
	root := t.TempDir()
	logRoot := filepath.Join(root, "logs")
	runner := &stubRunner{}
	engine := Engine{
		Runner: runner,
		Clock: func() time.Time {
			return time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		},
	}

	ctxPayload := Context{
		WorkspaceRoot: root,
		WorkspaceName: "demo",
		RepoName:      "repo-a",
		RepoDir:       "repo-a",
		RepoPath:      filepath.Join(root, "repo-a"),
		WorktreePath:  filepath.Join(root, "repo-a"),
		Branch:        "main",
		Event:         EventWorktreeCreated,
	}

	report, err := engine.Run(context.Background(), RunInput{
		Event:          EventWorktreeCreated,
		Hooks:          []Hook{{ID: "bootstrap", On: []Event{EventWorktreeCreated}, Run: []string{"echo", "{repo.name}"}}},
		DefaultOnError: OnErrorFail,
		LogRoot:        logRoot,
		Context:        ctxPayload,
	})
	if err != nil {
		t.Fatalf("run hooks: %v", err)
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result")
	}
	if runner.last.Command[1] != "repo-a" {
		t.Fatalf("expected interpolated arg, got %v", runner.last.Command)
	}
	if report.Results[0].LogPath == "" {
		t.Fatalf("expected log path")
	}
	data, err := os.ReadFile(report.Results[0].LogPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "hello from hook") {
		t.Fatalf("expected hook output in log")
	}
}
