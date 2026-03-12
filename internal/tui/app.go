package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "github.com/isalikov/cgram-proto/gen/proto"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/isalikov/cgram-cli/internal/client"
	"github.com/isalikov/cgram-cli/internal/crypto"
	"github.com/isalikov/cgram-cli/internal/store"
)

const (
	panelContacts = iota
	panelChat
)

const (
	contactsWidthPct = 25 // percentage of screen
	minContactsWidth = 20
	maxContactsWidth = 40
)

// Messages
type (
	tickMsg           time.Time
	pushFrameMsg      *pb.Frame
	contactsLoadedMsg []*pb.Contact
	statsLoadedMsg    *pb.StatsResponse
	messageSentMsg    store.Message
	reconnectedMsg    struct{}
	disconnectedMsg   struct{}
	statusMsg         struct {
		text    string
		isError bool
	}
)

type AppModel struct {
	client   *client.Client
	store    *store.Store
	identity *crypto.KeyPair
	username string
	platform string

	contacts  ContactsModel
	chat      ChatModel
	statusBar StatusBarModel

	activePanel int
	textInput   textinput.Model
	filterMode  bool // true when typing /query to filter contacts

	// Confirm mode for destructive actions
	confirmAction string // description of pending action
	confirmCmd    tea.Cmd

	statusText    string
	statusIsError bool
	statusExpiry  time.Time

	width  int
	height int
	ready  bool
}

func NewApp(c *client.Client, s *store.Store, identity *crypto.KeyPair, username, platform string) AppModel {
	ti := textinput.New()
	ti.Focus()
	ti.Prompt = "> "
	ti.PromptStyle = inputPromptStyle
	ti.TextStyle = lipgloss.NewStyle().Foreground(nord6)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(nord9)
	ti.CharLimit = 4096

	return AppModel{
		client:    c,
		store:     s,
		identity:  identity,
		username:  username,
		platform:  platform,
		contacts:  NewContactsModel(),
		chat:      NewChatModel(username),
		statusBar: NewStatusBarModel(username, platform),
		textInput: ti,
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadContacts,
		m.loadStats,
		m.waitForPush,
		m.tickEverySecond,
		m.watchConnection,
		textinput.Blink,
	)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.updateLayout()
		m.textInput.Width = m.width - 4
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		return m, m.tickEverySecond

	case pushFrameMsg:
		return m.handlePush(msg)

	case contactsLoadedMsg:
		m.contacts.SetContacts(msg)
		m.statusBar.SetContactCount(m.contacts.Count())
		for _, c := range msg {
			if err := m.store.SaveContact(c.UserId, c.Username); err != nil {
				// log error but continue
			}
		}
		return m, nil

	case statsLoadedMsg:
		if msg != nil {
			m.statusBar.SetStats(msg.TotalUsers, msg.OnlineUsers)
		}
		return m, nil

	case messageSentMsg:
		if m.chat.PeerID() == msg.PeerID {
			m.chat.AddMessage(store.Message(msg))
		}
		return m, nil

	case disconnectedMsg:
		m.statusBar.SetConnected(false)
		return m, tea.Batch(
			m.setStatus("connection lost, reconnecting...", true),
			m.reconnect,
		)

	case reconnectedMsg:
		m.statusBar.SetConnected(true)
		return m, tea.Batch(
			m.setStatus("reconnected", false),
			m.waitForPush,
			m.watchConnection,
			m.loadContacts,
			m.loadStats,
		)

	case statusMsg:
		m.statusText = msg.text
		m.statusIsError = msg.isError
		m.statusExpiry = time.Now().Add(5 * time.Second)
		return m, nil
	}

	// Pass through to textinput for cursor blink etc.
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m AppModel) View() string {
	if !m.ready {
		return "connecting..."
	}

	// Calculate widths
	contactsW := m.width * contactsWidthPct / 100
	if contactsW < minContactsWidth {
		contactsW = minContactsWidth
	}
	if contactsW > maxContactsWidth {
		contactsW = maxContactsWidth
	}
	chatW := m.width - contactsW - 1 // -1 for separator

	mainHeight := m.height - 3 // status bar + input + extra line

	// Contacts panel
	m.contacts.SetSize(contactsW, mainHeight)
	contactsView := m.contacts.View(m.activePanel == panelContacts)

	// Chat panel
	m.chat.SetSize(chatW, mainHeight)
	chatView := m.chat.View()

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(nord3).
		Height(mainHeight)
	separator := sepStyle.Render(strings.Repeat("|\n", mainHeight))

	// Main area: contacts | separator | chat
	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, contactsView, separator, chatView)

	// Input line
	inputView := m.renderInput()

	// Status bar
	m.statusBar.SetSize(m.width)
	m.statusBar.SetContactCount(m.contacts.Count())
	statusBarView := m.statusBar.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		mainArea,
		inputView,
		statusBarView,
	)
}

func (m *AppModel) updateLayout() {
	contactsW := m.width * contactsWidthPct / 100
	if contactsW < minContactsWidth {
		contactsW = minContactsWidth
	}
	if contactsW > maxContactsWidth {
		contactsW = maxContactsWidth
	}
	chatW := m.width - contactsW - 1
	mainHeight := m.height - 3

	m.contacts.SetSize(contactsW, mainHeight)
	m.chat.SetSize(chatW, mainHeight)
	m.statusBar.SetSize(m.width)
}

func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle confirm mode first
	if m.confirmAction != "" {
		switch key {
		case "y", "Y":
			cmd := m.confirmCmd
			m.confirmAction = ""
			m.confirmCmd = nil
			return m, cmd
		default:
			m.confirmAction = ""
			m.confirmCmd = nil
			return m, m.setStatus("cancelled", false)
		}
	}

	// Global keys
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		if m.filterMode {
			m.filterMode = false
			m.contacts.SetFilter("")
			m.textInput.SetValue("")
			m.textInput.Prompt = "> "
		}
		if m.activePanel == panelContacts {
			m.activePanel = panelChat
		} else {
			m.activePanel = panelContacts
		}
		return m, nil
	case "esc":
		if m.filterMode {
			m.filterMode = false
			m.contacts.SetFilter("")
			m.textInput.SetValue("")
			m.textInput.Prompt = "> "
			return m, nil
		}
		if m.textInput.Value() != "" {
			m.textInput.SetValue("")
			return m, nil
		}
		return m, nil
	}

	// Contacts panel navigation (only when input is empty and not in filter mode)
	if m.activePanel == panelContacts && !m.filterMode && m.textInput.Value() == "" {
		switch key {
		case "up", "k":
			m.contacts.MoveUp()
			return m, nil
		case "down", "j":
			m.contacts.MoveDown()
			return m, nil
		case "enter":
			return m.selectContact()
		case "/":
			m.filterMode = true
			m.textInput.SetValue("")
			m.textInput.Prompt = "/ "
			return m, nil
		}
	}

	// Chat panel scrolling (when input is empty)
	if m.activePanel == panelChat && m.textInput.Value() == "" {
		switch key {
		case "up":
			m.chat.ScrollUp()
			return m, nil
		case "down":
			m.chat.ScrollDown()
			return m, nil
		case "pgup":
			for i := 0; i < 10; i++ {
				m.chat.ScrollUp()
			}
			return m, nil
		case "pgdown":
			for i := 0; i < 10; i++ {
				m.chat.ScrollDown()
			}
			return m, nil
		}
	}

	// Filter mode input
	if m.filterMode {
		switch key {
		case "enter":
			m.filterMode = false
			m.textInput.Prompt = "> "
			// Select first match
			if sel := m.contacts.Selected(); sel != nil {
				m.textInput.SetValue("")
				return m.openChat(sel)
			}
			m.textInput.SetValue("")
			return m, nil
		default:
			// Let textinput handle the keystroke
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			m.contacts.SetFilter(m.textInput.Value())
			return m, cmd
		}
	}

	// Enter key handling
	if key == "enter" {
		return m.handleEnter()
	}

	// Update prompt style based on input
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Dynamically update prompt
	val := m.textInput.Value()
	if strings.HasPrefix(val, ":") {
		m.textInput.Prompt = ": "
	} else {
		m.textInput.Prompt = "> "
	}

	return m, cmd
}

func (m AppModel) handleEnter() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.textInput.Value())
	if text == "" {
		return m, nil
	}

	// Command handling
	if strings.HasPrefix(text, ":") {
		return m.handleCommand(text)
	}

	// Send message
	if m.chat.PeerID() == "" {
		m.textInput.SetValue("")
		return m, m.setStatus("no contact selected", true)
	}

	m.textInput.SetValue("")
	return m, m.sendMessage(m.chat.PeerID(), m.chat.PeerName(), text)
}

func (m AppModel) handleCommand(text string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return m, nil
	}

	cmd := strings.ToLower(parts[0])
	m.textInput.SetValue("")

	switch cmd {
	case ":add":
		if len(parts) < 2 {
			return m, m.setStatus(":add <username>", true)
		}
		return m, m.addContact(parts[1])

	case ":delete":
		if len(parts) < 2 {
			return m, m.setStatus(":delete <username>", true)
		}
		username := parts[1]
		m.confirmAction = fmt.Sprintf("delete contact '%s'? (y/n)", username)
		m.confirmCmd = m.deleteContact(username)
		return m, nil

	case ":rename":
		if len(parts) < 3 {
			return m, m.setStatus(":rename <username> <new_alias>", true)
		}
		if err := m.store.RenameContact(parts[1], parts[2]); err != nil {
			return m, m.setStatus(fmt.Sprintf("rename failed: %v", err), true)
		}
		return m, m.setStatus(fmt.Sprintf("renamed %s -> %s", parts[1], parts[2]), false)

	case ":search":
		if len(parts) < 2 {
			return m, m.setStatus(":search <query>", true)
		}
		query := strings.Join(parts[1:], " ")
		// Search contacts first
		if c := m.contacts.FindByUsername(query); c != nil {
			return m.openChat(c)
		}
		// Search messages
		msgs, err := m.store.SearchMessages(query, 20)
		if err == nil && len(msgs) > 0 {
			return m, m.setStatus(fmt.Sprintf("found %d message(s) for '%s'", len(msgs), query), false)
		}
		return m, m.setStatus(fmt.Sprintf("no results for '%s'", query), true)

	case ":q", ":quit":
		return m, tea.Quit

	default:
		return m, m.setStatus(fmt.Sprintf("unknown command: %s", cmd), true)
	}
}

func (m AppModel) selectContact() (tea.Model, tea.Cmd) {
	sel := m.contacts.Selected()
	if sel == nil {
		return m, nil
	}
	return m.openChat(sel)
}

func (m AppModel) openChat(c *ContactEntry) (tea.Model, tea.Cmd) {
	m.chat.SetPeer(c.UserID, c.Username)
	m.contacts.ClearUnread(c.UserID)
	m.activePanel = panelChat

	// Load messages from store
	msgs, err := m.store.GetMessages(c.UserID, 100, 0)
	if err != nil {
		return m, m.setStatus(fmt.Sprintf("load messages: %v", err), true)
	}
	m.chat.SetMessages(msgs)

	return m, nil
}

func (m AppModel) handlePush(frame *pb.Frame) (tea.Model, tea.Cmd) {
	if frame == nil {
		// Connection closed — watchConnection will handle reconnect
		return m, nil
	}

	switch p := frame.Payload.(type) {
	case *pb.Frame_Envelope:
		return m.handleIncomingEnvelope(p.Envelope)
	case *pb.Frame_PresenceEvent:
		m.contacts.UpdatePresence(p.PresenceEvent.UserId, p.PresenceEvent.Online)
		status := "offline"
		if p.PresenceEvent.Online {
			status = "online"
		}
		return m, tea.Batch(
			m.setStatus(fmt.Sprintf("%s is %s", p.PresenceEvent.Username, status), false),
			m.waitForPush,
		)
	}

	return m, m.waitForPush
}

func (m AppModel) handleIncomingEnvelope(env *pb.Envelope) (tea.Model, tea.Cmd) {
	// For now, treat ciphertext as plaintext (simplified — full E2E TBD)
	body := string(env.Ciphertext)
	senderID := string(env.SenderSealed)

	// Find sender name from contacts
	senderName := senderID
	for _, c := range m.contacts.contacts {
		if c.UserID == senderID {
			senderName = c.Username
			break
		}
	}

	now := time.Now()
	if env.Timestamp > 0 {
		now = time.Unix(env.Timestamp, 0)
	}

	msg := store.Message{
		PeerID:    senderID,
		Sender:    senderName,
		Body:      body,
		Timestamp: now,
		Outgoing:  false,
	}

	if err := m.store.SaveMessage(msg.PeerID, msg.Sender, msg.Body, msg.Timestamp, false); err != nil {
		// log save error but continue displaying the message
	}

	if m.chat.PeerID() == senderID {
		m.chat.AddMessage(msg)
	} else {
		m.contacts.IncrementUnread(senderID)
	}

	return m, m.waitForPush
}

func (m AppModel) renderInput() string {
	// Show confirm prompt if active
	if m.confirmAction != "" {
		prompt := inputPromptStyle.Render("? ") + inputStyle.Render(m.confirmAction)
		return inputStyle.Width(m.width).Render(prompt)
	}

	// Show status message alongside input if active
	if m.statusText != "" && time.Now().Before(m.statusExpiry) {
		style := infoStyle
		if m.statusIsError {
			style = errorStyle
		}
		right := style.Render(m.statusText)
		left := m.textInput.View()
		gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
		if gap < 1 {
			gap = 1
		}
		return inputStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
	}

	return inputStyle.Width(m.width).Render(m.textInput.View())
}

// Commands that return tea.Cmd
func (m AppModel) loadContacts() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	contacts, err := m.client.ListContacts(ctx)
	if err != nil {
		return statusMsg{text: fmt.Sprintf("failed to load contacts: %v", err), isError: true}
	}
	return contactsLoadedMsg(contacts)
}

func (m AppModel) loadStats() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stats, err := m.client.GetStats(ctx)
	if err != nil {
		return nil
	}
	return statsLoadedMsg(stats)
}

func (m AppModel) waitForPush() tea.Msg {
	frame := m.client.WaitForPush()
	return pushFrameMsg(frame)
}

func (m AppModel) tickEverySecond() tea.Msg {
	time.Sleep(time.Second)
	return tickMsg(time.Now())
}

// watchConnection waits for the current connection to drop, then signals the TUI.
func (m AppModel) watchConnection() tea.Msg {
	<-m.client.Done()
	return disconnectedMsg{}
}

// reconnect attempts to re-establish the WebSocket connection.
func (m AppModel) reconnect() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := m.client.ReconnectLoop(ctx); err != nil {
		return statusMsg{text: fmt.Sprintf("reconnect failed: %v", err), isError: true}
	}
	return reconnectedMsg{}
}

func (m AppModel) setStatus(text string, isError bool) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{text: text, isError: isError}
	}
}

func (m AppModel) sendMessage(peerID, peerName, text string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		now := time.Now()

		// Simplified: send plaintext for now (full E2E encryption TBD)
		env := &pb.Envelope{
			RecipientId:  peerID,
			SenderSealed: []byte(m.username),
			Ciphertext:   []byte(text),
			Timestamp:    now.Unix(),
		}

		if err := m.client.SendEnvelope(ctx, env); err != nil {
			return statusMsg{text: fmt.Sprintf("send failed: %v", err), isError: true}
		}

		// Save locally
		if err := m.store.SaveMessage(peerID, m.username, text, now, true); err != nil {
			// continue — message was sent, just not saved locally
		}

		return messageSentMsg(store.Message{
			PeerID:    peerID,
			Sender:    m.username,
			Body:      text,
			Timestamp: now,
			Outgoing:  true,
		})
	}
}

func (m AppModel) addContact(username string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := m.client.AddContact(ctx, username)
		if err != nil {
			return statusMsg{text: fmt.Sprintf("add failed: %v", err), isError: true}
		}

		if err := m.store.SaveContact(resp.UserId, resp.Username); err != nil {
			// non-fatal
		}

		// Reload contacts
		contacts, _ := m.client.ListContacts(ctx)
		if contacts != nil {
			return contactsLoadedMsg(contacts)
		}

		return statusMsg{text: fmt.Sprintf("added %s", username), isError: false}
	}
}

func (m AppModel) deleteContact(username string) tea.Cmd {
	return func() tea.Msg {
		// Find user ID by username
		userID, err := m.store.GetContactUserID(username)
		if err != nil {
			return statusMsg{text: fmt.Sprintf("contact '%s' not found", username), isError: true}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := m.client.RemoveContact(ctx, userID); err != nil {
			return statusMsg{text: fmt.Sprintf("delete failed: %v", err), isError: true}
		}

		if err := m.store.DeleteContact(userID); err != nil {
			// non-fatal
		}

		// Reload contacts
		contacts, _ := m.client.ListContacts(ctx)
		if contacts != nil {
			return contactsLoadedMsg(contacts)
		}

		return statusMsg{text: fmt.Sprintf("deleted %s", username), isError: false}
	}
}
