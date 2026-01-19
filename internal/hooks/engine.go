package hooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Engine struct {
	Runner Runner
	Clock  func() time.Time
}

type RunInput struct {
	Event          Event
	Hooks          []Hook
	DefaultOnError string
	LogRoot        string
	Context        Context
}

func (e Engine) Run(ctx context.Context, input RunInput) (RunReport, error) {
	if input.Event == "" {
		return RunReport{}, errors.New("hook event required")
	}
	runner := e.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	clock := e.Clock
	if clock == nil {
		clock = time.Now
	}
	report := RunReport{Event: input.Event}
	onErrorDefault := normalizeOnError(input.DefaultOnError)
	if onErrorDefault == "" {
		onErrorDefault = OnErrorFail
	}

	for _, hook := range input.Hooks {
		if !hookMatchesEvent(hook, input.Event) {
			continue
		}
		if err := validateHook(hook); err != nil {
			return report, err
		}
		onError := normalizeOnError(hook.OnError)
		if onError == "" {
			onError = onErrorDefault
		}

		command := interpolateArgs(hook.Run, input.Context.TokenMap())
		cwd := interpolateValue(hook.Cwd, input.Context.TokenMap())
		if cwd == "" {
			cwd = input.Context.RepoPath
		}

		logPath, file, err := openHookLog(input.LogRoot, input.Event, hook.ID, clock())
		if err != nil {
			return report, err
		}
		result := RunResult{HookID: hook.ID, Status: RunStatusOK, LogPath: logPath}

		if err := writeHookHeader(file, hook, input.Event, input.Context, command, cwd, clock()); err != nil {
			_ = file.Close()
			return report, err
		}
		env := append(os.Environ(), input.Context.Env()...)
		for key, value := range hook.Env {
			envValue := interpolateValue(value, input.Context.TokenMap())
			env = append(env, fmt.Sprintf("%s=%s", key, envValue))
		}

		runErr := runner.Run(ctx, RunRequest{
			Command: command,
			Cwd:     cwd,
			Env:     env,
			Stdout:  file,
			Stderr:  file,
		})

		if err := writeHookFooter(file, runErr, clock()); err != nil {
			_ = file.Close()
			return report, err
		}
		if closeErr := file.Close(); closeErr != nil {
			return report, closeErr
		}

		if runErr != nil {
			result.Status = RunStatusFailed
			result.Err = runErr
			report.Results = append(report.Results, result)
			if onError == OnErrorFail {
				return report, HookFailedError{HookID: hook.ID, LogPath: logPath, Err: runErr}
			}
			continue
		}
		report.Results = append(report.Results, result)
	}
	return report, nil
}

func hookMatchesEvent(hook Hook, event Event) bool {
	for _, target := range hook.On {
		if target == event {
			return true
		}
	}
	return false
}

func validateHook(hook Hook) error {
	if strings.TrimSpace(hook.ID) == "" {
		return errors.New("hook id required")
	}
	if len(hook.On) == 0 {
		return fmt.Errorf("hook %s: on events required", hook.ID)
	}
	if len(hook.Run) == 0 {
		return fmt.Errorf("hook %s: run command required", hook.ID)
	}
	if hook.OnError != "" && normalizeOnError(hook.OnError) == "" {
		return fmt.Errorf("hook %s: invalid on_error value %q", hook.ID, hook.OnError)
	}
	return nil
}

func normalizeOnError(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case OnErrorFail:
		return OnErrorFail
	case OnErrorWarn:
		return OnErrorWarn
	default:
		return ""
	}
}

func interpolateArgs(args []string, tokens map[string]string) []string {
	if len(args) == 0 {
		return nil
	}
	resolved := make([]string, 0, len(args))
	for _, arg := range args {
		resolved = append(resolved, interpolateValue(arg, tokens))
	}
	return resolved
}

func interpolateValue(value string, tokens map[string]string) string {
	if value == "" || len(tokens) == 0 {
		return value
	}
	pairs := make([]string, 0, len(tokens)*2)
	for token, tokenValue := range tokens {
		pairs = append(pairs, token, tokenValue)
	}
	replacer := strings.NewReplacer(pairs...)
	return replacer.Replace(value)
}

func openHookLog(root string, event Event, hookID string, now time.Time) (string, *os.File, error) {
	if root == "" {
		return "", nil, errors.New("hook log root required")
	}
	if hookID == "" {
		return "", nil, errors.New("hook id required for log")
	}
	dir := filepath.Join(root, string(event))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, err
	}
	safeID := sanitizeFilename(hookID)
	timestamp := now.UTC().Format("20060102-150405")
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.log", timestamp, safeID))
	file, err := os.Create(path)
	if err != nil {
		return "", nil, err
	}
	return path, file, nil
}

func sanitizeFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, string(os.PathSeparator), "-")
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func writeHookHeader(w io.Writer, hook Hook, event Event, ctx Context, command []string, cwd string, now time.Time) error {
	_, err := fmt.Fprintf(w, "workset hook %s\n", hook.ID)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	if ctx.RepoName != "" {
		if _, err := fmt.Fprintf(w, "repo: %s\n", ctx.RepoName); err != nil {
			return err
		}
	}
	if ctx.WorktreePath != "" {
		if _, err := fmt.Fprintf(w, "worktree: %s\n", ctx.WorktreePath); err != nil {
			return err
		}
	}
	if len(command) > 0 {
		if _, err := fmt.Fprintf(w, "command: %s\n", strings.Join(command, " ")); err != nil {
			return err
		}
	}
	if cwd != "" {
		if _, err := fmt.Fprintf(w, "cwd: %s\n", cwd); err != nil {
			return err
		}
	} else if hook.Cwd != "" {
		if _, err := fmt.Fprintf(w, "cwd: %s\n", hook.Cwd); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "started: %s\n", now.UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, "----")
	return err
}

func writeHookFooter(w io.Writer, runErr error, now time.Time) error {
	if _, err := fmt.Fprintln(w, "----"); err != nil {
		return err
	}
	if runErr != nil {
		if _, err := fmt.Fprintf(w, "error: %s\n", runErr); err != nil {
			return err
		}
		if exitErr, ok := runErr.(interface{ ExitCode() int }); ok {
			if _, err := fmt.Fprintf(w, "exit_code: %d\n", exitErr.ExitCode()); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(w, "finished: %s\n", now.UTC().Format(time.RFC3339))
	return err
}
