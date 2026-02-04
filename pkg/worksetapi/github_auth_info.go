package worksetapi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

// GetGitHubAuthInfo reports auth mode, status, and local CLI availability.
func (s *Service) GetGitHubAuthInfo(ctx context.Context) (GitHubAuthInfoJSON, error) {
	mode := githubAuthModeCLI
	if provider, ok := s.github.(GitHubAuthModeProvider); ok {
		mode = provider.AuthMode(ctx)
	}
	if err := s.importGitHubPATFromEnv(ctx); err != nil {
		return GitHubAuthInfoJSON{}, err
	}
	cli, err := s.GetGitHubCLIStatus(ctx)
	if err != nil {
		return GitHubAuthInfoJSON{}, err
	}
	status := GitHubAuthStatusJSON{Authenticated: false}
	if s.github != nil {
		if mode != githubAuthModeCLI || cli.Installed {
			current, err := s.github.AuthStatus(ctx)
			if err != nil {
				return GitHubAuthInfoJSON{}, err
			}
			status = current
		}
	}
	return GitHubAuthInfoJSON{
		Mode:   mode,
		Status: status,
		CLI:    cli,
	}, nil
}

// SetGitHubAuthMode switches the active GitHub auth provider.
func (s *Service) SetGitHubAuthMode(ctx context.Context, mode string) (GitHubAuthInfoJSON, error) {
	provider, ok := s.github.(GitHubAuthModeProvider)
	if !ok || provider == nil {
		return GitHubAuthInfoJSON{}, ValidationError{Message: "GitHub auth mode is not configurable"}
	}
	if err := provider.SetAuthMode(ctx, mode); err != nil {
		return GitHubAuthInfoJSON{}, err
	}
	return s.GetGitHubAuthInfo(ctx)
}

// GetGitHubCLIStatus reports whether the GitHub CLI is available on PATH.
func (s *Service) GetGitHubCLIStatus(ctx context.Context) (GitHubCLIStatusJSON, error) {
	configuredPath, err := s.gitHubCLIPathFromConfig(ctx)
	if err != nil {
		return GitHubCLIStatusJSON{}, err
	}
	configuredPath = normalizeCLIPath(configuredPath)
	status := GitHubCLIStatusJSON{
		ConfiguredPath: configuredPath,
	}
	if configuredPath != "" && isExecutableFile(configuredPath) {
		_ = os.Setenv("GH_PATH", configuredPath)
	}
	path := configuredPath
	if path == "" || !isExecutableFile(path) {
		path = ensureGitHubCLIPath()
		if path == "" {
			if configuredPath != "" && !isExecutableFile(configuredPath) {
				status.Error = "Configured GitHub CLI path is not executable"
			}
			status.Installed = false
			return status, nil
		}
	}
	status.Installed = true
	status.Path = path
	if configuredPath != "" && !isExecutableFile(configuredPath) {
		status.Error = "Configured GitHub CLI path is not executable"
	}
	version, err := s.ghVersion(ctx, path)
	if err != nil {
		status.Error = err.Error()
		return status, nil
	}
	status.Version = version
	return status, nil
}

func (s *Service) ghVersion(ctx context.Context, path string) (string, error) {
	if s.commands == nil {
		return "", errors.New("command runner unavailable")
	}
	if path == "" {
		path = "gh"
	}
	result, err := s.commands(ctx, "", []string{path, "--version"}, nil, "")
	if err != nil && result.ExitCode != 0 {
		return "", err
	}
	output := strings.TrimSpace(result.Stdout)
	if output == "" {
		output = strings.TrimSpace(result.Stderr)
	}
	if output == "" {
		return "", errors.New("empty gh --version output")
	}
	line := strings.SplitN(output, "\n", 2)[0]
	if parsed := parseGitHubCLIVersion(line); parsed != "" {
		return parsed, nil
	}
	return strings.TrimSpace(line), nil
}

func parseGitHubCLIVersion(line string) string {
	fields := strings.Fields(line)
	for i, field := range fields {
		if field == "version" && i+1 < len(fields) {
			return strings.TrimSpace(fields[i+1])
		}
	}
	return ""
}

func (s *Service) gitHubCLIPathFromConfig(ctx context.Context) (string, error) {
	cfg, _, err := s.loadGlobal(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cfg.GitHub.CLIPath), nil
}

// SetGitHubCLIPath stores an explicit path to the GitHub CLI binary.
func (s *Service) SetGitHubCLIPath(ctx context.Context, path string) (GitHubAuthInfoJSON, error) {
	path = normalizeCLIPath(path)
	if path != "" && !isExecutableFile(path) {
		return GitHubAuthInfoJSON{}, ValidationError{Message: "GitHub CLI path is not executable"}
	}
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, _ config.GlobalConfigLoadInfo) error {
		cfg.GitHub.CLIPath = path
		return nil
	}); err != nil {
		return GitHubAuthInfoJSON{}, err
	}
	if path != "" {
		_ = os.Setenv("GH_PATH", path)
	}
	return s.GetGitHubAuthInfo(ctx)
}

func (s *Service) importGitHubPATFromEnv(ctx context.Context) error {
	if importer, ok := s.github.(GitHubPATImporter); ok && importer != nil {
		_, err := importer.ImportPATFromEnv(ctx)
		return err
	}
	return nil
}

func normalizeCLIPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = os.ExpandEnv(path)
	if after, ok := strings.CutPrefix(path, "~"); ok {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			path = filepath.Join(home, after)
		}
	}
	return filepath.Clean(path)
}
