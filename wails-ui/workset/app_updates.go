package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	updatePreferencesVersion = 1
	updateStateVersion       = 1
	updateManifestSchema     = 1
	updateStatePhaseIdle     = "idle"
	updateStatePhaseChecking = "checking"
	updateStatePhaseDownload = "downloading"
	updateStatePhaseValidate = "validating"
	updateStatePhaseApply    = "applying"
	updateStatePhaseFailed   = "failed"
)

var (
	updateStateMu     sync.Mutex
	updateVersionExpr = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z]+)\.(\d+))?$`)
	teamIDExpr        = regexp.MustCompile(`TeamIdentifier=([A-Z0-9]+)`)
)

type UpdateChannel string

const (
	UpdateChannelStable UpdateChannel = "stable"
	UpdateChannelAlpha  UpdateChannel = "alpha"
)

type UpdatePreferences struct {
	Channel   string `json:"channel"`
	AutoCheck bool   `json:"autoCheck"`
}

type UpdatePreferencesInput struct {
	Channel   string `json:"channel"`
	AutoCheck *bool  `json:"autoCheck,omitempty"`
}

type UpdateCheckRequest struct {
	Channel string `json:"channel"`
}

type UpdateStartRequest struct {
	Channel string `json:"channel"`
}

type UpdateState struct {
	Phase          string `json:"phase"`
	Channel        string `json:"channel"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Message        string `json:"message"`
	Error          string `json:"error"`
	CheckedAt      string `json:"checkedAt"`
}

type UpdateManifest struct {
	SchemaVersion int           `json:"schemaVersion"`
	GeneratedAt   string        `json:"generatedAt"`
	Channel       string        `json:"channel"`
	Disabled      bool          `json:"disabled"`
	Message       string        `json:"message"`
	Latest        UpdateRelease `json:"latest"`
}

type UpdateRelease struct {
	Version        string               `json:"version"`
	PubDate        string               `json:"pubDate"`
	NotesURL       string               `json:"notesUrl"`
	MinimumVersion string               `json:"minimumVersion"`
	Asset          UpdateReleaseAsset   `json:"asset"`
	Signing        UpdateReleaseSigning `json:"signing"`
}

type UpdateReleaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type UpdateReleaseSigning struct {
	TeamID string `json:"teamId"`
}

type UpdateCheckResult struct {
	Status         string         `json:"status"`
	Channel        string         `json:"channel"`
	CurrentVersion string         `json:"currentVersion"`
	LatestVersion  string         `json:"latestVersion"`
	Message        string         `json:"message"`
	Release        *UpdateRelease `json:"release,omitempty"`
}

type UpdateStartResult struct {
	Started bool        `json:"started"`
	State   UpdateState `json:"state"`
}

type updatePreferencesFile struct {
	Version     int               `json:"version"`
	Preferences UpdatePreferences `json:"preferences"`
}

type updateStateFile struct {
	Version int         `json:"version"`
	State   UpdateState `json:"state"`
}

type parsedVersion struct {
	major       int
	minor       int
	patch       int
	preLabel    string
	preNum      int
	hasPrelabel bool
}

type queuedSymlink struct {
	path   string
	target string
}

func (a *App) GetUpdatePreferences() (UpdatePreferences, error) {
	updateStateMu.Lock()
	defer updateStateMu.Unlock()

	prefs, err := a.loadUpdatePreferencesLocked()
	if err != nil {
		return UpdatePreferences{}, err
	}
	return prefs, nil
}

func (a *App) SetUpdatePreferences(input UpdatePreferencesInput) (UpdatePreferences, error) {
	updateStateMu.Lock()
	defer updateStateMu.Unlock()

	prefs, err := a.loadUpdatePreferencesLocked()
	if err != nil {
		return UpdatePreferences{}, err
	}

	channel := normalizeUpdateChannel(input.Channel)
	if channel != "" {
		prefs.Channel = string(channel)
	}
	if input.AutoCheck != nil {
		prefs.AutoCheck = *input.AutoCheck
	}

	if err := a.persistUpdatePreferencesLocked(prefs); err != nil {
		return UpdatePreferences{}, err
	}

	return prefs, nil
}

func (a *App) CheckForUpdates(input UpdateCheckRequest) (UpdateCheckResult, error) {
	channel, err := a.resolveUpdateChannel(input.Channel)
	if err != nil {
		return UpdateCheckResult{}, err
	}

	currentVersion := normalizeVersion(a.GetAppVersion().Version)
	if currentVersion == "" {
		currentVersion = "v0.0.0"
	}

	_ = a.persistUpdateState(UpdateState{
		Phase:          updateStatePhaseChecking,
		Channel:        string(channel),
		CurrentVersion: currentVersion,
		Message:        "Checking for updates...",
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	})

	manifest, err := a.fetchUpdateManifest(channel)
	if err != nil {
		state := UpdateState{
			Phase:          updateStatePhaseFailed,
			Channel:        string(channel),
			CurrentVersion: currentVersion,
			Error:          err.Error(),
			Message:        "Update check failed.",
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		_ = a.persistUpdateState(state)
		return UpdateCheckResult{}, err
	}

	latestVersion := normalizeVersion(manifest.Latest.Version)
	if latestVersion == "" {
		err := errors.New("manifest latest version is missing")
		state := UpdateState{
			Phase:          updateStatePhaseFailed,
			Channel:        string(channel),
			CurrentVersion: currentVersion,
			Error:          err.Error(),
			Message:        "Update check failed.",
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		_ = a.persistUpdateState(state)
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
		_ = a.persistUpdateState(UpdateState{
			Phase:          updateStatePhaseIdle,
			Channel:        string(channel),
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Message:        result.Message,
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		})
		return result, nil
	}

	compare, err := compareVersions(latestVersion, currentVersion)
	if err != nil {
		state := UpdateState{
			Phase:          updateStatePhaseFailed,
			Channel:        string(channel),
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Error:          err.Error(),
			Message:        "Update check failed.",
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		_ = a.persistUpdateState(state)
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

	_ = a.persistUpdateState(UpdateState{
		Phase:          updateStatePhaseIdle,
		Channel:        string(channel),
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        message,
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	})

	return result, nil
}

func (a *App) StartUpdate(input UpdateStartRequest) (UpdateStartResult, error) {
	if runtime.GOOS != "darwin" {
		return UpdateStartResult{}, errors.New("in-app updates are currently supported on macOS only")
	}

	check, err := a.CheckForUpdates(UpdateCheckRequest{Channel: input.Channel})
	if err != nil {
		return UpdateStartResult{}, err
	}
	if check.Status != "update_available" || check.Release == nil {
		state, _ := a.GetUpdateState()
		return UpdateStartResult{Started: false, State: state}, nil
	}

	currentVersion := check.CurrentVersion
	latestVersion := check.LatestVersion
	channel := check.Channel

	_ = a.persistUpdateState(UpdateState{
		Phase:          updateStatePhaseDownload,
		Channel:        channel,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        "Downloading update...",
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	})

	zipPath, stageRoot, err := a.downloadUpdateAsset(check.Release.Asset.URL)
	if err != nil {
		state := UpdateState{
			Phase:          updateStatePhaseFailed,
			Channel:        channel,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Error:          err.Error(),
			Message:        "Failed to download update.",
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		_ = a.persistUpdateState(state)
		return UpdateStartResult{}, err
	}

	_ = a.persistUpdateState(UpdateState{
		Phase:          updateStatePhaseValidate,
		Channel:        channel,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        "Validating update package...",
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	})

	if err := verifySHA256(zipPath, check.Release.Asset.SHA256); err != nil {
		state := UpdateState{
			Phase:          updateStatePhaseFailed,
			Channel:        channel,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Error:          err.Error(),
			Message:        "Downloaded package failed validation.",
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		_ = a.persistUpdateState(state)
		return UpdateStartResult{}, err
	}

	appBundlePath, err := extractAppBundle(zipPath, stageRoot)
	if err != nil {
		state := UpdateState{
			Phase:          updateStatePhaseFailed,
			Channel:        channel,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Error:          err.Error(),
			Message:        "Unable to unpack update.",
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		_ = a.persistUpdateState(state)
		return UpdateStartResult{}, err
	}

	if err := verifyCodesign(appBundlePath, check.Release.Signing.TeamID); err != nil {
		state := UpdateState{
			Phase:          updateStatePhaseFailed,
			Channel:        channel,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Error:          err.Error(),
			Message:        "Downloaded app signature is invalid.",
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		_ = a.persistUpdateState(state)
		return UpdateStartResult{}, err
	}

	targetApp, err := currentBundlePath()
	if err != nil {
		state := UpdateState{
			Phase:          updateStatePhaseFailed,
			Channel:        channel,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Error:          err.Error(),
			Message:        "Could not determine installation path.",
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		_ = a.persistUpdateState(state)
		return UpdateStartResult{}, err
	}

	helperPath, err := updaterHelperPath(targetApp)
	if err != nil {
		state := UpdateState{
			Phase:          updateStatePhaseFailed,
			Channel:        channel,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Error:          err.Error(),
			Message:        "Updater helper is missing.",
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		_ = a.persistUpdateState(state)
		return UpdateStartResult{}, err
	}

	statePath, err := a.updateStatePath()
	if err != nil {
		return UpdateStartResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return UpdateStartResult{}, err
	}

	cmd := exec.Command(
		helperPath,
		"--parent-pid", strconv.Itoa(os.Getpid()),
		"--staged-app", appBundlePath,
		"--target-app", targetApp,
		"--state-file", statePath,
		"--channel", channel,
		"--current-version", currentVersion,
		"--latest-version", latestVersion,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		state := UpdateState{
			Phase:          updateStatePhaseFailed,
			Channel:        channel,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Error:          err.Error(),
			Message:        "Failed to launch updater helper.",
			CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		_ = a.persistUpdateState(state)
		return UpdateStartResult{}, err
	}

	state := UpdateState{
		Phase:          updateStatePhaseApply,
		Channel:        channel,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		Message:        "Applying update. The app will restart shortly.",
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	_ = a.persistUpdateState(state)

	if a.ctx != nil {
		go func(ctx context.Context) {
			time.Sleep(350 * time.Millisecond)
			wruntime.Quit(ctx)
		}(a.ctx)
	}

	return UpdateStartResult{Started: true, State: state}, nil
}

func (a *App) GetUpdateState() (UpdateState, error) {
	updateStateMu.Lock()
	defer updateStateMu.Unlock()

	state, err := a.loadUpdateStateLocked()
	if err != nil {
		return UpdateState{}, err
	}
	return state, nil
}

func (a *App) CancelUpdate() error {
	return nil
}

func (a *App) resolveUpdateChannel(raw string) (UpdateChannel, error) {
	channel := normalizeUpdateChannel(raw)
	if channel != "" {
		return channel, nil
	}
	prefs, err := a.GetUpdatePreferences()
	if err != nil {
		return UpdateChannelStable, err
	}
	channel = normalizeUpdateChannel(prefs.Channel)
	if channel == "" {
		channel = UpdateChannelStable
	}
	return channel, nil
}

func (a *App) loadUpdatePreferencesLocked() (UpdatePreferences, error) {
	defaults := UpdatePreferences{Channel: string(UpdateChannelStable), AutoCheck: true}
	path, err := a.updatePreferencesPath()
	if err != nil {
		return defaults, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaults, nil
		}
		return defaults, err
	}

	var file updatePreferencesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return defaults, err
	}

	prefs := file.Preferences
	if normalizeUpdateChannel(prefs.Channel) == "" {
		prefs.Channel = string(UpdateChannelStable)
	}
	return prefs, nil
}

func (a *App) persistUpdatePreferencesLocked(prefs UpdatePreferences) error {
	path, err := a.updatePreferencesPath()
	if err != nil {
		return err
	}
	prefs.Channel = string(normalizeUpdateChannel(prefs.Channel))
	if prefs.Channel == "" {
		prefs.Channel = string(UpdateChannelStable)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(updatePreferencesFile{
		Version:     updatePreferencesVersion,
		Preferences: prefs,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (a *App) loadUpdateStateLocked() (UpdateState, error) {
	path, err := a.updateStatePath()
	if err != nil {
		return UpdateState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return UpdateState{Phase: updateStatePhaseIdle}, nil
		}
		return UpdateState{}, err
	}
	var stateFile updateStateFile
	if err := json.Unmarshal(data, &stateFile); err != nil {
		return UpdateState{}, err
	}
	if stateFile.State.Phase == "" {
		stateFile.State.Phase = updateStatePhaseIdle
	}
	return stateFile.State, nil
}

func (a *App) persistUpdateState(state UpdateState) error {
	updateStateMu.Lock()
	defer updateStateMu.Unlock()

	path, err := a.updateStatePath()
	if err != nil {
		return err
	}
	if state.Phase == "" {
		state.Phase = updateStatePhaseIdle
	}
	if state.CheckedAt == "" {
		state.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(updateStateFile{
		Version: updateStateVersion,
		State:   state,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (a *App) updatePreferencesPath() (string, error) {
	dir, err := worksetAppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ui_update_preferences.json"), nil
}

func (a *App) updateStatePath() (string, error) {
	dir, err := worksetAppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ui_update_state.json"), nil
}

func normalizeUpdateChannel(raw string) UpdateChannel {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(UpdateChannelStable):
		return UpdateChannelStable
	case string(UpdateChannelAlpha):
		return UpdateChannelAlpha
	default:
		return ""
	}
}

func normalizeVersion(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" || v == "dev" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

func compareVersions(left, right string) (int, error) {
	lv, err := parseVersion(left)
	if err != nil {
		return 0, err
	}
	rv, err := parseVersion(right)
	if err != nil {
		return 0, err
	}
	if lv.major != rv.major {
		if lv.major > rv.major {
			return 1, nil
		}
		return -1, nil
	}
	if lv.minor != rv.minor {
		if lv.minor > rv.minor {
			return 1, nil
		}
		return -1, nil
	}
	if lv.patch != rv.patch {
		if lv.patch > rv.patch {
			return 1, nil
		}
		return -1, nil
	}
	if lv.hasPrelabel != rv.hasPrelabel {
		if !lv.hasPrelabel {
			return 1, nil
		}
		return -1, nil
	}
	if !lv.hasPrelabel {
		return 0, nil
	}
	if lv.preLabel != rv.preLabel {
		if lv.preLabel > rv.preLabel {
			return 1, nil
		}
		return -1, nil
	}
	if lv.preNum != rv.preNum {
		if lv.preNum > rv.preNum {
			return 1, nil
		}
		return -1, nil
	}
	return 0, nil
}

func parseVersion(raw string) (parsedVersion, error) {
	matches := updateVersionExpr.FindStringSubmatch(strings.TrimSpace(raw))
	if len(matches) == 0 {
		return parsedVersion{}, fmt.Errorf("invalid version format: %q", raw)
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	result := parsedVersion{
		major: major,
		minor: minor,
		patch: patch,
	}
	if matches[4] != "" {
		result.hasPrelabel = true
		result.preLabel = matches[4]
		preNum, _ := strconv.Atoi(matches[5])
		result.preNum = preNum
	}
	return result, nil
}

func (a *App) updatesBaseURL() string {
	if custom := strings.TrimSpace(os.Getenv("WORKSET_UPDATES_BASE_URL")); custom != "" {
		return strings.TrimRight(custom, "/")
	}
	return "https://strantalis.github.io/workset/updates"
}

func (a *App) fetchUpdateManifest(channel UpdateChannel) (UpdateManifest, error) {
	manifestURL := fmt.Sprintf("%s/%s.json", a.updatesBaseURL(), channel)
	req, err := http.NewRequest(http.MethodGet, manifestURL, nil)
	if err != nil {
		return UpdateManifest{}, err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return UpdateManifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UpdateManifest{}, fmt.Errorf("manifest request failed with status %d", resp.StatusCode)
	}
	var manifest UpdateManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return UpdateManifest{}, err
	}
	if manifest.SchemaVersion != updateManifestSchema {
		return UpdateManifest{}, fmt.Errorf("unsupported manifest schema version: %d", manifest.SchemaVersion)
	}
	if normalizeUpdateChannel(manifest.Channel) == "" {
		manifest.Channel = string(channel)
	}
	if strings.TrimSpace(manifest.Latest.Version) == "" {
		return UpdateManifest{}, errors.New("manifest latest.version is required")
	}
	if strings.TrimSpace(manifest.Latest.Asset.URL) == "" {
		return UpdateManifest{}, errors.New("manifest latest.asset.url is required")
	}
	if strings.TrimSpace(manifest.Latest.Asset.SHA256) == "" {
		return UpdateManifest{}, errors.New("manifest latest.asset.sha256 is required")
	}
	if strings.TrimSpace(manifest.Latest.Signing.TeamID) == "" {
		return UpdateManifest{}, errors.New("manifest latest.signing.teamId is required")
	}
	if minVersion := normalizeVersion(manifest.Latest.MinimumVersion); minVersion != "" {
		current := normalizeVersion(a.GetAppVersion().Version)
		if current != "" {
			compare, err := compareVersions(current, minVersion)
			if err != nil {
				return UpdateManifest{}, err
			}
			if compare < 0 {
				return UpdateManifest{}, fmt.Errorf("current version %s is below minimum update version %s", current, minVersion)
			}
		}
	}
	return manifest, nil
}

func (a *App) downloadUpdateAsset(rawURL string) (zipPath string, stageRoot string, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}
	filename := filepath.Base(parsed.Path)
	if filename == "" || filename == "." || filename == "/" {
		filename = "workset-update.zip"
	}

	stageRoot, err = os.MkdirTemp("", "workset-update-*")
	if err != nil {
		return "", "", err
	}
	zipPath = filepath.Join(stageRoot, filename)

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("asset download failed with status %d", resp.StatusCode)
	}
	file, err := os.Create(zipPath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", "", err
	}
	return zipPath, stageRoot, nil
}

func verifySHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return errors.New("expected checksum is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	sum := strings.ToLower(hex.EncodeToString(hash.Sum(nil)))
	if sum != expected {
		return fmt.Errorf("checksum mismatch: expected %s got %s", expected, sum)
	}
	return nil
}

func extractAppBundle(zipPath, stageRoot string) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	extractRoot := filepath.Join(stageRoot, "extracted")
	if err := os.MkdirAll(extractRoot, 0o755); err != nil {
		return "", err
	}
	absExtractRoot, err := filepath.Abs(extractRoot)
	if err != nil {
		return "", err
	}

	var symlinks []queuedSymlink
	for _, file := range reader.File {
		target, err := safeExtractTarget(absExtractRoot, file.Name)
		if err != nil {
			return "", err
		}
		mode := file.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		case mode&os.ModeSymlink != 0:
			src, err := file.Open()
			if err != nil {
				return "", err
			}
			targetBytes, readErr := io.ReadAll(src)
			closeErr := src.Close()
			if readErr != nil {
				return "", readErr
			}
			if closeErr != nil {
				return "", closeErr
			}
			symlinks = append(symlinks, queuedSymlink{
				path:   target,
				target: string(targetBytes),
			})
			continue
		case mode.IsRegular():
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			src, err := file.Open()
			if err != nil {
				return "", err
			}
			perm := mode.Perm()
			if perm == 0 {
				perm = 0o644
			}
			dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
			if err != nil {
				src.Close()
				return "", err
			}
			_, copyErr := io.Copy(dst, src)
			closeErr := dst.Close()
			srcCloseErr := src.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
			if srcCloseErr != nil {
				return "", srcCloseErr
			}
		default:
			return "", fmt.Errorf("unsupported entry type in archive: %s", file.Name)
		}
	}

	for _, link := range symlinks {
		symlinkTarget, err := sanitizeSymlinkTarget(absExtractRoot, link.path, link.target)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(link.path), 0o755); err != nil {
			return "", err
		}
		_ = os.Remove(link.path)
		if err := os.Symlink(symlinkTarget, link.path); err != nil {
			return "", err
		}
	}

	var found string
	walkErr := filepath.WalkDir(extractRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".app") {
			found = path
			return errors.New("found")
		}
		return nil
	})
	if walkErr != nil && walkErr.Error() != "found" {
		return "", walkErr
	}
	if found == "" {
		return "", errors.New("no .app bundle found in update archive")
	}
	return found, nil
}

func safeExtractTarget(absRoot, name string) (string, error) {
	if strings.ContainsRune(name, '\x00') {
		return "", fmt.Errorf("unsafe path in archive: %q", name)
	}
	cleanName := filepath.Clean(name)
	if filepath.IsAbs(cleanName) {
		return "", fmt.Errorf("unsafe absolute path in archive: %q", name)
	}
	target := filepath.Join(absRoot, cleanName)
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path in archive: %q", name)
	}
	return absTarget, nil
}

func sanitizeSymlinkTarget(absRoot, linkPath, rawTarget string) (string, error) {
	target := filepath.Clean(strings.TrimSpace(rawTarget))
	if target == "" || target == "." {
		return "", fmt.Errorf("unsafe symlink target: %q", rawTarget)
	}
	if filepath.IsAbs(target) {
		return "", fmt.Errorf("absolute symlink targets are not allowed: %q", rawTarget)
	}
	resolved := filepath.Join(filepath.Dir(linkPath), target)
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absResolved)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("symlink target escapes extraction root: %q", rawTarget)
	}
	return target, nil
}

func verifyCodesign(appPath, expectedTeamID string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("codesign validation is only supported on macOS")
	}
	expectedTeamID = strings.TrimSpace(expectedTeamID)
	if expectedTeamID == "" {
		return errors.New("missing expected signing team id")
	}
	verify := exec.Command("codesign", "--verify", "--deep", "--strict", "--verbose=2", appPath)
	if out, err := verify.CombinedOutput(); err != nil {
		return fmt.Errorf("codesign verify failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	details := exec.Command("codesign", "-dv", "--verbose=4", appPath)
	out, err := details.CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign inspect failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	matches := teamIDExpr.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return errors.New("could not read TeamIdentifier from codesign output")
	}
	teamID := strings.TrimSpace(matches[1])
	if teamID != expectedTeamID {
		return fmt.Errorf("unexpected TeamIdentifier: expected %s got %s", expectedTeamID, teamID)
	}
	return nil
}

func currentBundlePath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	marker := filepath.Join("Contents", "MacOS") + string(filepath.Separator)
	index := strings.Index(executablePath, marker)
	if index <= 0 {
		return "", errors.New("current binary is not running from a macOS app bundle")
	}
	bundlePath := executablePath[:index]
	if !strings.HasSuffix(strings.ToLower(bundlePath), ".app") {
		return "", errors.New("resolved bundle path is invalid")
	}
	return bundlePath, nil
}

func updaterHelperPath(bundlePath string) (string, error) {
	resourcePath := filepath.Join(bundlePath, "Contents", "Resources", "workset-updater")
	if info, err := os.Stat(resourcePath); err == nil && !info.IsDir() {
		return resourcePath, nil
	}
	overridePath := strings.TrimSpace(os.Getenv("WORKSET_UPDATER_HELPER_PATH"))
	if overridePath != "" {
		if !filepath.IsAbs(overridePath) {
			return "", errors.New("WORKSET_UPDATER_HELPER_PATH must be an absolute path")
		}
		if info, err := os.Stat(overridePath); err == nil && !info.IsDir() {
			return overridePath, nil
		}
		return "", fmt.Errorf("WORKSET_UPDATER_HELPER_PATH not found: %s", overridePath)
	}
	return "", errors.New("workset-updater helper executable not found in app bundle resources")
}
