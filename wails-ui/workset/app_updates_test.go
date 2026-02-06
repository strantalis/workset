package main

import (
	"archive/zip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		left    string
		right   string
		want    int
		wantErr bool
	}{
		{name: "newer stable", left: "v1.2.0", right: "v1.1.9", want: 1},
		{name: "equal stable", left: "v1.2.0", right: "v1.2.0", want: 0},
		{name: "alpha lower than stable", left: "v1.2.0-alpha.2", right: "v1.2.0", want: -1},
		{name: "alpha ordinal compare", left: "v1.2.0-alpha.10", right: "v1.2.0-alpha.2", want: 1},
		{name: "invalid", left: "1.2", right: "v1.2.0", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := compareVersions(tc.left, tc.right)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q vs %q", tc.left, tc.right)
				}
				return
			}
			if err != nil {
				t.Fatalf("compareVersions returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("compareVersions(%q, %q) = %d, want %d", tc.left, tc.right, got, tc.want)
			}
		})
	}
}

func TestNormalizeUpdateChannel(t *testing.T) {
	t.Parallel()
	if got := normalizeUpdateChannel("stable"); got != UpdateChannelStable {
		t.Fatalf("stable channel mismatch: %q", got)
	}
	if got := normalizeUpdateChannel("ALPHA"); got != UpdateChannelAlpha {
		t.Fatalf("alpha channel mismatch: %q", got)
	}
	if got := normalizeUpdateChannel("unknown"); got != "" {
		t.Fatalf("expected empty channel for unknown, got %q", got)
	}
}

func TestSafeExtractTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}

	target, err := safeExtractTarget(absRoot, "workset.app/Contents/MacOS/workset")
	if err != nil {
		t.Fatalf("safeExtractTarget returned error: %v", err)
	}
	if !strings.HasPrefix(target, absRoot+string(filepath.Separator)) {
		t.Fatalf("target %q escaped root %q", target, absRoot)
	}

	if _, err := safeExtractTarget(absRoot, "../../etc/passwd"); err == nil {
		t.Fatalf("expected traversal path to fail")
	}
}

func TestSanitizeSymlinkTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	linkPath := filepath.Join(absRoot, "workset.app", "Contents", "Frameworks", "Current")

	target, err := sanitizeSymlinkTarget(absRoot, linkPath, "../MacOS/workset")
	if err != nil {
		t.Fatalf("sanitizeSymlinkTarget returned error: %v", err)
	}
	if target != "../MacOS/workset" {
		t.Fatalf("unexpected symlink target %q", target)
	}

	if _, err := sanitizeSymlinkTarget(absRoot, linkPath, "/tmp/outside"); err == nil {
		t.Fatalf("expected absolute symlink target to fail")
	}
	if _, err := sanitizeSymlinkTarget(absRoot, linkPath, "../../../../outside"); err == nil {
		t.Fatalf("expected escaping symlink target to fail")
	}
}

func TestExtractAppBundleHandlesSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on windows CI images")
	}

	zipPath := filepath.Join(t.TempDir(), "update.zip")
	if err := writeTestZip(zipPath, []zipEntry{
		{name: "workset.app/Contents/MacOS/workset", data: []byte("binary"), mode: 0o755},
		{
			name:       "workset.app/Contents/Frameworks/Current",
			data:       []byte("../MacOS/workset"),
			mode:       0o777,
			isSymlink:  true,
			compressed: true,
		},
	}); err != nil {
		t.Fatalf("writeTestZip: %v", err)
	}

	stage := t.TempDir()
	bundlePath, err := extractAppBundle(zipPath, stage)
	if err != nil {
		t.Fatalf("extractAppBundle returned error: %v", err)
	}
	if !strings.HasSuffix(bundlePath, ".app") {
		t.Fatalf("expected .app bundle path, got %q", bundlePath)
	}

	linkPath := filepath.Join(bundlePath, "Contents", "Frameworks", "Current")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink mode at %s, got %v", linkPath, info.Mode())
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != "../MacOS/workset" {
		t.Fatalf("unexpected symlink target: %q", target)
	}
}

func TestExtractAppBundleRejectsEscapingSymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on windows CI images")
	}

	zipPath := filepath.Join(t.TempDir(), "bad.zip")
	if err := writeTestZip(zipPath, []zipEntry{
		{name: "workset.app/Contents/MacOS/workset", data: []byte("binary"), mode: 0o755},
		{
			name:       "workset.app/Contents/Frameworks/Current",
			data:       []byte("../../../../tmp/evil"),
			mode:       0o777,
			isSymlink:  true,
			compressed: true,
		},
	}); err != nil {
		t.Fatalf("writeTestZip: %v", err)
	}

	_, err := extractAppBundle(zipPath, t.TempDir())
	if err == nil {
		t.Fatalf("expected escaping symlink target to fail")
	}
	if !strings.Contains(err.Error(), "escapes extraction root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchUpdateManifestValidation(t *testing.T) {
	originalVersion := appVersion
	t.Cleanup(func() {
		appVersion = originalVersion
	})
	appVersion = "1.0.0"

	tests := []struct {
		name        string
		manifest    map[string]any
		wantErrPart string
	}{
		{
			name: "unsupported schema",
			manifest: map[string]any{
				"schemaVersion": 2,
				"channel":       "stable",
				"latest": map[string]any{
					"version": "v1.0.1",
					"asset": map[string]any{
						"url":    "https://example.com/workset.zip",
						"sha256": strings.Repeat("a", 64),
					},
					"signing": map[string]any{"teamId": "ABCDE12345"},
				},
			},
			wantErrPart: "unsupported manifest schema version",
		},
		{
			name: "missing checksum",
			manifest: map[string]any{
				"schemaVersion": 1,
				"channel":       "stable",
				"latest": map[string]any{
					"version": "v1.0.1",
					"asset": map[string]any{
						"url": "https://example.com/workset.zip",
					},
					"signing": map[string]any{"teamId": "ABCDE12345"},
				},
			},
			wantErrPart: "latest.asset.sha256 is required",
		},
		{
			name: "minimum version too high",
			manifest: map[string]any{
				"schemaVersion": 1,
				"channel":       "stable",
				"latest": map[string]any{
					"version":        "v1.0.1",
					"minimumVersion": "v9.0.0",
					"asset": map[string]any{
						"url":    "https://example.com/workset.zip",
						"sha256": strings.Repeat("b", 64),
					},
					"signing": map[string]any{"teamId": "ABCDE12345"},
				},
			},
			wantErrPart: "below minimum update version",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.manifest)
			if err != nil {
				t.Fatalf("Marshal manifest: %v", err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/stable.json" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(payload)
			}))
			defer server.Close()

			t.Setenv("WORKSET_UPDATES_ALLOW_INSECURE", "1")
			t.Setenv("WORKSET_UPDATES_BASE_URL", server.URL)
			a := &App{}
			_, err = a.fetchUpdateManifest(UpdateChannelStable)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErrPart)
			}
			if !strings.Contains(err.Error(), tc.wantErrPart) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErrPart, err)
			}
		})
	}
}

func TestValidateUpdateURL(t *testing.T) {
	if _, err := validateUpdateURL("https://example.com/update.json"); err != nil {
		t.Fatalf("expected https URL to be accepted: %v", err)
	}
	if _, err := validateUpdateURL("http://example.com/update.json"); err == nil {
		t.Fatalf("expected non-https URL to be rejected")
	}

	t.Setenv("WORKSET_UPDATES_ALLOW_INSECURE", "true")
	if _, err := validateUpdateURL("http://localhost/update.json"); err != nil {
		t.Fatalf("expected localhost http URL when insecure override enabled: %v", err)
	}
	if _, err := validateUpdateURL("http://127.0.0.1/update.json"); err != nil {
		t.Fatalf("expected loopback ip http URL when insecure override enabled: %v", err)
	}
	if _, err := validateUpdateURL("http://example.com/update.json"); err == nil {
		t.Fatalf("expected non-loopback http URL to remain rejected")
	}
}

func TestFetchUpdateManifestRejectsInsecureBaseURL(t *testing.T) {
	t.Setenv("WORKSET_UPDATES_BASE_URL", "http://example.com/updates")
	a := &App{}
	_, err := a.fetchUpdateManifest(UpdateChannelStable)
	if err == nil {
		t.Fatalf("expected insecure manifest base URL to be rejected")
	}
	if !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadUpdateAssetRejectsInsecureURL(t *testing.T) {
	t.Parallel()

	a := &App{}
	_, _, err := a.downloadUpdateAsset("http://example.com/workset-update.zip")
	if err == nil {
		t.Fatalf("expected insecure asset URL to be rejected")
	}
	if !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdaterHelperPathRequiresBundledOrExplicitPath(t *testing.T) {
	bundlePath := t.TempDir()
	if _, err := updaterHelperPath(bundlePath); err == nil {
		t.Fatalf("expected missing helper error")
	}

	t.Setenv("WORKSET_UPDATER_HELPER_PATH", "build/updater/workset-updater")
	if _, err := updaterHelperPath(bundlePath); err == nil || !strings.Contains(err.Error(), "absolute path") {
		t.Fatalf("expected relative override to be rejected, got %v", err)
	}

	t.Setenv("WORKSET_UPDATER_HELPER_PATH", "")
	cwd := t.TempDir()
	cwdHelper := filepath.Join(cwd, "build", "updater", "workset-updater")
	if err := os.MkdirAll(filepath.Dir(cwdHelper), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(cwdHelper, []byte("helper"), 0o755); err != nil {
		t.Fatalf("WriteFile cwd helper: %v", err)
	}
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})
	if _, err := updaterHelperPath(bundlePath); err == nil {
		t.Fatalf("expected cwd helper fallback to be rejected")
	}

	helper := filepath.Join(t.TempDir(), "workset-updater")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("WriteFile helper: %v", err)
	}
	t.Setenv("WORKSET_UPDATER_HELPER_PATH", helper)

	path, err := updaterHelperPath(bundlePath)
	if err != nil {
		t.Fatalf("updaterHelperPath returned error: %v", err)
	}
	if path != helper {
		t.Fatalf("expected helper path %q, got %q", helper, path)
	}
}

func TestStartUpdateUnsupportedOS(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("unsupported OS branch is not reachable on darwin")
	}
	a := &App{}
	_, err := a.StartUpdate(UpdateStartRequest{})
	if err == nil {
		t.Fatalf("expected StartUpdate to fail on non-darwin")
	}
	if !strings.Contains(err.Error(), "macOS only") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type zipEntry struct {
	name       string
	data       []byte
	mode       os.FileMode
	isSymlink  bool
	compressed bool
}

func writeTestZip(path string, entries []zipEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{
			Name: entry.name,
		}
		if entry.compressed {
			header.Method = zip.Deflate
		} else {
			header.Method = zip.Store
		}
		mode := entry.mode
		if entry.isSymlink {
			mode |= os.ModeSymlink
		}
		header.SetMode(mode)
		writer, err := zw.CreateHeader(header)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if len(entry.data) == 0 {
			continue
		}
		if _, err := writer.Write(entry.data); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}
