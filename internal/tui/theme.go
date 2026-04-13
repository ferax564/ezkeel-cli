package tui

import "github.com/charmbracelet/lipgloss"

// Brand colors
const (
	Green = "#00ff88"
	Dim   = "#666666"
	White = "#ffffff"
	Red   = "#ff4444"
	Brand = "⟩_"
)

// Status icons
const (
	IconDone    = "✓"
	IconFail    = "✗"
	IconActive  = "●"
	IconPending = "○"
	IconLive    = "🟢"
	IconDown    = "🔴"
)

// Styles
var (
	GreenStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(Green))
	DimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color(Dim))
	BoldStyle       = lipgloss.NewStyle().Bold(true)
	ErrorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(Red))
	BoxStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(Green)).Padding(1, 2)
	FailureBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(Red)).Padding(1, 2)
)

func Banner() string {
	return GreenStyle.Render(Brand + " ezkeel")
}
