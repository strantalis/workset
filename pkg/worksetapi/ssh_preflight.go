package worksetapi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type sshPublicKey struct {
	KeyType string
	KeyData string
	Comment string
}

type sshAgentCheck struct {
	Socket    string
	Reachable bool
	HasKeys   bool
	Keys      []sshPublicKey
	Message   string
}

func (s *Service) preflightSSHAuth(ctx context.Context, resolution repoResolution) error {
	headRemote := strings.TrimSpace(resolution.RepoDefaults.Remote)
	if headRemote == "" {
		headRemote = "origin"
	}
	rawURL, err := s.remoteURLFor(resolution.RepoPath, headRemote)
	if err != nil {
		return err
	}
	effectiveURL := rawURL
	if rules, err := gitURLInsteadOfRules(ctx, resolution.RepoPath, s.commands); err == nil && len(rules) > 0 {
		effectiveURL = applyInsteadOfRules(rawURL, rules)
	}
	if !isSSHRemoteURL(effectiveURL) {
		return nil
	}

	currentSock := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
	sshHost := sshHostFromURL(effectiveURL)
	sshConfig, err := sshConfigInfo(ctx, resolution.RepoPath, s.commands, sshHost)
	if err != nil {
		if currentSock == "" {
			return err
		}
		sshConfig = sshConfigInfoResult{}
	}
	identityAgent := normalizeIdentityAgent(sshConfig.IdentityAgent)
	identityFiles := normalizeIdentityFiles(sshConfig.IdentityFiles)
	identityFilesExist := hasIdentityFiles(identityFiles)

	signingRequired, err := gitConfigBool(ctx, resolution.RepoPath, "commit.gpgsign", s.commands)
	if err != nil {
		return err
	}
	if signingRequired {
		format, err := gitConfigGet(ctx, resolution.RepoPath, "gpg.format", s.commands)
		if err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(format), "ssh") {
			signingRequired = false
		}
	}

	var signingParsed sshPublicKey
	var hasSigningKey bool
	if signingRequired {
		signingParsed, hasSigningKey, err = resolveSigningKey(ctx, resolution.RepoPath, s.commands)
		if err != nil {
			return err
		}
	}

	sockets := make([]string, 0, 2)
	if currentSock != "" {
		sockets = append(sockets, currentSock)
	}
	if identityAgent != "" && identityAgent != currentSock {
		sockets = append(sockets, identityAgent)
	}
	if len(sockets) == 0 {
		if signingRequired {
			return ValidationError{Message: "ssh-agent required for SSH commit signing; SSH_AUTH_SOCK is not set"}
		}
		if identityFilesExist {
			return nil
		}
		return ValidationError{Message: formatSSHAgentFailure("SSH_AUTH_SOCK is not set and no IdentityAgent is configured", currentSock, identityAgent, nil)}
	}

	checks := make([]sshAgentCheck, 0, len(sockets))
	reachable := false
	hasKeys := false
	signingKeyFound := false
	for _, socket := range sockets {
		check, err := sshAddListKeys(ctx, resolution.RepoPath, s.commands, socket)
		if err != nil {
			return err
		}
		checks = append(checks, check)
		if check.Reachable {
			reachable = true
		}
		if check.HasKeys {
			hasKeys = true
		}
		if hasSigningKey && check.Reachable && sshKeyInList(check.Keys, signingParsed) {
			signingKeyFound = true
		}
	}

	if !reachable {
		if !signingRequired && identityFilesExist {
			return nil
		}
		return ValidationError{Message: formatSSHAgentFailure("ssh agent not available", currentSock, identityAgent, checks)}
	}
	if !hasKeys {
		if !signingRequired && identityFilesExist {
			return nil
		}
		return ValidationError{Message: formatSSHAgentFailure("ssh agent has no identities", currentSock, identityAgent, checks)}
	}
	if signingRequired && hasSigningKey && !signingKeyFound {
		display := formatSSHKeyDisplay(signingParsed)
		message := fmt.Sprintf("signing key not found in ssh-agent (%s)", display)
		return ValidationError{Message: formatSSHAgentFailure(message, currentSock, identityAgent, checks)}
	}
	return nil
}

func (s *Service) remoteURLFor(repoPath, remoteName string) (string, error) {
	if strings.TrimSpace(remoteName) == "" {
		return "", ValidationError{Message: "remote name required"}
	}
	urls, err := s.git.RemoteURLs(repoPath, remoteName)
	if err != nil {
		return "", err
	}
	if len(urls) == 0 {
		return "", ValidationError{Message: fmt.Sprintf("remote %q has no URL configured", remoteName)}
	}
	return urls[0], nil
}

func gitConfigGet(ctx context.Context, repoPath, key string, runner CommandRunner) (string, error) {
	result, err := runner(ctx, repoPath, []string{"git", "config", "--get", key}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		if result.ExitCode == 1 {
			return "", nil
		}
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "git config failed"
		}
		return "", ValidationError{Message: message}
	}
	return result.Stdout, nil
}

func gitConfigBool(ctx context.Context, repoPath, key string, runner CommandRunner) (bool, error) {
	result, err := runner(ctx, repoPath, []string{"git", "config", "--bool", "--get", key}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		if result.ExitCode == 1 {
			return false, nil
		}
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "git config failed"
		}
		return false, ValidationError{Message: message}
	}
	value := strings.ToLower(strings.TrimSpace(result.Stdout))
	return value == "true" || value == "yes" || value == "on" || value == "1", nil
}

type urlInsteadOfRule struct {
	Base      string
	InsteadOf string
}

func gitURLInsteadOfRules(ctx context.Context, repoPath string, runner CommandRunner) ([]urlInsteadOfRule, error) {
	result, err := runner(ctx, repoPath, []string{"git", "config", "--get-regexp", "^url\\..*\\.insteadof$"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		if result.ExitCode == 1 {
			return nil, nil
		}
		return nil, ValidationError{Message: "failed to read git url.insteadOf config"}
	}
	rules := []urlInsteadOfRule{}
	scanner := bufio.NewScanner(strings.NewReader(result.Stdout))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := fields[0]
		value := strings.Join(fields[1:], " ")
		base, ok := parseInsteadOfKeyBase(key)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		rules = append(rules, urlInsteadOfRule{Base: base, InsteadOf: value})
	}
	return rules, nil
}

func parseInsteadOfKeyBase(key string) (string, bool) {
	lower := strings.ToLower(key)
	if !strings.HasPrefix(lower, "url.") || !strings.HasSuffix(lower, ".insteadof") {
		return "", false
	}
	base := key[len("url.") : len(key)-len(".insteadof")]
	base = strings.Trim(base, "\"")
	return base, base != ""
}

func applyInsteadOfRules(raw string, rules []urlInsteadOfRule) string {
	bestLen := -1
	bestBase := ""
	bestInsteadOf := ""
	for _, rule := range rules {
		if strings.HasPrefix(raw, rule.InsteadOf) {
			if len(rule.InsteadOf) > bestLen {
				bestLen = len(rule.InsteadOf)
				bestBase = rule.Base
				bestInsteadOf = rule.InsteadOf
			}
		}
	}
	if bestLen == -1 {
		return raw
	}
	return bestBase + strings.TrimPrefix(raw, bestInsteadOf)
}

func isSSHRemoteURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if strings.Contains(raw, "://") {
		parsed, err := parseURL(raw)
		if err != nil {
			return false
		}
		scheme := strings.ToLower(parsed.Scheme)
		return scheme == "ssh" || scheme == "git+ssh"
	}
	return strings.Contains(raw, "@") && strings.Contains(raw, ":")
}

func sshHostFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		parsed, err := parseURL(raw)
		if err != nil {
			return ""
		}
		return parsed.Hostname()
	}
	if strings.Contains(raw, ":") {
		before, _, ok := strings.Cut(raw, ":")
		if !ok {
			return ""
		}
		if at := strings.LastIndex(before, "@"); at != -1 {
			return before[at+1:]
		}
		return before
	}
	return ""
}

type sshConfigInfoResult struct {
	IdentityAgent string
	IdentityFiles []string
}

func sshConfigInfo(ctx context.Context, repoPath string, runner CommandRunner, host string) (sshConfigInfoResult, error) {
	if strings.TrimSpace(host) == "" {
		return sshConfigInfoResult{}, nil
	}
	result, err := runner(ctx, repoPath, []string{"ssh", "-G", host}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return sshConfigInfoResult{}, ValidationError{Message: "ssh command not found"}
		}
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "ssh -G failed"
		}
		return sshConfigInfoResult{}, ValidationError{Message: message}
	}
	info := sshConfigInfoResult{}
	scanner := bufio.NewScanner(strings.NewReader(result.Stdout))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "identityagent ") {
			info.IdentityAgent = strings.TrimSpace(strings.TrimPrefix(line, "identityagent "))
			continue
		}
		if strings.HasPrefix(line, "identityfile ") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "identityfile "))
			if value != "" {
				info.IdentityFiles = append(info.IdentityFiles, value)
			}
		}
	}
	return info, nil
}

func normalizeIdentityAgent(agent string) string {
	agent = strings.TrimSpace(agent)
	if agent == "" {
		return ""
	}
	lower := strings.ToLower(agent)
	if lower == "none" || lower == "ssh_auth_sock" || lower == "*" {
		return ""
	}
	agent = expandSSHPath(agent)
	return strings.TrimSpace(agent)
}

func expandSSHPath(path string) string {
	expanded := strings.TrimSpace(os.ExpandEnv(path))
	if strings.HasPrefix(expanded, "~") {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			rest := strings.TrimPrefix(expanded, "~")
			rest = strings.TrimLeft(rest, "/\\")
			expanded = filepath.Join(home, rest)
		}
	}
	return expanded
}

func normalizeIdentityFiles(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	unique := map[string]struct{}{}
	values := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		path = expandSSHPath(path)
		if path == "" {
			continue
		}
		if _, ok := unique[path]; ok {
			continue
		}
		unique[path] = struct{}{}
		values = append(values, path)
	}
	return values
}

func hasIdentityFiles(paths []string) bool {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func resolveSigningKey(ctx context.Context, repoPath string, runner CommandRunner) (sshPublicKey, bool, error) {
	value, err := gitConfigGet(ctx, repoPath, "user.signingKey", runner)
	if err != nil {
		return sshPublicKey{}, false, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return sshPublicKey{}, false, nil
	}
	if key, ok := parseSSHPublicKey(value); ok {
		return key, true, nil
	}
	path := expandSSHPath(value)
	if path == "" {
		return sshPublicKey{}, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return sshPublicKey{}, false, ValidationError{Message: fmt.Sprintf("unable to read signing key file %s", path)}
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if key, ok := parseSSHPublicKey(line); ok {
			return key, true, nil
		}
	}
	return sshPublicKey{}, false, ValidationError{Message: fmt.Sprintf("no valid public key found in %s", path)}
}

func sshAddListKeys(ctx context.Context, repoPath string, runner CommandRunner, socket string) (sshAgentCheck, error) {
	env := withEnvVar(os.Environ(), "SSH_AUTH_SOCK", socket)
	result, err := runner(ctx, repoPath, []string{"ssh-add", "-L"}, env, "")
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return sshAgentCheck{}, ValidationError{Message: "ssh-add command not found"}
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return sshAgentCheck{}, err
		}
		return sshAgentCheck{}, fmt.Errorf("running ssh-add -L: %w", err)
	}
	check := sshAgentCheck{Socket: socket}
	message := strings.TrimSpace(result.Stderr)
	if message == "" {
		message = strings.TrimSpace(result.Stdout)
	}
	lower := strings.ToLower(message)
	if strings.Contains(lower, "no identities") {
		check.Reachable = true
		check.HasKeys = false
		check.Message = message
		return check, nil
	}
	if result.ExitCode != 0 {
		check.Reachable = false
		check.HasKeys = false
		check.Message = message
		return check, nil
	}
	keys := parseSSHPublicKeys(result.Stdout)
	check.Reachable = true
	check.HasKeys = len(keys) > 0
	check.Keys = keys
	return check, nil
}

func parseSSHPublicKeys(output string) []sshPublicKey {
	keys := []sshPublicKey{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if key, ok := parseSSHPublicKey(line); ok {
			keys = append(keys, key)
		}
	}
	return keys
}

func parseSSHPublicKey(value string) (sshPublicKey, bool) {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) < 2 {
		return sshPublicKey{}, false
	}
	keyType := fields[0]
	keyData := fields[1]
	if keyType == "" || keyData == "" {
		return sshPublicKey{}, false
	}
	if !isLikelyBase64(keyData) {
		return sshPublicKey{}, false
	}
	key := sshPublicKey{KeyType: keyType, KeyData: keyData}
	if len(fields) > 2 {
		key.Comment = strings.Join(fields[2:], " ")
	}
	return key, true
}

func isLikelyBase64(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '+' || r == '/' || r == '=':
		default:
			return false
		}
	}
	return value != ""
}

func sshKeyInList(keys []sshPublicKey, target sshPublicKey) bool {
	for _, key := range keys {
		if key.KeyType == target.KeyType && key.KeyData == target.KeyData {
			return true
		}
	}
	return false
}

func formatSSHKeyDisplay(key sshPublicKey) string {
	if key.Comment != "" {
		return key.Comment
	}
	if key.KeyData == "" {
		return key.KeyType
	}
	snippet := key.KeyData
	if len(snippet) > 12 {
		snippet = snippet[:12] + "..."
	}
	if key.KeyType != "" {
		return key.KeyType + " " + snippet
	}
	return snippet
}

func formatSSHAgentFailure(prefix, currentSock, identityAgent string, checks []sshAgentCheck) string {
	parts := []string{prefix}
	if currentSock != "" {
		parts = append(parts, "SSH_AUTH_SOCK="+currentSock)
	} else {
		parts = append(parts, "SSH_AUTH_SOCK not set")
	}
	if identityAgent != "" {
		parts = append(parts, "IdentityAgent="+identityAgent)
	}
	for _, check := range checks {
		if check.Message != "" {
			parts = append(parts, fmt.Sprintf("agent[%s]: %s", check.Socket, check.Message))
		}
	}
	return strings.Join(parts, "; ")
}

func withEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			replaced = true
			if value != "" {
				out = append(out, prefix+value)
			}
			continue
		}
		out = append(out, entry)
	}
	if !replaced && value != "" {
		out = append(out, prefix+value)
	}
	return out
}
