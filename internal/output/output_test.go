package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewStylesPlain(t *testing.T) {
	styles := NewStyles(&bytes.Buffer{}, true)
	if styles.Enabled {
		t.Fatal("expected styles to be disabled when plain is true")
	}
}

func TestNewStylesNonTerminal(t *testing.T) {
	styles := NewStyles(&bytes.Buffer{}, false)
	if styles.Enabled {
		t.Fatal("expected styles to be disabled for non-terminal writer")
	}
}

func TestStylesRenderDisabled(t *testing.T) {
	styles := Styles{Enabled: false}
	style := lipgloss.NewStyle().Bold(true)
	got := styles.Render(style, "hello")
	if got != "hello" {
		t.Fatalf("expected render to return input when disabled, got %q", got)
	}
}

func TestStylesRenderEnabled(t *testing.T) {
	styles := Styles{Enabled: true}
	style := lipgloss.NewStyle().Bold(true)
	got := styles.Render(style, "hello")
	want := style.Render("hello")
	if got != want {
		t.Fatalf("expected render to match style output, got %q want %q", got, want)
	}
}

func TestWriteJSON(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	want := payload{Name: "demo", Count: 2}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, want); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	var got payload
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("WriteJSON output invalid: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected payload: got %+v want %+v", got, want)
	}
	if !strings.Contains(buf.String(), "\n  \"name\": \"demo\"") {
		t.Fatalf("expected indented JSON, got %q", buf.String())
	}
}

func TestRenderTableDisabled(t *testing.T) {
	styles := Styles{Enabled: false}
	got := RenderTable(styles, []string{"A", "B"}, [][]string{{"1", "2"}, {"3", "4"}})
	want := "A\tB\n1\t2\n3\t4\n"
	if got != want {
		t.Fatalf("unexpected table output: got %q want %q", got, want)
	}
}

func TestRenderTableEnabled(t *testing.T) {
	styles := Styles{
		Enabled: true,
		Title:   lipgloss.NewStyle().Bold(true),
		Value:   lipgloss.NewStyle(),
		Muted:   lipgloss.NewStyle(),
	}
	got := RenderTable(styles, []string{"REPO", "STATE"}, [][]string{{"repo", "clean"}})
	if !strings.Contains(got, "REPO") || !strings.Contains(got, "repo") {
		t.Fatalf("expected rendered table to contain headers and rows, got %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected rendered table to end with newline, got %q", got)
	}
}

func TestPrintStatusPlain(t *testing.T) {
	var buf bytes.Buffer
	styles := Styles{Enabled: false}
	rows := []StatusRow{{Name: "repo1", State: "clean", Detail: "ok"}}
	if err := PrintStatus(&buf, styles, rows); err != nil {
		t.Fatalf("PrintStatus failed: %v", err)
	}
	want := "REPO\tSTATE\tDETAIL\nrepo1\tclean\tok\n"
	if buf.String() != want {
		t.Fatalf("unexpected status output: got %q want %q", buf.String(), want)
	}
}

func TestPrintWorkspaceCreatedPlain(t *testing.T) {
	var buf bytes.Buffer
	info := WorkspaceCreated{
		Name:    "demo",
		Path:    "/tmp/demo",
		Workset: "ws",
		Branch:  "main",
		Next:    "next-step",
	}
	if err := PrintWorkspaceCreated(&buf, info, Styles{Enabled: false}); err != nil {
		t.Fatalf("PrintWorkspaceCreated failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "workspace created" {
		t.Fatalf("unexpected title: %q", lines[0])
	}
	if !strings.Contains(lines[1], "name") || !strings.Contains(lines[1], info.Name) {
		t.Fatalf("missing name line: %q", lines[1])
	}
	if !strings.Contains(lines[2], "path") || !strings.Contains(lines[2], info.Path) {
		t.Fatalf("missing path line: %q", lines[2])
	}
	if !strings.Contains(lines[3], "workset") || !strings.Contains(lines[3], info.Workset) {
		t.Fatalf("missing workset line: %q", lines[3])
	}
	if !strings.Contains(lines[4], "branch") || !strings.Contains(lines[4], info.Branch) {
		t.Fatalf("missing branch line: %q", lines[4])
	}
	if !strings.Contains(lines[5], "next") || !strings.Contains(lines[5], info.Next) {
		t.Fatalf("missing next line: %q", lines[5])
	}
}
