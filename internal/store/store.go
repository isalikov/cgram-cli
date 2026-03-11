package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Contact struct {
	UserID    string
	Name      string
	Online    bool
	Unread    int
	LastMsgAt time.Time
}

type Message struct {
	ID        string
	ContactID string
	Content   string
	IsMine    bool
	Timestamp time.Time
	Read      bool
}

type Identity struct {
	UserID          string
	Username        string
	SessionToken    string
	AuthPassword    string
	Ed25519Private  []byte
	Ed25519Public   []byte
	X25519Private   []byte
	X25519Public    []byte
}

type CryptoSession struct {
	ContactID      string
	SharedSecret   []byte
	SendIndex      uint32
	RecvIndex      uint32
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS identity (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		user_id TEXT NOT NULL DEFAULT '',
		username TEXT NOT NULL DEFAULT '',
		session_token TEXT NOT NULL DEFAULT '',
		ed25519_private BLOB NOT NULL,
		ed25519_public BLOB NOT NULL,
		x25519_private BLOB NOT NULL,
		x25519_public BLOB NOT NULL
	);

	CREATE TABLE IF NOT EXISTS contacts (
		user_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		online INTEGER NOT NULL DEFAULT 0,
		unread INTEGER NOT NULL DEFAULT 0,
		last_msg_at TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		contact_id TEXT NOT NULL,
		content TEXT NOT NULL,
		is_mine INTEGER NOT NULL DEFAULT 0,
		timestamp TEXT NOT NULL,
		read INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (contact_id) REFERENCES contacts(user_id)
	);

	CREATE TABLE IF NOT EXISTS crypto_sessions (
		contact_id TEXT PRIMARY KEY,
		shared_secret BLOB NOT NULL,
		send_index INTEGER NOT NULL DEFAULT 0,
		recv_index INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (contact_id) REFERENCES contacts(user_id)
	);

	CREATE INDEX IF NOT EXISTS idx_messages_contact ON messages(contact_id, timestamp);
	`
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// Migration: add auth_password column if missing
	db.Exec("ALTER TABLE identity ADD COLUMN auth_password TEXT NOT NULL DEFAULT ''")

	return nil
}

// Identity

func (s *Store) SaveIdentity(id *Identity) error {
	_, err := s.db.Exec(`
		INSERT INTO identity (id, user_id, username, session_token, auth_password, ed25519_private, ed25519_public, x25519_private, x25519_public)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			user_id = excluded.user_id,
			username = excluded.username,
			session_token = excluded.session_token,
			auth_password = excluded.auth_password,
			ed25519_private = excluded.ed25519_private,
			ed25519_public = excluded.ed25519_public,
			x25519_private = excluded.x25519_private,
			x25519_public = excluded.x25519_public
	`, id.UserID, id.Username, id.SessionToken, id.AuthPassword, id.Ed25519Private, id.Ed25519Public, id.X25519Private, id.X25519Public)
	return err
}

func (s *Store) GetIdentity() (*Identity, error) {
	id := &Identity{}
	err := s.db.QueryRow(`
		SELECT user_id, username, session_token, auth_password, ed25519_private, ed25519_public, x25519_private, x25519_public
		FROM identity WHERE id = 1
	`).Scan(&id.UserID, &id.Username, &id.SessionToken, &id.AuthPassword, &id.Ed25519Private, &id.Ed25519Public, &id.X25519Private, &id.X25519Public)
	if err != nil {
		return nil, err
	}
	return id, nil
}

func (s *Store) UpdateSession(token string) error {
	_, err := s.db.Exec("UPDATE identity SET session_token = ? WHERE id = 1", token)
	return err
}

// Contacts

func (s *Store) AddContact(userID, name string) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO contacts (user_id, name) VALUES (?, ?)",
		userID, name,
	)
	return err
}

func (s *Store) RenameContact(userID, name string) error {
	_, err := s.db.Exec("UPDATE contacts SET name = ? WHERE user_id = ?", name, userID)
	return err
}

func (s *Store) DeleteContact(userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec("DELETE FROM messages WHERE contact_id = ?", userID)
	tx.Exec("DELETE FROM crypto_sessions WHERE contact_id = ?", userID)
	tx.Exec("DELETE FROM contacts WHERE user_id = ?", userID)

	return tx.Commit()
}

func (s *Store) ListContacts() ([]Contact, error) {
	rows, err := s.db.Query("SELECT user_id, name, online, unread, last_msg_at FROM contacts ORDER BY last_msg_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		var lastMsg string
		if err := rows.Scan(&c.UserID, &c.Name, &c.Online, &c.Unread, &lastMsg); err != nil {
			return nil, err
		}
		if lastMsg != "" {
			c.LastMsgAt, _ = time.Parse(time.RFC3339, lastMsg)
		}
		contacts = append(contacts, c)
	}
	return contacts, nil
}

func (s *Store) SetContactOnline(userID string, online bool) error {
	v := 0
	if online {
		v = 1
	}
	_, err := s.db.Exec("UPDATE contacts SET online = ? WHERE user_id = ?", v, userID)
	return err
}

func (s *Store) IncrementUnread(contactID string) error {
	_, err := s.db.Exec("UPDATE contacts SET unread = unread + 1 WHERE user_id = ?", contactID)
	return err
}

func (s *Store) ClearUnread(contactID string) error {
	_, err := s.db.Exec("UPDATE contacts SET unread = 0 WHERE user_id = ?", contactID)
	return err
}

// Messages

func (s *Store) SaveMessage(msg *Message) error {
	isMine := 0
	if msg.IsMine {
		isMine = 1
	}
	read := 0
	if msg.Read {
		read = 1
	}

	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO messages (id, contact_id, content, is_mine, timestamp, read)
		VALUES (?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.ContactID, msg.Content, isMine, msg.Timestamp.Format(time.RFC3339), read)

	if err == nil {
		s.db.Exec("UPDATE contacts SET last_msg_at = ? WHERE user_id = ?",
			msg.Timestamp.Format(time.RFC3339), msg.ContactID)
	}
	return err
}

func (s *Store) GetMessages(contactID string, limit int) ([]Message, error) {
	rows, err := s.db.Query(`
		SELECT id, contact_id, content, is_mine, timestamp, read
		FROM messages WHERE contact_id = ?
		ORDER BY timestamp ASC
		LIMIT ?
	`, contactID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var isMine, read int
		var ts string
		if err := rows.Scan(&m.ID, &m.ContactID, &m.Content, &isMine, &ts, &read); err != nil {
			return nil, err
		}
		m.IsMine = isMine == 1
		m.Read = read == 1
		m.Timestamp, _ = time.Parse(time.RFC3339, ts)
		messages = append(messages, m)
	}
	return messages, nil
}

// Crypto Sessions

func (s *Store) SaveCryptoSession(cs *CryptoSession) error {
	_, err := s.db.Exec(`
		INSERT INTO crypto_sessions (contact_id, shared_secret, send_index, recv_index)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (contact_id) DO UPDATE SET
			shared_secret = excluded.shared_secret,
			send_index = excluded.send_index,
			recv_index = excluded.recv_index
	`, cs.ContactID, cs.SharedSecret, cs.SendIndex, cs.RecvIndex)
	return err
}

func (s *Store) GetCryptoSession(contactID string) (*CryptoSession, error) {
	cs := &CryptoSession{}
	err := s.db.QueryRow(
		"SELECT contact_id, shared_secret, send_index, recv_index FROM crypto_sessions WHERE contact_id = ?",
		contactID,
	).Scan(&cs.ContactID, &cs.SharedSecret, &cs.SendIndex, &cs.RecvIndex)
	if err != nil {
		return nil, err
	}
	return cs, nil
}

func (s *Store) UpdateCryptoSessionSend(contactID string, index uint32) error {
	_, err := s.db.Exec("UPDATE crypto_sessions SET send_index = ? WHERE contact_id = ?", index, contactID)
	return err
}

func (s *Store) UpdateCryptoSessionRecv(contactID string, index uint32) error {
	_, err := s.db.Exec("UPDATE crypto_sessions SET recv_index = ? WHERE contact_id = ?", index, contactID)
	return err
}
