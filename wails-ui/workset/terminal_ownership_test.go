package main

import (
	"context"
	"testing"
)

func TestAllowWorkspaceTerminalStartMainOwnerAllowsClaim(t *testing.T) {
	app := NewApp()

	if err := app.allowWorkspaceTerminalStart("test-workspace", "workspace-test-popout"); err != nil {
		t.Fatalf("expected popout claim to succeed when main owns workspace: %v", err)
	}
	if got := app.GetWorkspaceTerminalOwner("test-workspace"); got != "workspace-test-popout" {
		t.Fatalf("expected owner workspace-test-popout, got %q", got)
	}
}

func TestAllowWorkspaceTerminalStartAllowsHandoff(t *testing.T) {
	app := NewApp()
	app.claimWorkspaceTerminalOwner("test-workspace", "workspace-test-popout")

	if err := app.allowWorkspaceTerminalStart("test-workspace", "main"); err != nil {
		t.Fatalf("expected ownership handoff to succeed: %v", err)
	}
	if got := app.GetWorkspaceTerminalOwner("test-workspace"); got != "main" {
		t.Fatalf("expected owner main after handoff, got %q", got)
	}
}

func TestAllowWorkspaceTerminalStartAllowsCurrentOwner(t *testing.T) {
	app := NewApp()
	app.claimWorkspaceTerminalOwner("test-workspace", "workspace-test-popout")

	if err := app.allowWorkspaceTerminalStart("test-workspace", "workspace-test-popout"); err != nil {
		t.Fatalf("expected current owner to restart terminal: %v", err)
	}
	if got := app.GetWorkspaceTerminalOwner("test-workspace"); got != "workspace-test-popout" {
		t.Fatalf("expected owner workspace-test-popout, got %q", got)
	}
}

func TestResizeWorkspaceTerminalForWindowNameIgnoresTransientMissingTerminal(t *testing.T) {
	app := NewApp()

	if err := app.ResizeWorkspaceTerminalForWindowName(
		context.Background(),
		"test-workspace",
		"missing-terminal",
		80,
		24,
		"main",
	); err != nil {
		t.Fatalf("expected transient resize miss to be ignored, got %v", err)
	}
}

func TestResizeWorkspaceTerminalForWindowNameRejectsMissingWorkspaceID(t *testing.T) {
	app := NewApp()

	err := app.ResizeWorkspaceTerminalForWindowName(
		context.Background(),
		"",
		"term-1",
		80,
		24,
		"main",
	)
	if err == nil {
		t.Fatal("expected missing workspace id to fail")
	}
}
