package sessiond

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func (s *Session) start(ctx context.Context) error {
	execName, execArgs := resolveShellCommand()
	cmd := exec.CommandContext(ctx, execName, execArgs...)
	cmd.Dir = s.cwd
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"WORKSET_WORKSPACE="+s.id,
		"WORKSET_ROOT="+s.cwd,
	)
	cmd.Env = setEnv(cmd.Env, "COLORTERM", "truecolor")
	cmd.Env = setEnv(cmd.Env, "SHELL", execName)

	ptmx, err := startPTY(cmd)
	if err != nil {
		return err
	}
	if err := s.openTranscript(); err != nil {
		_ = ptmx.Close()
		return err
	}
	s.openRecord()
	s.restoreSnapshot()

	s.mu.Lock()
	s.cmd = cmd
	s.pty = ptmx
	s.startedAt = time.Now()
	s.lastActivity = s.startedAt
	s.mu.Unlock()
	debugLogf("session_start id=%s cwd=%s", s.id, s.cwd)
	if s.emu != nil {
		s.emu.SetResponder(func(resp []byte) {
			if s.hasSubscribers() {
				return
			}
			_ = s.write(ctx, string(resp))
		})
	}

	if s.opts.IdleTimeout > 0 {
		s.idleTimer = time.AfterFunc(s.opts.IdleTimeout, func() {
			s.closeWithReason("idle")
		})
	}
	go s.readLoop(ctx)
	return nil
}

func (s *Session) closeWithReason(reason string) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if reason != "" {
		s.closeReason = reason
	}
	onClose := s.onClose
	debugLogf("session_close id=%s reason=%s", s.id, s.closeReason)
	idleTimer := s.idleTimer
	s.idleTimer = nil
	pty := s.pty
	s.pty = nil
	transcriptFile := s.transcriptFile
	s.transcriptFile = nil
	recordFile := s.recordFile
	s.recordFile = nil
	cmd := s.cmd
	s.mu.Unlock()
	if idleTimer != nil {
		_ = idleTimer.Stop()
	}
	if pty != nil {
		_ = pty.Close()
	}
	if transcriptFile != nil {
		_ = transcriptFile.Close()
	}
	if recordFile != nil {
		_ = recordFile.Close()
	}
	s.persistSnapshot()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		waitForCommandExit(cmd, 2*time.Second)
	}
	if onClose != nil {
		onClose(s)
	}
	s.closeSubscribers()
}

func waitForCommandExit(cmd *exec.Cmd, timeout time.Duration) {
	if cmd == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	if timeout <= 0 {
		<-done
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}

func resolveShellCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		if shell := os.Getenv("COMSPEC"); shell != "" {
			return shell, nil
		}
		return "cmd.exe", nil
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = lookupUserShell()
	}
	if shell == "" {
		shell = "/bin/sh"
	}
	switch strings.ToLower(filepath.Base(shell)) {
	case "zsh", "bash":
		return shell, []string{"-l", "-i"}
	case "fish":
		return shell, []string{"-l"}
	default:
		return shell, nil
	}
}

func lookupUserShell() string {
	current, err := user.Current()
	if err != nil || current.Username == "" {
		return ""
	}
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		if parts[0] == current.Username {
			return strings.TrimSpace(parts[6])
		}
	}
	return ""
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
