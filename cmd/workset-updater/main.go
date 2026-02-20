package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	stateVersion          = 1
	statePhaseIdle        = "idle"
	statePhaseApply       = "applying"
	statePhaseFinalize    = "finalizing"
	statePhaseFailed      = "failed"
	parentWaitMaxDuration = 45 * time.Second
	updateModeApply       = "apply"
	updateModeFinalize    = "finalize"
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

type updatedFile struct {
	relativePath string
	targetPath   string
	backupPath   string
	hadOriginal  bool
}

func main() {
	var mode string
	var parentPID int
	var stagedApp string
	var targetApp string
	var stageRoot string
	var stateFile string
	var channel string
	var currentVersion string
	var latestVersion string

	flag.StringVar(&mode, "mode", updateModeApply, "Update execution mode: apply or finalize")
	flag.IntVar(&parentPID, "parent-pid", 0, "PID of app process to wait for")
	flag.StringVar(&stagedApp, "staged-app", "", "Path to staged .app bundle")
	flag.StringVar(&targetApp, "target-app", "", "Path to target .app bundle")
	flag.StringVar(&stageRoot, "stage-root", "", "Path to downloaded/extracted update staging directory")
	flag.StringVar(&stateFile, "state-file", "", "Path to write update status JSON")
	flag.StringVar(&channel, "channel", "stable", "Update channel")
	flag.StringVar(&currentVersion, "current-version", "", "Current app version")
	flag.StringVar(&latestVersion, "latest-version", "", "Target app version")
	flag.Parse()

	mode = strings.ToLower(strings.TrimSpace(mode))
	if err := run(mode, parentPID, stagedApp, targetApp, stageRoot, stateFile, channel, currentVersion, latestVersion); err != nil {
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

func run(mode string, parentPID int, stagedApp, targetApp, stageRoot, stateFile, channel, currentVersion, latestVersion string) error {
	switch mode {
	case "", updateModeApply:
		return runApply(parentPID, stagedApp, targetApp, stageRoot, stateFile, channel, currentVersion, latestVersion)
	case updateModeFinalize:
		return runFinalize(parentPID, stagedApp, targetApp, stageRoot, stateFile, channel, currentVersion, latestVersion)
	default:
		return fmt.Errorf("unsupported mode %q: expected %q or %q", mode, updateModeApply, updateModeFinalize)
	}
}

func runApply(parentPID int, stagedApp, targetApp, stageRoot, stateFile, channel, currentVersion, latestVersion string) error {
	stagedApp = strings.TrimSpace(stagedApp)
	targetApp = strings.TrimSpace(targetApp)
	stageRoot = strings.TrimSpace(stageRoot)
	stateFile = strings.TrimSpace(stateFile)
	if err := validateRunInputs(parentPID, stagedApp, targetApp, stateFile); err != nil {
		return err
	}
	cleanupStageRootOnExit := true
	defer func() {
		if cleanupStageRootOnExit {
			cleanupStageRoot(stageRoot)
		}
	}()

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

	backupRoot, err := os.MkdirTemp("", "workset-update-file-backup-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(backupRoot)
	}()

	updatedFiles, err := updateBundleExecutablesInPlace(stagedApp, targetApp, backupRoot)
	if err != nil {
		return err
	}

	// Best-effort quarantine cleanup.
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", targetApp).Run()

	if err := launchApp(targetApp); err != nil {
		restoreErr := rollbackUpdatedFiles(updatedFiles)
		if restoreErr != nil {
			return errors.Join(
				fmt.Errorf("failed to relaunch app: %w", err),
				fmt.Errorf("rollback failed: %w", restoreErr),
			)
		}
		return err
	}

	if hasStagedHelperBinary(stagedApp) {
		finalizerPath, err := prepareFinalizeBinary(stageRoot)
		if err != nil {
			return fmt.Errorf("failed to prepare finalize helper: %w", err)
		}
		finalizeCmd := exec.Command(
			finalizerPath,
			"--mode", updateModeFinalize,
			"--parent-pid", strconv.Itoa(os.Getpid()),
			"--staged-app", stagedApp,
			"--target-app", targetApp,
			"--stage-root", stageRoot,
			"--state-file", stateFile,
			"--channel", channel,
			"--current-version", currentVersion,
			"--latest-version", latestVersion,
		)
		finalizeCmd.Stdout = os.Stdout
		finalizeCmd.Stderr = os.Stderr
		if err := finalizeCmd.Start(); err != nil {
			return fmt.Errorf("failed to start finalize helper: %w", err)
		}
		cleanupStageRootOnExit = false
		return writeState(stateFile, updateState{
			Phase:          statePhaseFinalize,
			Channel:        channel,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Message:        "Finalizing update...",
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		})
	}

	return writeState(stateFile, updateState{
		Phase:          statePhaseIdle,
		Channel:        channel,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        "Update installed. Relaunching.",
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	})
}

func runFinalize(parentPID int, stagedApp, targetApp, stageRoot, stateFile, channel, currentVersion, latestVersion string) error {
	stagedApp = strings.TrimSpace(stagedApp)
	targetApp = strings.TrimSpace(targetApp)
	stageRoot = strings.TrimSpace(stageRoot)
	stateFile = strings.TrimSpace(stateFile)
	if err := validateRunInputs(parentPID, stagedApp, targetApp, stateFile); err != nil {
		return err
	}
	if !hasStagedHelperBinary(stagedApp) {
		return errors.New("staged app is missing updater helper binary")
	}

	_ = writeState(stateFile, updateState{
		Phase:          statePhaseFinalize,
		Channel:        channel,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        "Finalizing update...",
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	})

	waitForParent(parentPID)

	backupRoot, err := os.MkdirTemp("", "workset-update-helper-backup-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(backupRoot)
	}()

	helperRelativePath := filepath.Join("Contents", "Resources", "workset-updater")
	stagedHelperPath := filepath.Join(stagedApp, helperRelativePath)
	targetHelperPath := filepath.Join(targetApp, helperRelativePath)
	backupPath := filepath.Join(backupRoot, helperRelativePath)
	updated, err := replaceTargetFile(stagedHelperPath, targetHelperPath, backupPath, helperRelativePath)
	if err != nil {
		return err
	}

	// Best-effort quarantine cleanup and stage cleanup.
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", targetApp).Run()
	cleanupStageRoot(stageRoot)

	if err := writeState(stateFile, updateState{
		Phase:          statePhaseIdle,
		Channel:        channel,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        "Update installed. Relaunching.",
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		rollbackErr := rollbackUpdatedFiles([]updatedFile{updated})
		if rollbackErr != nil {
			return errors.Join(
				fmt.Errorf("failed to persist finalize state: %w", err),
				fmt.Errorf("rollback failed: %w", rollbackErr),
			)
		}
		return err
	}

	return nil
}

func validateRunInputs(parentPID int, stagedApp, targetApp, stateFile string) error {
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
	return nil
}

func hasStagedHelperBinary(stagedApp string) bool {
	helperPath := filepath.Join(stagedApp, "Contents", "Resources", "workset-updater")
	info, err := os.Stat(helperPath)
	return err == nil && info.Mode().IsRegular()
}

func prepareFinalizeBinary(stageRoot string) (string, error) {
	stageRoot = strings.TrimSpace(stageRoot)
	if stageRoot == "" {
		return "", errors.New("stage-root is required for finalize helper")
	}
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	finalizePath := filepath.Join(stageRoot, ".finalize", "workset-updater-finalize")
	if err := copyWithDitto(executablePath, finalizePath); err != nil {
		return "", err
	}
	if err := os.Chmod(finalizePath, 0o755); err != nil {
		return "", err
	}
	return finalizePath, nil
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

func copyWithDitto(sourcePath, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("ditto", "--noqtn", sourcePath, targetPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ditto failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func updateBundleExecutablesInPlace(stagedApp, targetApp, backupRoot string) ([]updatedFile, error) {
	relativePaths, err := stagedUpdateBinaryRelativePaths(stagedApp)
	if err != nil {
		return nil, err
	}
	updatedFiles := make([]updatedFile, 0, len(relativePaths))
	for _, relativePath := range relativePaths {
		stagedPath := filepath.Join(stagedApp, relativePath)
		targetPath := filepath.Join(targetApp, relativePath)
		backupPath := filepath.Join(backupRoot, relativePath)
		updated, err := replaceTargetFile(stagedPath, targetPath, backupPath, relativePath)
		if err != nil {
			rollbackErr := rollbackUpdatedFiles(updatedFiles)
			if rollbackErr != nil {
				return nil, errors.Join(
					fmt.Errorf("failed to apply %s: %w", relativePath, err),
					fmt.Errorf("rollback failed: %w", rollbackErr),
				)
			}
			return nil, fmt.Errorf("failed to apply %s: %w", relativePath, err)
		}
		updatedFiles = append(updatedFiles, updated)
	}
	return updatedFiles, nil
}

func stagedExecutableRelativePaths(stagedApp string) ([]string, error) {
	macosDir := filepath.Join(stagedApp, "Contents", "MacOS")
	entries, err := os.ReadDir(macosDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read staged app executables: %w", err)
	}
	paths := make([]string, 0, len(entries)+1)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("failed to stat staged executable %s: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		paths = append(paths, filepath.Join("Contents", "MacOS", entry.Name()))
	}
	if len(paths) == 0 {
		return nil, errors.New("staged app has no executable files in Contents/MacOS")
	}
	sort.Strings(paths)
	return paths, nil
}

func stagedUpdateBinaryRelativePaths(stagedApp string) ([]string, error) {
	paths, err := stagedExecutableRelativePaths(stagedApp)
	if err != nil {
		return nil, err
	}

	// Keep sessiond in sync with the main app binary while skipping the updater helper.
	sessiondRelativePath := filepath.Join("Contents", "Resources", "workset-sessiond")
	sessiondPath := filepath.Join(stagedApp, sessiondRelativePath)
	if info, err := os.Stat(sessiondPath); err == nil && info.Mode().IsRegular() {
		paths = append(paths, sessiondRelativePath)
	}

	sort.Strings(paths)
	return paths, nil
}

func replaceTargetFile(stagedPath, targetPath, backupPath, relativePath string) (updatedFile, error) {
	stagedInfo, err := os.Stat(stagedPath)
	if err != nil {
		return updatedFile{}, fmt.Errorf("failed to stat staged file %s: %w", relativePath, err)
	}
	if !stagedInfo.Mode().IsRegular() {
		return updatedFile{}, fmt.Errorf("staged path is not a regular file: %s", relativePath)
	}

	updated := updatedFile{
		relativePath: relativePath,
		targetPath:   targetPath,
		backupPath:   backupPath,
	}
	if targetInfo, err := os.Stat(targetPath); err == nil && targetInfo.Mode().IsRegular() {
		updated.hadOriginal = true
		if err := copyWithDitto(targetPath, backupPath); err != nil {
			return updatedFile{}, fmt.Errorf("failed to backup target file %s: %w", relativePath, err)
		}
	}

	tempPath := targetPath + ".workset-update-tmp"
	_ = os.Remove(tempPath)
	if err := copyWithDitto(stagedPath, tempPath); err != nil {
		return updatedFile{}, fmt.Errorf("failed to stage updated file %s: %w", relativePath, err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return updatedFile{}, fmt.Errorf("failed to replace target file %s: %w", relativePath, err)
	}
	return updated, nil
}

func rollbackUpdatedFiles(updatedFiles []updatedFile) error {
	var errs []string
	for i := len(updatedFiles) - 1; i >= 0; i-- {
		updated := updatedFiles[i]
		if updated.hadOriginal {
			if err := copyWithDitto(updated.backupPath, updated.targetPath); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", updated.relativePath, err))
			}
			continue
		}
		if err := os.Remove(updated.targetPath); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Sprintf("%s: %v", updated.relativePath, err))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
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
