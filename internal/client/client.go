package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"crypto/rand"
	"encoding/hex"

	pb "github.com/isalikov/cgram-proto/gen/proto"
	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"
)

type Client struct {
	url  string
	conn *websocket.Conn
	mu   sync.Mutex

	// Request-response correlation
	pending sync.Map // requestID -> chan *pb.Frame

	// Server-push channel for incoming envelopes, presence events, etc.
	Push chan *pb.Frame

	// done is closed when readLoop exits; never closed again on reconnect.
	// Consumers check this to know when the connection dropped.
	done      chan struct{}
	doneMu    sync.Mutex

	connected atomic.Bool
	ctx       context.Context
	cancel    context.CancelFunc
}

func New(url string) *Client {
	return &Client{
		url:  url,
		Push: make(chan *pb.Frame, 256),
		done: make(chan struct{}),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	conn, _, err := websocket.Dial(c.ctx, c.url, nil)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	conn.SetReadLimit(64 * 1024)

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	c.connected.Store(true)

	// Reset done channel for this connection
	c.doneMu.Lock()
	c.done = make(chan struct{})
	c.doneMu.Unlock()

	go c.readLoop()
	return nil
}

func (c *Client) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close(websocket.StatusNormalClosure, "bye")
	}
	c.mu.Unlock()
}

func (c *Client) IsConnected() bool {
	return c.connected.Load()
}

// Done returns a channel that is closed when the current connection's readLoop exits.
func (c *Client) Done() <-chan struct{} {
	c.doneMu.Lock()
	defer c.doneMu.Unlock()
	return c.done
}

func (c *Client) Send(frame *pb.Frame) error {
	data, err := proto.Marshal(frame)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.Write(c.ctx, websocket.MessageBinary, data)
}

// SendAndWait sends a frame and waits for a correlated response.
func (c *Client) SendAndWait(ctx context.Context, frame *pb.Frame) (*pb.Frame, error) {
	if frame.RequestId == "" {
		frame.RequestId = generateRequestID()
	}

	ch := make(chan *pb.Frame, 1)
	c.pending.Store(frame.RequestId, ch)
	defer c.pending.Delete(frame.RequestId)

	if err := c.Send(frame); err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Client) readLoop() {
	defer func() {
		c.connected.Store(false)
		// Signal that this connection is dead. Do NOT close c.Push —
		// it survives across reconnections.
		c.doneMu.Lock()
		close(c.done)
		c.doneMu.Unlock()
	}()

	for {
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			if c.ctx.Err() != nil {
				return
			}
			log.Printf("read error: %v", err)
			return
		}

		frame := &pb.Frame{}
		if err := proto.Unmarshal(data, frame); err != nil {
			log.Printf("unmarshal error: %v", err)
			continue
		}

		// Check if this is a response to a pending request
		if frame.RequestId != "" {
			if ch, ok := c.pending.LoadAndDelete(frame.RequestId); ok {
				ch.(chan *pb.Frame) <- frame
				continue
			}
		}

		// Server-push message
		select {
		case c.Push <- frame:
		default:
			log.Printf("push channel full, dropping frame")
		}
	}
}

// Login sends login request and returns session token.
func (c *Client) Login(ctx context.Context, username string, password []byte) (string, error) {
	resp, err := c.SendAndWait(ctx, &pb.Frame{
		Payload: &pb.Frame_LoginRequest{LoginRequest: &pb.LoginRequest{
			Username:    username,
			AuthMessage: password,
		}},
	})
	if err != nil {
		return "", err
	}

	switch p := resp.Payload.(type) {
	case *pb.Frame_LoginResponse:
		return p.LoginResponse.SessionToken, nil
	case *pb.Frame_Error:
		return "", fmt.Errorf("%s", p.Error.Message)
	default:
		return "", fmt.Errorf("unexpected response")
	}
}

// Register sends registration request and returns user ID.
func (c *Client) Register(ctx context.Context, username string, password []byte, identityKey []byte) (string, error) {
	resp, err := c.SendAndWait(ctx, &pb.Frame{
		Payload: &pb.Frame_RegisterRequest{RegisterRequest: &pb.RegisterRequest{
			Username:          username,
			PasswordVerifier:  password,
			PublicIdentityKey: identityKey,
		}},
	})
	if err != nil {
		return "", err
	}

	switch p := resp.Payload.(type) {
	case *pb.Frame_RegisterResponse:
		return p.RegisterResponse.UserId, nil
	case *pb.Frame_Error:
		return "", fmt.Errorf("%s", p.Error.Message)
	default:
		return "", fmt.Errorf("unexpected response")
	}
}

// ListContacts fetches the contact list from the server.
func (c *Client) ListContacts(ctx context.Context) ([]*pb.Contact, error) {
	resp, err := c.SendAndWait(ctx, &pb.Frame{
		Payload: &pb.Frame_ListContactsRequest{ListContactsRequest: &pb.ListContactsRequest{}},
	})
	if err != nil {
		return nil, err
	}

	switch p := resp.Payload.(type) {
	case *pb.Frame_ListContactsResponse:
		return p.ListContactsResponse.Contacts, nil
	case *pb.Frame_Error:
		return nil, fmt.Errorf("%s", p.Error.Message)
	default:
		return nil, fmt.Errorf("unexpected response")
	}
}

// AddContact adds a contact by username.
func (c *Client) AddContact(ctx context.Context, username string) (*pb.AddContactResponse, error) {
	resp, err := c.SendAndWait(ctx, &pb.Frame{
		Payload: &pb.Frame_AddContactRequest{AddContactRequest: &pb.AddContactRequest{
			Username: username,
		}},
	})
	if err != nil {
		return nil, err
	}

	switch p := resp.Payload.(type) {
	case *pb.Frame_AddContactResponse:
		return p.AddContactResponse, nil
	case *pb.Frame_Error:
		return nil, fmt.Errorf("%s", p.Error.Message)
	default:
		return nil, fmt.Errorf("unexpected response")
	}
}

// RemoveContact removes a contact by user ID.
func (c *Client) RemoveContact(ctx context.Context, userID string) error {
	resp, err := c.SendAndWait(ctx, &pb.Frame{
		Payload: &pb.Frame_RemoveContactRequest{RemoveContactRequest: &pb.RemoveContactRequest{
			UserId: userID,
		}},
	})
	if err != nil {
		return err
	}

	switch p := resp.Payload.(type) {
	case *pb.Frame_RemoveContactResponse:
		_ = p
		return nil
	case *pb.Frame_Error:
		return fmt.Errorf("%s", p.Error.Message)
	default:
		return fmt.Errorf("unexpected response")
	}
}

// GetStats fetches server statistics.
func (c *Client) GetStats(ctx context.Context) (*pb.StatsResponse, error) {
	resp, err := c.SendAndWait(ctx, &pb.Frame{
		Payload: &pb.Frame_StatsRequest{StatsRequest: &pb.StatsRequest{}},
	})
	if err != nil {
		return nil, err
	}

	switch p := resp.Payload.(type) {
	case *pb.Frame_StatsResponse:
		return p.StatsResponse, nil
	case *pb.Frame_Error:
		return nil, fmt.Errorf("%s", p.Error.Message)
	default:
		return nil, fmt.Errorf("unexpected response")
	}
}

// SendEnvelope sends an encrypted envelope.
func (c *Client) SendEnvelope(ctx context.Context, env *pb.Envelope) error {
	resp, err := c.SendAndWait(ctx, &pb.Frame{
		Payload: &pb.Frame_Envelope{Envelope: env},
	})
	if err != nil {
		return err
	}

	switch p := resp.Payload.(type) {
	case *pb.Frame_Ack:
		_ = p
		return nil
	case *pb.Frame_Error:
		return fmt.Errorf("%s", p.Error.Message)
	default:
		return fmt.Errorf("unexpected response")
	}
}

// FetchPreKey fetches a user's pre-key bundle.
func (c *Client) FetchPreKey(ctx context.Context, userID string) (*pb.FetchPreKeyResponse, error) {
	resp, err := c.SendAndWait(ctx, &pb.Frame{
		Payload: &pb.Frame_FetchPreKeyRequest{FetchPreKeyRequest: &pb.FetchPreKeyRequest{
			UserId: userID,
		}},
	})
	if err != nil {
		return nil, err
	}

	switch p := resp.Payload.(type) {
	case *pb.Frame_FetchPreKeyResponse:
		return p.FetchPreKeyResponse, nil
	case *pb.Frame_Error:
		return nil, fmt.Errorf("%s", p.Error.Message)
	default:
		return nil, fmt.Errorf("unexpected response")
	}
}

// UploadPreKeys uploads pre-key bundle to server.
func (c *Client) UploadPreKeys(ctx context.Context, bundle *pb.PreKeyBundle) error {
	resp, err := c.SendAndWait(ctx, &pb.Frame{
		Payload: &pb.Frame_UploadPreKeysRequest{UploadPreKeysRequest: &pb.UploadPreKeysRequest{
			Bundle: bundle,
		}},
	})
	if err != nil {
		return err
	}

	switch p := resp.Payload.(type) {
	case *pb.Frame_UploadPreKeysResponse:
		_ = p
		return nil
	case *pb.Frame_Error:
		return fmt.Errorf("%s", p.Error.Message)
	default:
		return fmt.Errorf("unexpected response")
	}
}

func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// WaitForPush blocks until a push message arrives or the connection drops.
// Returns nil if the connection was closed.
func (c *Client) WaitForPush() *pb.Frame {
	done := c.Done()
	select {
	case frame := <-c.Push:
		return frame
	case <-done:
		return nil
	}
}

// ReconnectLoop attempts to reconnect with exponential backoff.
func (c *Client) ReconnectLoop(ctx context.Context) error {
	delays := []time.Duration{1, 2, 4, 8, 16, 30}
	for i := 0; ; i++ {
		delay := delays[len(delays)-1]
		if i < len(delays) {
			delay = delays[i]
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay * time.Second):
		}

		if err := c.Connect(ctx); err != nil {
			log.Printf("reconnect attempt %d failed: %v", i+1, err)
			continue
		}
		return nil
	}
}
