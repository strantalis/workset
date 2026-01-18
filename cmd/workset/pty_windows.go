//go:build windows

package main

import (
	"context"
	"fmt"
)

func runExecWithPTY(_ context.Context, _ string, _ []string, _ []string) error {
	return fmt.Errorf("interactive exec is not supported on Windows")
}
