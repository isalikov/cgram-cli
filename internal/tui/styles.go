package tui

import "github.com/charmbracelet/lipgloss"

// Nord palette
var (
	// Polar Night
	Nord0 = lipgloss.Color("#2E3440")
	Nord1 = lipgloss.Color("#3B4252")
	Nord2 = lipgloss.Color("#434C5E")
	Nord3 = lipgloss.Color("#4C566A")

	// Snow Storm
	Nord4 = lipgloss.Color("#D8DEE9")
	Nord5 = lipgloss.Color("#E5E9F0")
	Nord6 = lipgloss.Color("#ECEFF4")

	// Frost
	Nord7  = lipgloss.Color("#8FBCBB")
	Nord8  = lipgloss.Color("#88C0D0")
	Nord9  = lipgloss.Color("#81A1C1")
	Nord10 = lipgloss.Color("#5E81AC")

	// Aurora
	Nord11 = lipgloss.Color("#BF616A") // red
	Nord12 = lipgloss.Color("#D08770") // orange
	Nord13 = lipgloss.Color("#EBCB8B") // yellow
	Nord14 = lipgloss.Color("#A3BE8C") // green
	Nord15 = lipgloss.Color("#B48EAD") // purple

	// Logo gradient colors
	LogoColors = []lipgloss.Color{
		lipgloss.Color("#5E81AC"),
		lipgloss.Color("#81A1C1"),
		lipgloss.Color("#88C0D0"),
		lipgloss.Color("#A3BE8C"),
		lipgloss.Color("#EBCB8B"),
		lipgloss.Color("#D08770"),
	}
)

// Common styles (kept for welcome, help, notification)
var (
	BaseBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
	}

	PanelStyle = lipgloss.NewStyle().
			Border(BaseBorder).
			BorderForeground(Nord3)

	ActivePanelStyle = lipgloss.NewStyle().
				Border(BaseBorder).
				BorderForeground(Nord14)

	TitleStyle = lipgloss.NewStyle().
			Foreground(Nord14).
			Bold(true)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Nord3)

	LabelStyle = lipgloss.NewStyle().
			Foreground(Nord3)

	ValueStyle = lipgloss.NewStyle().
			Foreground(Nord6)

	OnlineStyle = lipgloss.NewStyle().
			Foreground(Nord14)

	OfflineStyle = lipgloss.NewStyle().
			Foreground(Nord11)

	SelectedStyle = lipgloss.NewStyle().
			Background(Nord2).
			Foreground(Nord6)

	UnreadBadge = lipgloss.NewStyle().
			Foreground(Nord13).
			Bold(true)

	MyMessageStyle = lipgloss.NewStyle().
			Foreground(Nord9)

	TheirMessageStyle = lipgloss.NewStyle().
				Foreground(Nord14)

	TimestampStyle = lipgloss.NewStyle().
			Foreground(Nord3)

	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.Border{Top: "─"}).
			BorderForeground(Nord3)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(Nord3).
			Background(Nord1).
			Padding(0, 1)

	CommandStyle = lipgloss.NewStyle().
			Foreground(Nord13)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Nord11)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Nord14)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(Nord14).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(Nord4)

	PlaceholderStyle = lipgloss.NewStyle().
				Foreground(Nord3)

	InputFieldStyle = lipgloss.NewStyle().
			Border(BaseBorder).
			BorderForeground(Nord3).
			Padding(0, 1)

	ActiveInputFieldStyle = lipgloss.NewStyle().
				Border(BaseBorder).
				BorderForeground(Nord14).
				Padding(0, 1)

	ButtonStyle = lipgloss.NewStyle().
			Foreground(Nord0).
			Background(Nord14).
			Padding(0, 3).
			Bold(true)

	InactiveButtonStyle = lipgloss.NewStyle().
				Foreground(Nord4).
				Background(Nord2).
				Padding(0, 3)

	ToastStyle = lipgloss.NewStyle().
			Border(BaseBorder).
			BorderForeground(Nord14).
			Background(Nord1).
			Foreground(Nord6).
			Padding(0, 2)

	ToastErrorStyle = lipgloss.NewStyle().
			Border(BaseBorder).
			BorderForeground(Nord11).
			Background(Nord1).
			Foreground(Nord11).
			Padding(0, 2)
)

// ── New styles for redesigned UI ──

var (
	// Top bar
	TopBarStyle = lipgloss.NewStyle().
			Background(Nord1).
			Foreground(Nord4)

	AppNameStyle = lipgloss.NewStyle().
			Foreground(Nord14).
			Bold(true)

	TopBarHintStyle = lipgloss.NewStyle().
			Foreground(Nord3)

	// Separator between panels
	SeparatorStyle = lipgloss.NewStyle().
			Foreground(Nord3)

	// Contacts panel
	ContactsHeaderStyle = lipgloss.NewStyle().
				Foreground(Nord14).
				Bold(true)

	ContactCountStyle = lipgloss.NewStyle().
				Foreground(Nord3)

	ContactNameStyle = lipgloss.NewStyle().
				Foreground(Nord4)

	SelectedContactStyle = lipgloss.NewStyle().
				Foreground(Nord14).
				Bold(true)

	OnlineDotStyle = lipgloss.NewStyle().
			Foreground(Nord14)

	OfflineDotStyle = lipgloss.NewStyle().
			Foreground(Nord11)

	IdleDotStyle = lipgloss.NewStyle().
			Foreground(Nord3)

	ContactHintStyle = lipgloss.NewStyle().
				Foreground(Nord3)

	// Chat header
	ChatHeaderNameStyle = lipgloss.NewStyle().
				Foreground(Nord14).
				Bold(true)

	ChatHeaderStatusStyle = lipgloss.NewStyle().
				Foreground(Nord14)

	ChatHeaderDimStyle = lipgloss.NewStyle().
				Foreground(Nord3)

	// Bubble borders
	BubbleBorderStyle = lipgloss.NewStyle().
				Foreground(Nord14)

	BubbleTextStyle = lipgloss.NewStyle().
			Foreground(Nord4)

	// Names in bubbles
	TheirNameStyle = lipgloss.NewStyle().
			Foreground(Nord14)

	MyNameStyle = lipgloss.NewStyle().
			Foreground(Nord7)

	// Chat separators (top/bottom rounded lines)
	ChatSeparatorStyle = lipgloss.NewStyle().
				Foreground(Nord3)

	// Mode line
	ModeNormalBadge = lipgloss.NewStyle().
			Foreground(Nord14).
			Bold(true)

	ModeInsertBadge = lipgloss.NewStyle().
			Foreground(Nord14).
			Bold(true)

	ModeCommandBadge = lipgloss.NewStyle().
			Foreground(Nord13).
			Bold(true)

	ModeHintStyle = lipgloss.NewStyle().
			Foreground(Nord3)

	// Input line
	PromptStyle = lipgloss.NewStyle().
			Foreground(Nord14).
			Bold(true)

	InputPlaceholderStyle = lipgloss.NewStyle().
				Foreground(Nord3)

	ColStyle = lipgloss.NewStyle().
			Foreground(Nord3)

	// Status bar badges
	StatusNormalBadge = lipgloss.NewStyle().
				Foreground(Nord0).
				Background(Nord14).
				Bold(true).
				Padding(0, 1)

	StatusInsertBadge = lipgloss.NewStyle().
				Foreground(Nord0).
				Background(Nord14).
				Bold(true).
				Padding(0, 1)

	StatusCommandBadge = lipgloss.NewStyle().
				Foreground(Nord0).
				Background(Nord13).
				Bold(true).
				Padding(0, 1)

	StatusConnectedStyle = lipgloss.NewStyle().
				Foreground(Nord14)

	StatusDisconnectedStyle = lipgloss.NewStyle().
				Foreground(Nord11)

	StatusE2EStyle = lipgloss.NewStyle().
			Foreground(Nord13)

	StatusPaneActiveStyle = lipgloss.NewStyle().
				Foreground(Nord6).
				Bold(true)

	StatusPaneDimStyle = lipgloss.NewStyle().
				Foreground(Nord3)
)
