package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestValidateRegisterValid(t *testing.T) {
	now := time.Date(2026, time.March, 4, 12, 0, 0, 0, time.UTC)
	reg := register{
		Version: 1,
		Items: []registerItem{
			{
				ID:            "legacy-config",
				Scope:         "global-config",
				Summary:       "legacy fallback",
				Introduced:    "2026-03-01",
				RemoveBy:      "2026-08-01",
				Owner:         "workset-core",
				TrackingIssue: "docs-dev/architecture/workset-hierarchy.md",
				Status:        statusActive,
				Evidence:      []string{"internal/config/global.go"},
			},
		},
	}

	problems, warnings := validateRegister(reg, now, 14)
	if len(problems) != 0 {
		t.Fatalf("expected no problems, got %v", problems)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestValidateRegisterOverdue(t *testing.T) {
	now := time.Date(2026, time.March, 4, 12, 0, 0, 0, time.UTC)
	reg := register{
		Version: 1,
		Items: []registerItem{
			{
				ID:            "legacy-config",
				Scope:         "global-config",
				Summary:       "legacy fallback",
				Introduced:    "2026-03-01",
				RemoveBy:      "2026-03-01",
				Owner:         "workset-core",
				TrackingIssue: "docs-dev/architecture/workset-hierarchy.md",
				Status:        statusActive,
				Evidence:      []string{"internal/config/global.go"},
			},
		},
	}

	problems, _ := validateRegister(reg, now, 14)
	if len(problems) == 0 {
		t.Fatalf("expected overdue problem, got none")
	}
	if !strings.Contains(strings.Join(problems, "\n"), "overdue") {
		t.Fatalf("expected overdue message, got %v", problems)
	}
}

func TestRunFailsOnInvalidStatus(t *testing.T) {
	t.Parallel()

	configPath := t.TempDir() + "/deprecations.yaml"
	content := `
version: 1
items:
  - id: bad-status
    scope: config
    summary: test
    introduced: 2026-03-01
    remove_by: 2026-08-01
    owner: workset-core
    tracking_issue: docs-dev/architecture/workset-hierarchy.md
    status: unknown
    evidence: [internal/config/types.go]
`
	if err := os.WriteFile(configPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out strings.Builder
	var errOut strings.Builder
	code := run(
		[]string{"--config", configPath},
		&out,
		&errOut,
		time.Date(2026, time.March, 4, 12, 0, 0, 0, time.UTC),
	)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "status must be") {
		t.Fatalf("expected status error, got %q", errOut.String())
	}
}
