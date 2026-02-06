package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	stateVersion          = 1
	statePhaseIdle        = "idle"
	statePhaseApply       = "applying"
	statePhaseFailed      = "failed"
	parentWaitMaxDuration = 45 * time.Second
)

type updateState struct {
	Phase          string `json:"phase"`
	Channel        string `json:"channel"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Message        string `json:"message"`
	Error          string `json:"error"`
	CheckedAt      string `json:"checkedAt"`
}

type updateStateFile struct {
	Version int         `json:"version"`
	State   updateState `json:"state"`
}

func main() {
	var parentPID int
	var stagedApp string
	var targetApp string
	var stageRoot string
	var stateFile string
	var channel string
	var currentVersion string
	var latestVersion string

	flag.IntVar(&parentPID, "parent-pid", 0, "PID of app process to wait for")
	flag.StringVar(&stagedApp, "staged-app", "", "Path to staged .app bundle")
	flag.StringVar(&targetApp, "target-app", "", "Path to target .app bundle")
	flag.StringVar(&stageRoot, "stage-root", "", "Path to downloaded/extracted update staging directory")
	flag.StringVar(&stateFile, "state-file", "", "Path to write update status JSON")
	flag.StringVar(&channel, "channel", "stable", "Update channel")
	flag.StringVar(&currentVersion, "current-version", "", "Current app version")
	flag.StringVar(&latestVersion, "latest-version", "", "Target app version")
	flag.Parse()

	if err := run(parentPID, stagedApp, targetApp, stageRoot, stateFile, channel, currentVersion, latestVersion); err != nil {
		_ = writeState(stateFile, updateState{
			Phase:          statePhaseFailed,
			Channel:        channel,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Message:        "Update failed.",
			Error:          err.Error(),
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		})
		fmt.Fprintf(os.Stderr, "workset-updater: %v\n", err)
		os.Exit(1)
	}
}

func run(parentPID int, stagedApp, targetApp, stageRoot, stateFile, channel, currentVersion, latestVersion string) error {
	stagedApp = strings.TrimSpace(stagedApp)
	targetApp = strings.TrimSpace(targetApp)
	stageRoot = strings.TrimSpace(stageRoot)
	stateFile = strings.TrimSpace(stateFile)
	defer cleanupStageRoot(stageRoot)
	if parentPID <= 0 {
		return errors.New("parent-pid is required")
	}
	if stagedApp == "" {
		return errors.New("staged-app is required")
	}
	if targetApp == "" {
		return errors.New("target-app is required")
	}
	if stateFile == "" {
		return errors.New("state-file is required")
	}
	if !strings.HasSuffix(strings.ToLower(stagedApp), ".app") {
		return fmt.Errorf("staged-app must be a .app bundle: %s", stagedApp)
	}
	if !strings.HasSuffix(strings.ToLower(targetApp), ".app") {
		return fmt.Errorf("target-app must be a .app bundle: %s", targetApp)
	}

	_ = writeState(stateFile, updateState{
		Phase:          statePhaseApply,
		Channel:        channel,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        "Applying update...",
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	})

	waitForParent(parentPID)

	if err := os.MkdirAll(filepath.Dir(targetApp), 0o755); err != nil {
		return err
	}

	backup := targetApp + ".previous"
	_ = os.RemoveAll(backup)

	targetExists := false
	if info, err := os.Stat(targetApp); err == nil && info.IsDir() {
		targetExists = true
		if err := os.Rename(targetApp, backup); err != nil {
			return fmt.Errorf("failed to backup target app: %w", err)
		}
	}

	if err := copyAppBundle(stagedApp, targetApp); err != nil {
		if targetExists {
			_ = os.RemoveAll(targetApp)
			_ = os.Rename(backup, targetApp)
		}
		return err
	}

	// Best-effort quarantine cleanup.
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", targetApp).Run()

	if err := launchApp(targetApp); err != nil {
		if targetExists {
			_ = os.RemoveAll(targetApp)
			_ = os.Rename(backup, targetApp)
		}
		return err
	}

	_ = os.RemoveAll(backup)

	return writeState(stateFile, updateState{
		Phase:          statePhaseIdle,
		Channel:        channel,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        "Update installed. Relaunching.",
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	})
}

func waitForParent(parentPID int) {
	deadline := time.Now().Add(parentWaitMaxDuration)
	for time.Now().Before(deadline) {
		err := syscall.Kill(parentPID, 0)
		if err != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func copyAppBundle(stagedApp, targetApp string) error {
	if err := os.RemoveAll(targetApp); err != nil {
		return err
	}
	cmd := exec.Command("ditto", "--noqtn", stagedApp, targetApp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ditto failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func launchApp(appPath string) error {
	cmd := exec.Command("open", "-n", appPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to relaunch app: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func writeState(path string, state updateState) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if state.CheckedAt == "" {
		state.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(updateStateFile{
		Version: stateVersion,
		State:   state,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	return nil
}

func cleanupStageRoot(path string) {
	if !isSafeStageRoot(path) {
		return
	}
	_ = os.RemoveAll(path)
}

func isSafeStageRoot(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	tempRoot, err := filepath.Abs(os.TempDir())
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(tempRoot, absPath)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return strings.HasPrefix(filepath.Base(absPath), "workset-update-")
}
