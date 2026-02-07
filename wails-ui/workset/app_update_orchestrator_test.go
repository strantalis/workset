package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestResolveUpdateChannelUsesPersistedPreference(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := &App{}

	prefs, err := a.SetUpdatePreferences(UpdatePreferencesInput{Channel: string(UpdateChannelAlpha)})
	if err != nil {
		t.Fatalf("SetUpdatePreferences returned error: %v", err)
	}
	if prefs.Channel != string(UpdateChannelAlpha) {
		t.Fatalf("expected persisted alpha channel, got %q", prefs.Channel)
	}

	channel, err := a.resolveUpdateChannel("")
	if err != nil {
		t.Fatalf("resolveUpdateChannel returned error: %v", err)
	}
	if channel != UpdateChannelAlpha {
		t.Fatalf("expected alpha channel from preferences, got %q", channel)
	}

	channel, err = a.resolveUpdateChannel(string(UpdateChannelStable))
	if err != nil {
		t.Fatalf("resolveUpdateChannel with explicit channel returned error: %v", err)
	}
	if channel != UpdateChannelStable {
		t.Fatalf("expected explicit stable channel to override preferences, got %q", channel)
	}
}

func TestUpdateOrchestratorCheckForUpdatesStateTransitions(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	var states []UpdateState

	orch := updateOrchestrator{
		resolveChannel: func(_ string) (UpdateChannel, error) {
			return UpdateChannelStable, nil
		},
		currentVersion: func() string {
			return "v1.0.0"
		},
		fetchManifest: func(UpdateChannel) (UpdateManifest, error) {
			return UpdateManifest{
				Latest: UpdateRelease{
					Version: "v1.1.0",
				},
			}, nil
		},
		persistState: func(state UpdateState) error {
			states = append(states, state)
			return nil
		},
		nowUTC:          func() time.Time { return now },
		compareVersions: compareVersions,
	}

	result, err := orch.CheckForUpdates(UpdateCheckRequest{})
	if err != nil {
		t.Fatalf("CheckForUpdates returned error: %v", err)
	}
	if result.Status != "update_available" {
		t.Fatalf("expected update_available, got %q", result.Status)
	}
	if result.Channel != string(UpdateChannelStable) {
		t.Fatalf("expected stable channel, got %q", result.Channel)
	}

	phases := collectUpdatePhases(states)
	wantPhases := []string{updateStatePhaseChecking, updateStatePhaseIdle}
	if !reflect.DeepEqual(phases, wantPhases) {
		t.Fatalf("unexpected phases: got %v want %v", phases, wantPhases)
	}
	if states[len(states)-1].LatestVersion != "v1.1.0" {
		t.Fatalf("expected latest version in final state, got %q", states[len(states)-1].LatestVersion)
	}
}

func TestUpdateOrchestratorCheckForUpdatesFailureTransitions(t *testing.T) {
	var states []UpdateState
	manifestErr := errors.New("manifest fetch failed")

	orch := updateOrchestrator{
		resolveChannel: func(_ string) (UpdateChannel, error) {
			return UpdateChannelStable, nil
		},
		currentVersion: func() string {
			return "v1.0.0"
		},
		fetchManifest: func(UpdateChannel) (UpdateManifest, error) {
			return UpdateManifest{}, manifestErr
		},
		persistState: func(state UpdateState) error {
			states = append(states, state)
			return nil
		},
		nowUTC:          func() time.Time { return time.Unix(0, 0).UTC() },
		compareVersions: compareVersions,
	}

	_, err := orch.CheckForUpdates(UpdateCheckRequest{})
	if err == nil {
		t.Fatalf("expected check error")
	}
	if !errors.Is(err, manifestErr) {
		t.Fatalf("expected manifest error, got %v", err)
	}

	phases := collectUpdatePhases(states)
	wantPhases := []string{updateStatePhaseChecking, updateStatePhaseFailed}
	if !reflect.DeepEqual(phases, wantPhases) {
		t.Fatalf("unexpected phases: got %v want %v", phases, wantPhases)
	}
}

func TestUpdateOrchestratorStartUpdateStateTransitionsSuccess(t *testing.T) {
	now := time.Date(2026, time.February, 1, 10, 9, 8, 0, time.UTC)
	stageRoot := filepath.Join(t.TempDir(), "stage-root")
	statePath := filepath.Join(t.TempDir(), "state", "ui_update_state.json")
	var states []UpdateState
	var startedArgs []string
	quitCalled := false

	orch := updateOrchestrator{
		resolveChannel: func(_ string) (UpdateChannel, error) {
			return UpdateChannelStable, nil
		},
		currentVersion: func() string {
			return "v1.0.0"
		},
		fetchManifest: func(UpdateChannel) (UpdateManifest, error) {
			return UpdateManifest{
				Latest: UpdateRelease{
					Version: "v1.1.0",
					Asset: UpdateReleaseAsset{
						URL:    "https://example.com/workset.zip",
						SHA256: "abc123",
					},
					Signing: UpdateReleaseSigning{
						TeamID: "ABCDE12345",
					},
				},
			}, nil
		},
		persistState: func(state UpdateState) error {
			states = append(states, state)
			return nil
		},
		getState: func() (UpdateState, error) {
			return UpdateState{Phase: updateStatePhaseIdle}, nil
		},
		nowUTC:          func() time.Time { return now },
		compareVersions: compareVersions,
		goos:            "darwin",
		downloadAsset: func(string) (string, string, error) {
			return filepath.Join(stageRoot, "update.zip"), stageRoot, nil
		},
		selectPackage: selectUpdatePackage,
		verifySHA256: func(string, string) error {
			return nil
		},
		extractAppBundle: func(_, stage string) (string, error) {
			return filepath.Join(stage, "workset.app"), nil
		},
		verifyCodesign: func(string, string) error {
			return nil
		},
		currentBundlePath: func() (string, error) {
			return "/Applications/Workset.app", nil
		},
		updaterHelperPath: func(string) (string, error) {
			return "/usr/local/bin/workset-updater", nil
		},
		updateStatePath: func() (string, error) {
			return statePath, nil
		},
		startCommand: func(cmd *exec.Cmd) error {
			startedArgs = append([]string{cmd.Path}, cmd.Args[1:]...)
			return nil
		},
		processID: func() int {
			return 4321
		},
		quitAfterStart: func() {
			quitCalled = true
		},
	}

	result, err := orch.StartUpdate(UpdateStartRequest{})
	if err != nil {
		t.Fatalf("StartUpdate returned error: %v", err)
	}
	if !result.Started {
		t.Fatalf("expected start to succeed")
	}

	phases := collectUpdatePhases(states)
	wantPhases := []string{
		updateStatePhaseChecking,
		updateStatePhaseIdle,
		updateStatePhaseDownload,
		updateStatePhaseValidate,
		updateStatePhaseApply,
	}
	if !reflect.DeepEqual(phases, wantPhases) {
		t.Fatalf("unexpected phases: got %v want %v", phases, wantPhases)
	}

	if got := argValue(startedArgs, "--channel"); got != string(UpdateChannelStable) {
		t.Fatalf("expected stable channel flag, got %q", got)
	}
	if got := argValue(startedArgs, "--current-version"); got != "v1.0.0" {
		t.Fatalf("unexpected current version flag: %q", got)
	}
	if got := argValue(startedArgs, "--latest-version"); got != "v1.1.0" {
		t.Fatalf("unexpected latest version flag: %q", got)
	}
	if !quitCalled {
		t.Fatalf("expected quit callback to be called")
	}
}

func TestUpdateOrchestratorStartUpdateStateTransitionsFailure(t *testing.T) {
	var states []UpdateState
	checksumErr := errors.New("checksum mismatch")

	orch := updateOrchestrator{
		resolveChannel: func(_ string) (UpdateChannel, error) {
			return UpdateChannelStable, nil
		},
		currentVersion: func() string {
			return "v1.0.0"
		},
		fetchManifest: func(UpdateChannel) (UpdateManifest, error) {
			return UpdateManifest{
				Latest: UpdateRelease{
					Version: "v1.1.0",
					Asset: UpdateReleaseAsset{
						URL:    "https://example.com/workset.zip",
						SHA256: "abc123",
					},
					Signing: UpdateReleaseSigning{
						TeamID: "ABCDE12345",
					},
				},
			}, nil
		},
		persistState: func(state UpdateState) error {
			states = append(states, state)
			return nil
		},
		getState: func() (UpdateState, error) {
			return UpdateState{Phase: updateStatePhaseIdle}, nil
		},
		nowUTC:          func() time.Time { return time.Unix(0, 0).UTC() },
		compareVersions: compareVersions,
		goos:            "darwin",
		downloadAsset: func(string) (string, string, error) {
			stageRoot := filepath.Join(t.TempDir(), "stage-root")
			return filepath.Join(stageRoot, "update.zip"), stageRoot, nil
		},
		selectPackage: selectUpdatePackage,
		verifySHA256: func(string, string) error {
			return checksumErr
		},
		extractAppBundle: func(string, string) (string, error) {
			t.Fatalf("extract should not run after checksum failure")
			return "", nil
		},
		verifyCodesign: func(string, string) error {
			t.Fatalf("codesign check should not run after checksum failure")
			return nil
		},
		currentBundlePath: func() (string, error) {
			return "", nil
		},
		updaterHelperPath: func(string) (string, error) {
			return "", nil
		},
		updateStatePath: func() (string, error) {
			return "", nil
		},
		startCommand: func(*exec.Cmd) error {
			return nil
		},
		processID:      func() int { return 0 },
		quitAfterStart: func() {},
	}

	_, err := orch.StartUpdate(UpdateStartRequest{})
	if err == nil {
		t.Fatalf("expected start failure")
	}
	if !errors.Is(err, checksumErr) {
		t.Fatalf("expected checksum error, got %v", err)
	}

	phases := collectUpdatePhases(states)
	wantPhases := []string{
		updateStatePhaseChecking,
		updateStatePhaseIdle,
		updateStatePhaseDownload,
		updateStatePhaseValidate,
		updateStatePhaseFailed,
	}
	if !reflect.DeepEqual(phases, wantPhases) {
		t.Fatalf("unexpected phases: got %v want %v", phases, wantPhases)
	}
}

func TestSelectUpdatePackageValidation(t *testing.T) {
	_, err := selectUpdatePackage(UpdateRelease{
		Asset: UpdateReleaseAsset{
			URL:    "https://example.com/workset.zip",
			SHA256: "abc123",
		},
	})
	if err == nil {
		t.Fatalf("expected missing signing team id error")
	}
}

func collectUpdatePhases(states []UpdateState) []string {
	phases := make([]string, 0, len(states))
	for _, state := range states {
		phases = append(phases, state.Phase)
	}
	return phases
}

func argValue(args []string, name string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}
