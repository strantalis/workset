package session

import (
	"os"
	"runtime"
)

func ResolveExecCommand(args []string) (string, []string) {
	if len(args) > 0 {
		return args[0], args[1:]
	}
	return DefaultShell(), nil
}

func DefaultShell() string {
	if runtime.GOOS == "windows" {
		if shell := os.Getenv("COMSPEC"); shell != "" {
			return shell
		}
		return "cmd.exe"
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}
