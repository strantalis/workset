package main

import (
	"strings"
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

func TestAllowWorkspaceTerminalStartRejectsNonOwner(t *testing.T) {
	app := NewApp()
	app.claimWorkspaceTerminalOwner("test-workspace", "workspace-test-popout")

	err := app.allowWorkspaceTerminalStart("test-workspace", "main")
	if err == nil {
		t.Fatalf("expected ownership error")
	}
	if !strings.Contains(err.Error(), `owned by window "workspace-test-popout"`) {
		t.Fatalf("expected owner mismatch error, got %q", err.Error())
	}
	if got := app.GetWorkspaceTerminalOwner("test-workspace"); got != "workspace-test-popout" {
		t.Fatalf("expected owner to remain workspace-test-popout, got %q", got)
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
