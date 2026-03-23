package main

import (
	"context"
	"testing"
)

func TestParseRepoLocalStatusOutput(t *testing.T) {
	t.Parallel()

	output := `# branch.oid abcdef1234567890
# branch.head feature/perf
# branch.upstream origin/feature/perf
# branch.ab +3 -2
1 .M N... 100644 100644 100644 abcdef1234567890 abcdef1234567890 src/app.ts
? scratch.txt
`

	status := parseRepoLocalStatusOutput(output, "main")
	if !status.HasUncommitted {
		t.Fatal("expected dirty repo")
	}
	if status.CurrentBranch != "feature/perf" {
		t.Fatalf("expected branch feature/perf, got %q", status.CurrentBranch)
	}
	if status.Ahead != 3 || status.Behind != 2 {
		t.Fatalf("expected ahead/behind 3/2, got %d/%d", status.Ahead, status.Behind)
	}
}

func TestParseRepoLocalStatusOutputFallsBackWhenDetached(t *testing.T) {
	t.Parallel()

	output := `# branch.oid abcdef1234567890
# branch.head (detached)
`

	status := parseRepoLocalStatusOutput(output, "main")
	if status.HasUncommitted {
		t.Fatal("expected clean repo")
	}
	if status.CurrentBranch != "main" {
		t.Fatalf("expected fallback branch main, got %q", status.CurrentBranch)
	}
}

func TestLoadRepoLocalStatusReturnsSignature(t *testing.T) {
	origRunCommand := repoLocalStatusRunCommand
	defer func() {
		repoLocalStatusRunCommand = origRunCommand
	}()

	repoLocalStatusRunCommand = func(_ context.Context, _ string) (string, error) {
		return "# branch.head perf\n# branch.ab +1 -0\n1 .M N... 100644 100644 100644 abc abc src/main.go\n", nil
	}

	snapshot, err := loadRepoLocalStatus(context.Background(), "/tmp/repo", "main")
	if err != nil {
		t.Fatalf("loadRepoLocalStatus: %v", err)
	}
	if snapshot.signature == "" {
		t.Fatal("expected non-empty signature")
	}
	if snapshot.summarySignature == "" {
		t.Fatal("expected non-empty summary signature for dirty repo")
	}
	if snapshot.payload.CurrentBranch != "perf" {
		t.Fatalf("expected branch perf, got %q", snapshot.payload.CurrentBranch)
	}
	if !snapshot.payload.HasUncommitted {
		t.Fatal("expected dirty repo from status output")
	}
}
