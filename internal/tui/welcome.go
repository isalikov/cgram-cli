package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var logoLines = []string{
	`   ██████╗██████╗ ██████╗  █████╗ ███╗   ███╗`,
	`  ██╔════╝██╔═══╝ ██╔══██╗██╔══██╗████╗ ████║`,
	`  ██║     ██║ ██╗ ██████╔╝███████║██╔████╔██║`,
	`  ██║     ██║  ██║██╔══██╗██╔══██║██║╚██╔╝██║`,
	`  ╚██████╗╚█████╔╝██║  ██║██║  ██║██║ ╚═╝ ██║`,
	`   ╚═════╝ ╚════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝`,
}

type welcomeField int

const (
	fieldUsername welcomeField = iota
	fieldPassword
	fieldLoginBtn
	fieldRegisterBtn
)

type WelcomeModel struct {
	username   InputModel
	password   InputModel
	active     welcomeField
	serverAddr string
	connected  bool
	err        string
	loading    bool
	autoLogin  bool
	width      int
	height     int
}

func NewWelcome(serverAddr string, connected bool) WelcomeModel {
	u := NewInput("username")
	p := NewMaskedInput("password")
	u.Focus()
	return WelcomeModel{
		username:   u,
		password:   p,
		active:     fieldUsername,
		serverAddr: serverAddr,
		connected:  connected,
	}
}

func (m WelcomeModel) Update(msg tea.Msg) (WelcomeModel, tea.Cmd) {
	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Tab cycles through fields
		if isKey(msg, "tab") {
			m.active = (m.active + 1) % 4
			m.updateFocus()
			return m, nil
		}

		if isKey(msg, "shift+tab") {
			m.active = (m.active + 3) % 4
			m.updateFocus()
			return m, nil
		}

		// Enter on buttons triggers action
		if isKey(msg, "enter") {
			switch m.active {
			case fieldUsername:
				m.active = fieldPassword
				m.updateFocus()
				return m, nil
			case fieldPassword:
				m.active = fieldLoginBtn
				m.updateFocus()
				return m, nil
			case fieldLoginBtn:
				return m, m.loginCmd()
			case fieldRegisterBtn:
				return m, m.registerCmd()
			}
		}

		// Update active input
		switch m.active {
		case fieldUsername:
			m.username, _ = m.username.Update(msg)
		case fieldPassword:
			m.password, _ = m.password.Update(msg)
		}
	}

	return m, nil
}

func (m *WelcomeModel) updateFocus() {
	m.username.Blur()
	m.password.Blur()
	switch m.active {
	case fieldUsername:
		m.username.Focus()
	case fieldPassword:
		m.password.Focus()
	}
}

func (m *WelcomeModel) SetConnected(c bool)  { m.connected = c }
func (m *WelcomeModel) SetError(e string)     { m.err = e }
func (m *WelcomeModel) SetSize(w, h int)      { m.width = w; m.height = h }
func (m *WelcomeModel) SetLoading(l bool)     { m.loading = l }
func (m *WelcomeModel) SetAutoLogin(a bool)   { m.autoLogin = a }
func (m *WelcomeModel) Username() string      { return m.username.Value() }
func (m *WelcomeModel) Password() string      { return m.password.Value() }

type LoginMsg struct {
	Username string
	Password string
}

type RegisterMsg struct {
	Username string
	Password string
}

func (m WelcomeModel) loginCmd() tea.Cmd {
	u, p := m.username.Value(), m.password.Value()
	return func() tea.Msg {
		return LoginMsg{Username: u, Password: p}
	}
}

func (m WelcomeModel) registerCmd() tea.Cmd {
	u, p := m.username.Value(), m.password.Value()
	return func() tea.Msg {
		return RegisterMsg{Username: u, Password: p}
	}
}

func (m WelcomeModel) View() string {
	// Logo with gradient
	var logo strings.Builder
	for i, line := range logoLines {
		color := LogoColors[i%len(LogoColors)]
		logo.WriteString(lipgloss.NewStyle().Foreground(color).Render(line))
		if i < len(logoLines)-1 {
			logo.WriteString("\n")
		}
	}

	// Info block (neofetch style)
	statusColor := OnlineStyle
	statusText := "connected"
	if !m.connected {
		statusColor = OfflineStyle
		statusText = "disconnected"
	}

	info := strings.Join([]string{
		ValueStyle.Bold(true).Render("user@cgram"),
		SubtitleStyle.Render("──────────"),
		LabelStyle.Render("server: ") + ValueStyle.Render(m.serverAddr),
		LabelStyle.Render("status: ") + statusColor.Render(statusText),
		LabelStyle.Render("e2e:    ") + ValueStyle.Render("enabled"),
	}, "\n")

	infoBlock := lipgloss.NewStyle().
		MarginLeft(4).
		MarginTop(1).
		Render(info)

	header := lipgloss.JoinHorizontal(lipgloss.Top, logo.String(), infoBlock)

	// Auto-login preloader — show only logo + status
	if m.autoLogin {
		status := SubtitleStyle.Render("Connecting...")
		if m.connected {
			status = SubtitleStyle.Render("Authenticating...")
		}
		content := lipgloss.JoinVertical(lipgloss.Center,
			header,
			"",
			"",
			status,
		)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	// Form
	usernameField := InputFieldStyle
	passwordField := InputFieldStyle
	if m.active == fieldUsername {
		usernameField = ActiveInputFieldStyle
	}
	if m.active == fieldPassword {
		passwordField = ActiveInputFieldStyle
	}

	fieldWidth := 36

	form := lipgloss.JoinVertical(lipgloss.Left,
		LabelStyle.Render("Username"),
		usernameField.Width(fieldWidth).Render(m.username.View()),
		"",
		LabelStyle.Render("Password"),
		passwordField.Width(fieldWidth).Render(m.password.View()),
	)

	// Buttons or loading
	var buttons string
	if m.loading {
		buttons = SubtitleStyle.Render("  Authenticating...")
	} else {
		loginBtn := InactiveButtonStyle.Render("Login")
		registerBtn := InactiveButtonStyle.Render("Register")
		if m.active == fieldLoginBtn {
			loginBtn = ButtonStyle.Render("Login")
		}
		if m.active == fieldRegisterBtn {
			registerBtn = ButtonStyle.Render("Register")
		}
		buttons = lipgloss.JoinHorizontal(lipgloss.Center, loginBtn, "  ", registerBtn)
	}

	// Error
	var errView string
	if m.err != "" {
		errView = "\n" + ErrorStyle.Render(fmt.Sprintf("  %s", m.err))
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		header,
		"",
		"",
		form,
		"",
		buttons,
		errView,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
