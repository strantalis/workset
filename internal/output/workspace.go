package output

import (
	"fmt"
	"io"
)

type WorkspaceCreated struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Workset string `json:"workset"`
	Branch  string `json:"branch"`
	Next    string `json:"next"`
}

func PrintWorkspaceCreated(w io.Writer, info WorkspaceCreated, styles Styles) error {
	title := "workspace created"
	if styles.Enabled {
		title = styles.Render(styles.Title, title)
	}
	if _, err := fmt.Fprintln(w, title); err != nil {
		return err
	}

	lines := []struct {
		key   string
		value string
	}{
		{key: "name", value: info.Name},
		{key: "path", value: info.Path},
		{key: "workset", value: info.Workset},
		{key: "branch", value: info.Branch},
		{key: "next", value: info.Next},
	}

	maxKey := 0
	for _, line := range lines {
		if len(line.key) > maxKey {
			maxKey = len(line.key)
		}
	}

	for _, line := range lines {
		key := fmt.Sprintf("%-*s", maxKey, line.key)
		if styles.Enabled {
			key = styles.Render(styles.Key, key)
		}
		value := line.value
		if line.key == "next" {
			value = styles.Render(styles.Accent, value)
		} else if styles.Enabled {
			value = styles.Render(styles.Value, value)
		}
		if _, err := fmt.Fprintf(w, "%s  %s\n", key, value); err != nil {
			return err
		}
	}
	return nil
}
