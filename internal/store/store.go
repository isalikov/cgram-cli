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

type Message struct {
	ID        int64
	PeerID    string
	Sender    string
	Body      string
	Timestamp time.Time
	Outgoing  bool
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) Close() {
	s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			peer_id   TEXT NOT NULL,
			sender    TEXT NOT NULL,
			body      TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			outgoing  INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_messages_peer ON messages(peer_id, timestamp);

		CREATE TABLE IF NOT EXISTS crypto_sessions (
			peer_id       TEXT PRIMARY KEY,
			shared_secret BLOB NOT NULL
		);

		CREATE TABLE IF NOT EXISTS contacts (
			user_id   TEXT PRIMARY KEY,
			username  TEXT NOT NULL,
			alias     TEXT
		);

		CREATE TABLE IF NOT EXISTS prekeys (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			key_type      TEXT NOT NULL,
			public_key    BLOB NOT NULL,
			private_key   BLOB NOT NULL,
			used          INTEGER NOT NULL DEFAULT 0
		);
	`)
	return err
}

// SaveMessage stores a message locally.
func (s *Store) SaveMessage(peerID, sender, body string, ts time.Time, outgoing bool) error {
	out := 0
	if outgoing {
		out = 1
	}
	_, err := s.db.Exec(
		"INSERT INTO messages (peer_id, sender, body, timestamp, outgoing) VALUES (?, ?, ?, ?, ?)",
		peerID, sender, body, ts.Unix(), out,
	)
	return err
}

// GetMessages returns messages for a peer, newest first, with limit and offset.
func (s *Store) GetMessages(peerID string, limit, offset int) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, peer_id, sender, body, timestamp, outgoing FROM messages
		 WHERE peer_id = ? ORDER BY timestamp DESC, id DESC LIMIT ? OFFSET ?`,
		peerID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var ts int64
		var out int
		if err := rows.Scan(&m.ID, &m.PeerID, &m.Sender, &m.Body, &ts, &out); err != nil {
			return nil, err
		}
		m.Timestamp = time.Unix(ts, 0)
		m.Outgoing = out == 1
		msgs = append(msgs, m)
	}

	// Reverse to get chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// SaveContact stores a contact locally.
func (s *Store) SaveContact(userID, username string) error {
	_, err := s.db.Exec(
		"INSERT INTO contacts (user_id, username) VALUES (?, ?) ON CONFLICT(user_id) DO UPDATE SET username = ?",
		userID, username, username,
	)
	return err
}

// RenameContact sets an alias for a contact.
func (s *Store) RenameContact(username, alias string) error {
	_, err := s.db.Exec(
		"UPDATE contacts SET alias = ? WHERE username = ?",
		alias, username,
	)
	return err
}

// DeleteContact removes a contact locally.
func (s *Store) DeleteContact(userID string) error {
	_, err := s.db.Exec("DELETE FROM contacts WHERE user_id = ?", userID)
	return err
}

// GetContactAlias returns the alias for a contact, or empty string.
func (s *Store) GetContactAlias(userID string) string {
	var alias sql.NullString
	s.db.QueryRow("SELECT alias FROM contacts WHERE user_id = ?", userID).Scan(&alias)
	if alias.Valid {
		return alias.String
	}
	return ""
}

// SaveCryptoSession stores a shared secret for a peer.
func (s *Store) SaveCryptoSession(peerID string, sharedSecret []byte) error {
	_, err := s.db.Exec(
		"INSERT INTO crypto_sessions (peer_id, shared_secret) VALUES (?, ?) ON CONFLICT(peer_id) DO UPDATE SET shared_secret = ?",
		peerID, sharedSecret, sharedSecret,
	)
	return err
}

// GetCryptoSession retrieves a shared secret for a peer.
func (s *Store) GetCryptoSession(peerID string) ([]byte, error) {
	var secret []byte
	err := s.db.QueryRow("SELECT shared_secret FROM crypto_sessions WHERE peer_id = ?", peerID).Scan(&secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

// GetContactUserID returns the user ID for a contact by username or alias.
func (s *Store) GetContactUserID(nameOrAlias string) (string, error) {
	var userID string
	err := s.db.QueryRow(
		"SELECT user_id FROM contacts WHERE username = ? OR alias = ?",
		nameOrAlias, nameOrAlias,
	).Scan(&userID)
	return userID, err
}

// SearchMessages searches messages containing the query string.
func (s *Store) SearchMessages(query string, limit int) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, peer_id, sender, body, timestamp, outgoing FROM messages
		 WHERE body LIKE ? ORDER BY timestamp DESC LIMIT ?`,
		"%"+query+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var ts int64
		var out int
		if err := rows.Scan(&m.ID, &m.PeerID, &m.Sender, &m.Body, &ts, &out); err != nil {
			return nil, err
		}
		m.Timestamp = time.Unix(ts, 0)
		m.Outgoing = out == 1
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// SavePreKeys stores pre-key private keys locally.
func (s *Store) SavePreKeys(signedPub, signedPriv []byte, oneTimePubs, oneTimePrivs [][]byte) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear old keys
	if _, err := tx.Exec("DELETE FROM prekeys"); err != nil {
		return err
	}

	// Save signed pre-key
	if _, err := tx.Exec(
		"INSERT INTO prekeys (key_type, public_key, private_key) VALUES ('signed', ?, ?)",
		signedPub, signedPriv,
	); err != nil {
		return err
	}

	// Save one-time pre-keys
	for i := range oneTimePubs {
		if _, err := tx.Exec(
			"INSERT INTO prekeys (key_type, public_key, private_key) VALUES ('onetime', ?, ?)",
			oneTimePubs[i], oneTimePrivs[i],
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetSignedPreKeyPrivate returns the signed pre-key private key.
func (s *Store) GetSignedPreKeyPrivate() ([]byte, error) {
	var priv []byte
	err := s.db.QueryRow("SELECT private_key FROM prekeys WHERE key_type = 'signed' LIMIT 1").Scan(&priv)
	return priv, err
}

// ConsumeOneTimeKey finds a one-time key by public key and marks it as used, returning the private key.
func (s *Store) ConsumeOneTimeKey(publicKey []byte) ([]byte, error) {
	var id int64
	var priv []byte
	err := s.db.QueryRow(
		"SELECT id, private_key FROM prekeys WHERE key_type = 'onetime' AND public_key = ? AND used = 0",
		publicKey,
	).Scan(&id, &priv)
	if err != nil {
		return nil, err
	}

	_, err = s.db.Exec("UPDATE prekeys SET used = 1 WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return priv, nil
}
