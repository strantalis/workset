package worksetapi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/ops"
)

type fakeGitClient struct {
	remoteURLs map[string][]string
}

func (f fakeGitClient) Clone(_ context.Context, _, _, _ string) error {
	return errors.New("not implemented")
}
func (f fakeGitClient) CloneBare(_ context.Context, _, _, _ string) error {
	return errors.New("not implemented")
}
func (f fakeGitClient) AddRemote(_, _, _ string) error { return errors.New("not implemented") }
func (f fakeGitClient) RemoteNames(_ string) ([]string, error) {
	return nil, errors.New("not implemented")
}
func (f fakeGitClient) RemoteURLs(_ string, remoteName string) ([]string, error) {
	return f.remoteURLs[remoteName], nil
}
func (f fakeGitClient) ReferenceExists(_ context.Context, _, _ string) (bool, error) {
	return false, errors.New("not implemented")
}
func (f fakeGitClient) Fetch(_ context.Context, _ string, _ string) error {
	return errors.New("not implemented")
}
func (f fakeGitClient) UpdateBranch(_ context.Context, _, _, _ string) error {
	return errors.New("not implemented")
}
func (f fakeGitClient) Status(_ string) (git.StatusSummary, error) {
	return git.StatusSummary{}, errors.New("not implemented")
}
func (f fakeGitClient) IsRepo(_ string) (bool, error) { return false, errors.New("not implemented") }
func (f fakeGitClient) IsAncestor(_, _, _ string) (bool, error) {
	return false, errors.New("not implemented")
}
func (f fakeGitClient) IsContentMerged(_, _, _ string) (bool, error) {
	return false, errors.New("not implemented")
}
func (f fakeGitClient) CurrentBranch(_ string) (string, bool, error) {
	return "", false, errors.New("not implemented")
}
func (f fakeGitClient) RemoteExists(_ string, _ string) (bool, error) {
	return false, errors.New("not implemented")
}
func (f fakeGitClient) WorktreeAdd(_ context.Context, _ git.WorktreeAddOptions) error {
	return errors.New("not implemented")
}
func (f fakeGitClient) WorktreeRemove(_ git.WorktreeRemoveOptions) error {
	return errors.New("not implemented")
}
func (f fakeGitClient) WorktreeList(_ string) ([]string, error) {
	return nil, errors.New("not implemented")
}

func TestPreflightSSHAuthAllowsIdentityFileWithoutAgent(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	service := &Service{
		git:      fakeGitClient{remoteURLs: map[string][]string{"origin": {"git@github.com:owner/repo.git"}}},
		commands: stubSSHPreflightRunner(t, keyPath),
	}
	resolution := repoResolution{RepoPath: "/tmp", RepoDefaults: ops.RepoDefaults{Remote: "origin"}}

	t.Setenv("SSH_AUTH_SOCK", "")
	if err := service.preflightSSHAuth(context.Background(), resolution); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestParseSSHPublicKeyAcceptsECDSA(t *testing.T) {
	key := "ecdsa-sha2-nistp256 AAAA comment"
	parsed, ok := parseSSHPublicKey(key)
	if !ok {
		t.Fatalf("expected parse to succeed")
	}
	if parsed.KeyType != "ecdsa-sha2-nistp256" || parsed.KeyData != "AAAA" {
		t.Fatalf("unexpected parse result: %#v", parsed)
	}
}

func TestExpandSSHPathHandlesTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Fatalf("missing home dir")
	}
	expanded := expandSSHPath("~/.ssh/id_ed25519")
	if !strings.HasPrefix(expanded, home) {
		t.Fatalf("expected %q to start with %q", expanded, home)
	}
}

func stubSSHPreflightRunner(t *testing.T, identityFile string) CommandRunner {
	t.Helper()
	return func(_ context.Context, _ string, command []string, _ []string, _ string) (CommandResult, error) {
		if len(command) == 0 {
			return CommandResult{}, errors.New("missing command")
		}
		switch command[0] {
		case "git":
			return CommandResult{ExitCode: 1}, nil
		case "ssh":
			if len(command) >= 3 && command[1] == "-G" {
				stdout := "identityfile " + identityFile + "\n"
				return CommandResult{Stdout: stdout, ExitCode: 0}, nil
			}
		case "ssh-add":
			return CommandResult{ExitCode: 1, Stderr: "no identities"}, nil
		}
		return CommandResult{ExitCode: 1}, nil
	}
}
