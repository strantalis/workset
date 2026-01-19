package worksetapi

import (
	"strings"
	"testing"

	"github.com/strantalis/workset/internal/ops"
)

func TestSummarizeRepoSafety(t *testing.T) {
	report := ops.RepoSafetyReport{
		RepoName:    "demo",
		BaseRemote:  "origin",
		BaseBranch:  "main",
		WriteRemote: "origin",
		Branches: []ops.RepoBranchSafety{
			{
				Branch:       "main",
				Dirty:        true,
				Unmerged:     true,
				Unpushed:     true,
				StatusErr:    "status failed",
				FetchBaseErr: "fetch failed",
			},
		},
	}
	dirty, unmerged, unpushed, warnings := summarizeRepoSafety(report)
	if len(dirty) != 1 || dirty[0] != "main" {
		t.Fatalf("unexpected dirty: %v", dirty)
	}
	if len(unmerged) != 1 || unmerged[0] != "main" {
		t.Fatalf("unexpected unmerged: %v", unmerged)
	}
	if len(unpushed) != 1 || unpushed[0] != "main" {
		t.Fatalf("unexpected unpushed: %v", unpushed)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected warnings")
	}
}

func TestUnmergedDetails(t *testing.T) {
	report := ops.RepoSafetyReport{
		RepoName:    "demo",
		BaseRemote:  "origin",
		BaseBranch:  "main",
		WriteRemote: "origin",
		Branches: []ops.RepoBranchSafety{
			{
				Branch:         "feature",
				Unmerged:       true,
				UnmergedReason: "not in history",
			},
		},
	}
	details := unmergedRepoDetails(report)
	if len(details) != 1 {
		t.Fatalf("expected one detail")
	}
	if !strings.Contains(details[0], "feature") {
		t.Fatalf("missing branch in detail: %s", details[0])
	}

	wsDetails := unmergedWorkspaceDetails(ops.WorkspaceSafetyReport{Repos: []ops.RepoSafetyReport{report}})
	if len(wsDetails) != 1 {
		t.Fatalf("expected workspace detail")
	}
	if !strings.Contains(wsDetails[0], "demo") {
		t.Fatalf("missing repo name in detail: %s", wsDetails[0])
	}
}

func TestSummarizeWorkspaceSafety(t *testing.T) {
	report := ops.WorkspaceSafetyReport{
		Repos: []ops.RepoSafetyReport{
			{
				RepoName: "app",
				Branches: []ops.RepoBranchSafety{
					{Branch: "main", Dirty: true, Unpushed: true},
				},
			},
			{
				RepoName: "lib",
				Branches: []ops.RepoBranchSafety{
					{Branch: "dev", Unmerged: true},
				},
			},
		},
	}
	dirty, unmerged, unpushed, warnings := summarizeWorkspaceSafety(report)
	if len(dirty) != 1 || dirty[0] != "app:main" {
		t.Fatalf("unexpected dirty: %v", dirty)
	}
	if len(unmerged) != 1 || unmerged[0] != "lib:dev" {
		t.Fatalf("unexpected unmerged: %v", unmerged)
	}
	if len(unpushed) != 1 || unpushed[0] != "app:main" {
		t.Fatalf("unexpected unpushed: %v", unpushed)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}
