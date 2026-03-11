package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type InputModel struct {
	lines       [][]rune
	cursorRow   int
	cursorCol   int
	focused     bool
	placeholder string
	width       int
	masked      bool
}

func NewInput(placeholder string) InputModel {
	return InputModel{
		lines:       [][]rune{{}},
		placeholder: placeholder,
		width:       40,
	}
}

func NewMaskedInput(placeholder string) InputModel {
	m := NewInput(placeholder)
	m.masked = true
	return m
}

func (m *InputModel) Focus()         { m.focused = true }
func (m *InputModel) Blur()          { m.focused = false }
func (m *InputModel) Focused() bool  { return m.focused }
func (m *InputModel) SetWidth(w int) { m.width = w }
func (m InputModel) CursorCol() int  { return m.cursorCol + 1 }

func (m *InputModel) Value() string {
	parts := make([]string, len(m.lines))
	for i, line := range m.lines {
		parts[i] = string(line)
	}
	return strings.Join(parts, "\n")
}

func (m *InputModel) Reset() {
	m.lines = [][]rune{{}}
	m.cursorRow = 0
	m.cursorCol = 0
}

func (m *InputModel) SetValue(s string) {
	strLines := strings.Split(s, "\n")
	m.lines = make([][]rune, len(strLines))
	for i, sl := range strLines {
		m.lines[i] = []rune(sl)
	}
	if len(m.lines) == 0 {
		m.lines = [][]rune{{}}
	}
	m.cursorRow = len(m.lines) - 1
	m.cursorCol = len(m.lines[m.cursorRow])
}

func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyRunes:
			line := m.lines[m.cursorRow]
			newLine := make([]rune, 0, len(line)+len(msg.Runes))
			newLine = append(newLine, line[:m.cursorCol]...)
			newLine = append(newLine, msg.Runes...)
			newLine = append(newLine, line[m.cursorCol:]...)
			m.lines[m.cursorRow] = newLine
			m.cursorCol += len(msg.Runes)

		case tea.KeySpace:
			line := m.lines[m.cursorRow]
			newLine := make([]rune, 0, len(line)+1)
			newLine = append(newLine, line[:m.cursorCol]...)
			newLine = append(newLine, ' ')
			newLine = append(newLine, line[m.cursorCol:]...)
			m.lines[m.cursorRow] = newLine
			m.cursorCol++

		case tea.KeyBackspace:
			if m.cursorCol > 0 {
				line := m.lines[m.cursorRow]
				m.lines[m.cursorRow] = append(line[:m.cursorCol-1], line[m.cursorCol:]...)
				m.cursorCol--
			} else if m.cursorRow > 0 {
				prevLen := len(m.lines[m.cursorRow-1])
				m.lines[m.cursorRow-1] = append(m.lines[m.cursorRow-1], m.lines[m.cursorRow]...)
				m.lines = append(m.lines[:m.cursorRow], m.lines[m.cursorRow+1:]...)
				m.cursorRow--
				m.cursorCol = prevLen
			}

		case tea.KeyLeft:
			if m.cursorCol > 0 {
				m.cursorCol--
			}

		case tea.KeyRight:
			if m.cursorCol < len(m.lines[m.cursorRow]) {
				m.cursorCol++
			}

		case tea.KeyUp:
			if m.cursorRow > 0 {
				m.cursorRow--
				if m.cursorCol > len(m.lines[m.cursorRow]) {
					m.cursorCol = len(m.lines[m.cursorRow])
				}
			}

		case tea.KeyDown:
			if m.cursorRow < len(m.lines)-1 {
				m.cursorRow++
				if m.cursorCol > len(m.lines[m.cursorRow]) {
					m.cursorCol = len(m.lines[m.cursorRow])
				}
			}

		case tea.KeyHome, tea.KeyCtrlA:
			m.cursorCol = 0

		case tea.KeyEnd, tea.KeyCtrlE:
			m.cursorCol = len(m.lines[m.cursorRow])
		}
	}

	return m, nil
}

func (m InputModel) InsertNewline() InputModel {
	line := m.lines[m.cursorRow]
	before := make([]rune, m.cursorCol)
	copy(before, line[:m.cursorCol])
	after := make([]rune, len(line)-m.cursorCol)
	copy(after, line[m.cursorCol:])

	m.lines[m.cursorRow] = before
	newLines := make([][]rune, 0, len(m.lines)+1)
	newLines = append(newLines, m.lines[:m.cursorRow+1]...)
	newLines = append(newLines, after)
	newLines = append(newLines, m.lines[m.cursorRow+1:]...)
	m.lines = newLines

	m.cursorRow++
	m.cursorCol = 0
	return m
}

func (m InputModel) View() string {
	if !m.focused && m.Value() == "" {
		return PlaceholderStyle.Render(m.placeholder)
	}

	var parts []string
	for i, line := range m.lines {
		display := line
		if m.masked {
			display = make([]rune, len(line))
			for j := range display {
				display[j] = '\u2022' // bullet •
			}
		}

		if m.focused && i == m.cursorRow {
			if m.cursorCol >= len(display) {
				parts = append(parts, string(display)+lipgloss.NewStyle().Reverse(true).Render(" "))
			} else {
				before := string(display[:m.cursorCol])
				cursor := lipgloss.NewStyle().Reverse(true).Render(string(display[m.cursorCol]))
				after := string(display[m.cursorCol+1:])
				parts = append(parts, before+cursor+after)
			}
		} else {
			if len(display) == 0 {
				parts = append(parts, " ")
			} else {
				parts = append(parts, string(display))
			}
		}
	}

	return strings.Join(parts, "\n")
}
