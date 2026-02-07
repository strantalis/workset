package worksetapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

func gitAddAll(ctx context.Context, repoPath string, runner CommandRunner) error {
	result, err := runner(ctx, repoPath, []string{"git", "add", "-A"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "git add failed"
		}
		return ValidationError{Message: message}
	}
	return nil
}

func gitHasStagedChanges(ctx context.Context, repoPath string, runner CommandRunner) (bool, error) {
	result, err := runner(ctx, repoPath, []string{"git", "diff", "--cached", "--name-only"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "unable to check staged changes"
		}
		return false, ValidationError{Message: message}
	}
	return strings.TrimSpace(result.Stdout) != "", nil
}

func gitCommitMessage(ctx context.Context, repoPath, message string, runner CommandRunner) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return ValidationError{Message: "commit message required"}
	}
	result, err := runner(ctx, repoPath, []string{"git", "commit", "-m", message}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		msg := strings.TrimSpace(result.Stderr)
		if msg == "" && err != nil {
			msg = err.Error()
		}
		if msg == "" {
			msg = "git commit failed"
		}
		return ValidationError{Message: msg}
	}
	return nil
}

func gitPushBranch(ctx context.Context, repoPath, remote, branch string, runner CommandRunner) error {
	if strings.TrimSpace(remote) == "" {
		return ValidationError{Message: "remote name required to push head branch"}
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return ValidationError{Message: "head branch required"}
	}
	result, err := runner(ctx, repoPath, []string{"git", "push", "-u", remote, branch}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "git push failed"
		}
		return ValidationError{Message: message}
	}
	return nil
}

func remoteBranchExists(ctx context.Context, repoPath, remote, branch string, runner CommandRunner) (bool, error) {
	if strings.TrimSpace(repoPath) == "" {
		return false, errors.New("repo path required")
	}
	if strings.TrimSpace(remote) == "" {
		return false, ValidationError{Message: "remote name required to verify head branch"}
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false, ValidationError{Message: "head branch required"}
	}
	result, err := runner(ctx, repoPath, []string{"git", "ls-remote", "--heads", remote, branch}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "unable to verify remote head branch"
		}
		return false, ValidationError{Message: message}
	}
	return strings.TrimSpace(result.Stdout) != "", nil
}

func buildRepoPatch(ctx context.Context, repoPath string, limit int, runner CommandRunner) (string, error) {
	if repoPath == "" {
		return "", errors.New("repo path required")
	}
	parts := []string{}
	staged, err := runGitDiff(ctx, repoPath, runner, true, "")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(staged) != "" {
		parts = append(parts, staged)
	}
	unstaged, err := runGitDiff(ctx, repoPath, runner, false, "")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(unstaged) != "" {
		parts = append(parts, unstaged)
	}
	untracked, err := gitUntracked(ctx, repoPath, runner)
	if err != nil {
		return "", err
	}
	for _, file := range untracked {
		diff, err := gitDiffNoIndex(ctx, repoPath, runner, file)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(diff) != "" {
			parts = append(parts, diff)
		}
	}
	patch := strings.Join(parts, "\n")
	if limit > 0 && len(patch) > limit {
		patch = patch[:limit] + "\n... (diff truncated)\n"
	}
	return patch, nil
}

func runGitDiff(ctx context.Context, repoPath string, runner CommandRunner, staged bool, file string) (string, error) {
	args := []string{"git", "diff"}
	if staged {
		args = append(args, "--cached")
	}
	if file != "" {
		args = append(args, "--", file)
	}
	result, err := runner(ctx, repoPath, args, os.Environ(), "")
	if err != nil && result.ExitCode != 1 {
		return "", err
	}
	return result.Stdout, nil
}

func gitDiffNoIndex(ctx context.Context, repoPath string, runner CommandRunner, file string) (string, error) {
	if strings.TrimSpace(file) == "" {
		return "", nil
	}
	args := []string{"git", "diff", "--no-index", "--", "/dev/null", file}
	result, err := runner(ctx, repoPath, args, os.Environ(), "")
	if err != nil && result.ExitCode != 1 {
		return "", err
	}
	return result.Stdout, nil
}

func gitUntracked(ctx context.Context, repoPath string, runner CommandRunner) ([]string, error) {
	result, err := runner(ctx, repoPath, []string{"git", "ls-files", "--others", "--exclude-standard"}, os.Environ(), "")
	if err != nil && result.ExitCode != 1 {
		return nil, err
	}
	lines := strings.Split(result.Stdout, "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, line)
	}
	return files, nil
}

func gitHasUncommittedChanges(ctx context.Context, repoPath string, runner CommandRunner) (bool, error) {
	result, err := runner(ctx, repoPath, []string{"git", "status", "--porcelain"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "unable to check uncommitted changes"
		}
		return false, ValidationError{Message: message}
	}
	return strings.TrimSpace(result.Stdout) != "", nil
}

func gitAheadBehind(ctx context.Context, repoPath, branch string, runner CommandRunner) (int, int, error) {
	// Get upstream tracking branch
	upstreamResult, err := runner(ctx, repoPath, []string{"git", "rev-parse", "--abbrev-ref", branch + "@{upstream}"}, os.Environ(), "")
	if err != nil || upstreamResult.ExitCode != 0 {
		return 0, 0, ValidationError{Message: "no upstream tracking branch configured"}
	}
	upstream := strings.TrimSpace(upstreamResult.Stdout)
	if upstream == "" {
		return 0, 0, ValidationError{Message: "no upstream tracking branch configured"}
	}

	// Get ahead count
	aheadResult, err := runner(ctx, repoPath, []string{"git", "rev-list", "--count", upstream + ".." + branch}, os.Environ(), "")
	ahead := 0
	if err == nil && aheadResult.ExitCode == 0 {
		if parsed, parseErr := parseCount(aheadResult.Stdout); parseErr == nil {
			ahead = parsed
		}
	}

	// Get behind count
	behindResult, err := runner(ctx, repoPath, []string{"git", "rev-list", "--count", branch + ".." + upstream}, os.Environ(), "")
	behind := 0
	if err == nil && behindResult.ExitCode == 0 {
		if parsed, parseErr := parseCount(behindResult.Stdout); parseErr == nil {
			behind = parsed
		}
	}

	return ahead, behind, nil
}

func parseCount(output string) (int, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return 0, errors.New("empty output")
	}
	var count int
	_, err := fmt.Sscanf(output, "%d", &count)
	return count, err
}

func gitHeadSHA(ctx context.Context, repoPath string, runner CommandRunner) (string, error) {
	result, err := runner(ctx, repoPath, []string{"git", "rev-parse", "HEAD"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		return "", errors.New("unable to get HEAD SHA")
	}
	return strings.TrimSpace(result.Stdout), nil
}
