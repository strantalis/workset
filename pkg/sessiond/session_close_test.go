package sessiond

import (
	"errors"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestSessionCloseWithReasonReapsProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty not supported on windows")
	}

	cmd := exec.Command("sh", "-c", "sleep 30")
	ptmx, err := startPTY(cmd)
	if err != nil {
		t.Fatalf("start pty: %v", err)
	}
	session := &Session{
		cmd: cmd,
		pty: ptmx,
	}
	pid := cmd.Process.Pid

	session.closeWithReason("test")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected process %d to be reaped after close", pid)
}
