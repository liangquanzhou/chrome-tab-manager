package client

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ctm/internal/protocol"
)

// mockServer creates a Unix socket server for testing.
type mockServer struct {
	listener net.Listener
	sockPath string
}

func newMockServer(t *testing.T) *mockServer {
	t.Helper()
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	return &mockServer{listener: listener, sockPath: sockPath}
}

func (s *mockServer) close() {
	s.listener.Close()
	os.Remove(s.sockPath)
}

func (s *mockServer) accept() (net.Conn, error) {
	return s.listener.Accept()
}

func TestClientConnect(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	if !c.Connected() {
		t.Error("Connected() = false after Connect")
	}
}

func TestClientConnectFailure(t *testing.T) {
	c := New("/tmp/nonexistent-ctm-test.sock")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("Connect to nonexistent socket should fail")
	}
}

func TestClientRequestResponse(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	// Accept server-side connection
	serverConn, err := srv.accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	defer serverConn.Close()

	serverReader := protocol.NewReader(serverConn)
	serverWriter := protocol.NewWriter(serverConn)

	// Server goroutine: read request, send response
	go func() {
		req, err := serverReader.Read()
		if err != nil {
			return
		}

		resp := &protocol.Message{
			ID:              req.ID,
			ProtocolVersion: protocol.ProtocolVersion,
			Type:            protocol.TypeResponse,
			Action:          req.Action,
			Payload:         json.RawMessage(`{"tabs":[]}`),
		}
		serverWriter.Write(resp)
	}()

	resp, err := c.Request(ctx, "tabs.list", nil, nil)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}

	if resp.Type != protocol.TypeResponse {
		t.Errorf("response Type = %q, want response", resp.Type)
	}
	if resp.Action != "tabs.list" {
		t.Errorf("response Action = %q, want tabs.list", resp.Action)
	}
}

func TestClientRequestWithPayload(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	serverConn, err := srv.accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	defer serverConn.Close()

	serverReader := protocol.NewReader(serverConn)
	serverWriter := protocol.NewWriter(serverConn)

	go func() {
		req, err := serverReader.Read()
		if err != nil {
			return
		}

		// Echo back the payload in the response
		resp := &protocol.Message{
			ID:              req.ID,
			ProtocolVersion: protocol.ProtocolVersion,
			Type:            protocol.TypeResponse,
			Action:          req.Action,
			Payload:         req.Payload,
		}
		serverWriter.Write(resp)
	}()

	payload := map[string]string{"name": "work-afternoon"}
	target := &protocol.TargetSelector{TargetID: "target_1"}
	resp, err := c.Request(ctx, "sessions.save", payload, target)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}
	if result["name"] != "work-afternoon" {
		t.Errorf("payload name = %q, want work-afternoon", result["name"])
	}
}

func TestClientRequestTimeout(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	// Accept but never respond
	serverConn, err := srv.accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	defer serverConn.Close()

	_, err = c.Request(ctx, "tabs.list", nil, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestClientRequestNotConnected(t *testing.T) {
	c := New("/tmp/nonexistent-ctm-test.sock")

	ctx := context.Background()
	_, err := c.Request(ctx, "tabs.list", nil, nil)
	if err == nil {
		t.Fatal("expected error for request when not connected")
	}
}

func TestClientErrorResponse(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	serverConn, err := srv.accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	defer serverConn.Close()

	serverReader := protocol.NewReader(serverConn)
	serverWriter := protocol.NewWriter(serverConn)

	go func() {
		req, err := serverReader.Read()
		if err != nil {
			return
		}

		resp := &protocol.Message{
			ID:              req.ID,
			ProtocolVersion: protocol.ProtocolVersion,
			Type:            protocol.TypeError,
			Error: &protocol.ErrorBody{
				Code:    protocol.ErrUnknownAction,
				Message: "unknown action: foo.bar",
			},
		}
		serverWriter.Write(resp)
	}()

	resp, err := c.Request(ctx, "foo.bar", nil, nil)
	if err == nil {
		t.Fatal("expected error for error response")
	}

	if resp == nil {
		t.Fatal("response should not be nil for error response")
	}
	if resp.Error.Code != protocol.ErrUnknownAction {
		t.Errorf("error code = %q, want UNKNOWN_ACTION", resp.Error.Code)
	}
}

func TestClientEvents(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	serverConn, err := srv.accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	defer serverConn.Close()

	serverReader := protocol.NewReader(serverConn)
	serverWriter := protocol.NewWriter(serverConn)

	// Handle subscribe request, then send events
	go func() {
		req, err := serverReader.Read()
		if err != nil {
			return
		}

		// Respond to subscribe
		resp := &protocol.Message{
			ID:              req.ID,
			ProtocolVersion: protocol.ProtocolVersion,
			Type:            protocol.TypeResponse,
			Payload:         json.RawMessage(`{"subscribed":true}`),
		}
		serverWriter.Write(resp)

		// Send an event
		event := &protocol.Message{
			ID:              "evt_1",
			ProtocolVersion: protocol.ProtocolVersion,
			Type:            protocol.TypeEvent,
			Action:          "tabs.created",
			Payload:         json.RawMessage(`{"tab":{"id":123},"_target":{"targetId":"target_1"}}`),
		}
		serverWriter.Write(event)
	}()

	eventCh, err := c.Subscribe(ctx, []string{"tabs.*"})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	select {
	case evt := <-eventCh:
		if evt.Action != "tabs.created" {
			t.Errorf("event Action = %q, want tabs.created", evt.Action)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestClientDisconnect(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	serverConn, err := srv.accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}

	// Close server side → client should detect disconnect
	serverConn.Close()

	// Wait for readLoop to detect disconnect
	time.Sleep(100 * time.Millisecond)

	if c.Connected() {
		t.Error("Connected() = true after server disconnect")
	}
}

func TestClientClose(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if c.Connected() {
		t.Error("Connected() = true after Close")
	}

	// Connect after close should fail
	err := c.Connect(ctx)
	if err == nil {
		t.Error("Connect after Close should fail")
	}
}

// --- Reconnect tests ---

func TestReconnectSuccess(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Don't call Connect first — Reconnect should succeed on first attempt
	if err := c.Reconnect(ctx); err != nil {
		t.Fatalf("Reconnect: %v", err)
	}

	if !c.Connected() {
		t.Error("Connected() = false after Reconnect")
	}
}

func TestReconnectAfterDisconnect(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	// Start first server
	srv1, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen srv1: %v", err)
	}

	c := New(sockPath)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to first server
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Accept the connection on server side (required for readLoop)
	srvConn, err := srv1.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}

	if !c.Connected() {
		t.Fatal("expected Connected() = true after Connect")
	}

	// Shut down first server
	srvConn.Close()
	srv1.Close()
	os.Remove(sockPath)

	// Wait for client to detect disconnect
	time.Sleep(200 * time.Millisecond)

	if c.Connected() {
		t.Fatal("expected Connected() = false after server shutdown")
	}

	// Start a NEW server on the same socket path
	srv2, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen srv2: %v", err)
	}
	defer srv2.Close()
	defer os.Remove(sockPath)

	// Reconnect should succeed (backoff retries until new server is up)
	if err := c.Reconnect(ctx); err != nil {
		t.Fatalf("Reconnect: %v", err)
	}

	if !c.Connected() {
		t.Error("Connected() = false after Reconnect to new server")
	}
}

func TestReconnectContextCancel(t *testing.T) {
	// No server — nothing to connect to
	c := New("/tmp/nonexistent-ctm-reconnect-test.sock")
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Reconnect(ctx)
	}()

	// Cancel after 500ms
	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Errorf("Reconnect error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Reconnect to return after cancel")
	}
}

func TestReconnectExponentialBackoff(t *testing.T) {
	// No server — all attempts will fail
	c := New("/tmp/nonexistent-ctm-backoff-test.sock")
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	err := c.Reconnect(ctx)
	elapsed := time.Since(start)

	// Should fail with context deadline exceeded
	if err != context.DeadlineExceeded {
		t.Errorf("Reconnect error = %v, want context.DeadlineExceeded", err)
	}

	// Should have taken close to 3 seconds (the timeout), not returning immediately
	if elapsed < 2*time.Second {
		t.Errorf("Reconnect returned too quickly (%v); expected backoff retries for ~3s", elapsed)
	}

	// With initial backoff of 1s, doubling: attempt at 0s, retry after 1s, retry after 2s (=3s total)
	// So we expect at least 2 connection attempts were made (within the 3s window).
	// The elapsed time confirms retries with backoff occurred.
}

// --- Error injection tests ---

func TestSubscribeNotConnected(t *testing.T) {
	// Create client without connecting
	c := New("/tmp/nonexistent-ctm-sub-test.sock")
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := c.Subscribe(ctx, []string{"tabs.*"})
	if err == nil {
		t.Fatal("expected error when subscribing without connection")
	}
}

// newShortMockServer creates a mock server using a short /tmp path to avoid
// macOS 104-byte Unix socket path limit.
func newShortMockServer(t *testing.T) *mockServer {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ctm-c-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "t.sock")

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	return &mockServer{listener: listener, sockPath: sockPath}
}

func TestDrainPending_WithPendingRequests(t *testing.T) {
	srv := newShortMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	// Accept the server side connection
	serverConn, err := srv.accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}

	// Register a pending request that won't get a response
	ch := make(chan *protocol.Message, 1)
	c.pendingMu.Lock()
	c.pending["test-drain-id"] = ch
	c.pendingMu.Unlock()

	// Close the server side — triggers readLoop exit and drainPending
	serverConn.Close()

	// Wait for the drain to happen
	select {
	case msg := <-ch:
		if msg.Type != protocol.TypeError {
			t.Errorf("drained message type = %q, want error", msg.Type)
		}
		if msg.Error == nil || msg.Error.Code != protocol.ErrDaemonUnavailable {
			t.Errorf("drained error code = %v, want DAEMON_UNAVAILABLE", msg.Error)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for pending request to be drained")
	}
}

func TestRequestNotConnected_AfterClose(t *testing.T) {
	srv := newShortMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	c.Close()

	_, err := c.Request(ctx, "tabs.list", nil, nil)
	if err == nil {
		t.Fatal("expected error after close")
	}
}

func TestClientCloseWithoutConnect(t *testing.T) {
	c := New("/tmp/nonexistent-ctm-close-test.sock")
	// Closing without connecting should not error
	if err := c.Close(); err != nil {
		t.Fatalf("Close without connect: %v", err)
	}
}

func TestConnectAfterClose(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	c := New(srv.sockPath)
	c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("expected error connecting after close")
	}
	if err.Error() != "client closed" {
		t.Errorf("error = %q, want 'client closed'", err.Error())
	}
}
