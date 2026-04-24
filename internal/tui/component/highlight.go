// Package component defines reusable bubble tea components (status bar,
// JSON editor/highlighter) used across TUI pages.
package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/invopop/cli/internal/tui/ui"
)

var (
	jsonKeyStyle    = lipgloss.NewStyle().Foreground(ui.Green)
	jsonStringStyle = lipgloss.NewStyle().Foreground(ui.OffWhite)
	jsonNumberStyle = lipgloss.NewStyle().Foreground(ui.Amber)
	jsonBoolStyle   = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(37)) // cyan
	jsonNullStyle   = lipgloss.NewStyle().Foreground(ui.Muted)
	jsonBraceStyle  = lipgloss.NewStyle().Foreground(ui.FaintText)
	jsonPunctStyle  = lipgloss.NewStyle().Foreground(ui.Muted)
)

// highlightJSONLine applies syntax coloring to a single line of
// pretty-printed JSON, preserving leading indentation.
func highlightJSONLine(line string) string {
	// Find leading whitespace
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]

	if trimmed == "" {
		return line
	}

	var out strings.Builder
	out.WriteString(indent)

	s := trimmed
	pos := 0

	for pos < len(s) {
		ch := s[pos]

		switch {
		case ch == '"':
			// Find end of string
			end := findStringEnd(s, pos)
			str := s[pos : end+1]

			// Check if this is a key (followed by ':')
			rest := strings.TrimLeft(s[end+1:], " \t")
			if len(rest) > 0 && rest[0] == ':' {
				// It's a key
				out.WriteString(jsonKeyStyle.Render(str))
			} else {
				// It's a string value
				out.WriteString(jsonStringStyle.Render(str))
			}
			pos = end + 1

		case ch == ':':
			out.WriteString(jsonPunctStyle.Render(":"))
			pos++
			// Skip space after colon
			if pos < len(s) && s[pos] == ' ' {
				out.WriteByte(' ')
				pos++
			}

		case ch == ',':
			out.WriteString(jsonPunctStyle.Render(","))
			pos++

		case ch == '{' || ch == '}' || ch == '[' || ch == ']':
			out.WriteString(jsonBraceStyle.Render(string(ch)))
			pos++

		case ch == 't' || ch == 'f':
			// true or false
			if strings.HasPrefix(s[pos:], "true") {
				out.WriteString(jsonBoolStyle.Render("true"))
				pos += 4
			} else if strings.HasPrefix(s[pos:], "false") {
				out.WriteString(jsonBoolStyle.Render("false"))
				pos += 5
			} else {
				out.WriteByte(ch)
				pos++
			}

		case ch == 'n':
			if strings.HasPrefix(s[pos:], "null") {
				out.WriteString(jsonNullStyle.Render("null"))
				pos += 4
			} else {
				out.WriteByte(ch)
				pos++
			}

		case ch == '-' || (ch >= '0' && ch <= '9'):
			// Number
			end := pos + 1
			for end < len(s) && isNumberChar(s[end]) {
				end++
			}
			out.WriteString(jsonNumberStyle.Render(s[pos:end]))
			pos = end

		case ch == ' ' || ch == '\t':
			out.WriteByte(ch)
			pos++

		default:
			out.WriteByte(ch)
			pos++
		}
	}

	return out.String()
}

func findStringEnd(s string, start int) int {
	for i := start + 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // skip escaped char
			continue
		}
		if s[i] == '"' {
			return i
		}
	}
	return len(s) - 1
}

func isNumberChar(ch byte) bool {
	return (ch >= '0' && ch <= '9') || ch == '.' || ch == 'e' || ch == 'E' || ch == '+' || ch == '-'
}
