package output

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type Styles struct {
	Enabled bool
	Title   lipgloss.Style
	Key     lipgloss.Style
	Value   lipgloss.Style
	Muted   lipgloss.Style
	Accent  lipgloss.Style
	Success lipgloss.Style
	Warn    lipgloss.Style
	Error   lipgloss.Style
}

func NewStyles(w io.Writer, plain bool) Styles {
	if plain {
		return Styles{Enabled: false}
	}
	enabled := styleEnabled(w)
	if !enabled {
		return Styles{Enabled: false}
	}

	accent := lipgloss.Color("#5AA9E6")
	muted := lipgloss.Color("#94A3B8")
	success := lipgloss.Color("#16A34A")
	warn := lipgloss.Color("#F59E0B")
	bad := lipgloss.Color("#EF4444")

	return Styles{
		Enabled: true,
		Title:   lipgloss.NewStyle().Bold(true).Foreground(accent),
		Key:     lipgloss.NewStyle().Foreground(muted),
		Value:   lipgloss.NewStyle(),
		Muted:   lipgloss.NewStyle().Foreground(muted),
		Accent:  lipgloss.NewStyle().Foreground(accent),
		Success: lipgloss.NewStyle().Foreground(success),
		Warn:    lipgloss.NewStyle().Foreground(warn),
		Error:   lipgloss.NewStyle().Foreground(bad).Bold(true),
	}
}

func (s Styles) Render(style lipgloss.Style, text string) string {
	if !s.Enabled {
		return text
	}
	return style.Render(text)
}

func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func styleEnabled(w io.Writer) bool {
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}
