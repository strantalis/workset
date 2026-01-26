//go:build windows

package sessiond

import "syscall"

func daemonSysProcAttr() *syscall.SysProcAttr {
	return nil
}
