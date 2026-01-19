package worksetapi

import (
	"fmt"
	"strings"

	"github.com/strantalis/workset/internal/ops"
)

func summarizeRepoSafety(report ops.RepoSafetyReport) (dirty []string, unmerged []string, unpushed []string, warnings []string) {
	for _, branch := range report.Branches {
		if branch.StatusErr != "" {
			warnings = append(warnings, fmt.Sprintf("%s: status failed (%s)", branch.Branch, branch.StatusErr))
		}
		if branch.FetchBaseErr != "" {
			warnings = append(warnings, fmt.Sprintf("%s: base fetch failed (%s)", branch.Branch, branch.FetchBaseErr))
		}
		if branch.FetchWriteErr != "" {
			warnings = append(warnings, fmt.Sprintf("%s: write fetch failed (%s)", branch.Branch, branch.FetchWriteErr))
		}
		if branch.UnmergedErr != "" {
			warnings = append(warnings, fmt.Sprintf("%s: unmerged check failed (%s)", branch.Branch, branch.UnmergedErr))
		}
		if branch.UnpushedErr != "" {
			warnings = append(warnings, fmt.Sprintf("%s: unpushed check failed (%s)", branch.Branch, branch.UnpushedErr))
		}
		if branch.Dirty {
			dirty = append(dirty, branch.Branch)
		}
		if branch.Unmerged {
			unmerged = append(unmerged, branch.Branch)
		}
		if branch.Unpushed {
			unpushed = append(unpushed, branch.Branch)
		}
	}
	return dirty, unmerged, unpushed, warnings
}

func summarizeWorkspaceSafety(report ops.WorkspaceSafetyReport) (dirty []string, unmerged []string, unpushed []string, warnings []string) {
	for _, repo := range report.Repos {
		repoDirty, repoUnmerged, repoUnpushed, repoWarnings := summarizeRepoSafety(repo)
		for _, branch := range repoDirty {
			dirty = append(dirty, fmt.Sprintf("%s:%s", repo.RepoName, branch))
		}
		for _, branch := range repoUnmerged {
			unmerged = append(unmerged, fmt.Sprintf("%s:%s", repo.RepoName, branch))
		}
		for _, branch := range repoUnpushed {
			unpushed = append(unpushed, fmt.Sprintf("%s:%s", repo.RepoName, branch))
		}
		for _, warning := range repoWarnings {
			warnings = append(warnings, fmt.Sprintf("%s: %s", repo.RepoName, warning))
		}
	}
	return dirty, unmerged, unpushed, warnings
}

func unmergedRepoDetails(report ops.RepoSafetyReport) []string {
	baseRef := ""
	if report.BaseRemote != "" && report.BaseBranch != "" {
		baseRef = fmt.Sprintf("%s/%s", report.BaseRemote, report.BaseBranch)
	}
	details := make([]string, 0)
	for _, branch := range report.Branches {
		if !branch.Unmerged {
			continue
		}
		reason := branch.UnmergedReason
		if reason == "" && baseRef != "" {
			reason = fmt.Sprintf("branch content not found in %s history", baseRef)
		} else if reason == "" {
			reason = "branch content not found in base history"
		}
		if baseRef != "" && !strings.Contains(reason, baseRef) {
			reason = fmt.Sprintf("%s (base %s)", reason, baseRef)
		}
		details = append(details, fmt.Sprintf("%s: %s", branch.Branch, reason))
	}
	return details
}

func unmergedWorkspaceDetails(report ops.WorkspaceSafetyReport) []string {
	details := make([]string, 0)
	for _, repo := range report.Repos {
		for _, detail := range unmergedRepoDetails(repo) {
			details = append(details, fmt.Sprintf("%s: %s", repo.RepoName, detail))
		}
	}
	return details
}
