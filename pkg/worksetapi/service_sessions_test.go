package worksetapi

import (
	"context"
	"errors"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/session"
	"github.com/strantalis/workset/internal/workspace"
)

func TestStartSessionExecBackend(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	env.runner.results[commandKey("true", nil)] = session.CommandResult{ExitCode: 0}

	result, err := env.svc.StartSession(context.Background(), SessionStartInput{
		Workspace: WorkspaceSelector{Value: root},
		Backend:   "exec",
		Command:   []string{"true"},
		Attach:    true,
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if result.Attached {
		t.Fatalf("expected exec backend to skip attach")
	}
	if result.Notice.AttachNote == "" {
		t.Fatalf("expected attach note for exec backend")
	}

	state, err := env.svc.workspaces.LoadState(context.Background(), root)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Sessions) != 0 {
		t.Fatalf("expected exec backend to skip session recording")
	}
}

func TestStartSessionInteractiveRequiresExec(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	_, err := env.svc.StartSession(context.Background(), SessionStartInput{
		Workspace:   WorkspaceSelector{Value: root},
		Backend:     "tmux",
		Interactive: true,
		Confirmed:   true,
	})
	_ = requireErrorType[ValidationError](t, err)
}

func TestStartSessionTmuxNameNormalization(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	result, err := env.svc.StartSession(context.Background(), SessionStartInput{
		Workspace: WorkspaceSelector{Value: root},
		Backend:   "tmux",
		Name:      "bad name!",
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if result.Notice.NameNotice == "" {
		t.Fatalf("expected name notice")
	}

	state, err := env.svc.workspaces.LoadState(context.Background(), root)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if _, ok := state.Sessions["bad_name_"]; !ok {
		t.Fatalf("expected normalized session name recorded")
	}
}

func TestStartSessionTmuxAttach(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	env.runner.results[commandKey("tmux", []string{"new-session", "-d", "-s", "workset-demo", "-c", root})] = session.CommandResult{ExitCode: 0}
	env.runner.results[commandKey("tmux", []string{"attach", "-t", "workset-demo"})] = session.CommandResult{ExitCode: 0}

	result, err := env.svc.StartSession(context.Background(), SessionStartInput{
		Workspace: WorkspaceSelector{Value: root},
		Backend:   "tmux",
		Attach:    true,
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if !result.Attached {
		t.Fatalf("expected attached session")
	}
}

func TestAttachSessionNameNormalization(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"bad name!": {Backend: "tmux", Name: "bad name!"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	env.runner.results[commandKey("tmux", []string{"attach", "-t", "bad_name_"})] = session.CommandResult{ExitCode: 0}

	_, err := env.svc.AttachSession(context.Background(), SessionAttachInput{
		Workspace: WorkspaceSelector{Value: root},
		Name:      "bad name!",
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("attach session: %v", err)
	}
	updated, err := env.svc.workspaces.LoadState(context.Background(), root)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if _, ok := updated.Sessions["bad_name_"]; !ok {
		t.Fatalf("expected normalized session name")
	}
}

func TestStopSessionNameNormalization(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"bad name!": {Backend: "tmux", Name: "bad name!"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	env.runner.results[commandKey("tmux", []string{"kill-session", "-t", "bad_name_"})] = session.CommandResult{ExitCode: 0}

	_, err := env.svc.StopSession(context.Background(), SessionStopInput{
		Workspace: WorkspaceSelector{Value: root},
		Name:      "bad name!",
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("stop session: %v", err)
	}
}

func TestAttachAndStopSessionTmux(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	env.runner.lookPath["tmux"] = nil
	env.runner.results[commandKey("tmux", []string{"attach", "-t", "workset-demo"})] = session.CommandResult{ExitCode: 0}
	env.runner.results[commandKey("tmux", []string{"kill-session", "-t", "workset-demo"})] = session.CommandResult{ExitCode: 0}

	_, err := env.svc.AttachSession(context.Background(), SessionAttachInput{
		Workspace: WorkspaceSelector{Value: root},
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("attach session: %v", err)
	}

	_, err = env.svc.StopSession(context.Background(), SessionStopInput{
		Workspace: WorkspaceSelector{Value: root},
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("stop session: %v", err)
	}
}

func TestAttachSessionRequiresConfirmation(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	_, err := env.svc.AttachSession(context.Background(), SessionAttachInput{
		Workspace: WorkspaceSelector{Value: root},
	})
	_ = requireErrorType[ConfirmationRequired](t, err)
}

func TestStopSessionRequiresConfirmation(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	_, err := env.svc.StopSession(context.Background(), SessionStopInput{
		Workspace: WorkspaceSelector{Value: root},
	})
	_ = requireErrorType[ConfirmationRequired](t, err)
}

func TestListAndShowSessions(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux", Name: "workset-demo"},
			"workset-alt":  {Backend: "tmux", Name: "workset-alt"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	env.runner.lookPath["tmux"] = nil
	env.runner.results[commandKey("tmux", []string{"has-session", "-t", "workset-demo"})] = session.CommandResult{ExitCode: 0}
	env.runner.errors[commandKey("tmux", []string{"has-session", "-t", "workset-demo"})] = nil
	env.runner.results[commandKey("tmux", []string{"has-session", "-t", "workset-alt"})] = session.CommandResult{ExitCode: 1}
	env.runner.errors[commandKey("tmux", []string{"has-session", "-t", "workset-alt"})] = errors.New("exit status 1")

	list, err := env.svc.ListSessions(context.Background(), WorkspaceSelector{Value: root})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list.Sessions) != 2 {
		t.Fatalf("expected 2 sessions")
	}

	record, _, err := env.svc.ShowSession(context.Background(), SessionShowInput{
		Workspace: WorkspaceSelector{Value: root},
		Name:      "workset-demo",
	})
	if err != nil {
		t.Fatalf("show session: %v", err)
	}
	if record.Name != "workset-demo" {
		t.Fatalf("unexpected session name: %s", record.Name)
	}
}

func TestWorkspaceNameFallback(t *testing.T) {
	ws := workspace.Workspace{Config: config.WorkspaceConfig{}}
	name := workspaceName(ws, "", "/tmp/demo")
	if name != "demo" {
		t.Fatalf("unexpected workspace name: %s", name)
	}
}

func TestAttachSessionExecUnsupported(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"exec-session": {Backend: "exec", Name: "exec-session"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	_, err := env.svc.AttachSession(context.Background(), SessionAttachInput{
		Workspace: WorkspaceSelector{Value: root},
		Name:      "exec-session",
		Confirmed: true,
	})
	if err == nil {
		t.Fatalf("expected attach exec error")
	}
}

func TestStopSessionExecUnsupported(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"exec-session": {Backend: "exec", Name: "exec-session"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	_, err := env.svc.StopSession(context.Background(), SessionStopInput{
		Workspace: WorkspaceSelector{Value: root},
		Name:      "exec-session",
		Confirmed: true,
	})
	if err == nil {
		t.Fatalf("expected stop exec error")
	}
}

func TestShowSessionMissing(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")
	_, _, err := env.svc.ShowSession(context.Background(), SessionShowInput{
		Workspace: WorkspaceSelector{Value: root},
		Name:      "missing",
	})
	_ = requireErrorType[NotFoundError](t, err)
}

func TestAttachSessionMissing(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	env.runner.results[commandKey("tmux", []string{"attach", "-t", "missing"})] = session.CommandResult{ExitCode: 0}
	_, err := env.svc.AttachSession(context.Background(), SessionAttachInput{
		Workspace: WorkspaceSelector{Value: root},
		Name:      "missing",
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("attach session: %v", err)
	}
}

func TestStopSessionMissing(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	env.runner.results[commandKey("tmux", []string{"kill-session", "-t", "missing"})] = session.CommandResult{ExitCode: 0}
	_, err := env.svc.StopSession(context.Background(), SessionStopInput{
		Workspace: WorkspaceSelector{Value: root},
		Name:      "missing",
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("stop session: %v", err)
	}
}

func TestListSessionsBackendAuto(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	state := workspace.State{
		CurrentBranch: "demo",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "auto", Name: "workset-demo"},
		},
	}
	if err := env.svc.workspaces.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	env.runner.results[commandKey("tmux", []string{"has-session", "-t", "workset-demo"})] = session.CommandResult{ExitCode: 1}
	env.runner.errors[commandKey("tmux", []string{"has-session", "-t", "workset-demo"})] = errors.New("exit status 1")

	list, err := env.svc.ListSessions(context.Background(), WorkspaceSelector{Value: root})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list.Sessions) != 1 {
		t.Fatalf("expected one session")
	}
}
