package output

import (
	"fmt"
	"io"
)

type StatusRow struct {
	Name   string
	State  string
	Detail string
}

func PrintStatus(w io.Writer, styles Styles, rows []StatusRow) error {
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		name := row.Name
		state := row.State
		detail := row.Detail

		if styles.Enabled {
			name = styles.Render(styles.Accent, name)
			switch row.State {
			case "clean":
				state = styles.Render(styles.Success, state)
			case "dirty":
				state = styles.Render(styles.Warn, state)
			case "missing":
				state = styles.Render(styles.Error, state)
			case "error":
				state = styles.Render(styles.Error, state)
			default:
				state = styles.Render(styles.Muted, state)
			}
		}

		tableRows = append(tableRows, []string{name, state, detail})
	}

	rendered := RenderTable(styles, []string{"REPO", "STATE", "DETAIL"}, tableRows)
	_, err := fmt.Fprint(w, rendered)
	return err
}
