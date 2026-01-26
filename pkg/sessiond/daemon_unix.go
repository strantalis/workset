//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package sessiond

import "syscall"

func daemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid: true,
	}
}
