package worksetapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitHubCLIVersionStrict(t *testing.T) {
	if got := parseGitHubCLIVersion("gh version 2.87.2 (2026-02-20)"); got != "2.87.2" {
		t.Fatalf("expected gh version parse, got %q", got)
	}
	if got := parseGitHubCLIVersion("codex version 1.2.3"); got != "" {
		t.Fatalf("expected non-gh version output to be rejected, got %q", got)
	}
}

func TestEnsureGitHubCLIPathIgnoresNonGhPath(t *testing.T) {
	dir := t.TempDir()
	codexPath := filepath.Join(dir, "codex")
	if err := os.WriteFile(codexPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	t.Setenv("GH_PATH", codexPath)

	if got := ensureGitHubCLIPath(); got == codexPath {
		t.Fatalf("expected non-gh GH_PATH to be ignored")
	}
}

func TestEnsureGitHubCLIPathIgnoresMissingGhPath(t *testing.T) {
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "gh")
	t.Setenv("GH_PATH", missingPath)

	if got := ensureGitHubCLIPath(); got == missingPath {
		t.Fatalf("expected missing GH_PATH to be ignored")
	}
}

func TestSetGitHubCLIPathRejectsNonGhBinaryName(t *testing.T) {
	env := newTestEnv(t)
	dir := t.TempDir()
	codexPath := filepath.Join(dir, "codex")
	if err := os.WriteFile(codexPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	_, err := env.svc.SetGitHubCLIPath(context.Background(), codexPath)
	validationErr := requireErrorType[ValidationError](t, err)
	if !strings.Contains(validationErr.Message, "must point to the `gh` binary") {
		t.Fatalf("unexpected validation error: %q", validationErr.Message)
	}
}

func TestGetGitHubCLIStatusTreatsInvalidVersionAsNotInstalled(t *testing.T) {
	env := newTestEnv(t)
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	cfg := env.loadConfig()
	cfg.GitHub.CLIPath = ghPath
	env.saveConfig(cfg)

	env.svc.commands = func(_ context.Context, _ string, command []string, _ []string, _ string) (CommandResult, error) {
		if len(command) == 2 && command[0] == ghPath && command[1] == "--version" {
			return CommandResult{Stdout: "codex version 1.2.3\n", ExitCode: 0}, nil
		}
		return CommandResult{Stdout: "", Stderr: "", ExitCode: 0}, nil
	}

	status, err := env.svc.GetGitHubCLIStatus(context.Background())
	if err != nil {
		t.Fatalf("GetGitHubCLIStatus returned error: %v", err)
	}
	if status.Installed {
		t.Fatalf("expected installed=false for invalid gh --version output")
	}
	if strings.TrimSpace(status.Error) == "" {
		t.Fatalf("expected error for invalid gh --version output")
	}
}

func TestGetGitHubCLIStatusWarnsOnInvalidGHPathEnv(t *testing.T) {
	env := newTestEnv(t)
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "gh")
	t.Setenv("GH_PATH", missingPath)

	status, err := env.svc.GetGitHubCLIStatus(context.Background())
	if err != nil {
		t.Fatalf("GetGitHubCLIStatus returned error: %v", err)
	}
	if !strings.Contains(status.Error, "Ignoring GH_PATH=") {
		t.Fatalf("expected GH_PATH warning, got %q", status.Error)
	}
}
