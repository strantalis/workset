package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type updateOrchestrator struct {
	resolveChannel    func(string) (UpdateChannel, error)
	currentVersion    func() string
	fetchManifest     func(UpdateChannel) (UpdateManifest, error)
	persistState      func(UpdateState) error
	getState          func() (UpdateState, error)
	nowUTC            func() time.Time
	compareVersions   func(string, string) (int, error)
	goos              string
	downloadAsset     func(string) (string, string, error)
	selectPackage     func(UpdateRelease) (updatePackageSelection, error)
	verifySHA256      func(string, string) error
	extractAppBundle  func(string, string) (string, error)
	verifyCodesign    func(string, string) error
	currentBundlePath func() (string, error)
	updaterHelperPath func(string) (string, error)
	updateStatePath   func() (string, error)
	startCommand      func(*exec.Cmd) error
	processID         func() int
	quitAfterStart    func()
}

func (a *App) newUpdateOrchestrator() updateOrchestrator {
	return updateOrchestrator{
		resolveChannel: a.resolveUpdateChannel,
		currentVersion: func() string {
			return normalizeVersion(a.GetAppVersion().Version)
		},
		fetchManifest:     a.fetchUpdateManifest,
		persistState:      a.persistUpdateState,
		getState:          a.GetUpdateState,
		nowUTC:            func() time.Time { return time.Now().UTC() },
		compareVersions:   compareVersions,
		goos:              runtime.GOOS,
		downloadAsset:     a.downloadUpdateAsset,
		selectPackage:     selectUpdatePackage,
		verifySHA256:      verifySHA256,
		extractAppBundle:  extractAppBundle,
		verifyCodesign:    verifyCodesign,
		currentBundlePath: currentBundlePath,
		updaterHelperPath: updaterHelperPath,
		updateStatePath:   a.updateStatePath,
		startCommand: func(cmd *exec.Cmd) error {
			return cmd.Start()
		},
		processID: os.Getpid,
		quitAfterStart: func() {
			if a.ctx == nil {
				return
			}
			go func(ctx context.Context) {
				time.Sleep(350 * time.Millisecond)
				wruntime.Quit(ctx)
			}(a.ctx)
		},
	}
}

func (o updateOrchestrator) CheckForUpdates(input UpdateCheckRequest) (UpdateCheckResult, error) {
	channel, err := o.resolveChannel(input.Channel)
	if err != nil {
		return UpdateCheckResult{}, err
	}

	currentVersion := strings.TrimSpace(o.currentVersion())
	if currentVersion == "" {
		currentVersion = "v0.0.0"
	}

	o.persistPhase(updateStatePhaseChecking, string(channel), currentVersion, "", "Checking for updates...")

	manifest, err := o.fetchManifest(channel)
	if err != nil {
		o.persistFailure(string(channel), currentVersion, "", "Update check failed.", err)
		return UpdateCheckResult{}, err
	}

	latestVersion := normalizeVersion(manifest.Latest.Version)
	if latestVersion == "" {
		err := errors.New("manifest latest version is missing")
		o.persistFailure(string(channel), currentVersion, "", "Update check failed.", err)
		return UpdateCheckResult{}, err
	}

	if manifest.Disabled {
		result := UpdateCheckResult{
			Status:         "unavailable",
			Channel:        string(channel),
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Message:        strings.TrimSpace(manifest.Message),
			Release:        &manifest.Latest,
		}
		o.persistPhase(updateStatePhaseIdle, string(channel), currentVersion, latestVersion, result.Message)
		return result, nil
	}

	compare, err := o.compareVersions(latestVersion, currentVersion)
	if err != nil {
		o.persistFailure(string(channel), currentVersion, latestVersion, "Update check failed.", err)
		return UpdateCheckResult{}, err
	}

	status := "up_to_date"
	message := "You are on the latest version."
	var release *UpdateRelease
	if compare > 0 {
		status = "update_available"
		message = fmt.Sprintf("Update available: %s", latestVersion)
		release = &manifest.Latest
	}

	result := UpdateCheckResult{
		Status:         status,
		Channel:        string(channel),
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        message,
		Release:        release,
	}
	o.persistPhase(updateStatePhaseIdle, string(channel), currentVersion, latestVersion, message)

	return result, nil
}

func (o updateOrchestrator) StartUpdate(input UpdateStartRequest) (UpdateStartResult, error) {
	if o.goos != "darwin" {
		return UpdateStartResult{}, errors.New("in-app updates are currently supported on macOS only")
	}

	check, err := o.CheckForUpdates(UpdateCheckRequest{Channel: input.Channel})
	if err != nil {
		return UpdateStartResult{}, err
	}
	if check.Status != "update_available" || check.Release == nil {
		state, stateErr := o.getState()
		if stateErr != nil {
			state = UpdateState{Phase: updateStatePhaseIdle}
		}
		return UpdateStartResult{Started: false, State: state}, nil
	}

	channel := check.Channel
	currentVersion := check.CurrentVersion
	latestVersion := check.LatestVersion

	pkgSelection, err := o.selectPackage(*check.Release)
	if err != nil {
		o.persistFailure(channel, currentVersion, latestVersion, "Update package metadata is invalid.", err)
		return UpdateStartResult{}, err
	}

	o.persistPhase(updateStatePhaseDownload, channel, currentVersion, latestVersion, "Downloading update...")

	zipPath, stageRoot, err := o.downloadAsset(pkgSelection.AssetURL)
	if err != nil {
		o.persistFailure(channel, currentVersion, latestVersion, "Failed to download update.", err)
		return UpdateStartResult{}, err
	}
	cleanupStageRoot := stageRoot
	defer func() {
		if cleanupStageRoot != "" {
			_ = os.RemoveAll(cleanupStageRoot)
		}
	}()

	o.persistPhase(updateStatePhaseValidate, channel, currentVersion, latestVersion, "Validating update package...")

	if err := o.verifySHA256(zipPath, pkgSelection.AssetSHA256); err != nil {
		o.persistFailure(channel, currentVersion, latestVersion, "Downloaded package failed validation.", err)
		return UpdateStartResult{}, err
	}

	appBundlePath, err := o.extractAppBundle(zipPath, stageRoot)
	if err != nil {
		o.persistFailure(channel, currentVersion, latestVersion, "Unable to unpack update.", err)
		return UpdateStartResult{}, err
	}

	if err := o.verifyCodesign(appBundlePath, pkgSelection.SigningTeamID); err != nil {
		o.persistFailure(channel, currentVersion, latestVersion, "Downloaded app signature is invalid.", err)
		return UpdateStartResult{}, err
	}

	targetApp, err := o.currentBundlePath()
	if err != nil {
		o.persistFailure(channel, currentVersion, latestVersion, "Could not determine installation path.", err)
		return UpdateStartResult{}, err
	}

	helperPath, err := o.updaterHelperPath(targetApp)
	if err != nil {
		o.persistFailure(channel, currentVersion, latestVersion, "Updater helper is missing.", err)
		return UpdateStartResult{}, err
	}

	statePath, err := o.updateStatePath()
	if err != nil {
		o.persistFailure(channel, currentVersion, latestVersion, "Could not prepare updater state.", err)
		return UpdateStartResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		o.persistFailure(channel, currentVersion, latestVersion, "Could not prepare updater state.", err)
		return UpdateStartResult{}, err
	}

	cmd := exec.Command(
		helperPath,
		"--parent-pid", strconv.Itoa(o.processID()),
		"--staged-app", appBundlePath,
		"--target-app", targetApp,
		"--stage-root", stageRoot,
		"--state-file", statePath,
		"--channel", channel,
		"--current-version", currentVersion,
		"--latest-version", latestVersion,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := o.startCommand(cmd); err != nil {
		o.persistFailure(channel, currentVersion, latestVersion, "Failed to launch updater helper.", err)
		return UpdateStartResult{}, err
	}

	cleanupStageRoot = ""

	state := UpdateState{
		Phase:          updateStatePhaseApply,
		Channel:        channel,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        "Applying update. The app will restart shortly.",
		CheckedAt:      o.nowRFC3339(),
	}
	_ = o.persistState(state)
	if o.quitAfterStart != nil {
		o.quitAfterStart()
	}

	return UpdateStartResult{Started: true, State: state}, nil
}

func (o updateOrchestrator) persistPhase(phase, channel, currentVersion, latestVersion, message string) {
	_ = o.persistState(UpdateState{
		Phase:          phase,
		Channel:        channel,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        message,
		CheckedAt:      o.nowRFC3339(),
	})
}

func (o updateOrchestrator) persistFailure(channel, currentVersion, latestVersion, message string, err error) {
	if err == nil {
		return
	}
	_ = o.persistState(UpdateState{
		Phase:          updateStatePhaseFailed,
		Channel:        channel,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Error:          err.Error(),
		Message:        message,
		CheckedAt:      o.nowRFC3339(),
	})
}

func (o updateOrchestrator) nowRFC3339() string {
	return o.nowUTC().Format(time.RFC3339)
}
