package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpEntry struct {
	Key  string
	Desc string
}

var helpEntries = []helpEntry{
	{"Tab", "Switch between contacts and chat"},
	{"j/k, Up/Down", "Navigate contacts / scroll messages"},
	{"Enter", "Open chat / send message"},
	{"Alt+Enter", "New line in message input"},
	{":", "Command mode"},
	{":add <username>", "Add contact by username"},
	{":rename <name>", "Rename selected contact"},
	{":delete", "Delete selected contact"},
	{":quit, :q", "Quit application"},
	{":help", "Show this help"},
	{"Esc", "Close overlay / exit command mode"},
	{"Ctrl+C", "Quit"},
}

func renderHelp(width, height int) string {
	title := TitleStyle.Render("Keyboard Shortcuts")

	var lines []string
	lines = append(lines, title)
	lines = append(lines, strings.Repeat("─", 40))

	for _, e := range helpEntries {
		key := HelpKeyStyle.Width(20).Render(e.Key)
		desc := HelpDescStyle.Render(e.Desc)
		lines = append(lines, key+desc)
	}

	lines = append(lines, "")
	lines = append(lines, SubtitleStyle.Render("Press Esc to close"))

	content := strings.Join(lines, "\n")

	box := PanelStyle.
		Width(50).
		Padding(1, 2).
		Background(Nord0).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
