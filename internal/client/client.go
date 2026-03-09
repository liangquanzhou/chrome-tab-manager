package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"ctm/internal/protocol"
)

// Client connects to the CTM daemon over a Unix socket.
type Client struct {
	socketPath string

	mu        sync.Mutex
	conn      net.Conn
	reader    *protocol.Reader
	writer    *protocol.Writer
	connected bool
	closed    bool

	pendingMu sync.Mutex
	pending   map[string]chan *protocol.Message

	eventCh chan *protocol.Message
}

func New(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		pending:    make(map[string]chan *protocol.Message),
		eventCh:    make(chan *protocol.Message, 64),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("client closed")
	}

	// Close existing connection if any
	if c.conn != nil {
		c.conn.Close()
		c.connected = false
	}

	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("connect %s: %w", c.socketPath, err)
	}

	c.conn = conn
	c.reader = protocol.NewReader(conn)
	c.writer = protocol.NewWriter(conn)
	c.connected = true

	go c.readLoop(c.conn, c.reader)

	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true
	c.connected = false

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *Client) Request(ctx context.Context, action string, payload any, target *protocol.TargetSelector) (*protocol.Message, error) {
	id := protocol.MakeID()

	var rawPayload json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		rawPayload = data
	}

	msg := &protocol.Message{
		ID:              id,
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeRequest,
		Action:          action,
		Target:          target,
		Payload:         rawPayload,
	}

	ch := make(chan *protocol.Message, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	c.mu.Lock()
	writer := c.writer
	isConnected := c.connected
	c.mu.Unlock()

	if !isConnected || writer == nil {
		return nil, fmt.Errorf("not connected")
	}

	if err := writer.Write(msg); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	timeout := 10 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}

	select {
	case resp := <-ch:
		if resp.Type == protocol.TypeError && resp.Error != nil {
			return resp, &protocol.ProtocolError{
				Action:  action,
				Code:    resp.Error.Code,
				Message: resp.Error.Message,
			}
		}
		return resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("request %s: timeout after %s", action, timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Client) Subscribe(ctx context.Context, patterns []string) (<-chan *protocol.Message, error) {
	payload := map[string]any{"patterns": patterns}
	_, err := c.Request(ctx, "subscribe", payload, nil)
	if err != nil {
		return nil, fmt.Errorf("subscribe: %w", err)
	}
	return c.eventCh, nil
}

// readLoop reads messages from conn and dispatches them.
// Exits when the connection is closed or an error occurs.
// Each readLoop is bound to a specific conn to avoid races on reconnect.
func (c *Client) readLoop(conn net.Conn, reader *protocol.Reader) {
	for {
		msg, err := reader.Read()
		if err != nil {
			c.mu.Lock()
			// Only mark disconnected if this readLoop's conn is still current
			if c.conn == conn {
				c.connected = false
			}
			c.mu.Unlock()

			c.drainPending()
			return
		}

		switch msg.Type {
		case protocol.TypeResponse, protocol.TypeError:
			c.pendingMu.Lock()
			if ch, ok := c.pending[msg.ID]; ok {
				ch <- msg
			}
			c.pendingMu.Unlock()
		case protocol.TypeEvent:
			select {
			case c.eventCh <- msg:
			default:
				// drop if channel full
			}
		}
	}
}

func (c *Client) drainPending() {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	for id, ch := range c.pending {
		select {
		case ch <- &protocol.Message{
			ID:   id,
			Type: protocol.TypeError,
			Error: &protocol.ErrorBody{
				Code:    protocol.ErrDaemonUnavailable,
				Message: "connection lost",
			},
		}:
		default:
		}
	}
}

// Reconnect attempts to re-establish the connection with exponential backoff.
// Blocks until connected or ctx is cancelled.
func (c *Client) Reconnect(ctx context.Context) error {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		err := c.Connect(ctx)
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
