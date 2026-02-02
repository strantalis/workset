//go:build !windows

package sessiond

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

var startPTYFunc = pty.Start

func startPTY(cmd *exec.Cmd) (*os.File, error) {
	return startPTYFunc(cmd)
}

func resizePTY(file *os.File, cols, rows int) error {
	return pty.Setsize(file, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
}
