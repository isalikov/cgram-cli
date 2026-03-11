package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	pb "github.com/isalikov/cgram-proto/gen/proto"
	"github.com/isalikov/cgram-cli/internal/client"
	"github.com/isalikov/cgram-cli/internal/crypto"
	"github.com/isalikov/cgram-cli/internal/notify"
	"github.com/isalikov/cgram-cli/internal/store"
	"google.golang.org/protobuf/proto"
)

type screen int

const (
	screenWelcome screen = iota
	screenMain
)

type focus int

const (
	focusContacts focus = iota
	focusChat
)

type AppModel struct {
	screen     screen
	focus      focus
	welcome    WelcomeModel
	contacts   ContactsModel
	chat       ChatModel
	toast      *Toast
	showHelp   bool
	commandMode bool
	commandBuf  string
	width      int
	height     int

	client   *client.Client
	store    *store.Store // nil until login/register
	identity *store.Identity

	dataDir      string
	serverAddr   string
	wasConnected bool

	// Kept in memory for re-auth on reconnect
	authUsername string
	authPassword string
}

// Messages
type connectedMsg struct{}
type disconnectedMsg struct{}
type incomingFrameMsg struct{ frame *pb.Frame }
type authSuccessMsg struct {
	store    *store.Store
	identity *store.Identity
	token    string
	username string
	password string
}
type authErrorMsg struct{ err string }
type contactsUpdatedMsg struct{}
type messagesUpdatedMsg struct{}
type addContactResultMsg struct {
	userID   string
	username string
	err      string
}
type sendErrorMsg struct{ err string }

func NewApp(cl *client.Client, dataDir string, serverAddr string) AppModel {
	return AppModel{
		screen:     screenWelcome,
		client:     cl,
		dataDir:    dataDir,
		serverAddr: serverAddr,
		welcome:    NewWelcome(serverAddr, cl.Connected()),
		contacts:   NewContacts(),
		chat:       NewChat(),
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		tea.WindowSize(),
		m.connectCmd(),
		m.waitForStatus(),
	)
}

func (m AppModel) connectCmd() tea.Cmd {
	cl := m.client
	return func() tea.Msg {
		ctx := context.Background()
		if err := cl.Connect(ctx); err != nil {
			return disconnectedMsg{}
		}
		return connectedMsg{}
	}
}

// waitForStatus listens for connection state changes from the client.
func (m AppModel) waitForStatus() tea.Cmd {
	ch := m.client.Status
	return func() tea.Msg {
		connected := <-ch
		if connected {
			return connectedMsg{}
		}
		return disconnectedMsg{}
	}
}

// openStore creates ~/.cgram/ if needed and opens ~/.cgram/<username>.db
func openStore(dataDir, username string) (*store.Store, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(dataDir, username+".db")
	return store.New(dbPath)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.welcome.SetSize(msg.Width, msg.Height)
		m.updateLayout()
		return m, nil

	case tea.KeyMsg:
		// Global keys
		if isKey(msg, "ctrl+c") {
			if m.store != nil {
				m.store.Close()
			}
			return m, tea.Quit
		}

		// Help overlay toggle
		if m.showHelp {
			if isKey(msg, "esc") {
				m.showHelp = false
			}
			return m, nil
		}

		// Command mode
		if m.commandMode {
			return m.handleCommandMode(msg)
		}

		if m.screen == screenMain && isKey(msg, ":") {
			m.commandMode = true
			m.commandBuf = ""
			return m, nil
		}

		// Screen-specific keys
		switch m.screen {
		case screenWelcome:
			var cmd tea.Cmd
			m.welcome, cmd = m.welcome.Update(msg)
			cmds = append(cmds, cmd)

		case screenMain:
			if isKey(msg, "tab") {
				if m.focus == focusContacts {
					m.focus = focusChat
				} else {
					m.focus = focusContacts
				}
				m.updateFocus()
				return m, nil
			}

			if m.focus == focusContacts && isKey(msg, "enter") {
				if c := m.contacts.SelectedContact(); c != nil {
					m.openChat(c)
				}
				return m, nil
			}

			switch m.focus {
			case focusContacts:
				var cmd tea.Cmd
				m.contacts, cmd = m.contacts.Update(msg)
				cmds = append(cmds, cmd)
			case focusChat:
				var cmd tea.Cmd
				m.chat, cmd = m.chat.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case connectedMsg:
		m.welcome.SetConnected(true)
		m.wasConnected = true
		// Re-authenticate if we have stored credentials (reconnect scenario)
		if m.authUsername != "" && m.authPassword != "" && m.store != nil {
			cmds = append(cmds, m.reAuthCmd())
		}
		cmds = append(cmds, m.waitForStatus())

	case disconnectedMsg:
		m.welcome.SetConnected(false)
		if m.wasConnected {
			go notify.ConnectionError("Lost connection to server")
		}
		cmds = append(cmds, m.waitForStatus())

	case LoginMsg:
		m.welcome.SetLoading(true)
		return m, m.doLogin(msg.Username, msg.Password)

	case RegisterMsg:
		m.welcome.SetLoading(true)
		return m, m.doRegister(msg.Username, msg.Password)

	case authSuccessMsg:
		m.welcome.SetLoading(false)
		m.store = msg.store
		m.identity = msg.identity
		m.authUsername = msg.username
		m.authPassword = msg.password
		m.screen = screenMain
		m.focus = focusContacts
		m.updateFocus()
		m.updateLayout()
		// Set up re-auth on reconnect
		cl := m.client
		username := msg.username
		password := msg.password
		cl.OnReconnect(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cl.Send(ctx, &pb.Frame{
				RequestId: uuid.NewString(),
				Payload: &pb.Frame_LoginRequest{LoginRequest: &pb.LoginRequest{
					Username:    username,
					AuthMessage: []byte(password),
				}},
			})
		})
		toastCmd := m.showToast(fmt.Sprintf("Your ID: %s", msg.identity.UserID), false)
		return m, tea.Batch(toastCmd, m.loadContactsCmd(), m.waitForMessage())

	case authErrorMsg:
		m.welcome.SetLoading(false)
		m.welcome.SetError(msg.err)

	case contactsUpdatedMsg:
		if m.store != nil {
			contacts, _ := m.store.ListContacts()
			m.contacts.SetContacts(contacts)
		}

	case messagesUpdatedMsg:
		if m.store != nil && m.chat.ContactID() != "" {
			msgs, _ := m.store.GetMessages(m.chat.ContactID(), 500)
			m.chat.SetMessages(msgs)
		}

	case addContactResultMsg:
		if msg.err != "" {
			cmds = append(cmds, m.showToast(msg.err, true))
		} else {
			m.store.AddContact(msg.userID, msg.username)
			cmds = append(cmds, m.showToast(fmt.Sprintf("Added %s", msg.username), false))
			cmds = append(cmds, m.loadContactsCmd())
		}

	case SendMessageMsg:
		return m, m.sendMessageCmd(msg.ContactID, msg.Content)

	case sendErrorMsg:
		cmds = append(cmds, m.showToast("Failed to send: "+msg.err, true))

	case incomingFrameMsg:
		cmd := m.handleIncoming(msg.frame)
		// Re-subscribe for next message
		return m, tea.Batch(cmd, m.waitForMessage())

	case dismissToastMsg:
		m.toast = nil

	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) handleCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isKey(msg, "esc"):
		m.commandMode = false
		m.commandBuf = ""
	case isKey(msg, "enter"):
		cmd := m.executeCommand(m.commandBuf)
		m.commandMode = false
		m.commandBuf = ""
		return m, cmd
	case isKey(msg, "backspace"):
		if len(m.commandBuf) > 0 {
			runes := []rune(m.commandBuf)
			m.commandBuf = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.commandBuf += string(msg.Runes)
		} else if msg.Type == tea.KeySpace {
			m.commandBuf += " "
		}
	}
	return m, nil
}

func (m *AppModel) executeCommand(cmd string) tea.Cmd {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "quit", "q":
		if m.store != nil {
			m.store.Close()
		}
		return tea.Quit

	case "help":
		m.showHelp = true
		return nil

	case "id":
		if m.identity != nil && m.identity.UserID != "" {
			return m.showToast(fmt.Sprintf("Your ID: %s", m.identity.UserID), false)
		}
		return m.showToast("Not logged in", true)

	case "add":
		if len(parts) < 2 {
			return m.showToast("Usage: :add <username>", true)
		}
		if m.store == nil {
			return m.showToast("Not logged in", true)
		}
		username := parts[1]
		// Prevent adding yourself
		if m.identity != nil && username == m.identity.Username {
			return m.showToast("Cannot add yourself as a contact", true)
		}
		cl := m.client
		return tea.Batch(m.showToast(fmt.Sprintf("Resolving %s...", username), false), func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resp, err := cl.Send(ctx, &pb.Frame{
				RequestId: uuid.NewString(),
				Payload:   &pb.Frame_ResolveUsernameRequest{ResolveUsernameRequest: &pb.ResolveUsernameRequest{Username: username}},
			})
			if err != nil {
				return addContactResultMsg{err: fmt.Sprintf("User %s not found", username)}
			}
			rr, ok := resp.Payload.(*pb.Frame_ResolveUsernameResponse)
			if !ok {
				return addContactResultMsg{err: "unexpected response"}
			}
			return addContactResultMsg{userID: rr.ResolveUsernameResponse.UserId, username: username}
		})

	case "rename":
		if len(parts) < 2 {
			return m.showToast("Usage: :rename <new_name>", true)
		}
		if m.store == nil {
			return m.showToast("Not logged in", true)
		}
		c := m.contacts.SelectedContact()
		if c == nil {
			return m.showToast("No contact selected", true)
		}
		newName := strings.Join(parts[1:], " ")
		if err := m.store.RenameContact(c.UserID, newName); err != nil {
			return m.showToast("Failed to rename contact", true)
		}
		// Update chat header if this contact is open
		if m.chat.ContactID() == c.UserID {
			m.chat.SetContact(c.UserID, newName)
		}
		return tea.Batch(
			m.showToast(fmt.Sprintf("Renamed to %s", newName), false),
			func() tea.Msg { return contactsUpdatedMsg{} },
		)

	case "delete":
		if m.store == nil {
			return m.showToast("Not logged in", true)
		}
		c := m.contacts.SelectedContact()
		if c == nil {
			return m.showToast("No contact selected", true)
		}
		return m.showToast(fmt.Sprintf("Delete %s? Type :delete! to confirm", c.Name), false)

	case "delete!":
		if m.store == nil {
			return m.showToast("Not logged in", true)
		}
		c := m.contacts.SelectedContact()
		if c == nil {
			return m.showToast("No contact selected", true)
		}
		if err := m.store.DeleteContact(c.UserID); err != nil {
			return m.showToast("Failed to delete contact", true)
		}
		m.chat.SetContact("", "")
		return tea.Batch(
			m.showToast(fmt.Sprintf("Deleted %s", c.Name), false),
			func() tea.Msg { return contactsUpdatedMsg{} },
		)

	default:
		return m.showToast(fmt.Sprintf("Unknown command: %s", parts[0]), true)
	}
}

func (m *AppModel) showToast(msg string, isErr bool) tea.Cmd {
	m.toast = &Toast{Message: msg, IsError: isErr, ShowAt: time.Now()}
	return toastTick()
}

func (m *AppModel) updateFocus() {
	m.contacts.SetFocused(m.focus == focusContacts)
	m.chat.SetFocused(m.focus == focusChat)
}

func (m *AppModel) updateLayout() {
	if m.screen != screenMain {
		return
	}
	contactsW := m.width / 4
	if contactsW < 20 {
		contactsW = 20
	}
	chatW := m.width - contactsW - 1
	mainH := m.height - 1 // status bar

	m.contacts.SetSize(contactsW, mainH)
	m.chat.SetSize(chatW, mainH)
}

func (m *AppModel) openChat(c *store.Contact) {
	m.chat.SetContact(c.UserID, c.Name)
	if m.identity != nil {
		m.chat.SetMyUsername(m.identity.Username)
	}
	if m.store != nil {
		m.store.ClearUnread(c.UserID)
		msgs, _ := m.store.GetMessages(c.UserID, 500)
		m.chat.SetMessages(msgs)
	}
	m.focus = focusChat
	m.updateFocus()
}

func (m AppModel) loadContactsCmd() tea.Cmd {
	return func() tea.Msg {
		return contactsUpdatedMsg{}
	}
}

func (m AppModel) reAuthCmd() tea.Cmd {
	cl := m.client
	username := m.authUsername
	password := m.authPassword
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cl.Send(ctx, &pb.Frame{
			RequestId: uuid.NewString(),
			Payload: &pb.Frame_LoginRequest{LoginRequest: &pb.LoginRequest{
				Username:    username,
				AuthMessage: []byte(password),
			}},
		})
		return nil
	}
}

func (m AppModel) doRegister(username, password string) tea.Cmd {
	cl := m.client
	dataDir := m.dataDir
	return func() tea.Msg {
		if username == "" || password == "" {
			return authErrorMsg{err: "username and password are required"}
		}

		// Open per-user database
		st, err := openStore(dataDir, username)
		if err != nil {
			return authErrorMsg{err: fmt.Sprintf("database error: %v", err)}
		}

		// Generate identity keys
		edKey, err := crypto.GenerateEd25519()
		if err != nil {
			st.Close()
			return authErrorMsg{err: "failed to generate keys"}
		}

		x25519Key, err := crypto.GenerateX25519()
		if err != nil {
			st.Close()
			return authErrorMsg{err: "failed to generate keys"}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := cl.Send(ctx, &pb.Frame{
			RequestId: uuid.NewString(),
			Payload: &pb.Frame_RegisterRequest{RegisterRequest: &pb.RegisterRequest{
				Username:         username,
				PasswordVerifier: []byte(password),
				PublicIdentityKey: edKey.Public,
			}},
		})
		if err != nil {
			st.Close()
			return authErrorMsg{err: err.Error()}
		}

		rr, ok := resp.Payload.(*pb.Frame_RegisterResponse)
		if !ok {
			st.Close()
			return authErrorMsg{err: "unexpected response"}
		}

		// Save identity
		identity := &store.Identity{
			UserID:         rr.RegisterResponse.UserId,
			Username:       username,
			Ed25519Private: edKey.Private,
			Ed25519Public:  edKey.Public,
			X25519Private:  x25519Key.Private,
			X25519Public:   x25519Key.Public,
		}

		// Login
		loginResp, err := cl.Send(ctx, &pb.Frame{
			RequestId: uuid.NewString(),
			Payload: &pb.Frame_LoginRequest{LoginRequest: &pb.LoginRequest{
				Username:    username,
				AuthMessage: []byte(password),
			}},
		})
		if err != nil {
			st.Close()
			return authErrorMsg{err: err.Error()}
		}

		lr, ok := loginResp.Payload.(*pb.Frame_LoginResponse)
		if !ok {
			st.Close()
			return authErrorMsg{err: "login failed after registration"}
		}

		identity.SessionToken = lr.LoginResponse.SessionToken
		if err := st.SaveIdentity(identity); err != nil {
			st.Close()
			return authErrorMsg{err: "failed to save identity"}
		}

		// Upload pre-key bundle
		spk, sig, otks, err := crypto.GeneratePreKeyBundle(edKey.Private, 10)
		if err == nil {
			oneTimeKeys := make([][]byte, len(otks))
			for i, k := range otks {
				oneTimeKeys[i] = k.Public
			}
			cl.Send(ctx, &pb.Frame{
				RequestId: uuid.NewString(),
				Payload: &pb.Frame_UploadPreKeysRequest{UploadPreKeysRequest: &pb.UploadPreKeysRequest{
					Bundle: &pb.PreKeyBundle{
						IdentityKey:          edKey.Public,
						SignedPreKey:         spk.Public,
						SignedPreKeySignature: sig,
						OneTimePreKeys:       oneTimeKeys,
					},
				}},
			})
		}

		return authSuccessMsg{store: st, identity: identity, token: lr.LoginResponse.SessionToken, username: username, password: password}
	}
}

func (m AppModel) doLogin(username, password string) tea.Cmd {
	cl := m.client
	dataDir := m.dataDir
	return func() tea.Msg {
		if username == "" || password == "" {
			return authErrorMsg{err: "username and password are required"}
		}

		// Open per-user database
		st, err := openStore(dataDir, username)
		if err != nil {
			return authErrorMsg{err: fmt.Sprintf("database error: %v", err)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := cl.Send(ctx, &pb.Frame{
			RequestId: uuid.NewString(),
			Payload: &pb.Frame_LoginRequest{LoginRequest: &pb.LoginRequest{
				Username:    username,
				AuthMessage: []byte(password),
			}},
		})
		if err != nil {
			st.Close()
			return authErrorMsg{err: err.Error()}
		}

		lr, ok := resp.Payload.(*pb.Frame_LoginResponse)
		if !ok {
			st.Close()
			return authErrorMsg{err: "unexpected response"}
		}

		// Load existing identity — if none exists, require registration
		identity, err := st.GetIdentity()
		if err != nil {
			st.Close()
			return authErrorMsg{err: "No local identity found. Please register first."}
		}

		identity.Username = username
		identity.SessionToken = lr.LoginResponse.SessionToken
		st.SaveIdentity(identity)

		return authSuccessMsg{store: st, identity: identity, token: lr.LoginResponse.SessionToken, username: username, password: password}
	}
}

func (m AppModel) sendMessageCmd(contactID, content string) tea.Cmd {
	cl := m.client
	st := m.store
	identity := m.identity
	return func() tea.Msg {
		if st == nil {
			return nil
		}

		msgID := uuid.NewString()
		now := time.Now()

		// Build message payload
		payload := &pb.MessagePayload{
			MessageId: msgID,
			SenderId:  "",
			SentAt:    now.Unix(),
			Content:   &pb.MessagePayload_Text{Text: &pb.TextMessage{Body: content}},
		}

		if identity != nil {
			payload.SenderId = identity.UserID
		}

		payloadBytes, err := proto.Marshal(payload)
		if err != nil {
			return sendErrorMsg{err: "failed to encode message"}
		}

		// Encrypt
		ciphertext := payloadBytes // fallback: unencrypted if no session
		var ratchetIdx uint32

		if cs, err := st.GetCryptoSession(contactID); err == nil {
			ratchetIdx = cs.SendIndex
			if encrypted, err := crypto.Encrypt(cs.SharedSecret, ratchetIdx, payloadBytes); err == nil {
				ciphertext = encrypted
				st.UpdateCryptoSessionSend(contactID, ratchetIdx+1)
			}
		}

		// Send envelope
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err = cl.Send(ctx, &pb.Frame{
			RequestId: uuid.NewString(),
			Payload: &pb.Frame_Envelope{Envelope: &pb.Envelope{
				RecipientId:  contactID,
				Ciphertext:   ciphertext,
				RatchetIndex: ratchetIdx,
				Timestamp:    now.Unix(),
			}},
		})
		if err != nil {
			return sendErrorMsg{err: err.Error()}
		}

		// Save locally only after successful send
		st.SaveMessage(&store.Message{
			ID:        msgID,
			ContactID: contactID,
			Content:   content,
			IsMine:    true,
			Timestamp: now,
			Read:      true,
		})

		return messagesUpdatedMsg{}
	}
}

func (m *AppModel) handleIncoming(frame *pb.Frame) tea.Cmd {
	switch p := frame.Payload.(type) {
	case *pb.Frame_Envelope:
		return m.handleIncomingEnvelope(p.Envelope)
	}
	return nil
}

func (m *AppModel) handleIncomingEnvelope(env *pb.Envelope) tea.Cmd {
	if m.store == nil {
		return nil
	}

	// Try to decrypt
	var payload pb.MessagePayload
	decrypted := false

	senderID := ""

	// Try decryption with crypto session
	contacts, _ := m.store.ListContacts()
	for _, c := range contacts {
		if cs, err := m.store.GetCryptoSession(c.UserID); err == nil {
			if pt, err := crypto.Decrypt(cs.SharedSecret, env.RatchetIndex, env.Ciphertext); err == nil {
				if err := proto.Unmarshal(pt, &payload); err == nil {
					decrypted = true
					senderID = c.UserID
					m.store.UpdateCryptoSessionRecv(c.UserID, env.RatchetIndex+1)
					break
				}
			}
		}
	}

	if !decrypted {
		// Try to unmarshal as plaintext (no encryption)
		if err := proto.Unmarshal(env.Ciphertext, &payload); err == nil {
			decrypted = true
			senderID = payload.SenderId
		}
	}

	if !decrypted {
		return nil
	}

	// Extract text content
	var content string
	switch c := payload.Content.(type) {
	case *pb.MessagePayload_Text:
		content = c.Text.Body
	case *pb.MessagePayload_File:
		content = fmt.Sprintf("[File: %s]", c.File.Filename)
	}

	if content == "" {
		return nil
	}

	// Ensure contact exists with a readable name
	displayName := senderID
	if len(senderID) > 8 {
		displayName = "user-" + senderID[:8]
	}
	// Check if contact already exists with a proper name
	for _, c := range contacts {
		if c.UserID == senderID {
			displayName = c.Name
			break
		}
	}
	m.store.AddContact(senderID, displayName)

	msgID := payload.MessageId
	if msgID == "" {
		msgID = uuid.NewString()
	}

	ts := time.Unix(payload.SentAt, 0)
	if payload.SentAt == 0 {
		ts = time.Unix(env.Timestamp, 0)
	}

	msg := &store.Message{
		ID:        msgID,
		ContactID: senderID,
		Content:   content,
		IsMine:    false,
		Timestamp: ts,
	}

	m.store.SaveMessage(msg)

	isActiveChat := m.chat.ContactID() == senderID

	if isActiveChat {
		m.chat.AppendMessage(*msg)
	} else {
		m.store.IncrementUnread(senderID)
		// Find sender name for notification
		name := displayName
		go notify.NewMessage(name, content)
	}

	return func() tea.Msg { return contactsUpdatedMsg{} }
}

func (m AppModel) View() string {
	var view string

	switch m.screen {
	case screenWelcome:
		view = m.welcome.View()
	case screenMain:
		view = m.mainView()
	}

	// Help overlay
	if m.showHelp {
		view = renderHelp(m.width, m.height)
	}

	// Toast overlay
	if m.toast != nil && time.Since(m.toast.ShowAt) < 3*time.Second {
		toastView := m.toast.View(m.width)
		// Overlay toast at top
		lines := strings.Split(view, "\n")
		toastLines := strings.Split(toastView, "\n")
		for i, tl := range toastLines {
			if i < len(lines) {
				lines[i] = tl
			}
		}
		view = strings.Join(lines, "\n")
	}

	return view
}

func (m AppModel) mainView() string {
	contactsView := m.contacts.View()
	chatView := m.chat.View()

	main := lipgloss.JoinHorizontal(lipgloss.Top, contactsView, chatView)

	// Status bar
	statusConnected := OnlineStyle.Render("● connected")
	if !m.client.Connected() {
		statusConnected = OfflineStyle.Render("● disconnected")
	}

	// Show username and truncated ID
	var userInfo string
	if m.identity != nil && m.identity.UserID != "" {
		uid := m.identity.UserID
		if len(uid) > 8 {
			uid = uid[:8]
		}
		userInfo = " " + SubtitleStyle.Render(m.identity.Username+"@"+uid)
	}

	statusRight := SubtitleStyle.Render(":help")

	var statusCmd string
	if m.commandMode {
		statusCmd = CommandStyle.Render(fmt.Sprintf(":%s", m.commandBuf)) + lipgloss.NewStyle().Reverse(true).Render(" ")
	}

	statusContent := statusConnected + userInfo
	if statusCmd != "" {
		statusContent = statusCmd
	}

	padLen := m.width - lipgloss.Width(statusContent) - lipgloss.Width(statusRight) - 4
	if padLen < 0 {
		padLen = 0
	}

	statusBar := StatusBarStyle.Width(m.width).Render(
		" " + statusContent + strings.Repeat(" ", padLen) + statusRight + " ",
	)

	return lipgloss.JoinVertical(lipgloss.Left, main, statusBar)
}

func (m AppModel) waitForMessage() tea.Cmd {
	ch := m.client.Incoming
	return func() tea.Msg {
		frame := <-ch
		return incomingFrameMsg{frame: frame}
	}
}
