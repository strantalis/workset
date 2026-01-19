//go:build windows

package session

import (
	"context"
	"fmt"
)

func RunExecWithPTY(_ context.Context, _ string, _ []string, _ []string) error {
	return fmt.Errorf("interactive exec is not supported on Windows")
}
