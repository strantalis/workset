package main

import (
	"context"
	"testing"
)

func TestNewReadOnlyGitCommandContextDisablesOptionalLocks(t *testing.T) {
	t.Parallel()

	cmd := newReadOnlyGitCommandContext(context.Background(), "status", "--short")

	for _, env := range cmd.Env {
		if env == "GIT_OPTIONAL_LOCKS=0" {
			return
		}
	}

	t.Fatal("expected read-only git command to disable optional locks")
}
