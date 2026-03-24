package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/strantalis/workset/pkg/worksetapi"
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

func TestBuildRepoFileIndexOmitsDeletedTrackedFiles(t *testing.T) {
	repoPath := t.TempDir()
	runGit(t, repoPath, "init", "-q")

	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	runGit(t, repoPath, "add", "README.md")
	runGit(t, repoPath, "commit", "-qm", "add readme")

	if err := os.Remove(filepath.Join(repoPath, "README.md")); err != nil {
		t.Fatalf("remove tracked file: %v", err)
	}

	items, err := buildRepoFileIndex(context.Background(), workspaceRepoRef{
		id:   "thread-alpha::api",
		name: "api",
		path: repoPath,
	})
	if err != nil {
		t.Fatalf("buildRepoFileIndex failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected deleted tracked files to be omitted, got %+v", items)
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

func TestMimeTypeFromImageExt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want string
	}{
		{path: "logo.png", want: "image/png"},
		{path: "photo.jpg", want: "image/jpeg"},
		{path: "photo.jpeg", want: "image/jpeg"},
		{path: "anim.gif", want: "image/gif"},
		{path: "modern.webp", want: "image/webp"},
		{path: "icon.svg", want: "image/svg+xml"},
		{path: "favicon.ico", want: "image/x-icon"},
		{path: "old.bmp", want: "image/bmp"},
		{path: "next.avif", want: "image/avif"},
		{path: "LOGO.PNG", want: "image/png"},
		{path: "dir/sub/photo.JPG", want: "image/jpeg"},
		{path: "readme.md", want: ""},
		{path: "code.go", want: ""},
		{path: "noext", want: ""},
	}

	for _, tc := range cases {
		if got := mimeTypeFromImageExt(tc.path); got != tc.want {
			t.Fatalf("mimeTypeFromImageExt(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestReadRepoFileBytesRespectsLimit(t *testing.T) {
	t.Parallel()

	smallPath := filepath.Join(t.TempDir(), "small.png")
	smallData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic bytes
	if err := os.WriteFile(smallPath, smallData, 0o644); err != nil {
		t.Fatalf("write small file: %v", err)
	}

	data, err := readRepoFileBytes(smallPath, 1024)
	if err != nil {
		t.Fatalf("readRepoFileBytes failed: %v", err)
	}
	if len(data) != len(smallData) {
		t.Fatalf("expected %d bytes, got %d", len(smallData), len(data))
	}

	largePath := filepath.Join(t.TempDir(), "large.png")
	largeData := make([]byte, 256)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	if err := os.WriteFile(largePath, largeData, 0o644); err != nil {
		t.Fatalf("write large file: %v", err)
	}

	_, err = readRepoFileBytes(largePath, 64)
	if err == nil {
		t.Fatalf("expected readRepoFileBytes to fail for oversized file")
	}
}

func setupRepoFilesAppWithWorkspace(
	t *testing.T,
	workspaceName string,
	worksetYAML string,
) (*App, string) {
	t.Helper()

	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("mkdir workspace root: %v", err)
	}

	configPath := filepath.Join(root, "config.yaml")
	globalConfig := fmt.Sprintf(
		"defaults:\n  workset_root: %s\nworksets:\n  %s:\n    path: %s\n",
		root,
		workspaceName,
		workspaceRoot,
	)
	if err := os.WriteFile(configPath, []byte(globalConfig), 0o644); err != nil {
		t.Fatalf("write global config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "workset.yaml"), []byte(worksetYAML), 0o644); err != nil {
		t.Fatalf("write workset config: %v", err)
	}

	app := NewApp()
	app.service = worksetapi.NewService(worksetapi.Options{ConfigPath: configPath})
	return app, workspaceRoot
}

func TestListWorkspaceExtraRootsExcludesConfiguredRepos(t *testing.T) {
	app, workspaceRoot := setupRepoFilesAppWithWorkspace(t, "ws-1", "name: ws-1\nrepos:\n  - name: repo-a\n    repo_dir: repo-a\n")

	for _, dir := range []string{"repo-a", "scratch", "manual-repo", ".workset"} {
		if err := os.MkdirAll(filepath.Join(workspaceRoot, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "manual-repo", ".git"), []byte("gitdir\n"), 0o644); err != nil {
		t.Fatalf("write .git marker: %v", err)
	}

	roots, err := app.ListWorkspaceExtraRoots("ws-1")
	if err != nil {
		t.Fatalf("ListWorkspaceExtraRoots: %v", err)
	}
	if len(roots) != 2 {
		t.Fatalf("expected 2 extra roots, got %+v", roots)
	}
	if roots[0].Label != "manual-repo" || !roots[0].GitDetected {
		t.Fatalf("expected manual-repo git root, got %+v", roots[0])
	}
	if roots[1].Label != "scratch" || roots[1].GitDetected {
		t.Fatalf("expected scratch non-git root, got %+v", roots[1])
	}
}

func TestListRepoDirectorySupportsWorkspaceExtraRoots(t *testing.T) {
	app, workspaceRoot := setupRepoFilesAppWithWorkspace(t, "ws-1", "name: ws-1\nrepos: []\n")

	scratchRoot := filepath.Join(workspaceRoot, "scratch")
	if err := os.MkdirAll(filepath.Join(scratchRoot, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir scratch/docs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(scratchRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir scratch/.git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scratchRoot, "notes.md"), []byte("# notes\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	entries, err := app.ListRepoDirectory(RepoDirectoryListRequest{
		WorkspaceID: "ws-1",
		RepoID:      buildWorkspaceExtraRootID("ws-1", "scratch"),
		DirPath:     "",
	})
	if err != nil {
		t.Fatalf("ListRepoDirectory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %+v", entries)
	}
	if !entries[0].IsDir || entries[0].Name != "docs" {
		t.Fatalf("expected docs directory first, got %+v", entries[0])
	}
	if entries[1].Name != "notes.md" || !entries[1].IsMarkdown {
		t.Fatalf("expected markdown file second, got %+v", entries[1])
	}
}
