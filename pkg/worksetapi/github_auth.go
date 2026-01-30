package worksetapi

import (
	"context"
	"strings"
)

// GetGitHubAuthStatus reports the current authentication state.
func (s *Service) GetGitHubAuthStatus(ctx context.Context) (GitHubAuthStatusJSON, error) {
	if s.github == nil {
		return GitHubAuthStatusJSON{Authenticated: false}, nil
	}
	if err := s.importGitHubPATFromEnv(ctx); err != nil {
		return GitHubAuthStatusJSON{}, err
	}
	return s.github.AuthStatus(ctx)
}

// SetGitHubToken stores a personal access token for GitHub API usage.
func (s *Service) SetGitHubToken(ctx context.Context, input GitHubTokenInput) (GitHubAuthStatusJSON, error) {
	if s.github == nil {
		return GitHubAuthStatusJSON{}, ValidationError{Message: "GitHub authentication is not configured"}
	}
	token := strings.TrimSpace(input.Token)
	if token == "" {
		return GitHubAuthStatusJSON{}, ValidationError{Message: "GitHub token required"}
	}
	return s.github.SetToken(ctx, token)
}

// ClearGitHubAuth removes any stored authentication token.
func (s *Service) ClearGitHubAuth(ctx context.Context) error {
	if s.github == nil {
		return nil
	}
	return s.github.ClearAuth(ctx)
}
