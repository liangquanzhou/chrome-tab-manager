package tui

import "github.com/charmbracelet/lipgloss"

// Dracula color palette
var (
	colorPrimary     = lipgloss.Color("#bd93f9") // purple - current view highlight
	colorAccent      = lipgloss.Color("#50fa7b") // green - success / connected
	colorWarn        = lipgloss.Color("#f1fa8c") // yellow - warning
	colorDanger      = lipgloss.Color("#ff5555") // red - error
	colorInfo        = lipgloss.Color("#8be9fd") // cyan - secondary info
	colorDim         = lipgloss.Color("#6272a4") // comment - muted text
	colorURL         = lipgloss.Color("#8be9fd") // cyan - URL / domain
	colorPurple      = lipgloss.Color("#ff79c6") // pink - bookmarks / session accent
	colorRowActiveBg = lipgloss.Color("#44475a") // selection - cursor row background
	colorText        = lipgloss.Color("#f8f8f2") // foreground - body text
	colorOrange      = lipgloss.Color("#ffb86c") // orange - flags / counts
)

var (
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)

	styleActiveTab = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#282a36")).
			Background(colorPrimary).
			Padding(0, 1)

	styleInactiveTab = lipgloss.NewStyle().
				Foreground(colorDim).
				Padding(0, 1)

	styleCursor   = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	styleSelected = lipgloss.NewStyle().Foreground(colorWarn)

	styleError   = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)
	styleToast   = lipgloss.NewStyle().Foreground(colorAccent)
	styleConfirm = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)

	styleDim       = lipgloss.NewStyle().Foreground(colorDim)
	styleStatusBar = lipgloss.NewStyle().Foreground(colorText)
	styleFilter    = lipgloss.NewStyle().Foreground(colorWarn)
	styleHelp      = lipgloss.NewStyle().Foreground(colorDim)

	// Item rendering styles
	styleTitle      = lipgloss.NewStyle().Foreground(colorText)
	styleURLDomain  = lipgloss.NewStyle().Foreground(colorURL)
	styleFlags      = lipgloss.NewStyle().Foreground(colorOrange)
	styleAccent     = lipgloss.NewStyle().Foreground(colorAccent)
	stylePurple     = lipgloss.NewStyle().Foreground(colorPurple)
	styleInfo       = lipgloss.NewStyle().Foreground(colorInfo)
	styleDanger     = lipgloss.NewStyle().Foreground(colorDanger)
	styleRowActive  = lipgloss.NewStyle().Background(colorRowActiveBg)
	styleGroupColor = lipgloss.NewStyle().Bold(true)
)
