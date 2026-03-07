package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestListRepoFilesIncludesTrackedAndUntracked(t *testing.T) {
	t.Parallel()

	repoPath := t.TempDir()
	runGit(t, repoPath, "init", "-q")

	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	runGit(t, repoPath, "add", "README.md")

	if err := os.WriteFile(filepath.Join(repoPath, "notes.txt"), []byte("draft\n"), 0o644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	paths, err := listRepoFiles(context.Background(), repoPath)
	if err != nil {
		t.Fatalf("listRepoFiles failed: %v", err)
	}

	if len(paths) != 2 {
		t.Fatalf("expected 2 files, got %d: %+v", len(paths), paths)
	}
	if paths[0] != "README.md" || paths[1] != "notes.txt" {
		t.Fatalf("unexpected paths: %+v", paths)
	}
}

func TestMatchesRepoFileQueryAllowsEmptyQuery(t *testing.T) {
	t.Parallel()

	if !matchesRepoFileQuery("api", "docs/README.md", "") {
		t.Fatalf("expected empty query to include file")
	}
}

func TestNormalizeRepoFileSearchLimitCapsHighValues(t *testing.T) {
	t.Parallel()

	if got := normalizeRepoFileSearchLimit(999999); got != maxRepoFileSearchLimit {
		t.Fatalf("expected cap %d, got %d", maxRepoFileSearchLimit, got)
	}
}

func TestLoadRepoFileIndexUsesCacheWithinTTL(t *testing.T) {
	app := NewApp()
	repo := workspaceRepoRef{
		id:   "thread-alpha::api",
		name: "api",
		path: t.TempDir(),
	}

	originalBuilder := buildRepoFileIndex
	defer func() {
		buildRepoFileIndex = originalBuilder
	}()

	callCount := 0
	buildRepoFileIndex = func(_ context.Context, _ workspaceRepoRef) ([]repoFileIndexItem, error) {
		callCount += 1
		return []repoFileIndexItem{{path: "README.md"}}, nil
	}

	first, err := app.loadRepoFileIndex(context.Background(), repo)
	if err != nil {
		t.Fatalf("first loadRepoFileIndex failed: %v", err)
	}
	second, err := app.loadRepoFileIndex(context.Background(), repo)
	if err != nil {
		t.Fatalf("second loadRepoFileIndex failed: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("expected builder to be called once, got %d", callCount)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected cached repo index items")
	}
}

func TestLoadRepoFileIndexRefreshesExpiredCache(t *testing.T) {
	app := NewApp()
	repo := workspaceRepoRef{
		id:   "thread-alpha::api",
		name: "api",
		path: t.TempDir(),
	}

	app.repoFileIndexes[repo.id] = repoFileIndexCacheEntry{
		loadedAt: time.Now().Add(-repoFileIndexCacheTTL - time.Second),
		items:    []repoFileIndexItem{{path: "stale.md"}},
	}

	originalBuilder := buildRepoFileIndex
	defer func() {
		buildRepoFileIndex = originalBuilder
	}()

	buildRepoFileIndex = func(_ context.Context, _ workspaceRepoRef) ([]repoFileIndexItem, error) {
		return []repoFileIndexItem{{path: "fresh.md"}}, nil
	}

	items, err := app.loadRepoFileIndex(context.Background(), repo)
	if err != nil {
		t.Fatalf("loadRepoFileIndex failed: %v", err)
	}
	if len(items) != 1 || items[0].path != "fresh.md" {
		t.Fatalf("expected refreshed repo index, got %+v", items)
	}
}

func TestResolveRepoFilePathRejectsTraversal(t *testing.T) {
	t.Parallel()

	repoPath := t.TempDir()
	if _, _, err := resolveRepoFilePath(repoPath, "../secret.txt"); err == nil {
		t.Fatalf("expected traversal path to fail")
	}
	if _, _, err := resolveRepoFilePath(repoPath, "/tmp/secret.txt"); err == nil {
		t.Fatalf("expected absolute path to fail")
	}
}

func TestResolveRepoFilePathRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on windows CI images")
	}

	repoPath := t.TempDir()
	outsidePath := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outsidePath, []byte("outside\n"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	linkPath := filepath.Join(repoPath, "escape.txt")
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if _, _, err := resolveRepoFilePath(repoPath, "escape.txt"); err == nil {
		t.Fatalf("expected symlink escape to fail")
	}
}

func TestReadRepoFileContentDetectsBinaryAndTruncation(t *testing.T) {
	t.Parallel()

	binaryPath := filepath.Join(t.TempDir(), "binary.dat")
	if err := os.WriteFile(binaryPath, []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}
	binaryContent, err := readRepoFileContent(binaryPath, 64)
	if err != nil {
		t.Fatalf("read binary file: %v", err)
	}
	if !binaryContent.binary {
		t.Fatalf("expected binary detection")
	}
	if binaryContent.content != "" {
		t.Fatalf("expected binary content to be omitted")
	}

	largePath := filepath.Join(t.TempDir(), "large.txt")
	large := make([]byte, 128)
	for i := range large {
		large[i] = 'a'
	}
	if err := os.WriteFile(largePath, large, 0o644); err != nil {
		t.Fatalf("write large file: %v", err)
	}
	largeContent, err := readRepoFileContent(largePath, 32)
	if err != nil {
		t.Fatalf("read large file: %v", err)
	}
	if !largeContent.truncated {
		t.Fatalf("expected large file to be truncated")
	}
	if len(largeContent.content) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(largeContent.content))
	}
	if largeContent.sizeBytes != 128 {
		t.Fatalf("expected original size to be reported")
	}
}

func TestIsMarkdownPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want bool
	}{
		{path: "README.md", want: true},
		{path: "docs/guide.mdx", want: true},
		{path: "docs/guide.markdown", want: true},
		{path: "src/main.go", want: false},
	}

	for _, tc := range cases {
		if got := isMarkdownPath(tc.path); got != tc.want {
			t.Fatalf("isMarkdownPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
