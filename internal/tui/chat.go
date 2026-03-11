package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isalikov/cgram-cli/internal/store"
)

type ChatModel struct {
	messages    []store.Message
	contactName string
	contactID   string
	input       InputModel
	focused     bool
	width       int
	height      int
	scrollOff   int
	myUsername  string
}

func NewChat() ChatModel {
	return ChatModel{
		input: NewInput("Type a message..."),
	}
}

func (m *ChatModel) SetMessages(msgs []store.Message)  { m.messages = msgs; m.scrollOff = 0 }
func (m *ChatModel) SetContact(id, name string)        { m.contactID = id; m.contactName = name }
func (m *ChatModel) SetFocused(f bool)                 { m.focused = f; if f { m.input.Focus() } else { m.input.Blur() } }
func (m *ChatModel) SetSize(w, h int)                  { m.width = w; m.height = h; m.input.SetWidth(w - 4) }
func (m *ChatModel) SetMyUsername(u string)             { m.myUsername = u }
func (m ChatModel) ContactID() string                   { return m.contactID }
func (m ChatModel) InputValue() string                  { return m.input.Value() }
func (m *ChatModel) ResetInput()                        { m.input.Reset() }
func (m *ChatModel) AppendMessage(msg store.Message)    { m.messages = append(m.messages, msg) }

type SendMessageMsg struct {
	ContactID string
	Content   string
}

func (m ChatModel) Update(msg tea.Msg) (ChatModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case isKey(msg, "alt+enter"):
			m.input = m.input.InsertNewline()
			return m, nil

		case isKey(msg, "enter"):
			content := strings.TrimSpace(m.input.Value())
			if content != "" {
				m.input.Reset()
				return m, func() tea.Msg {
					return SendMessageMsg{ContactID: m.contactID, Content: content}
				}
			}
			return m, nil

		case isKey(msg, "pgup"):
			m.scrollOff += 5
			return m, nil

		case isKey(msg, "pgdown"):
			m.scrollOff -= 5
			if m.scrollOff < 0 {
				m.scrollOff = 0
			}
			return m, nil

		default:
			m.input, _ = m.input.Update(msg)
		}
	}

	return m, nil
}

func (m ChatModel) msgAreaHeight() int {
	// Total height - title(2) - input(3) - borders
	return m.height - 7
}

func (m ChatModel) View() string {
	style := PanelStyle
	if m.focused {
		style = ActivePanelStyle
	}

	if m.contactID == "" {
		empty := lipgloss.Place(
			m.width-2, m.height-2,
			lipgloss.Center, lipgloss.Center,
			SubtitleStyle.Render("Select a contact to start chatting"),
		)
		return style.Width(m.width).Height(m.height).Render(empty)
	}

	// Title
	title := TitleStyle.Render(fmt.Sprintf("  Chat with %s", m.contactName))

	// Messages area
	msgHeight := m.msgAreaHeight()
	msgWidth := m.width - 4

	if msgHeight < 1 {
		msgHeight = 1
	}

	var msgArea string

	if len(m.messages) == 0 {
		// Empty state hint
		msgArea = lipgloss.Place(
			msgWidth, msgHeight,
			lipgloss.Center, lipgloss.Center,
			SubtitleStyle.Render("No messages yet. Say hello!"),
		)
	} else {
		// Build all visual lines (each message may produce multiple lines)
		var allLines []string
		for _, msg := range m.messages {
			rendered := m.renderMessage(msg, msgWidth)
			allLines = append(allLines, strings.Split(rendered, "\n")...)
		}

		// Apply scroll offset (from bottom, in visual line units)
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

		// Pad to fill height
		lineCount := len(visible)
		if lineCount < msgHeight {
			msgArea += strings.Repeat("\n", msgHeight-lineCount)
		}
	}

	// Input area
	inputSep := SubtitleStyle.Render(strings.Repeat("─", m.width-4))

	inputPrefix := lipgloss.NewStyle().Foreground(Nord8).Render("> ")
	inputView := inputPrefix + m.input.View()
	inputHint := SubtitleStyle.Render("  (Alt+Enter for new line)")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		msgArea,
		inputSep,
		inputView,
		inputHint,
	)

	return style.Width(m.width).Height(m.height).Render(content)
}

func (m ChatModel) renderMessage(msg store.Message, width int) string {
	timeStr := TimestampStyle.Render(formatTimestamp(msg.Timestamp))

	if msg.IsMine {
		name := MyMessageStyle.Render(m.myUsername + " ")
		arrow := SubtitleStyle.Render("◄")

		// Calculate available content width
		overhead := lipgloss.Width(timeStr) + lipgloss.Width(name) + lipgloss.Width(arrow) + 6
		contentWidth := width - overhead
		if contentWidth < 10 {
			contentWidth = 10
		}

		wrapped := wrapText(msg.Content, contentWidth)
		wLines := strings.Split(wrapped, "\n")

		var result []string
		for i, line := range wLines {
			styledContent := lipgloss.NewStyle().Foreground(Nord4).Render(line)
			if i == len(wLines)-1 {
				// Last line: include metadata
				right := fmt.Sprintf("%s  %s %s%s", styledContent, timeStr, name, arrow)
				pad := width - lipgloss.Width(right)
				if pad < 0 {
					pad = 0
				}
				result = append(result, strings.Repeat(" ", pad)+right)
			} else {
				// Continuation: right-aligned
				pad := width - lipgloss.Width(styledContent)
				if pad < 0 {
					pad = 0
				}
				result = append(result, strings.Repeat(" ", pad)+styledContent)
			}
		}
		return strings.Join(result, "\n")
	}

	name := TheirMessageStyle.Render(m.contactName)
	arrow := SubtitleStyle.Render("►")

	overhead := lipgloss.Width(timeStr) + lipgloss.Width(name) + lipgloss.Width(arrow) + 8
	contentWidth := width - overhead
	if contentWidth < 10 {
		contentWidth = 10
	}

	wrapped := wrapText(msg.Content, contentWidth)
	wLines := strings.Split(wrapped, "\n")

	var result []string
	indent := "  " + strings.Repeat(" ", lipgloss.Width(arrow)+lipgloss.Width(name)+2)
	for i, line := range wLines {
		styledContent := lipgloss.NewStyle().Foreground(Nord4).Render(line)
		if i == 0 {
			result = append(result, fmt.Sprintf("  %s %s %s  %s", arrow, name, styledContent, timeStr))
		} else {
			result = append(result, indent+styledContent)
		}
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
