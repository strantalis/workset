package main

import (
	"context"
	"os/exec"
	"sync"
)

var (
	gitExecutableOnce sync.Once
	gitExecutablePath = "git"
)

func gitExecutable() string {
	gitExecutableOnce.Do(func() {
		if resolved, err := exec.LookPath("git"); err == nil && resolved != "" {
			gitExecutablePath = resolved
		}
	})
	return gitExecutablePath
}

func newGitCommand(args ...string) *exec.Cmd {
	return exec.Command(gitExecutable(), args...)
}

func newGitCommandContext(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, gitExecutable(), args...)
}
