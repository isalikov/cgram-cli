package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "github.com/isalikov/cgram-proto/gen/proto"
	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"
)

type Client struct {
	addr string
	conn *websocket.Conn
	mu   sync.Mutex

	connected bool

	// Incoming holds pushed frames (envelopes, etc.) for the TUI to consume.
	Incoming chan *pb.Frame

	// Status receives connection state changes (true=connected, false=disconnected).
	Status chan bool

	pending   map[string]chan *pb.Frame
	pendingMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc

	onReconnect func()
}

func New(addr string) *Client {
	return &Client{
		addr:     addr,
		Incoming: make(chan *pb.Frame, 64),
		Status:   make(chan bool, 8),
		pending:  make(map[string]chan *pb.Frame),
	}
}

func (c *Client) OnReconnect(fn func()) { c.onReconnect = fn }

func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *Client) Connect(parentCtx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(parentCtx)

	url := fmt.Sprintf("ws://%s/ws", c.addr)
	conn, _, err := websocket.Dial(c.ctx, url, nil)
	if err != nil {
		// Start background reconnect so the app can recover
		go c.reconnect()
		return fmt.Errorf("dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	go c.readLoop()
	return nil
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}
	if c.conn != nil {
		c.conn.Close(websocket.StatusNormalClosure, "bye")
		c.conn = nil
	}
	c.connected = false
}

func (c *Client) readLoop() {
	for {
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			c.mu.Lock()
			c.connected = false
			c.conn = nil
			c.mu.Unlock()

			// Notify TUI of disconnect
			select {
			case c.Status <- false:
			default:
			}

			c.reconnect()
			return
		}

		frame := &pb.Frame{}
		if err := proto.Unmarshal(data, frame); err != nil {
			continue
		}

		// Check if this is a response to a pending request
		if frame.RequestId != "" {
			c.pendingMu.Lock()
			ch, ok := c.pending[frame.RequestId]
			if ok {
				delete(c.pending, frame.RequestId)
				c.pendingMu.Unlock()
				ch <- frame
				continue
			}
			c.pendingMu.Unlock()
		}

		// Push message (envelope, etc.) to the TUI
		select {
		case c.Incoming <- frame:
		default:
			// drop if buffer full
		}
	}
}

func (c *Client) reconnect() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}

		url := fmt.Sprintf("ws://%s/ws", c.addr)
		conn, _, err := websocket.Dial(c.ctx, url, nil)
		if err != nil {
			continue
		}

		c.mu.Lock()
		c.conn = conn
		c.connected = true
		c.mu.Unlock()

		// Notify TUI of reconnect
		select {
		case c.Status <- true:
		default:
		}

		if c.onReconnect != nil {
			c.onReconnect()
		}

		go c.readLoop()
		return
	}
}

// Send sends a frame and waits for a response with matching request_id.
func (c *Client) Send(ctx context.Context, frame *pb.Frame) (*pb.Frame, error) {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Register pending response channel
	ch := make(chan *pb.Frame, 1)
	c.pendingMu.Lock()
	c.pending[frame.RequestId] = ch
	c.pendingMu.Unlock()

	data, err := proto.Marshal(frame)
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, frame.RequestId)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("marshal: %w", err)
	}

	err = conn.Write(ctx, websocket.MessageBinary, data)
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, frame.RequestId)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("write: %w", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-ch:
		// Check for error response
		if errPayload, ok := resp.Payload.(*pb.Frame_Error); ok {
			return resp, fmt.Errorf("server error %d: %s", errPayload.Error.Code, errPayload.Error.Message)
		}
		return resp, nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, frame.RequestId)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		c.pendingMu.Lock()
		delete(c.pending, frame.RequestId)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("request timeout")
	}
}

// SendFire sends a frame without waiting for a response.
func (c *Client) SendFire(ctx context.Context, frame *pb.Frame) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := proto.Marshal(frame)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	return conn.Write(ctx, websocket.MessageBinary, data)
}
