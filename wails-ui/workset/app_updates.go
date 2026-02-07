package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
	updateStateMu sync.Mutex
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
	return a.newUpdateOrchestrator().CheckForUpdates(input)
}

func (a *App) StartUpdate(input UpdateStartRequest) (UpdateStartResult, error) {
	return a.newUpdateOrchestrator().StartUpdate(input)
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
