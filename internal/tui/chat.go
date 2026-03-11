package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/isalikov/cgram-cli/internal/store"
)

type ChatModel struct {
	messages    []store.Message
	contactName string
	contactID   string
	focused     bool
	width       int
	height      int
	scrollOff   int
	myUsername  string
}

func NewChat() ChatModel {
	return ChatModel{}
}

func (m *ChatModel) SetMessages(msgs []store.Message)  { m.messages = msgs; m.scrollOff = 0 }
func (m *ChatModel) SetContact(id, name string)        { m.contactID = id; m.contactName = name }
func (m *ChatModel) SetFocused(f bool)                 { m.focused = f }
func (m *ChatModel) SetSize(w, h int)                  { m.width = w; m.height = h }
func (m *ChatModel) SetMyUsername(u string)             { m.myUsername = u }
func (m ChatModel) ContactID() string                   { return m.contactID }
func (m ChatModel) ContactName() string                 { return m.contactName }
func (m *ChatModel) AppendMessage(msg store.Message)    { m.messages = append(m.messages, msg) }
func (m ChatModel) MessageCount() int                   { return len(m.messages) }

func (m *ChatModel) ScrollUp(n int) {
	m.scrollOff += n
}

func (m *ChatModel) ScrollDown(n int) {
	m.scrollOff -= n
	if m.scrollOff < 0 {
		m.scrollOff = 0
	}
}

func (m ChatModel) msgAreaHeight() int {
	// Total height - header(1) - top sep(1) - bottom sep(1)
	h := m.height - 3
	if h < 1 {
		h = 1
	}
	return h
}

func (m ChatModel) View() string {
	if m.contactID == "" {
		empty := lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			SubtitleStyle.Render("Select a contact to start chatting"),
		)
		return empty
	}

	// Header: @name [online] left, last seen: now right
	headerLeft := ChatHeaderNameStyle.Render("@"+m.contactName) + " " +
		ChatHeaderStatusStyle.Render("[online]")
	headerRight := ChatHeaderDimStyle.Render("last seen: now")
	headerPad := m.width - lipgloss.Width(headerLeft) - lipgloss.Width(headerRight)
	if headerPad < 1 {
		headerPad = 1
	}
	header := headerLeft + strings.Repeat(" ", headerPad) + headerRight

	// Top separator ╭──────╮
	topSep := ChatSeparatorStyle.Render("╭" + strings.Repeat("─", max(0, m.width-2)) + "╮")

	// Messages area
	msgHeight := m.msgAreaHeight()
	msgWidth := m.width - 2 // some margin

	var msgArea string

	if len(m.messages) == 0 {
		msgArea = lipgloss.Place(
			m.width, msgHeight,
			lipgloss.Center, lipgloss.Center,
			SubtitleStyle.Render("No messages yet. Say hello!"),
		)
	} else {
		var allLines []string
		for _, msg := range m.messages {
			rendered := m.renderMessage(msg, msgWidth)
			allLines = append(allLines, strings.Split(rendered, "\n")...)
		}

		totalLines := len(allLines)
		scrollOff := m.scrollOff
		maxScroll := max(0, totalLines-msgHeight)
		if scrollOff > maxScroll {
			scrollOff = maxScroll
		}

		start := max(0, totalLines-msgHeight-scrollOff)
		end := start + msgHeight
		if end > totalLines {
			end = totalLines
		}

		visible := allLines[start:end]
		msgArea = strings.Join(visible, "\n")

		lineCount := len(visible)
		if lineCount < msgHeight {
			msgArea += strings.Repeat("\n", msgHeight-lineCount)
		}
	}

	// Bottom separator ╰──────╯
	bottomSep := ChatSeparatorStyle.Render("╰" + strings.Repeat("─", max(0, m.width-2)) + "╯")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		topSep,
		msgArea,
		bottomSep,
	)
}

func (m ChatModel) renderMessage(msg store.Message, width int) string {
	timeStr := TimestampStyle.Render("[" + formatTimestamp(msg.Timestamp) + "]")

	if msg.IsMine {
		label := MyNameStyle.Render("you") + " " + timeStr
		contentWidth := max(10, width*2/3)

		wrapped := wrapText(msg.Content, contentWidth)
		wLines := strings.Split(wrapped, "\n")

		var result []string
		// Label right-aligned
		labelPad := width - lipgloss.Width(label)
		if labelPad < 0 {
			labelPad = 0
		}
		result = append(result, strings.Repeat(" ", labelPad)+label)

		// Content lines right-aligned with > arrow
		arrow := MyNameStyle.Render("> ")
		for _, line := range wLines {
			styled := arrow + BubbleTextStyle.Render(line)
			pad := width - lipgloss.Width(styled)
			if pad < 0 {
				pad = 0
			}
			result = append(result, strings.Repeat(" ", pad)+styled)
		}
		return strings.Join(result, "\n")
	}

	// Left-aligned
	label := TheirNameStyle.Render("@"+m.contactName) + " " + timeStr
	contentWidth := max(10, width*2/3)

	wrapped := wrapText(msg.Content, contentWidth)
	wLines := strings.Split(wrapped, "\n")

	var result []string
	result = append(result, label)

	arrow := TheirNameStyle.Render("< ")
	for _, line := range wLines {
		result = append(result, " "+arrow+BubbleTextStyle.Render(line))
	}
	return strings.Join(result, "\n")
}

func formatTimestamp(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 02 15:04")
	}
	return t.Format("2006-01-02 15:04")
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if lipgloss.Width(paragraph) <= width {
			lines = append(lines, paragraph)
			continue
		}
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		current := words[0]
		for _, word := range words[1:] {
			test := current + " " + word
			if lipgloss.Width(test) <= width {
				current = test
			} else {
				lines = append(lines, current)
				current = word
			}
		}
		lines = append(lines, current)
	}

	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

