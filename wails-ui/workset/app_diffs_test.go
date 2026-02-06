package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseNameStatusZ(t *testing.T) {
	input := []byte("M\x00readme.md\x00R100\x00old.txt\x00new.txt\x00")
	entries := parseNameStatusZ(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].status != "M" || entries[0].path != "readme.md" {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].status != "R100" || entries[1].prevPath != "old.txt" || entries[1].path != "new.txt" {
		t.Fatalf("unexpected rename entry: %+v", entries[1])
	}
}

func TestParseNumstatZ(t *testing.T) {
	input := []byte("2\t1\treadme.md\x00-\t-\tbin.dat\x003\t2\told.txt\x00new.txt\x00")
	entries := parseNumstatZ(input)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].path != "readme.md" || entries[0].added != 2 || entries[0].removed != 1 {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if !entries[1].binary || entries[1].path != "bin.dat" {
		t.Fatalf("unexpected binary entry: %+v", entries[1])
	}
	if entries[2].prevPath != "old.txt" || entries[2].path != "new.txt" {
		t.Fatalf("unexpected rename entry: %+v", entries[2])
	}
}

func TestFinalizePatch(t *testing.T) {
	patch := "line\n"
	result := finalizePatch(patch)
	if result.Truncated {
		t.Fatalf("did not expect truncation")
	}
	if result.Patch != patch {
		t.Fatalf("expected patch to be preserved")
	}

	largePatch := make([]byte, maxDiffBytes+1)
	for i := range largePatch {
		largePatch[i] = 'a'
	}
	largeResult := finalizePatch(string(largePatch))
	if !largeResult.Truncated {
		t.Fatalf("expected truncation for large patch")
	}
	if largeResult.Patch != "" {
		t.Fatalf("expected truncated patch to be empty")
	}
}

func TestNormalizeNoIndexPath(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "relative dot slash", input: "./src/main.go", want: "src/main.go"},
		{name: "windows dot slash", input: ".\\src\\main.go", want: "src/main.go"},
		{name: "dev null", input: "/dev/null", want: ""},
		{name: "windows nul", input: "NUL", want: ""},
		{name: "empty", input: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeNoIndexPath(tc.input); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestGitUntrackedNumstatBatch(t *testing.T) {
	ctx := context.Background()
	repoPath := t.TempDir()
	runGit(t, repoPath, "init", "-q")

	textPath := filepath.Join(repoPath, "notes.txt")
	if err := os.WriteFile(textPath, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("write text file: %v", err)
	}

	binaryPath := filepath.Join(repoPath, "assets.bin")
	if err := os.WriteFile(binaryPath, []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}

	stats, err := gitUntrackedNumstat(ctx, repoPath, []string{"notes.txt", "assets.bin"})
	if err != nil {
		t.Fatalf("gitUntrackedNumstat failed: %v", err)
	}

	textEntry, ok := stats["notes.txt"]
	if !ok {
		t.Fatalf("missing notes.txt entry: %+v", stats)
	}
	if textEntry.added != 2 || textEntry.removed != 0 || textEntry.binary {
		t.Fatalf("unexpected notes.txt stats: %+v", textEntry)
	}

	binaryEntry, ok := stats["assets.bin"]
	if !ok {
		t.Fatalf("missing assets.bin entry: %+v", stats)
	}
	if !binaryEntry.binary || binaryEntry.removed != 0 {
		t.Fatalf("unexpected assets.bin stats: %+v", binaryEntry)
	}
}

func runGit(t *testing.T, repoPath string, args ...string) {
	t.Helper()
	cmdArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, string(output))
	}
}
