package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/strantalis/workset/pkg/worksetapi"
)

type repoLocalStatusSnapshot struct {
	payload          worksetapi.RepoLocalStatusJSON
	signature        string
	summarySignature string
}

var repoLocalStatusRunCommand = func(ctx context.Context, repoPath string) (string, error) {
	cmd := newGitCommandContext(ctx, "-C", repoPath, "status", "--porcelain=v2", "--branch")
	cmd.Env = os.Environ()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("git status failed for %s: %s", repoPath, message)
	}
	return stdout.String(), nil
}

func loadRepoLocalStatus(
	ctx context.Context,
	repoPath string,
	fallbackBranch string,
) (repoLocalStatusSnapshot, error) {
	output, err := repoLocalStatusRunCommand(ctx, repoPath)
	if err != nil {
		return repoLocalStatusSnapshot{}, err
	}
	payload := parseRepoLocalStatusOutput(output, fallbackBranch)
	return repoLocalStatusSnapshot{
		payload:          payload,
		signature:        strings.TrimSpace(output),
		summarySignature: buildRepoSummarySignature(repoPath, output),
	}, nil
}

func parseRepoLocalStatusOutput(output string, fallbackBranch string) worksetapi.RepoLocalStatusJSON {
	status := worksetapi.RepoLocalStatusJSON{
		CurrentBranch: strings.TrimSpace(fallbackBranch),
	}

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "# ") {
			status.HasUncommitted = true
			continue
		}

		switch {
		case strings.HasPrefix(line, "# branch.head "):
			head := strings.TrimSpace(strings.TrimPrefix(line, "# branch.head "))
			if head != "" && head != "(detached)" {
				status.CurrentBranch = head
			}
		case strings.HasPrefix(line, "# branch.ab "):
			for _, field := range strings.Fields(strings.TrimPrefix(line, "# branch.ab ")) {
				if strings.HasPrefix(field, "+") {
					if parsed, err := strconv.Atoi(strings.TrimPrefix(field, "+")); err == nil {
						status.Ahead = parsed
					}
				}
				if strings.HasPrefix(field, "-") {
					if parsed, err := strconv.Atoi(strings.TrimPrefix(field, "-")); err == nil {
						status.Behind = parsed
					}
				}
			}
		}
	}

	return status
}

func buildRepoSummarySignature(repoPath string, output string) string {
	parts := make([]string, 0)
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "# ") {
			continue
		}
		parts = append(parts, line)
		if path := parseRepoStatusPath(line); path != "" {
			if stat, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(path))); err == nil {
				parts = append(parts, fmt.Sprintf("%s|%d|%d", path, stat.Size(), stat.ModTime().UTC().UnixNano()))
			}
		}
	}
	return strings.Join(parts, "\n")
}

func parseRepoStatusPath(line string) string {
	switch {
	case strings.HasPrefix(line, "? "), strings.HasPrefix(line, "! "):
		return strings.TrimSpace(line[2:])
	case strings.HasPrefix(line, "1 "), strings.HasPrefix(line, "u "):
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return ""
		}
		return fields[len(fields)-1]
	case strings.HasPrefix(line, "2 "):
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return ""
		}
		return strings.TrimSpace(parts[1])
	default:
		return ""
	}
}
