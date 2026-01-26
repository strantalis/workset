//go:build windows

package sessiond

import (
	"fmt"
	"os"
	"os/exec"
)

func startPTY(_ *exec.Cmd) (*os.File, error) {
	return nil, fmt.Errorf("pty not supported on windows")
}

func resizePTY(_ *os.File, _ int, _ int) error {
	return nil
}
