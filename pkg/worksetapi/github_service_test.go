package worksetapi

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-github/v75/github"
)

func TestPrepareAgentCommandCodexAddsExecAndPrompt(t *testing.T) {
	schemaPath := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	command, env, stdin, err := prepareAgentCommand([]string{"codex"}, "prompt", schemaPath)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if len(command) < 2 || filepath.Base(command[0]) != "codex" || command[1] != "exec" {
		t.Fatalf("unexpected command: %v", command)
	}
	if !sliceHasExact(command, "-") {
		t.Fatalf("expected stdin prompt, got: %v", command)
	}
	if !hasFlag(command, "--output-schema") {
		t.Fatalf("expected output schema flag: %v", command)
	}
	if stdin != "prompt" {
		t.Fatalf("unexpected stdin: %q", stdin)
	}
	if !envHas(env, "WORKSET_PR_PROMPT=prompt") {
		t.Fatalf("missing prompt env")
	}
	if !envHas(env, "WORKSET_PR_JSON=1") {
		t.Fatalf("missing json env")
	}
}

func TestPrepareAgentCommandCodexKeepsPromptArg(t *testing.T) {
	schemaPath := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	command, _, _, err := prepareAgentCommand([]string{"codex", "exec", "summarize", "diff"}, "prompt", schemaPath)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if sliceHasExact(command, "-") {
		t.Fatalf("unexpected stdin prompt: %v", command)
	}
}

func TestPrepareAgentCommandCodexPreservesResolvedPath(t *testing.T) {
	schemaPath := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	tempDir := t.TempDir()
	codexPath := filepath.Join(tempDir, "codex")
	if err := os.WriteFile(codexPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write codex: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(codexPath, 0o755); err != nil {
			t.Fatalf("chmod codex: %v", err)
		}
	}

	command, _, _, err := prepareAgentCommand([]string{codexPath}, "prompt", schemaPath)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if len(command) == 0 {
		t.Fatalf("unexpected empty command")
	}
	want := filepath.Clean(codexPath)
	if command[0] != want {
		t.Fatalf("unexpected command path: got %q want %q", command[0], want)
	}
}

func TestIsInvalidHeadError(t *testing.T) {
	err := &github.ErrorResponse{
		Message: "Validation Failed",
		Errors: []github.Error{
			{Resource: "PullRequest", Field: "head", Code: "invalid"},
		},
	}
	if !isInvalidHeadError(err) {
		t.Fatalf("expected invalid head error")
	}
}

func TestFormatGitHubAPIError(t *testing.T) {
	err := &github.ErrorResponse{
		Message: "Validation Failed",
		Errors: []github.Error{
			{Resource: "PullRequest", Field: "head", Code: "invalid"},
		},
	}
	message := formatGitHubAPIError(err)
	if !strings.Contains(message, "Validation Failed") || !strings.Contains(message, "PullRequest head invalid") {
		t.Fatalf("unexpected message: %q", message)
	}
}

func TestParseCommitJSON(t *testing.T) {
	message, err := parseCommitJSON(`{"message":"feat(core): add commit"}`)
	if err != nil {
		t.Fatalf("parse commit json: %v", err)
	}
	if message != "feat(core): add commit" {
		t.Fatalf("unexpected message: %q", message)
	}
}

func envHas(env []string, key string) bool {
	return slices.Contains(env, key)
}

func sliceHasExact(values []string, target string) bool {
	return slices.Contains(values, target)
}
