package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isalikov/cgram-cli/internal/store"
)

type ContactsModel struct {
	contacts []store.Contact
	selected int
	focused  bool
	width    int
	height   int
	offset   int
}

func NewContacts() ContactsModel {
	return ContactsModel{}
}

func (m *ContactsModel) SetContacts(c []store.Contact) {
	m.contacts = c
	if m.selected >= len(c) {
		m.selected = max(0, len(c)-1)
	}
}
func (m *ContactsModel) SetFocused(f bool)             { m.focused = f }
func (m *ContactsModel) SetSize(w, h int)              { m.width = w; m.height = h }
func (m ContactsModel) SelectedContact() *store.Contact {
	if len(m.contacts) == 0 {
		return nil
	}
	return &m.contacts[m.selected]
}

func (m ContactsModel) Update(msg tea.Msg) (ContactsModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case isKey(msg, "j", "down"):
			if m.selected < len(m.contacts)-1 {
				m.selected++
				m.ensureVisible()
			}
		case isKey(msg, "k", "up"):
			if m.selected > 0 {
				m.selected--
				m.ensureVisible()
			}
		}
	}

	return m, nil
}

func (m *ContactsModel) ensureVisible() {
	visible := m.height - 4 // account for borders and title
	if visible < 1 {
		visible = 1
	}
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+visible {
		m.offset = m.selected - visible + 1
	}
}

func (m ContactsModel) View() string {
	style := PanelStyle
	if m.focused {
		style = ActivePanelStyle
	}

	title := TitleStyle.Render("  Contacts")

	if len(m.contacts) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			SubtitleStyle.Render("  No contacts yet"),
			SubtitleStyle.Render("  :add <username>"),
		)
		return style.Width(m.width).Height(m.height).Render(content)
	}

	visible := m.height - 4
	if visible < 1 {
		visible = 1
	}

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "")

	end := m.offset + visible
	if end > len(m.contacts) {
		end = len(m.contacts)
	}

	for i := m.offset; i < end; i++ {
		c := m.contacts[i]
		line := m.renderContact(c, i == m.selected)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	return style.Width(m.width).Height(m.height).Render(content)
}

func (m ContactsModel) renderContact(c store.Contact, selected bool) string {
	// Online indicator
	indicator := OfflineStyle.Render("  ")
	if c.Online {
		indicator = OnlineStyle.Render("● ")
	}

	// Name
	name := c.Name
	if name == "" {
		name = c.UserID[:8]
	}

	// Unread badge
	badge := ""
	if c.Unread > 0 {
		badge = " " + UnreadBadge.Render(fmt.Sprintf("[%d]", c.Unread))
	}

	line := fmt.Sprintf(" %s%s%s", indicator, name, badge)

	if selected {
		// Pad to width
		padded := line + strings.Repeat(" ", max(0, m.width-4-lipgloss.Width(line)))
		return SelectedStyle.Render(padded)
	}

	return line
}
