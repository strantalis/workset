package main

import (
	"os"
	"path/filepath"
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
