package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (a *App) updatesBaseURL() string {
	if custom := strings.TrimSpace(os.Getenv("WORKSET_UPDATES_BASE_URL")); custom != "" {
		return strings.TrimRight(custom, "/")
	}
	return "https://strantalis.github.io/workset/updates"
}

func (a *App) fetchUpdateManifest(channel UpdateChannel) (UpdateManifest, error) {
	manifestURL := fmt.Sprintf("%s/%s.json", a.updatesBaseURL(), channel)
	validManifestURL, err := validateUpdateURL(manifestURL)
	if err != nil {
		return UpdateManifest{}, err
	}
	req, err := http.NewRequest(http.MethodGet, validManifestURL.String(), nil)
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
	if _, err := validateUpdateURL(manifest.Latest.Asset.URL); err != nil {
		return UpdateManifest{}, fmt.Errorf("manifest latest.asset.url is invalid: %w", err)
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
	parsed, err := validateUpdateURL(rawURL)
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

func validateUpdateURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("update URL is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "https" {
		return parsed, nil
	}
	if parsed.Scheme == "http" && updatesAllowInsecureHTTP() && isLoopbackHost(parsed.Hostname()) {
		return parsed, nil
	}
	return nil, fmt.Errorf("update URL must use https: %s", trimmed)
}

func updatesAllowInsecureHTTP() bool {
	allowRaw := strings.TrimSpace(os.Getenv("WORKSET_UPDATES_ALLOW_INSECURE"))
	if allowRaw == "" {
		return false
	}
	allow, err := strconv.ParseBool(allowRaw)
	if err != nil {
		return false
	}
	return allow
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
