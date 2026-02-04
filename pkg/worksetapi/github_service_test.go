package worksetapi

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-github/v75/github"
	"github.com/strantalis/workset/internal/config"
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

func TestApplyAgentModelCodexInsertsBeforePrompt(t *testing.T) {
	command := []string{"codex", "exec", "--output-schema", "schema.json", "-"}
	updated := applyAgentModel(command, "gpt-4o-mini")
	if !sliceHasExact(updated, "-m") {
		t.Fatalf("expected model flag in command: %v", updated)
	}
	promptIdx := slices.Index(updated, "-")
	modelIdx := slices.Index(updated, "-m")
	if modelIdx == -1 || promptIdx == -1 || modelIdx > promptIdx {
		t.Fatalf("expected model flag before prompt: %v", updated)
	}
}

func TestApplyAgentModelSkipsExistingCodexModelFlag(t *testing.T) {
	command := []string{"codex", "exec", "-m", "gpt-4o", "-"}
	updated := applyAgentModel(command, "gpt-4o-mini")
	if countExact(updated, "-m") != 1 {
		t.Fatalf("expected single model flag: %v", updated)
	}
}

func TestApplyAgentModelClaudeAddsModelFlag(t *testing.T) {
	command := []string{"claude", "--output-format", "json"}
	updated := applyAgentModel(command, "haiku")
	if !sliceHasExact(updated, "--model") || !sliceHasExact(updated, "haiku") {
		t.Fatalf("expected claude model flag: %v", updated)
	}
}

func TestRunAgentPromptWithModelFallbacksOnInvalidJSON(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.GlobalConfig{
		Defaults: config.Defaults{
			Remote:      "origin",
			BaseBranch:  "main",
			Agent:       "codex",
			AgentLaunch: "strict",
		},
	}
	if err := config.SaveGlobal(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	var calls []string
	runner := func(_ context.Context, _ string, command []string, _ []string, _ string) (CommandResult, error) {
		calls = append(calls, strings.Join(command, " "))
		if sliceHasExact(command, "-m") {
			return CommandResult{ExitCode: 0, Stdout: "not-json"}, nil
		}
		return CommandResult{ExitCode: 0, Stdout: `{"title":"t","body":"b"}`}, nil
	}
	svc := NewService(Options{
		ConfigPath:    cfgPath,
		CommandRunner: runner,
		Logf:          func(string, ...any) {},
	})
	agentPath := filepath.Join(t.TempDir(), "codex")
	result, err := svc.runAgentPrompt(context.Background(), t.TempDir(), agentPath, "prompt", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "t" || result.Body != "b" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(calls) != 2 {
		t.Fatalf("expected fallback call, got %d", len(calls))
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

func countExact(values []string, target string) int {
	count := 0
	for _, value := range values {
		if value == target {
			count++
		}
	}
	return count
}
