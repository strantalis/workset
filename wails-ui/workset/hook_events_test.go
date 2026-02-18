package main

import (
	"context"
	"testing"

	"github.com/strantalis/workset/pkg/worksetapi"
)

func TestHookOperation(t *testing.T) {
	if got := hookOperation("workspace.create"); got != "workspace.create" {
		t.Fatalf("unexpected operation: %s", got)
	}
	if got := hookOperation("repo.add.extra"); got != "repo.add" {
		t.Fatalf("unexpected operation: %s", got)
	}
	if got := hookOperation(""); got != "hooks.run" {
		t.Fatalf("unexpected operation: %s", got)
	}
}

func TestAppHookObserverEmitsProgress(t *testing.T) {
	origEmit := hookEventsEmit
	defer func() {
		hookEventsEmit = origEmit
	}()

	var (
		eventName string
		payload   HookProgressPayload
	)
	hookEventsEmit = func(_ context.Context, name string, data ...interface{}) {
		eventName = name
		if len(data) > 0 {
			typed, ok := data[0].(HookProgressPayload)
			if !ok {
				t.Fatalf("unexpected payload type: %T", data[0])
			}
			payload = typed
		}
	}

	observer := appHookObserver{app: &App{ctx: context.Background()}}
	observer.OnHookProgress(worksetapi.HookProgress{
		Phase:     "finished",
		Event:     "worktree.created",
		HookID:    "bootstrap",
		Workspace: "demo",
		Repo:      "repo-a",
		Reason:    "workspace.create",
		Status:    worksetapi.HookRunStatusFailed,
		LogPath:   "/tmp/hook.log",
		Error:     "boom",
	})

	if eventName != EventHooksProgress {
		t.Fatalf("unexpected event: %s", eventName)
	}
	if payload.Operation != "workspace.create" {
		t.Fatalf("unexpected operation: %s", payload.Operation)
	}
	if payload.Repo != "repo-a" || payload.HookID != "bootstrap" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Error == "" || payload.Status != "failed" {
		t.Fatalf("expected error and failed status: %+v", payload)
	}
}
