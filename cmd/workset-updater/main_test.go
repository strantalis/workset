package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIsSafeStageRoot(t *testing.T) {
	t.Parallel()

	valid := filepath.Join(t.TempDir(), "workset-update-valid")
	if err := os.MkdirAll(valid, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if !isSafeStageRoot(valid) {
		t.Fatalf("expected %q to be a safe stage root", valid)
	}

	outsideTemp := filepath.Join("/tmp", "not-workset-update")
	if isSafeStageRoot(outsideTemp) {
		t.Fatalf("expected %q to be an unsafe stage root", outsideTemp)
	}

	unsafe := t.TempDir()
	if isSafeStageRoot(unsafe) {
		t.Fatalf("expected %q to be an unsafe stage root", unsafe)
	}
}

func TestCleanupStageRoot(t *testing.T) {
	t.Parallel()

	stageRoot := filepath.Join(t.TempDir(), "workset-update-cleanup")
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	stageFile := filepath.Join(stageRoot, "payload")
	if err := os.WriteFile(stageFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cleanupStageRoot(stageRoot)
	if _, err := os.Stat(stageRoot); !os.IsNotExist(err) {
		t.Fatalf("expected stage root to be removed, stat err=%v", err)
	}
}

func TestCleanupStageRootSkipsUnsafePath(t *testing.T) {
	t.Parallel()

	unsafe := t.TempDir()
	marker := filepath.Join(unsafe, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cleanupStageRoot(unsafe)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("expected unsafe path to be left intact, stat err=%v", err)
	}
}

func TestStagedExecutableRelativePaths(t *testing.T) {
	t.Parallel()

	stagedApp := filepath.Join(t.TempDir(), "workset.app")
	macosDir := filepath.Join(stagedApp, "Contents", "MacOS")
	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macosDir, "workset"), []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile workset: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macosDir, "helper"), []byte("helper"), 0o755); err != nil {
		t.Fatalf("WriteFile helper: %v", err)
	}
	if err := os.Mkdir(filepath.Join(macosDir, "nested"), 0o755); err != nil {
		t.Fatalf("Mkdir nested: %v", err)
	}

	got, err := stagedExecutableRelativePaths(stagedApp)
	if err != nil {
		t.Fatalf("stagedExecutableRelativePaths returned error: %v", err)
	}
	want := []string{
		filepath.Join("Contents", "MacOS", "helper"),
		filepath.Join("Contents", "MacOS", "workset"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected executable paths: got %v want %v", got, want)
	}
}

func TestStagedExecutableRelativePathsNoExecutables(t *testing.T) {
	t.Parallel()

	stagedApp := filepath.Join(t.TempDir(), "workset.app")
	macosDir := filepath.Join(stagedApp, "Contents", "MacOS")
	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Mkdir(filepath.Join(macosDir, "empty-dir"), 0o755); err != nil {
		t.Fatalf("Mkdir empty-dir: %v", err)
	}

	_, err := stagedExecutableRelativePaths(stagedApp)
	if err == nil {
		t.Fatalf("expected missing executables error")
	}
}

func TestStagedUpdateBinaryRelativePathsIncludesSessiond(t *testing.T) {
	t.Parallel()

	stagedApp := filepath.Join(t.TempDir(), "workset.app")
	macosDir := filepath.Join(stagedApp, "Contents", "MacOS")
	resourcesDir := filepath.Join(stagedApp, "Contents", "Resources")
	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		t.Fatalf("MkdirAll MacOS: %v", err)
	}
	if err := os.MkdirAll(resourcesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll Resources: %v", err)
	}

	if err := os.WriteFile(filepath.Join(macosDir, "workset"), []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile workset: %v", err)
	}
	if err := os.WriteFile(filepath.Join(resourcesDir, "workset-sessiond"), []byte("sessiond"), 0o755); err != nil {
		t.Fatalf("WriteFile workset-sessiond: %v", err)
	}
	if err := os.WriteFile(filepath.Join(resourcesDir, "workset-updater"), []byte("updater"), 0o755); err != nil {
		t.Fatalf("WriteFile workset-updater: %v", err)
	}

	got, err := stagedUpdateBinaryRelativePaths(stagedApp)
	if err != nil {
		t.Fatalf("stagedUpdateBinaryRelativePaths returned error: %v", err)
	}
	want := []string{
		filepath.Join("Contents", "MacOS", "workset"),
		filepath.Join("Contents", "Resources", "workset-sessiond"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected update binary paths: got %v want %v", got, want)
	}
}

func TestHasStagedHelperBinary(t *testing.T) {
	t.Parallel()

	stagedApp := filepath.Join(t.TempDir(), "workset.app")
	resourcesDir := filepath.Join(stagedApp, "Contents", "Resources")
	if err := os.MkdirAll(resourcesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll Resources: %v", err)
	}

	if hasStagedHelperBinary(stagedApp) {
		t.Fatalf("expected helper to be missing")
	}

	helperPath := filepath.Join(resourcesDir, "workset-updater")
	if err := os.WriteFile(helperPath, []byte("helper"), 0o755); err != nil {
		t.Fatalf("WriteFile helper: %v", err)
	}
	if !hasStagedHelperBinary(stagedApp) {
		t.Fatalf("expected helper to be detected")
	}
}

func TestPrepareFinalizeBinary(t *testing.T) {
	t.Parallel()

	stageRoot := filepath.Join(t.TempDir(), "workset-update-finalize")
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll stageRoot: %v", err)
	}

	path, err := prepareFinalizeBinary(stageRoot)
	if err != nil {
		t.Fatalf("prepareFinalizeBinary returned error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat finalize binary: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("expected regular finalize binary, got mode=%v", info.Mode())
	}
}

func TestPrepareFinalizeBinaryRequiresStageRoot(t *testing.T) {
	t.Parallel()

	if _, err := prepareFinalizeBinary(""); err == nil {
		t.Fatalf("expected stage-root validation error")
	}
}
