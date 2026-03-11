package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Toast struct {
	Message string
	IsError bool
	ShowAt  time.Time
}

type toastTickMsg struct{}
type dismissToastMsg struct{}

func toastTick() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return dismissToastMsg{}
	})
}

func (t Toast) View(width int) string {
	style := ToastStyle
	if t.IsError {
		style = ToastErrorStyle
	}

	rendered := style.Width(width - 4).Render(t.Message)
	return lipgloss.Place(width, lipgloss.Height(rendered), lipgloss.Center, lipgloss.Top, rendered)
}
