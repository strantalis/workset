package output

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

func RenderTable(styles Styles, headers []string, rows [][]string) string {
	if !styles.Enabled {
		var b strings.Builder
		if len(headers) > 0 {
			b.WriteString(strings.Join(headers, "\t"))
			b.WriteString("\n")
		}
		for _, row := range rows {
			b.WriteString(strings.Join(row, "\t"))
			b.WriteString("\n")
		}
		return b.String()
	}

	headerStyle := styles.Title
	cellStyle := styles.Value.Padding(0, 1)
	borderStyle := styles.Muted

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(borderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		}).
		Headers(headers...).
		Rows(rows...)

	return t.String() + "\n"
}
