package tui

import "github.com/charmbracelet/lipgloss"

// Nord color palette
var (
	// Polar Night
	nord0 = lipgloss.Color("#2E3440")
	nord1 = lipgloss.Color("#3B4252")
	nord2 = lipgloss.Color("#434C5E")
	nord3 = lipgloss.Color("#4C566A")

	// Snow Storm
	nord4 = lipgloss.Color("#D8DEE9")
	nord5 = lipgloss.Color("#E5E9F0")
	nord6 = lipgloss.Color("#ECEFF4")

	// Frost
	nord7  = lipgloss.Color("#8FBCBB")
	nord8  = lipgloss.Color("#88C0D0")
	nord9  = lipgloss.Color("#81A1C1")
	nord10 = lipgloss.Color("#5E81AC")

	// Aurora
	nord11 = lipgloss.Color("#BF616A") // red
	nord12 = lipgloss.Color("#D08770") // orange
	nord13 = lipgloss.Color("#EBCB8B") // yellow
	nord14 = lipgloss.Color("#A3BE8C") // green
	nord15 = lipgloss.Color("#B48EAD") // purple
)

var (
	// Panel styles
	contactsPanelStyle = lipgloss.NewStyle().
		Background(nord0).
		Padding(0, 1)

	contactsPanelActiveStyle = lipgloss.NewStyle().
		Background(nord0).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(nord9).
		BorderLeft(true).
		BorderRight(true).
		Padding(0, 1)

	chatPanelStyle = lipgloss.NewStyle().
		Background(nord0).
		Padding(0, 1)

	// Contact list
	contactSelectedStyle = lipgloss.NewStyle().
		Foreground(nord6).
		Background(nord2).
		Bold(true).
		Padding(0, 1)

	contactNormalStyle = lipgloss.NewStyle().
		Foreground(nord4).
		Padding(0, 1)

	onlineDotStyle  = lipgloss.NewStyle().Foreground(nord14)
	offlineDotStyle = lipgloss.NewStyle().Foreground(nord3)

	unreadBadgeStyle = lipgloss.NewStyle().
		Foreground(nord0).
		Background(nord13).
		Bold(true).
		Padding(0, 1)

	// Messages
	myMessageStyle = lipgloss.NewStyle().
		Foreground(nord6).
		Background(nord10).
		Padding(0, 1).
		MarginLeft(2)

	theirMessageStyle = lipgloss.NewStyle().
		Foreground(nord6).
		Background(nord2).
		Padding(0, 1).
		MarginRight(2)

	messageSenderStyle = lipgloss.NewStyle().
		Foreground(nord8).
		Bold(true)

	messageTimeStyle = lipgloss.NewStyle().
		Foreground(nord3)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
		Background(nord1).
		Foreground(nord4).
		Padding(0, 1)

	statusBarKeyStyle = lipgloss.NewStyle().
		Foreground(nord8).
		Bold(true)

	statusBarSepStyle = lipgloss.NewStyle().
		Foreground(nord3)

	// Input
	inputStyle = lipgloss.NewStyle().
		Background(nord0).
		Foreground(nord6).
		Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().
		Foreground(nord9).
		Bold(true)

	// Header
	headerStyle = lipgloss.NewStyle().
		Background(nord1).
		Foreground(nord8).
		Bold(true).
		Padding(0, 1)

	// Notification / status messages
	infoStyle = lipgloss.NewStyle().
		Foreground(nord8)

	errorStyle = lipgloss.NewStyle().
		Foreground(nord11)

	successStyle = lipgloss.NewStyle().
		Foreground(nord14)

	// Encryption indicator
	encryptionStyle = lipgloss.NewStyle().
		Foreground(nord14).
		Bold(true)
)
