package tui

import (
	"fmt"
	"strings"
	"time"
)

type StatusBarModel struct {
	username    string
	platform    string
	contacts    int
	onlineUsers uint32
	totalUsers  uint32
	connected   bool
	width       int
}

func NewStatusBarModel(username, platform string) StatusBarModel {
	return StatusBarModel{
		username:  username,
		platform:  platform,
		connected: true,
	}
}

func (m *StatusBarModel) SetSize(w int) {
	m.width = w
}

func (m *StatusBarModel) SetStats(total, online uint32) {
	m.totalUsers = total
	m.onlineUsers = online
}

func (m *StatusBarModel) SetContactCount(n int) {
	m.contacts = n
}

func (m *StatusBarModel) SetConnected(v bool) {
	m.connected = v
}

func (m *StatusBarModel) View() string {
	now := time.Now().Format("15:04:05")

	connStatus := encryptionStyle.Render("E2E:X3DH+NaCl")
	if !m.connected {
		connStatus = errorStyle.Render("disconnected")
	}

	parts := []string{
		statusBarKeyStyle.Render(m.username),
		sep(),
		fmt.Sprintf("contacts:%d", m.contacts),
		sep(),
		fmt.Sprintf("online:%d", m.onlineUsers),
		sep(),
		fmt.Sprintf("users:%d", m.totalUsers),
		sep(),
		connStatus,
		sep(),
		m.platform,
		sep(),
		now,
	}

	left := strings.Join(parts, " ")
	return statusBarStyle.Width(m.width).Render(left)
}

func sep() string {
	return statusBarSepStyle.Render("|")
}
