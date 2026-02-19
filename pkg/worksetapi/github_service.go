package worksetapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/google/go-github/v75/github"
)

const (
	defaultGitHubHost = "github.com"
	defaultDiffLimit  = 120000
)

func formatPRPrompt(repoName, branch, patch string) string {
	builder := strings.Builder{}
	builder.WriteString("Generate a pull request title and body based on this diff.\n")
	builder.WriteString("Return JSON only: {\"title\":\"...\",\"body\":\"...\"}.\n")
	builder.WriteString(fmt.Sprintf("Repo: %s\n", repoName))
	builder.WriteString(fmt.Sprintf("Branch: %s\n\n", branch))
	builder.WriteString("Diff:\n")
	builder.WriteString(patch)
	builder.WriteString("\n")
	return builder.String()
}

func formatCommitPrompt(repoName, branch, patch string) string {
	builder := strings.Builder{}
	builder.WriteString("Generate a conventional commit message for this diff.\n")
	builder.WriteString("Use format: type(scope): subject. Keep it concise.\n")
	builder.WriteString("Return JSON only: {\"message\":\"...\"}.\n")
	builder.WriteString(fmt.Sprintf("Repo: %s\n", repoName))
	builder.WriteString(fmt.Sprintf("Branch: %s\n\n", branch))
	builder.WriteString("Diff:\n")
	builder.WriteString(patch)
	builder.WriteString("\n")
	return builder.String()
}

func (s *Service) runAgentPrompt(ctx context.Context, repoPath, agent, prompt, model string) (PullRequestGeneratedJSON, error) {
	schema, err := ensurePRSchema()
	if err != nil {
		return PullRequestGeneratedJSON{}, err
	}
	model = strings.TrimSpace(model)
	output, err := s.runAgentPromptRaw(ctx, repoPath, agent, prompt, schema, model)
	if err != nil {
		if model == "" {
			return PullRequestGeneratedJSON{}, err
		}
		output, err = s.runAgentPromptRaw(ctx, repoPath, agent, prompt, schema, "")
		if err != nil {
			return PullRequestGeneratedJSON{}, err
		}
	}
	result, err := parseAgentJSON(output)
	if err == nil || model == "" {
		return result, err
	}
	output, err = s.runAgentPromptRaw(ctx, repoPath, agent, prompt, schema, "")
	if err != nil {
		return PullRequestGeneratedJSON{}, err
	}
	return parseAgentJSON(output)
}

func (s *Service) runAgentPromptRaw(ctx context.Context, repoPath, agent, prompt, schema, model string) (string, error) {
	command := strings.Fields(agent)
	if len(command) == 0 {
		return "", ValidationError{Message: "agent command required"}
	}
	cfg, _, err := s.loadGlobal(ctx)
	if err != nil {
		return "", err
	}
	configuredPath := normalizeCLIPath(cfg.Agent.CLIPath)
	if configuredPath != "" && isExecutableCandidate(configuredPath) {
		if filepath.Base(configuredPath) == filepath.Base(command[0]) {
			command[0] = configuredPath
		}
	}
	settings := resolveAgentExecSettings()
	command, env, stdin, err := prepareAgentCommand(command, prompt, schema)
	if err != nil {
		return "", err
	}
	command = applyAgentModel(command, model)
	command = resolveAgentCommandPath(command)
	if shouldWrapAgentCommand(settings) {
		wrapped, wrapErr := wrapAgentCommandForShell(command, settings)
		if wrapErr != nil {
			return "", ValidationError{Message: wrapErr.Error()}
		}
		command = wrapped
	}

	var result CommandResult
	switch settings.PTYMode {
	case agentPTYAlways:
		result, err = runCommandWithPTY(ctx, repoPath, command, env, stdin)
	default:
		result, err = s.commands(ctx, repoPath, command, env, stdin)
		if err != nil || result.ExitCode != 0 {
			if settings.PTYMode == agentPTYAuto && shouldRetryWithPTY(err, result) {
				ptyResult, ptyErr := runCommandWithPTY(ctx, repoPath, command, env, stdin)
				if ptyErr == nil && ptyResult.ExitCode == 0 {
					return ptyResult.Stdout, nil
				}
				if ptyErr != nil && err == nil {
					err = ptyErr
				}
				if ptyResult.ExitCode != 0 && ptyResult.Stdout != "" {
					result = ptyResult
				}
			}
		}
	}
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			message = "agent command not found: " + command[0]
		} else if message == "" {
			message = "agent command failed"
		}
		return "", ValidationError{Message: message}
	}
	return result.Stdout, nil
}

func (s *Service) runCommitMessageWithModel(ctx context.Context, repoPath, agent, prompt, model string) (string, error) {
	schema, err := ensureCommitSchema()
	if err != nil {
		return "", err
	}
	model = strings.TrimSpace(model)
	output, err := s.runAgentPromptRaw(ctx, repoPath, agent, prompt, schema, model)
	if err != nil {
		if model == "" {
			return "", err
		}
		output, err = s.runAgentPromptRaw(ctx, repoPath, agent, prompt, schema, "")
		if err != nil {
			return "", err
		}
	}
	message, err := parseCommitJSON(output)
	if err == nil || model == "" {
		return message, err
	}
	output, err = s.runAgentPromptRaw(ctx, repoPath, agent, prompt, schema, "")
	if err != nil {
		return "", err
	}
	return parseCommitJSON(output)
}

func wrapAgentCommandForShell(command []string, settings agentExecSettings) ([]string, error) {
	if runtime.GOOS == "windows" || len(command) == 0 {
		return command, nil
	}
	shell, err := resolveAgentShellPath(settings)
	if err != nil {
		return nil, err
	}
	shellBase := strings.ToLower(filepath.Base(shell))
	commandLine := shellJoinArgs(command)
	args := shellArgsForMode(shellBase, commandLine, settings.ShellMode)
	return append([]string{shell}, args...), nil
}

func shellArgsForMode(shellBase, command, mode string) []string {
	switch mode {
	case agentShellModeInteractive:
		if shellBase == "fish" || shellBase == "csh" || shellBase == "tcsh" {
			return []string{"-i", "-c", command}
		}
		return []string{"-ic", command}
	case agentShellModePlain:
		return []string{"-c", command}
	case agentShellModeLoginAndI:
		if shellBase == "fish" || shellBase == "csh" || shellBase == "tcsh" {
			return []string{"-l", "-i", "-c", command}
		}
		return []string{"-lic", command}
	default:
		if shellBase == "fish" || shellBase == "csh" || shellBase == "tcsh" {
			return []string{"-l", "-c", command}
		}
		return []string{"-lc", command}
	}
}

func shellJoinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, shellEscape(arg))
	}
	return strings.Join(parts, " ")
}

func shellEscape(value string) string {
	if value == "" {
		return "''"
	}
	escaped := strings.ReplaceAll(value, "'", `'"'"'`)
	return "'" + escaped + "'"
}

func parseAgentJSON(output string) (PullRequestGeneratedJSON, error) {
	output = strings.TrimSpace(stripANSI(output))
	if output == "" {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "agent returned empty output"}
	}
	payload, err := decodeJSON(output)
	if err != nil {
		return PullRequestGeneratedJSON{}, err
	}
	if strings.TrimSpace(payload.Title) == "" {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "agent output missing title"}
	}
	return payload, nil
}

func parseCommitJSON(output string) (string, error) {
	output = strings.TrimSpace(stripANSI(output))
	if output == "" {
		return "", ValidationError{Message: "agent returned empty output"}
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		re := regexp.MustCompile(`\{[\s\S]*\}`)
		match := re.FindString(output)
		if match == "" {
			return "", ValidationError{Message: "unable to parse agent JSON output"}
		}
		if err := json.Unmarshal([]byte(match), &payload); err != nil {
			return "", ValidationError{Message: "invalid agent JSON output"}
		}
	}
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		return "", ValidationError{Message: "agent output missing commit message"}
	}
	return message, nil
}

func decodeJSON(output string) (PullRequestGeneratedJSON, error) {
	var payload PullRequestGeneratedJSON
	if err := json.Unmarshal([]byte(output), &payload); err == nil {
		return payload, nil
	}
	re := regexp.MustCompile(`\{[\s\S]*\}`)
	match := re.FindString(output)
	if match == "" {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "unable to parse agent JSON output"}
	}
	if err := json.Unmarshal([]byte(match), &payload); err != nil {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "invalid agent JSON output"}
	}
	return payload, nil
}

func stripANSI(value string) string {
	if value == "" {
		return value
	}
	re := regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	return re.ReplaceAllString(value, "")
}

func isStdinNotTerminal(err error, result CommandResult) bool {
	message := strings.ToLower(strings.TrimSpace(result.Stderr))
	if message == "" {
		message = strings.ToLower(strings.TrimSpace(result.Stdout))
	}
	if err != nil && message == "" {
		message = strings.ToLower(err.Error())
	}
	return strings.Contains(message, "stdin is not a terminal") || strings.Contains(message, "not a tty")
}

func shouldRetryWithPTY(err error, result CommandResult) bool {
	if isStdinNotTerminal(err, result) {
		return true
	}
	if err == nil || result.ExitCode == 0 {
		return false
	}
	output := strings.TrimSpace(result.Stdout + result.Stderr)
	return output == ""
}

func isInvalidHeadError(err error) bool {
	var ghErr *github.ErrorResponse
	if !errors.As(err, &ghErr) {
		return false
	}
	for _, entry := range ghErr.Errors {
		if strings.EqualFold(entry.Resource, "PullRequest") && strings.EqualFold(entry.Field, "head") && strings.EqualFold(entry.Code, "invalid") {
			return true
		}
	}
	return false
}

func formatGitHubAPIError(err error) string {
	var ghErr *github.ErrorResponse
	if !errors.As(err, &ghErr) {
		if err != nil {
			return err.Error()
		}
		return "GitHub API error"
	}
	details := make([]string, 0, len(ghErr.Errors))
	for _, entry := range ghErr.Errors {
		detail := strings.TrimSpace(entry.Message)
		if detail == "" {
			parts := []string{}
			if entry.Resource != "" {
				parts = append(parts, entry.Resource)
			}
			if entry.Field != "" {
				parts = append(parts, entry.Field)
			}
			if entry.Code != "" {
				parts = append(parts, entry.Code)
			}
			detail = strings.TrimSpace(strings.Join(parts, " "))
		}
		if detail != "" {
			details = append(details, detail)
		}
	}
	message := strings.TrimSpace(ghErr.Message)
	if len(details) == 0 {
		if message != "" {
			return message
		}
		if err != nil {
			return err.Error()
		}
		return "GitHub API error"
	}
	if message == "" {
		return strings.Join(details, "; ")
	}
	return fmt.Sprintf("%s (%s)", message, strings.Join(details, "; "))
}

var (
	prSchemaOnce     sync.Once
	prSchemaPath     string
	errPRSchema      error
	commitSchemaOnce sync.Once
	commitSchemaPath string
	errCommitSchema  error
)

func prepareAgentCommand(command []string, prompt string, schema string) ([]string, []string, string, error) {
	env := append(os.Environ(),
		"WORKSET_PR_PROMPT="+prompt,
		"WORKSET_PR_JSON=1",
	)
	if len(command) == 0 {
		return nil, nil, "", errors.New("agent command required")
	}
	if filepath.Base(command[0]) != "codex" {
		return command, env, prompt, nil
	}
	if schema == "" {
		return nil, nil, "", errors.New("agent schema required")
	}

	args := command[1:]
	switch {
	case len(args) == 0 || strings.HasPrefix(args[0], "-"):
		args = append([]string{"exec"}, args...)
	case args[0] == "exec" || args[0] == "e":
		// ok
	default:
		// Any other subcommand should pass through unchanged.
		return command, env, prompt, nil
	}

	promptProvided := hasPromptArg(args)
	if !hasFlag(args, "--color") {
		args = append(args, "--color", "never")
	}
	if !hasFlag(args, "--output-schema") {
		args = append(args, "--output-schema", schema)
	}
	// In non-interactive mode, read the prompt from stdin.
	if !promptProvided {
		args = append(args, "-")
	}
	return append([]string{command[0]}, args...), env, prompt, nil
}

func hasFlag(args []string, name string) bool {
	for i := range args {
		arg := args[i]
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

func hasPromptArg(args []string) bool {
	sawExec := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if !sawExec && (arg == "exec" || arg == "e") {
			sawExec = true
			continue
		}
		if arg == "-" {
			return true
		}
		// first non-flag arg is prompt; treat it as present
		return true
	}
	return false
}

func applyAgentModel(command []string, model string) []string {
	model = strings.TrimSpace(model)
	if model == "" || len(command) == 0 {
		return command
	}
	base := strings.ToLower(filepath.Base(command[0]))
	if hasExplicitModelFlag(base, command[1:]) {
		return command
	}
	switch base {
	case "codex":
		return insertModelArg(command, "-m", model)
	case "claude":
		return insertModelArg(command, "--model", model)
	default:
		return command
	}
}

func hasExplicitModelFlag(command string, args []string) bool {
	switch command {
	case "codex":
		return hasCodexModelFlag(args)
	case "claude":
		return hasClaudeModelFlag(args)
	default:
		return false
	}
}

func hasCodexModelFlag(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-m" || arg == "--model":
			return true
		case strings.HasPrefix(arg, "-m=") || strings.HasPrefix(arg, "--model="):
			return true
		case arg == "-c" || arg == "--config":
			if i+1 < len(args) && strings.HasPrefix(args[i+1], "model=") {
				return true
			}
		case strings.HasPrefix(arg, "-c=") || strings.HasPrefix(arg, "--config="):
			if strings.Contains(arg, "model=") {
				return true
			}
		}
	}
	return false
}

func hasClaudeModelFlag(args []string) bool {
	for _, arg := range args {
		switch {
		case arg == "-m" || arg == "--model":
			return true
		case strings.HasPrefix(arg, "-m=") || strings.HasPrefix(arg, "--model="):
			return true
		}
	}
	return false
}

func insertModelArg(command []string, flag string, model string) []string {
	if len(command) == 0 {
		return command
	}
	args := command[1:]
	idx := findPromptIndex(args)
	if idx == -1 {
		return append(command, flag, model)
	}
	out := make([]string, 0, len(command)+2)
	out = append(out, command[0])
	out = append(out, args[:idx]...)
	out = append(out, flag, model)
	out = append(out, args[idx:]...)
	return out
}

func findPromptIndex(args []string) int {
	for i := len(args) - 1; i >= 0; i-- {
		if args[i] == "-" {
			return i
		}
	}
	for i := len(args) - 1; i >= 0; i-- {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if arg == "exec" || arg == "e" {
			continue
		}
		return i
	}
	return -1
}

func ensurePRSchema() (string, error) {
	prSchemaOnce.Do(func() {
		path := filepath.Join(os.TempDir(), "workset-pr-schema.json")
		payload := `{"type":"object","properties":{"title":{"type":"string"},"body":{"type":"string"}},"required":["title","body"],"additionalProperties":false}`
		errPRSchema = os.WriteFile(path, []byte(payload), 0o644)
		if errPRSchema == nil {
			prSchemaPath = path
		}
	})
	return prSchemaPath, errPRSchema
}

func ensureCommitSchema() (string, error) {
	commitSchemaOnce.Do(func() {
		path := filepath.Join(os.TempDir(), "workset-commit-schema.json")
		payload := `{"type":"object","properties":{"message":{"type":"string"}},"required":["message"],"additionalProperties":false}`
		errCommitSchema = os.WriteFile(path, []byte(payload), 0o644)
		if errCommitSchema == nil {
			commitSchemaPath = path
		}
	})
	return commitSchemaPath, errCommitSchema
}
