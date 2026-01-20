//go:build !windows

package main

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

func startPTY(cmd *exec.Cmd) (*os.File, error) {
	return pty.Start(cmd)
}

func resizePTY(file *os.File, cols, rows int) error {
	return pty.Setsize(file, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
}
