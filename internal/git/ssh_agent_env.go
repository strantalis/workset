package git

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kevinburke/ssh_config"
)

var ensureSSHAuthSockOnce sync.Once

// EnsureSSHAuthSock maps ssh_config IdentityAgent to SSH_AUTH_SOCK when unset.
func EnsureSSHAuthSock() {
	ensureSSHAuthSockOnce.Do(func() {
		current := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
		if isSocket(current) {
			return
		}

		identityAgent := resolveIdentityAgent()
		if identityAgent == "" {
			return
		}

		identityAgent = expandSSHPath(identityAgent)
		if identityAgent == "" || !isSocket(identityAgent) {
			return
		}

		_ = os.Setenv("SSH_AUTH_SOCK", identityAgent)
	})
}

func resolveIdentityAgent() string {
	for _, host := range []string{"github.com", "gitlab.com", "bitbucket.org"} {
		agent := strings.TrimSpace(ssh_config.Get(host, "IdentityAgent"))
		if shouldUseIdentityAgent(agent) {
			return agent
		}
	}
	return ""
}

func shouldUseIdentityAgent(agent string) bool {
	if agent == "" {
		return false
	}
	lower := strings.ToLower(agent)
	if lower == "none" || lower == "ssh_auth_sock" {
		return false
	}
	return true
}

func expandSSHPath(path string) string {
	expanded := os.ExpandEnv(path)
	if strings.HasPrefix(expanded, "~") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return expanded
		}
		expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~"))
	}
	return expanded
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
