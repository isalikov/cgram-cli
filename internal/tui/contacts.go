package tui

import (
	"fmt"
	"strings"

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
func (m *ContactsModel) SetFocused(f bool)  { m.focused = f }
func (m *ContactsModel) SetSize(w, h int)   { m.width = w; m.height = h }
func (m ContactsModel) SelectedContact() *store.Contact {
	if len(m.contacts) == 0 {
		return nil
	}
	return &m.contacts[m.selected]
}

func (m *ContactsModel) MoveUp() {
	if m.selected > 0 {
		m.selected--
		m.ensureVisible()
	}
}

func (m *ContactsModel) MoveDown() {
	if m.selected < len(m.contacts)-1 {
		m.selected++
		m.ensureVisible()
	}
}

func (m *ContactsModel) ensureVisible() {
	visible := m.visibleRows()
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+visible {
		m.offset = m.selected - visible + 1
	}
}

func (m ContactsModel) visibleRows() int {
	// height minus header(1) and footer hint(1)
	v := m.height - 2
	if v < 1 {
		v = 1
	}
	return v
}

func (m ContactsModel) View() string {
	w := m.width

	// Header: CONTACTS  [N]
	header := ContactsHeaderStyle.Render("CONTACTS") + "  " +
		ContactCountStyle.Render(fmt.Sprintf("[%d]", len(m.contacts)))
	header = padRight(header, w)

	if len(m.contacts) == 0 {
		var lines []string
		lines = append(lines, header)
		lines = append(lines, "")
		lines = append(lines, ContactHintStyle.Render("  No contacts yet"))
		lines = append(lines, ContactHintStyle.Render("  :add <username>"))
		// Pad to height
		for len(lines) < m.height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:m.height], "\n")
	}

	visible := m.visibleRows()

	end := m.offset + visible
	if end > len(m.contacts) {
		end = len(m.contacts)
	}

	var lines []string
	lines = append(lines, header)

	for i := m.offset; i < end; i++ {
		c := m.contacts[i]
		lines = append(lines, m.renderContact(c, i == m.selected, w))
	}

	// Pad middle
	for len(lines) < m.height-1 {
		lines = append(lines, "")
	}

	// Footer hint
	hint := ContactHintStyle.Render("j/k:nav") +
		strings.Repeat(" ", max(1, w-20)) +
		ContactHintStyle.Render("Enter:sel")
	lines = append(lines, hint)

	if len(lines) > m.height {
		lines = lines[:m.height]
	}

	return strings.Join(lines, "\n")
}

func (m ContactsModel) renderContact(c store.Contact, selected bool, width int) string {
	// Selector
	sel := "  "
	if selected {
		sel = "> "
	}

	// Status dot
	dot := IdleDotStyle.Render("· ")
	if c.Online {
		dot = OnlineDotStyle.Render("● ")
	}

	// Name
	name := c.Name
	if name == "" {
		name = c.UserID[:8]
	}

	// Unread badge
	badge := ""
	if c.Unread > 0 {
		badge = UnreadBadge.Render(fmt.Sprintf("[%d]", c.Unread))
	}

	// Build left and right parts
	left := sel + dot + name
	right := badge

	// Calculate padding between name and badge
	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}

	line := left + strings.Repeat(" ", pad) + right

	if selected {
		nameStyled := sel + dot + SelectedContactStyle.Render(name)
		pad = width - lipgloss.Width(nameStyled) - lipgloss.Width(right)
		if pad < 1 {
			pad = 1
		}
		line = nameStyled + strings.Repeat(" ", pad) + right
	}

	return line
}

func padRight(s string, w int) string {
	pad := w - lipgloss.Width(s)
	if pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}
