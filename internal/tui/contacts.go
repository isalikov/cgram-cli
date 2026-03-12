package tui

import (
	"fmt"
	"strings"

	pb "github.com/isalikov/cgram-proto/gen/proto"
	"github.com/charmbracelet/lipgloss"
)

type ContactEntry struct {
	UserID   string
	Username string
	Online   bool
	Unread   int
}

type ContactsModel struct {
	contacts []ContactEntry
	cursor   int
	filter   string
	width    int
	height   int
}

func NewContactsModel() ContactsModel {
	return ContactsModel{}
}

func (m *ContactsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *ContactsModel) SetContacts(contacts []*pb.Contact) {
	m.contacts = make([]ContactEntry, len(contacts))
	for i, c := range contacts {
		m.contacts[i] = ContactEntry{
			UserID:   c.UserId,
			Username: c.Username,
			Online:   c.Online,
		}
	}
	// Preserve unread counts
}

func (m *ContactsModel) UpdatePresence(userID string, online bool) {
	for i := range m.contacts {
		if m.contacts[i].UserID == userID {
			m.contacts[i].Online = online
			return
		}
	}
}

func (m *ContactsModel) AddContact(userID, username string, online bool) {
	// Check if already exists
	for i := range m.contacts {
		if m.contacts[i].UserID == userID {
			m.contacts[i].Username = username
			m.contacts[i].Online = online
			return
		}
	}
	m.contacts = append(m.contacts, ContactEntry{
		UserID:   userID,
		Username: username,
		Online:   online,
	})
}

func (m *ContactsModel) RemoveContact(userID string) {
	for i := range m.contacts {
		if m.contacts[i].UserID == userID {
			m.contacts = append(m.contacts[:i], m.contacts[i+1:]...)
			if m.cursor >= len(m.filtered()) && m.cursor > 0 {
				m.cursor--
			}
			return
		}
	}
}

func (m *ContactsModel) IncrementUnread(userID string) {
	for i := range m.contacts {
		if m.contacts[i].UserID == userID {
			m.contacts[i].Unread++
			return
		}
	}
}

func (m *ContactsModel) ClearUnread(userID string) {
	for i := range m.contacts {
		if m.contacts[i].UserID == userID {
			m.contacts[i].Unread = 0
			return
		}
	}
}

func (m *ContactsModel) SetFilter(f string) {
	m.filter = f
	if m.cursor >= len(m.filtered()) {
		m.cursor = 0
	}
}

func (m *ContactsModel) filtered() []ContactEntry {
	if m.filter == "" {
		return m.contacts
	}
	var result []ContactEntry
	query := strings.ToLower(m.filter)
	for _, c := range m.contacts {
		if strings.Contains(strings.ToLower(c.Username), query) {
			result = append(result, c)
		}
	}
	return result
}

func (m *ContactsModel) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *ContactsModel) MoveDown() {
	filtered := m.filtered()
	if m.cursor < len(filtered)-1 {
		m.cursor++
	}
}

func (m *ContactsModel) Selected() *ContactEntry {
	filtered := m.filtered()
	if len(filtered) == 0 {
		return nil
	}
	if m.cursor >= len(filtered) {
		m.cursor = 0
	}
	return &filtered[m.cursor]
}

func (m *ContactsModel) FindByUsername(username string) *ContactEntry {
	query := strings.ToLower(username)
	for i := range m.contacts {
		if strings.ToLower(m.contacts[i].Username) == query {
			return &m.contacts[i]
		}
	}
	// Partial match
	for i := range m.contacts {
		if strings.Contains(strings.ToLower(m.contacts[i].Username), query) {
			return &m.contacts[i]
		}
	}
	return nil
}

func (m *ContactsModel) Count() int {
	return len(m.contacts)
}

func (m *ContactsModel) OnlineCount() int {
	count := 0
	for _, c := range m.contacts {
		if c.Online {
			count++
		}
	}
	return count
}

func (m *ContactsModel) View(active bool) string {
	filtered := m.filtered()

	var b strings.Builder

	// Header
	title := " contacts"
	if m.filter != "" {
		title = fmt.Sprintf(" /%s", m.filter)
	}
	titleStyle := headerStyle.Width(m.width)
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")

	if len(filtered) == 0 {
		if len(m.contacts) == 0 {
			b.WriteString(infoStyle.Render("  no contacts yet"))
			b.WriteString("\n")
			b.WriteString(infoStyle.Render("  :add <username>"))
		} else {
			b.WriteString(infoStyle.Render("  no matches"))
		}
		return lipgloss.NewStyle().Width(m.width).Height(m.height).Render(b.String())
	}

	// Calculate visible range
	maxVisible := m.height - 2 // -1 for header, -1 for padding
	if maxVisible < 1 {
		maxVisible = 1
	}

	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(filtered) {
		endIdx = len(filtered)
	}

	for i := startIdx; i < endIdx; i++ {
		c := filtered[i]

		// Status dot
		dot := offlineDotStyle.Render("●")
		if c.Online {
			dot = onlineDotStyle.Render("●")
		}

		// Username
		name := c.Username

		// Unread badge
		badge := ""
		if c.Unread > 0 {
			badge = " " + unreadBadgeStyle.Render(fmt.Sprintf(" %d ", c.Unread))
		}

		line := fmt.Sprintf(" %s %s%s", dot, name, badge)

		if i == m.cursor && active {
			line = contactSelectedStyle.Width(m.width).Render(line)
		} else {
			line = contactNormalStyle.Width(m.width).Render(line)
		}

		b.WriteString(line)
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	return lipgloss.NewStyle().Width(m.width).Height(m.height).Render(b.String())
}
