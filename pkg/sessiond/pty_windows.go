//go:build windows

package sessiond

import (
	"fmt"
	"os"
	"os/exec"
)

var startPTYFunc = func(_ *exec.Cmd) (*os.File, error) {
	return nil, fmt.Errorf("pty not supported on windows")
}

func startPTY(cmd *exec.Cmd) (*os.File, error) {
	return startPTYFunc(cmd)
}

func resizePTY(_ *os.File, _ int, _ int) error {
	return nil
}
