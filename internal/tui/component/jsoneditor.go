package component

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/invopop/cli/internal/tui/ui"
)

// JSONEditor provides a JSON viewer with syntax highlighting
// and a textarea for editing.
type JSONEditor struct {
	textarea textarea.Model
	viewport viewport.Model
	readOnly bool
	err      string
	width    int
	height   int
}

// NewJSONEditor creates a new JSON editor.
func NewJSONEditor() JSONEditor {
	ta := textarea.New()
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(ui.Green)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(ui.DarkGreen)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(ui.Green)
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(ui.OffWhite)
	ta.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(ui.FaintText)
	ta.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(ui.FaintText)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(ui.FaintText)
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(ui.OffWhite)
	ta.BlurredStyle.LineNumber = lipgloss.NewStyle().Foreground(ui.FaintText)
	ta.BlurredStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(ui.FaintText)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(ui.FaintText)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(ui.FaintText)
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle()

	return JSONEditor{
		textarea: ta,
		viewport: vp,
		readOnly: true,
	}
}

// SetSize updates the editor dimensions.
func (e *JSONEditor) SetSize(w, h int) {
	if w < 10 {
		w = 10
	}
	if h < 4 {
		h = 4
	}
	e.width = w
	e.height = h
	e.textarea.SetWidth(w - 2)
	e.textarea.SetHeight(h - 2)
	e.viewport.Width = w
	e.viewport.Height = h
}

// SetContent loads JSON content into the editor.
func (e *JSONEditor) SetContent(data json.RawMessage) {
	var pretty json.RawMessage
	if err := json.Unmarshal(data, &pretty); err == nil {
		formatted, err := json.MarshalIndent(pretty, "", "  ")
		if err == nil {
			content := string(formatted)
			e.textarea.SetValue(content)
			e.setViewportHighlighted(content)
			e.err = ""
			return
		}
	}
	content := string(data)
	e.textarea.SetValue(content)
	e.setViewportHighlighted(content)
}

// Content returns the current editor content as JSON.
func (e *JSONEditor) Content() (json.RawMessage, error) {
	val := e.textarea.Value()
	var js json.RawMessage
	if err := json.Unmarshal([]byte(val), &js); err != nil {
		e.err = "invalid JSON: " + err.Error()
		return nil, err
	}
	e.err = ""
	return js, nil
}

// SetReadOnly toggles read-only mode.
func (e *JSONEditor) SetReadOnly(ro bool) {
	e.readOnly = ro
	if ro {
		e.setViewportHighlighted(e.textarea.Value())
	}
}

// Focus gives focus to the editor.
func (e *JSONEditor) Focus() {
	e.textarea.Focus()
}

// Blur removes focus from the editor.
func (e *JSONEditor) Blur() {
	e.textarea.Blur()
}

// Focused returns whether the editor has focus.
func (e *JSONEditor) Focused() bool {
	return e.textarea.Focused()
}

// Update handles messages.
func (e *JSONEditor) Update(msg tea.Msg) tea.Cmd {
	if e.readOnly {
		var cmd tea.Cmd
		e.viewport, cmd = e.viewport.Update(msg)
		return cmd
	}
	var cmd tea.Cmd
	e.textarea, cmd = e.textarea.Update(msg)
	return cmd
}

// View renders the editor.
func (e JSONEditor) View() string {
	var view string
	if e.readOnly {
		view = e.viewport.View()
	} else {
		view = e.textarea.View()
	}

	if e.err != "" {
		errLine := lipgloss.NewStyle().
			Foreground(ui.Danger).
			Render(e.err)
		return view + "\n" + errLine
	}

	return view
}

func (e *JSONEditor) setViewportHighlighted(content string) {
	lines := strings.Split(content, "\n")
	lineNumWidth := numDigits(len(lines))
	lineNumFmt := lipgloss.NewStyle().Foreground(ui.FaintText)

	var out strings.Builder
	for i, line := range lines {
		numStr := padLeft(i+1, lineNumWidth)
		out.WriteString(lineNumFmt.Render(numStr))
		out.WriteString(" ")
		out.WriteString(highlightJSONLine(line))
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}
	e.viewport.SetContent(out.String())
	e.viewport.GotoTop()
}

func numDigits(n int) int {
	if n < 10 {
		return 1
	}
	if n < 100 {
		return 2
	}
	if n < 1000 {
		return 3
	}
	return 4
}

func padLeft(n, width int) string {
	s := strings.Builder{}
	ns := intToStr(n)
	for range width - len(ns) {
		s.WriteByte(' ')
	}
	s.WriteString(ns)
	return s.String()
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
