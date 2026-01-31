package sessiond

import (
	"os"
	"path/filepath"
	"strings"
)

func DefaultSocketPath() (string, error) {
	if socket := strings.TrimSpace(os.Getenv("WORKSET_SESSIOND_SOCKET")); socket != "" {
		return socket, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "sessiond.sock"), nil
}

func DefaultTranscriptDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "terminal_logs"), nil
}

func DefaultRecordDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "terminal_records"), nil
}

func DefaultStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "terminal_state"), nil
}

func sanitizeID(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
