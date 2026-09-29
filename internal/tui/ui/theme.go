package ui

import "github.com/charmbracelet/lipgloss"

// Invopop palette — using ANSI 256 colors for broad terminal compatibility.
// The hex values are in comments for reference.
var (
	// Core colors — ANSI 256 palette
	Green     = lipgloss.ANSIColor(35)  // #00af5f — bright green
	DarkGreen = lipgloss.ANSIColor(22)  // #005f00 — dark green
	DarkestBg = lipgloss.ANSIColor(232) // near black
	Teal      = lipgloss.ANSIColor(29)  // #00875f — teal green
	MidGreen  = lipgloss.ANSIColor(23)  // #005f5f — used for selection bg
	Purple    = lipgloss.ANSIColor(53)  // dark purple
	White     = lipgloss.ANSIColor(15)  // bright white
	OffWhite  = lipgloss.ANSIColor(252) // light gray
	Amber     = lipgloss.ANSIColor(214) // #ffaf00 — amber/orange
	Red       = lipgloss.ANSIColor(196) // bright red
	Cyan      = lipgloss.ANSIColor(37)  // #00afaf

	// Semantic aliases
	Primary   = Green
	Secondary = Teal
	Accent    = Purple
	Success   = Green
	Warning   = Amber
	Danger    = Red
	Muted     = lipgloss.ANSIColor(65)  // #5f875f — muted green
	Border    = lipgloss.ANSIColor(65)  // #5f875f — muted green for borders
	FaintText = lipgloss.ANSIColor(108) // #87af87 — faint green
	TextColor = White

	// Layout constants
	SidebarWidth = 26
	FooterHeight = 1
	HeaderHeight = 3

	// ── Sidebar ──────────────────────────────────────────

	SidebarStyle = lipgloss.NewStyle().
			Width(SidebarWidth)

	SidebarItemStyle = lipgloss.NewStyle().
				Foreground(OffWhite).
				PaddingLeft(2)

	SidebarSelectedStyle = lipgloss.NewStyle().
				Foreground(Green).
				Bold(true).
				PaddingLeft(1)

	SidebarHeaderStyle = lipgloss.NewStyle().
				Foreground(FaintText).
				Bold(true).
				PaddingLeft(2).
				MarginTop(1).
				MarginBottom(0)

	// ── Content ──────────────────────────────────────────

	ContentStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(1).
			PaddingTop(1)

	// ── Typography ───────────────────────────────────────

	TitleStyle = lipgloss.NewStyle().
			Foreground(White).
			Bold(true)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(FaintText)

	// ── Table ────────────────────────────────────────────

	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(FaintText).
				Bold(true)

	TableRowStyle = lipgloss.NewStyle().
			Foreground(OffWhite)

	TableSelectedStyle = lipgloss.NewStyle().
				Background(DarkGreen).
				Foreground(White).
				Bold(true)

	TableDimStyle = lipgloss.NewStyle().
			Foreground(FaintText)

	// ── Workflow steps ───────────────────────────────────

	StepBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Green).
			Padding(0, 1)

	StepBoxSelectedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Amber).
				Padding(0, 1)

	RescueBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Red).
			Padding(0, 1)

	// ── Badges ───────────────────────────────────────────

	BadgeDraft = lipgloss.NewStyle().
			Foreground(DarkestBg).
			Background(Amber).
			Padding(0, 1).
			Bold(true)

	BadgeSigned = lipgloss.NewStyle().
			Foreground(DarkestBg).
			Background(Green).
			Padding(0, 1).
			Bold(true)

	BadgeError = lipgloss.NewStyle().
			Foreground(White).
			Background(Red).
			Padding(0, 1).
			Bold(true)

	BadgeState = lipgloss.NewStyle().
			Foreground(DarkestBg).
			Background(Teal).
			Padding(0, 1)

	// ── Help / Footer ────────────────────────────────────

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(Green).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(FaintText)

	HelpSepStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// ── Info box ─────────────────────────────────────────

	InfoBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Muted).
			Padding(0, 1)

	// ── Input ────────────────────────────────────────────

	InputLabelStyle = lipgloss.NewStyle().
			Foreground(Green).
			Bold(true)
)

// HelpEntry renders a single key/description pair for the footer.
func HelpEntry(key, desc string) string {
	return HelpKeyStyle.Render(key) + " " + HelpDescStyle.Render(desc)
}

// HelpBar renders a row of help entries separated by dots.
func HelpBar(entries ...string) string {
	sep := HelpSepStyle.Render(" · ")
	result := ""
	for i, item := range entries {
		if i > 0 {
			result += sep
		}
		result += item
	}
	return result
}
