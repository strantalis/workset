package main

import (
	"fmt"
	"io"

	"github.com/strantalis/workset/internal/output"
	"github.com/strantalis/workset/pkg/worksetapi"
)

func printHookRunReport(w io.Writer, styles output.Styles, repo, event string, runs []worksetapi.HookRunJSON) error {
	header := fmt.Sprintf("hooks run for %s (%s)", repo, event)
	if styles.Enabled {
		header = styles.Render(styles.Title, header)
	}
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}
	for _, run := range runs {
		line := fmt.Sprintf("- %s: %s", run.ID, run.Status)
		if run.LogPath != "" {
			line = fmt.Sprintf("%s (log: %s)", line, run.LogPath)
		}
		if styles.Enabled && run.Status == worksetapi.HookRunStatusOK {
			line = styles.Render(styles.Success, line)
		}
		if styles.Enabled && run.Status == worksetapi.HookRunStatusFailed {
			line = styles.Render(styles.Error, line)
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func printHookExecutionResults(w io.Writer, styles output.Styles, runs []worksetapi.HookExecutionJSON) error {
	if len(runs) == 0 {
		return nil
	}
	currentRepo := ""
	currentEvent := ""
	group := make([]worksetapi.HookRunJSON, 0, len(runs))
	flush := func() error {
		if len(group) == 0 {
			return nil
		}
		if err := printHookRunReport(w, styles, currentRepo, currentEvent, group); err != nil {
			return err
		}
		group = group[:0]
		return nil
	}

	for _, run := range runs {
		if run.Repo != currentRepo || run.Event != currentEvent {
			if err := flush(); err != nil {
				return err
			}
			currentRepo = run.Repo
			currentEvent = run.Event
		}
		group = append(group, worksetapi.HookRunJSON{
			ID:      run.ID,
			Status:  run.Status,
			LogPath: run.LogPath,
		})
	}
	return flush()
}
