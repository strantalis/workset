package git

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var ensureSSHAuthSockOnce sync.Once

// EnsureSSHAuthSock maps ssh IdentityAgent to SSH_AUTH_SOCK when available.
func EnsureSSHAuthSock() {
	ensureSSHAuthSockOnce.Do(func() {
		current := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
		identityAgent := resolveIdentityAgentFromSSH()
		if identityAgent == "" {
			return
		}
		if next, ok := applySSHAuthSock(current, identityAgent); ok {
			_ = os.Setenv("SSH_AUTH_SOCK", next)
		}
	})
}

func shouldUseIdentityAgent(agent string) bool {
	if agent == "" {
		return false
	}
	lower := strings.ToLower(agent)
	if lower == "none" || lower == "ssh_auth_sock" || lower == "*" {
		return false
	}
	return true
}

func expandSSHPath(path string) string {
	expanded := trimQuotes(os.ExpandEnv(path))
	if strings.HasPrefix(expanded, "~") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return expanded
		}
		expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~"))
	}
	return expanded
}

func applySSHAuthSock(current, identityAgent string) (string, bool) {
	identityAgent = strings.TrimSpace(identityAgent)
	if !shouldUseIdentityAgent(identityAgent) {
		return "", false
	}
	identityAgent = expandSSHPath(identityAgent)
	if identityAgent == "" {
		return "", false
	}
	if current == identityAgent {
		return "", false
	}
	return identityAgent, true
}

func isSocket(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0
}

func trimQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	if value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	return value
}

func resolveIdentityAgentFromSSH() string {
	for _, host := range []string{"github.com", "gitlab.com", "bitbucket.org"} {
		if agent, ok := sshConfigIdentityAgent(host); ok {
			return agent
		}
	}
	return ""
}

func sshConfigIdentityAgent(host string) (string, bool) {
	output, err := exec.Command("ssh", "-G", host).Output()
	if err != nil {
		return "", false
	}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "identityagent ") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		return strings.Join(parts[1:], " "), true
	}
	return "", false
}
