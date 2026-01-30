package worksetapi

import (
	"context"
	"errors"
	"testing"
)

type fakeTokenStore struct {
	values map[string]string
	errors map[string]error
}

func newFakeTokenStore() *fakeTokenStore {
	return &fakeTokenStore{
		values: map[string]string{},
		errors: map[string]error{},
	}
}

func (s *fakeTokenStore) Get(_ context.Context, key string) (string, error) {
	if err, ok := s.errors[key]; ok {
		return "", err
	}
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrTokenNotFound
}

func (s *fakeTokenStore) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func (s *fakeTokenStore) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

type fakeGitHubProvider struct {
	status        GitHubAuthStatusJSON
	setTokenCalls []string
	setTokenErr   error
	clearErr      error
	clientErr     error
}

func (p *fakeGitHubProvider) AuthStatus(_ context.Context) (GitHubAuthStatusJSON, error) {
	return p.status, nil
}

func (p *fakeGitHubProvider) SetToken(_ context.Context, token string) (GitHubAuthStatusJSON, error) {
	p.setTokenCalls = append(p.setTokenCalls, token)
	if p.setTokenErr != nil {
		return GitHubAuthStatusJSON{}, p.setTokenErr
	}
	return p.status, nil
}

func (p *fakeGitHubProvider) ClearAuth(_ context.Context) error {
	return p.clearErr
}

func (p *fakeGitHubProvider) Client(_ context.Context, _ string) (GitHubClient, error) {
	if p.clientErr != nil {
		return nil, p.clientErr
	}
	return nil, nil
}

func TestGitHubProviderSelectorDefaultsToCLI(t *testing.T) {
	store := newFakeTokenStore()
	cli := &fakeGitHubProvider{status: GitHubAuthStatusJSON{Authenticated: true, Login: "cli"}}
	pat := &fakeGitHubProvider{status: GitHubAuthStatusJSON{Authenticated: true, Login: "pat"}}
	selector := &GitHubProviderSelector{store: store, cli: cli, pat: pat}

	status, err := selector.AuthStatus(context.Background())
	if err != nil {
		t.Fatalf("AuthStatus: %v", err)
	}
	if status.Login != "cli" {
		t.Fatalf("expected CLI auth status, got %q", status.Login)
	}
}

func TestGitHubProviderSelectorHonorsPATMode(t *testing.T) {
	store := newFakeTokenStore()
	store.values[tokenAuthModeKey] = githubAuthModePAT
	cli := &fakeGitHubProvider{status: GitHubAuthStatusJSON{Authenticated: true, Login: "cli"}}
	pat := &fakeGitHubProvider{status: GitHubAuthStatusJSON{Authenticated: true, Login: "pat"}}
	selector := &GitHubProviderSelector{store: store, cli: cli, pat: pat}

	status, err := selector.AuthStatus(context.Background())
	if err != nil {
		t.Fatalf("AuthStatus: %v", err)
	}
	if status.Login != "pat" {
		t.Fatalf("expected PAT auth status, got %q", status.Login)
	}
}

func TestGitHubProviderSelectorImportPATFromEnv(t *testing.T) {
	store := newFakeTokenStore()
	pat := &fakeGitHubProvider{status: GitHubAuthStatusJSON{Authenticated: true, Login: "pat"}}
	selector := &GitHubProviderSelector{store: store, cli: &fakeGitHubProvider{}, pat: pat}

	t.Setenv(worksetGitHubPATEnv, "token-123")
	imported, err := selector.ImportPATFromEnv(context.Background())
	if err != nil {
		t.Fatalf("ImportPATFromEnv: %v", err)
	}
	if !imported {
		t.Fatalf("expected token to be imported")
	}
	if len(pat.setTokenCalls) != 1 || pat.setTokenCalls[0] != "token-123" {
		t.Fatalf("expected PAT provider to receive token")
	}
	if store.values[tokenAuthModeKey] != githubAuthModePAT {
		t.Fatalf("expected auth mode to be set to PAT")
	}
}

func TestGitHubProviderSelectorImportPATSkipsExistingToken(t *testing.T) {
	store := newFakeTokenStore()
	store.values[tokenStoreKey] = "token-123"
	pat := &fakeGitHubProvider{status: GitHubAuthStatusJSON{Authenticated: true, Login: "pat"}}
	selector := &GitHubProviderSelector{store: store, cli: &fakeGitHubProvider{}, pat: pat}

	t.Setenv(worksetGitHubPATEnv, "token-123")
	imported, err := selector.ImportPATFromEnv(context.Background())
	if err != nil {
		t.Fatalf("ImportPATFromEnv: %v", err)
	}
	if imported {
		t.Fatalf("expected import to be skipped for existing token")
	}
	if len(pat.setTokenCalls) != 0 {
		t.Fatalf("expected PAT provider not to be called")
	}
	if store.values[tokenAuthModeKey] != githubAuthModePAT {
		t.Fatalf("expected auth mode to be set to PAT")
	}
}

func TestGitHubProviderSelectorImportPATPropagatesStoreErrors(t *testing.T) {
	store := newFakeTokenStore()
	store.errors[tokenStoreKey] = errors.New("boom")
	selector := &GitHubProviderSelector{store: store, cli: &fakeGitHubProvider{}, pat: &fakeGitHubProvider{}}

	t.Setenv(worksetGitHubPATEnv, "token-123")
	_, err := selector.ImportPATFromEnv(context.Background())
	if err == nil {
		t.Fatalf("expected error when token store fails")
	}
}
