package worksetapi

import (
	"context"
	"errors"
	"os"
	"strings"
)

const (
	githubAuthModeCLI   = "cli"
	githubAuthModePAT   = "pat"
	worksetGitHubPATEnv = "WORKSET_GITHUB_PAT"
)

// GitHubProvider selects an authenticated GitHub client and auth status.
type GitHubProvider interface {
	AuthStatus(ctx context.Context) (GitHubAuthStatusJSON, error)
	SetToken(ctx context.Context, token string) (GitHubAuthStatusJSON, error)
	ClearAuth(ctx context.Context) error
	Client(ctx context.Context, host string) (GitHubClient, error)
}

// GitHubPATImporter supports importing PAT tokens from environment variables.
type GitHubPATImporter interface {
	ImportPATFromEnv(ctx context.Context) (bool, error)
}

// GitHubAuthModeProvider exposes the selected GitHub auth mode.
type GitHubAuthModeProvider interface {
	AuthMode(ctx context.Context) string
	SetAuthMode(ctx context.Context, mode string) error
}

// GitHubClient executes GitHub API operations for a given host.
type GitHubClient interface {
	CreatePullRequest(ctx context.Context, owner, repo string, pr GitHubNewPullRequest) (GitHubPullRequest, error)
	GetPullRequest(ctx context.Context, owner, repo string, number int) (GitHubPullRequest, error)
	ListPullRequests(ctx context.Context, owner, repo, head, state string, page, perPage int) ([]GitHubPullRequest, int, error)
	ListReviewComments(ctx context.Context, owner, repo string, number, page, perPage int) ([]PullRequestReviewCommentJSON, int, error)
	CreateReplyComment(ctx context.Context, owner, repo string, number int, commentID int64, body string) (PullRequestReviewCommentJSON, error)
	EditReviewComment(ctx context.Context, owner, repo string, commentID int64, body string) (PullRequestReviewCommentJSON, error)
	DeleteReviewComment(ctx context.Context, owner, repo string, commentID int64) error
	ListCheckRuns(ctx context.Context, owner, repo, ref string, page, perPage int) ([]PullRequestCheckJSON, int, error)
	GetCheckRunAnnotations(ctx context.Context, owner, repo string, checkRunID int64) ([]CheckAnnotationJSON, error)
	GetRepoDefaultBranch(ctx context.Context, owner, repo string) (string, error)
	GetCurrentUser(ctx context.Context) (GitHubUserJSON, []string, error)
	ReviewThreadMap(ctx context.Context, owner, repo string, number int) (map[string]threadInfo, error)
	GetReviewThreadID(ctx context.Context, commentNodeID string) (string, error)
	ResolveReviewThread(ctx context.Context, threadID string, resolve bool) (bool, error)
}

// GitHubPullRequest captures the fields used by Workset.
type GitHubPullRequest struct {
	Number    int
	URL       string
	Title     string
	Body      string
	Draft     bool
	State     string
	BaseRef   string
	HeadRef   string
	HeadSHA   string
	Mergeable *bool
}

// GitHubNewPullRequest describes an API payload.
type GitHubNewPullRequest struct {
	Title string
	Head  string
	Base  string
	Body  string
	Draft bool
}

// GitHubProviderSelector chooses the auth provider based on stored mode.
type GitHubProviderSelector struct {
	store TokenStore
	pat   GitHubProvider
	cli   GitHubProvider
}

func NewGitHubProviderSelector(store TokenStore) *GitHubProviderSelector {
	if store == nil {
		store = KeyringTokenStore{}
	}
	return &GitHubProviderSelector{
		store: store,
		pat:   NewGitHubPATProvider(store),
		cli:   NewGitHubCLIProvider(),
	}
}

func (p *GitHubProviderSelector) AuthStatus(ctx context.Context) (GitHubAuthStatusJSON, error) {
	return p.provider(ctx).AuthStatus(ctx)
}

func (p *GitHubProviderSelector) AuthMode(ctx context.Context) string {
	return p.mode(ctx)
}

func (p *GitHubProviderSelector) SetAuthMode(ctx context.Context, mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "", githubAuthModeCLI:
		return p.setMode(ctx, githubAuthModeCLI)
	case githubAuthModePAT:
		return p.setMode(ctx, githubAuthModePAT)
	default:
		return ValidationError{Message: "Unknown GitHub auth mode"}
	}
}

func (p *GitHubProviderSelector) ImportPATFromEnv(ctx context.Context) (bool, error) {
	token := strings.TrimSpace(os.Getenv(worksetGitHubPATEnv))
	if token == "" {
		return false, nil
	}
	if p.store == nil {
		return false, errors.New("token store unavailable")
	}
	stored, err := p.store.Get(ctx, tokenStoreKey)
	if err == nil && stored == token {
		_ = p.setMode(ctx, githubAuthModePAT)
		return false, nil
	}
	if err != nil && !errors.Is(err, ErrTokenNotFound) {
		return false, err
	}
	if _, err := p.pat.SetToken(ctx, token); err != nil {
		return false, err
	}
	_ = p.setMode(ctx, githubAuthModePAT)
	return true, nil
}

func (p *GitHubProviderSelector) SetToken(ctx context.Context, token string) (GitHubAuthStatusJSON, error) {
	status, err := p.pat.SetToken(ctx, token)
	if err != nil {
		return GitHubAuthStatusJSON{}, err
	}
	_ = p.setMode(ctx, githubAuthModePAT)
	return status, nil
}

func (p *GitHubProviderSelector) ClearAuth(ctx context.Context) error {
	if err := p.pat.ClearAuth(ctx); err != nil {
		return err
	}
	return p.setMode(ctx, githubAuthModeCLI)
}

func (p *GitHubProviderSelector) Client(ctx context.Context, host string) (GitHubClient, error) {
	return p.provider(ctx).Client(ctx, host)
}

func (p *GitHubProviderSelector) provider(ctx context.Context) GitHubProvider {
	mode := p.mode(ctx)
	if mode == githubAuthModePAT {
		return p.pat
	}
	return p.cli
}

func (p *GitHubProviderSelector) mode(ctx context.Context) string {
	if p.store == nil {
		return githubAuthModeCLI
	}
	mode, err := p.store.Get(ctx, tokenAuthModeKey)
	if err != nil {
		return githubAuthModeCLI
	}
	mode = strings.TrimSpace(mode)
	if mode == githubAuthModePAT {
		return mode
	}
	return githubAuthModeCLI
}

func (p *GitHubProviderSelector) setMode(ctx context.Context, mode string) error {
	if p.store == nil {
		return nil
	}
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = githubAuthModeCLI
	}
	return p.store.Set(ctx, tokenAuthModeKey, mode)
}
