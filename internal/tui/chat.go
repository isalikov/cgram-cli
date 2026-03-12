package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/isalikov/cgram-cli/internal/store"
)

var urlRegex = regexp.MustCompile(`https?://[^\s]+`)

type ChatModel struct {
	messages []store.Message
	peerID   string
	peerName string
	username string // our username
	scroll   int    // offset from bottom (0 = latest)
	width    int
	height   int
}

func NewChatModel(username string) ChatModel {
	return ChatModel{username: username}
}

func (m *ChatModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *ChatModel) SetPeer(peerID, peerName string) {
	m.peerID = peerID
	m.peerName = peerName
	m.scroll = 0
}

func (m *ChatModel) SetMessages(msgs []store.Message) {
	m.messages = msgs
	m.scroll = 0
}

func (m *ChatModel) AddMessage(msg store.Message) {
	m.messages = append(m.messages, msg)
	// Stay at bottom when scroll is 0
}

func (m *ChatModel) ScrollUp() {
	maxScroll := len(m.messages) - m.height + 4
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scroll < maxScroll {
		m.scroll++
	}
}

func (m *ChatModel) ScrollDown() {
	if m.scroll > 0 {
		m.scroll--
	}
}

func (m *ChatModel) PeerID() string   { return m.peerID }
func (m *ChatModel) PeerName() string { return m.peerName }

func (m *ChatModel) View() string {
	if m.peerID == "" {
		return m.emptyView()
	}

	var b strings.Builder

	// Header with peer name
	titleStyle := headerStyle.Width(m.width)
	b.WriteString(titleStyle.Render(fmt.Sprintf(" %s", m.peerName)))
	b.WriteString("\n")

	// Available height for messages
	msgHeight := m.height - 2 // header + padding

	if len(m.messages) == 0 {
		center := lipgloss.NewStyle().
			Width(m.width).
			Height(msgHeight).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(nord3)
		b.WriteString(center.Render("no messages yet"))
		return b.String()
	}

	// Calculate visible messages
	endIdx := len(m.messages) - m.scroll
	if endIdx < 0 {
		endIdx = 0
	}
	startIdx := endIdx - msgHeight
	if startIdx < 0 {
		startIdx = 0
	}

	var lines []string
	for i := startIdx; i < endIdx; i++ {
		msg := m.messages[i]
		line := m.renderMessage(msg)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	// Pad to fill height
	renderedLines := strings.Count(content, "\n") + 1
	if renderedLines < msgHeight {
		padding := strings.Repeat("\n", msgHeight-renderedLines)
		content = padding + content
	}

	b.WriteString(content)
	return b.String()
}

func (m *ChatModel) renderMessage(msg store.Message) string {
	timeStr := msg.Timestamp.Format("15:04")
	body := msg.Body

	// Make URLs clickable using OSC 8
	body = urlRegex.ReplaceAllStringFunc(body, func(url string) string {
		return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, url)
	})

	maxWidth := m.width - 6 // padding
	if maxWidth < 20 {
		maxWidth = 20
	}

	// Wrap text if too long (using display width for Unicode support)
	if runewidth.StringWidth(body) > maxWidth {
		body = wrapText(body, maxWidth)
	}

	if msg.Outgoing {
		// Right-aligned
		meta := messageTimeStyle.Render(timeStr) + " " + messageSenderStyle.Render("you")
		bubble := myMessageStyle.Render(body)
		metaLine := lipgloss.NewStyle().Width(m.width - 2).Align(lipgloss.Right).Render(meta)
		bubbleLine := lipgloss.NewStyle().Width(m.width - 2).Align(lipgloss.Right).Render(bubble)
		return metaLine + "\n" + bubbleLine
	}

	// Left-aligned
	meta := messageSenderStyle.Render(msg.Sender) + " " + messageTimeStyle.Render(timeStr)
	bubble := theirMessageStyle.Render(body)
	return meta + "\n" + bubble
}

func (m *ChatModel) emptyView() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(nord3)

	return style.Render("select a contact to start chatting")
}

// wrapText wraps text at the given display width, respecting Unicode character widths.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		lines = append(lines, wrapLine(line, width)...)
	}
	return strings.Join(lines, "\n")
}

func wrapLine(line string, width int) []string {
	if runewidth.StringWidth(line) <= width {
		return []string{line}
	}

	var result []string
	var current strings.Builder
	currentWidth := 0

	words := strings.Fields(line)
	for i, word := range words {
		wordWidth := runewidth.StringWidth(word)

		if currentWidth == 0 {
			// First word on line
			if wordWidth > width {
				// Word is longer than width — break by rune
				for _, r := range word {
					rw := runewidth.RuneWidth(r)
					if currentWidth+rw > width && currentWidth > 0 {
						result = append(result, current.String())
						current.Reset()
						currentWidth = 0
					}
					current.WriteRune(r)
					currentWidth += rw
				}
			} else {
				current.WriteString(word)
				currentWidth = wordWidth
			}
		} else if currentWidth+1+wordWidth <= width {
			current.WriteRune(' ')
			current.WriteString(word)
			currentWidth += 1 + wordWidth
		} else {
			result = append(result, current.String())
			current.Reset()
			currentWidth = 0
			// Re-process current word
			if wordWidth > width {
				for _, r := range word {
					rw := runewidth.RuneWidth(r)
					if currentWidth+rw > width && currentWidth > 0 {
						result = append(result, current.String())
						current.Reset()
						currentWidth = 0
					}
					current.WriteRune(r)
					currentWidth += rw
				}
			} else {
				current.WriteString(word)
				currentWidth = wordWidth
			}
		}
		_ = i
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

// FormatTimestamp formats a timestamp for display.
func FormatTimestamp(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("Jan 02 15:04")
}

