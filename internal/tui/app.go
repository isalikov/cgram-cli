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

type inputMode int

const (
	modeNormal inputMode = iota
	modeInsert
	modeCommand
)

type SendMessageMsg struct {
	ContactID string
	Content   string
}

type AppModel struct {
	screen     screen
	focus      focus
	mode       inputMode
	welcome    WelcomeModel
	contacts   ContactsModel
	chat       ChatModel
	input      InputModel
	toast      *Toast
	showHelp   bool
	commandBuf string
	width      int
	height     int

	client   *client.Client
	store    *store.Store
	identity *store.Identity

	dataDir      string
	serverAddr   string
	wasConnected bool

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
		mode:       modeNormal,
		client:     cl,
		dataDir:    dataDir,
		serverAddr: serverAddr,
		welcome:    NewWelcome(serverAddr, cl.Connected()),
		contacts:   NewContacts(),
		chat:       NewChat(),
		input:      NewInput("Type a message..."),
	}
}

type autoLoginMsg struct {
	store    *store.Store
	identity *store.Identity
	username string
	password string
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		tea.WindowSize(),
		m.connectCmd(),
		m.waitForStatus(),
		m.tryAutoLogin(),
	)
}

func (m AppModel) tryAutoLogin() tea.Cmd {
	dataDir := m.dataDir
	return func() tea.Msg {
		sessionFile := filepath.Join(dataDir, "session")
		data, err := os.ReadFile(sessionFile)
		if err != nil {
			return nil
		}
		username := strings.TrimSpace(string(data))
		if username == "" {
			return nil
		}
		st, err := openStore(dataDir, username)
		if err != nil {
			return nil
		}
		identity, err := st.GetIdentity()
		if err != nil || identity.AuthPassword == "" {
			st.Close()
			return nil
		}
		return autoLoginMsg{
			store:    st,
			identity: identity,
			username: username,
			password: identity.AuthPassword,
		}
	}
}

func saveSession(dataDir, username string) {
	os.MkdirAll(dataDir, 0700)
	os.WriteFile(filepath.Join(dataDir, "session"), []byte(username), 0600)
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
		// Global quit
		if isKey(msg, "ctrl+c") {
			if m.store != nil {
				m.store.Close()
			}
			return m, tea.Quit
		}

		// Help overlay
		if m.showHelp {
			if isKey(msg, "esc") {
				m.showHelp = false
			}
			return m, nil
		}

		// Route by screen
		switch m.screen {
		case screenWelcome:
			var cmd tea.Cmd
			m.welcome, cmd = m.welcome.Update(msg)
			cmds = append(cmds, cmd)

		case screenMain:
			return m.updateMain(msg)
		}

	case autoLoginMsg:
		// Saved session found — auto-login
		if m.screen == screenWelcome && msg.store != nil {
			m.welcome.SetLoading(true)
			return m, m.doAutoLogin(msg.store, msg.identity, msg.username, msg.password)
		}

	case connectedMsg:
		m.welcome.SetConnected(true)
		m.wasConnected = true
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
		// Persist session
		saveSession(m.dataDir, msg.username)
		msg.identity.AuthPassword = msg.password
		m.store.SaveIdentity(msg.identity)
		m.screen = screenMain
		m.focus = focusContacts
		m.mode = modeNormal
		m.updateFocus()
		m.updateLayout()
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
		return m, tea.Batch(cmd, m.waitForMessage())

	case dismissToastMsg:
		m.toast = nil
	}

	return m, tea.Batch(cmds...)
}

// ── Vim mode routing ──

func (m *AppModel) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeNormal:
		return m.updateNormal(msg)
	case modeInsert:
		return m.updateInsert(msg)
	case modeCommand:
		return m.updateCommand(msg)
	}
	return m, nil
}

func (m *AppModel) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isKey(msg, "i"):
		m.mode = modeInsert
		m.input.Focus()
		return m, nil

	case isKey(msg, "a"):
		m.mode = modeInsert
		m.input.Focus()
		// Move cursor to end
		row := len(m.input.lines) - 1
		m.input.cursorRow = row
		m.input.cursorCol = len(m.input.lines[row])
		return m, nil

	case isKey(msg, ":"):
		m.mode = modeCommand
		m.commandBuf = ""
		return m, nil

	case isKey(msg, "tab"):
		if m.focus == focusContacts {
			m.focus = focusChat
		} else {
			m.focus = focusContacts
		}
		m.updateFocus()
		return m, nil

	case isKey(msg, "j", "down"):
		if m.focus == focusContacts {
			m.contacts.MoveDown()
		} else {
			m.chat.ScrollDown(3)
		}
		return m, nil

	case isKey(msg, "k", "up"):
		if m.focus == focusContacts {
			m.contacts.MoveUp()
		} else {
			m.chat.ScrollUp(3)
		}
		return m, nil

	case isKey(msg, "pgup"):
		m.chat.ScrollUp(10)
		return m, nil

	case isKey(msg, "pgdown"):
		m.chat.ScrollDown(10)
		return m, nil

	case isKey(msg, "enter"):
		if m.focus == focusContacts {
			if c := m.contacts.SelectedContact(); c != nil {
				m.openChat(c)
			}
		}
		return m, nil

	case isKey(msg, "esc"):
		m.showHelp = false
		return m, nil
	}

	return m, nil
}

func (m *AppModel) updateInsert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isKey(msg, "esc"):
		m.mode = modeNormal
		m.input.Blur()
		return m, nil

	case isKey(msg, "alt+enter"):
		m.input = m.input.InsertNewline()
		return m, nil

	case isKey(msg, "enter"):
		content := strings.TrimSpace(m.input.Value())
		if content != "" && m.chat.ContactID() != "" {
			m.input.Reset()
			return m, func() tea.Msg {
				return SendMessageMsg{ContactID: m.chat.ContactID(), Content: content}
			}
		}
		return m, nil

	default:
		m.input, _ = m.input.Update(msg)
		return m, nil
	}
}

func (m *AppModel) updateCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isKey(msg, "esc"):
		m.mode = modeNormal
		m.commandBuf = ""
		return m, nil
	case isKey(msg, "enter"):
		cmd := m.executeCommand(m.commandBuf)
		m.mode = modeNormal
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

// ── Commands ──

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
		if m.chat.ContactID() == c.UserID {
			m.chat.SetContact(c.UserID, newName)
		}
		return tea.Batch(
			m.showToast(fmt.Sprintf("Renamed to %s", newName), false),
			func() tea.Msg { return contactsUpdatedMsg{} },
		)

	case "delete", "delete!":
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

// ── Helpers ──

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
	contactsW := max(18, m.width/5)
	// -1 for separator column
	chatW := m.width - contactsW - 1
	// height - topbar(1) - modeline(1) - inputline(1) - colline(1) - statusbar(1)
	mainH := m.height - 5

	m.contacts.SetSize(contactsW, mainH)
	m.chat.SetSize(chatW, mainH)
	m.input.SetWidth(chatW)
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

// ── Auth commands ──

func (m AppModel) doAutoLogin(st *store.Store, identity *store.Identity, username, password string) tea.Cmd {
	cl := m.client
	return func() tea.Msg {
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
			return authErrorMsg{err: "Auto-login failed: " + err.Error()}
		}

		lr, ok := resp.Payload.(*pb.Frame_LoginResponse)
		if !ok {
			st.Close()
			return authErrorMsg{err: "Auto-login failed"}
		}

		identity.SessionToken = lr.LoginResponse.SessionToken
		st.SaveIdentity(identity)

		return authSuccessMsg{store: st, identity: identity, token: lr.LoginResponse.SessionToken, username: username, password: password}
	}
}

func (m AppModel) doRegister(username, password string) tea.Cmd {
	cl := m.client
	dataDir := m.dataDir
	return func() tea.Msg {
		if username == "" || password == "" {
			return authErrorMsg{err: "username and password are required"}
		}

		st, err := openStore(dataDir, username)
		if err != nil {
			return authErrorMsg{err: fmt.Sprintf("database error: %v", err)}
		}

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

		identity := &store.Identity{
			UserID:         rr.RegisterResponse.UserId,
			Username:       username,
			Ed25519Private: edKey.Private,
			Ed25519Public:  edKey.Public,
			X25519Private:  x25519Key.Private,
			X25519Public:   x25519Key.Public,
		}

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

// ── Message handling ──

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

		payload := &pb.MessagePayload{
			MessageId: msgID,
			SenderId:  "",
			SentAt:    now.Unix(),
			Content:   &pb.MessagePayload_Text{Text: &pb.TextMessage{Body: content}},
		}

		if identity != nil {
			payload.SenderId = identity.UserID + ":" + identity.Username
		}

		payloadBytes, err := proto.Marshal(payload)
		if err != nil {
			return sendErrorMsg{err: "failed to encode message"}
		}

		ciphertext := payloadBytes
		var ratchetIdx uint32

		if cs, err := st.GetCryptoSession(contactID); err == nil {
			ratchetIdx = cs.SendIndex
			if encrypted, err := crypto.Encrypt(cs.SharedSecret, ratchetIdx, payloadBytes); err == nil {
				ciphertext = encrypted
				st.UpdateCryptoSessionSend(contactID, ratchetIdx+1)
			}
		}

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

	var payload pb.MessagePayload
	decrypted := false
	senderID := ""

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
		if err := proto.Unmarshal(env.Ciphertext, &payload); err == nil {
			decrypted = true
			senderID = payload.SenderId
		}
	}

	if !decrypted {
		return nil
	}

	// Parse "userId:username" format from SenderId
	senderUsername := ""
	if parts := strings.SplitN(senderID, ":", 2); len(parts) == 2 {
		senderID = parts[0]
		senderUsername = parts[1]
	}

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

	// Use extracted username, then fall back to existing contact name, then truncated ID
	displayName := senderUsername
	if displayName == "" {
		displayName = senderID
		if len(senderID) > 8 {
			displayName = "user-" + senderID[:8]
		}
	}
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
	}

	go notify.NewMessage(displayName, content)

	return func() tea.Msg { return contactsUpdatedMsg{} }
}

// ── View ──

func (m AppModel) View() string {
	var view string

	switch m.screen {
	case screenWelcome:
		view = m.welcome.View()
	case screenMain:
		view = m.mainView()
	}

	if m.showHelp {
		view = renderHelp(m.width, m.height)
	}

	if m.toast != nil && time.Since(m.toast.ShowAt) < 3*time.Second {
		toastView := m.toast.View(m.width)
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
	topBar := m.renderTopBar()
	contactsView := m.contacts.View()
	chatView := m.chat.View()

	// Separator column
	contactsW := m.contacts.width
	chatW := m.chat.width
	mainH := m.height - 5

	sep := make([]string, mainH)
	for i := range sep {
		sep[i] = SeparatorStyle.Render("│")
	}
	separator := strings.Join(sep, "\n")

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, contactsView, separator, chatView)

	modeLine := m.renderModeLine()
	inputLine := m.renderInputLine(contactsW + 1 + chatW)
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		mainArea,
		modeLine,
		inputLine,
		statusBar,
	)
}

func (m AppModel) renderTopBar() string {
	left := " " + AppNameStyle.Render("cgram") + " " + SubtitleStyle.Render("v1.0.0")
	right := TopBarHintStyle.Render(":help  Tab:switch  :q")

	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if pad < 0 {
		pad = 0
	}

	content := left + strings.Repeat(" ", pad) + right + " "
	return TopBarStyle.Width(m.width).Render(content)
}

func (m AppModel) renderModeLine() string {
	var badge string
	var hints string

	switch m.mode {
	case modeNormal:
		badge = ModeNormalBadge.Render("-- NORMAL --")
		hints = ModeHintStyle.Render("  i:insert | a:append | ::command")
	case modeInsert:
		badge = ModeInsertBadge.Render("-- INSERT --")
		hints = ModeHintStyle.Render("  Esc:normal | Enter:send | Alt+Enter:newline")
	case modeCommand:
		badge = ModeCommandBadge.Render("-- COMMAND --")
		cmdText := CommandStyle.Render(":" + m.commandBuf)
		cursor := lipgloss.NewStyle().Reverse(true).Render(" ")
		hints = "  " + cmdText + cursor
	}

	return badge + hints
}

func (m AppModel) renderInputLine(totalW int) string {
	prompt := PromptStyle.Render("$ ")

	if m.mode != modeInsert {
		placeholder := InputPlaceholderStyle.Render("Press 'i' to insert")
		colInfo := ColStyle.Render("col: 1")
		left := prompt + placeholder
		pad := totalW - lipgloss.Width(left) - lipgloss.Width(colInfo)
		if pad < 0 {
			pad = 0
		}
		return left + strings.Repeat(" ", pad) + colInfo
	}

	inputView := m.input.View()
	colInfo := ColStyle.Render(fmt.Sprintf("col: %d", m.input.CursorCol()))
	left := prompt + inputView
	pad := totalW - lipgloss.Width(left) - lipgloss.Width(colInfo)
	if pad < 0 {
		pad = 0
	}
	return left + strings.Repeat(" ", pad) + colInfo
}

func (m AppModel) renderStatusBar() string {
	// Mode badge
	var modeBadge string
	switch m.mode {
	case modeNormal:
		modeBadge = StatusNormalBadge.Render("NORMAL")
	case modeInsert:
		modeBadge = StatusInsertBadge.Render("INSERT")
	case modeCommand:
		modeBadge = StatusCommandBadge.Render("COMMAND")
	}

	// Chat info
	chatInfo := ""
	if m.chat.ContactID() != "" {
		chatInfo = "  " + SubtitleStyle.Render("chat:") + " " +
			ChatHeaderNameStyle.Render("@"+m.chat.ContactName()) +
			" " + SubtitleStyle.Render(fmt.Sprintf("[%d]", m.chat.MessageCount()))
	}

	// Pane indicators
	contactsPane := StatusPaneDimStyle.Render("[1:contacts]")
	chatPane := StatusPaneDimStyle.Render("[2:chat]")
	inputPane := StatusPaneDimStyle.Render("[3:input]")
	if m.focus == focusContacts {
		contactsPane = StatusPaneActiveStyle.Render("[1:contacts]")
	} else if m.focus == focusChat {
		chatPane = StatusPaneActiveStyle.Render("[2:chat]")
	}
	if m.mode == modeInsert {
		inputPane = StatusPaneActiveStyle.Render("[3:input]")
	}
	panes := "  " + contactsPane + " " + chatPane + " " + inputPane

	// Connection + e2e + time
	connStatus := StatusConnectedStyle.Render("● connected")
	if !m.client.Connected() {
		connStatus = StatusDisconnectedStyle.Render("● disconnected")
	}
	e2e := StatusE2EStyle.Render("■ e2e")
	clock := SubtitleStyle.Render(time.Now().Format("15:04:05"))
	right := "  " + connStatus + "  " + e2e + "  " + clock

	left := modeBadge + chatInfo + panes

	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if pad < 0 {
		pad = 0
	}

	content := " " + left + strings.Repeat(" ", pad) + right + " "
	return StatusBarStyle.Width(m.width).Render(content)
}

func (m AppModel) waitForMessage() tea.Cmd {
	ch := m.client.Incoming
	return func() tea.Msg {
		frame := <-ch
		return incomingFrameMsg{frame: frame}
	}
}
