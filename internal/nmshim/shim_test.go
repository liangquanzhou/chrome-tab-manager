package nmshim

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ctm/internal/daemon"
	"ctm/internal/protocol"
)

func TestReadWriteNMFrameRoundTrip(t *testing.T) {
	messages := []map[string]any{
		{"type": "request", "action": "tabs.list"},
		{"id": "abc-123", "type": "response", "payload": map[string]any{"tabs": []any{}}},
		{"key": "value", "nested": map[string]any{"a": 1, "b": "two"}},
	}

	for _, msg := range messages {
		original, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var buf bytes.Buffer
		if err := WriteNMFrame(&buf, original); err != nil {
			t.Fatalf("WriteNMFrame: %v", err)
		}

		// Verify wire format: 4-byte LE length + data
		wireData := buf.Bytes()
		expectedLen := 4 + len(original)
		if len(wireData) != expectedLen {
			t.Fatalf("wire length: got %d, want %d", len(wireData), expectedLen)
		}

		got, err := ReadNMFrame(&buf)
		if err != nil {
			t.Fatalf("ReadNMFrame: %v", err)
		}
		if !bytes.Equal(got, original) {
			t.Errorf("round trip mismatch:\n  got:  %s\n  want: %s", got, original)
		}
	}
}

func TestReadWriteNMFrameMultiple(t *testing.T) {
	// Write multiple frames to one buffer, then read them all back.
	payloads := [][]byte{
		[]byte(`{"a":1}`),
		[]byte(`{"b":2}`),
		[]byte(`{"c":3}`),
	}

	var buf bytes.Buffer
	for _, p := range payloads {
		if err := WriteNMFrame(&buf, p); err != nil {
			t.Fatalf("WriteNMFrame: %v", err)
		}
	}

	for i, want := range payloads {
		got, err := ReadNMFrame(&buf)
		if err != nil {
			t.Fatalf("ReadNMFrame[%d]: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("frame[%d]: got %s, want %s", i, got, want)
		}
	}

	// After all frames consumed, next read should return error (EOF).
	_, err := ReadNMFrame(&buf)
	if err == nil {
		t.Error("expected error after all frames consumed")
	}
}

func TestReadNMFrameLargeMessage(t *testing.T) {
	// 900KB payload - under the 1MB limit.
	payload := make([]byte, 900*1024)
	for i := range payload {
		payload[i] = 'A' + byte(i%26)
	}
	// Wrap in valid JSON
	data, _ := json.Marshal(string(payload))

	var buf bytes.Buffer
	if err := WriteNMFrame(&buf, data); err != nil {
		t.Fatalf("WriteNMFrame: %v", err)
	}

	got, err := ReadNMFrame(&buf)
	if err != nil {
		t.Fatalf("ReadNMFrame 900KB: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Error("900KB round trip data mismatch")
	}
}

func TestReadNMFrameTruncated(t *testing.T) {
	// Write a valid length header claiming 100 bytes, but only provide 10.
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(100))
	buf.Write([]byte("0123456789"))

	_, err := ReadNMFrame(&buf)
	if err == nil {
		t.Fatal("expected error for truncated frame")
	}
}

func TestReadNMFrameZeroLength(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	_, err := ReadNMFrame(&buf)
	if err == nil {
		t.Fatal("expected error for zero-length frame")
	}
	if got := err.Error(); got != "empty frame" {
		t.Errorf("error message: got %q, want %q", got, "empty frame")
	}
}

func TestReadNMFrameTooLarge(t *testing.T) {
	// Length header claiming > 1MB.
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(maxMessageSize+1))

	_, err := ReadNMFrame(&buf)
	if err == nil {
		t.Fatal("expected error for oversized frame")
	}
}

func TestReadNMFrameEmptyReader(t *testing.T) {
	var buf bytes.Buffer
	_, err := ReadNMFrame(&buf)
	if err == nil {
		t.Fatal("expected error for empty reader")
	}
}

func TestWriteNMFrameEmptyData(t *testing.T) {
	// Writing zero-length data should succeed (WriteNMFrame doesn't validate).
	// ReadNMFrame will reject it on the read side.
	var buf bytes.Buffer
	if err := WriteNMFrame(&buf, []byte{}); err != nil {
		t.Fatalf("WriteNMFrame empty: %v", err)
	}

	// Verify it wrote a 0-length header
	var length uint32
	binary.Read(&buf, binary.LittleEndian, &length)
	if length != 0 {
		t.Errorf("expected length 0, got %d", length)
	}
}

func TestReadNMFrameExactMaxSize(t *testing.T) {
	// Exactly maxMessageSize should succeed.
	data := bytes.Repeat([]byte("x"), maxMessageSize)
	var buf bytes.Buffer
	if err := WriteNMFrame(&buf, data); err != nil {
		t.Fatalf("WriteNMFrame: %v", err)
	}

	got, err := ReadNMFrame(&buf)
	if err != nil {
		t.Fatalf("ReadNMFrame at exact max: %v", err)
	}
	if len(got) != maxMessageSize {
		t.Errorf("got length %d, want %d", len(got), maxMessageSize)
	}
}

func TestWriteNMFrameWriterError(t *testing.T) {
	w := &failWriter{failAfter: 0}
	err := WriteNMFrame(w, []byte("hello"))
	if err == nil {
		t.Fatal("expected error from failing writer")
	}
}

// failWriter fails after writing failAfter bytes.
type failWriter struct {
	failAfter int
	written   int
}

func (f *failWriter) Write(p []byte) (int, error) {
	if f.written >= f.failAfter {
		return 0, io.ErrClosedPipe
	}
	n := len(p)
	f.written += n
	return n, nil
}

// --- Run integration tests ---

// startTestDaemon creates a daemon Server on a temp socket and returns the socket path.
// The server is stopped when the test completes.
func startTestDaemon(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")
	lockPath := filepath.Join(dir, "test.lock")
	sessionsDir := filepath.Join(dir, "sessions")
	collectionsDir := filepath.Join(dir, "collections")
	bookmarksDir := filepath.Join(dir, "bookmarks")
	overlaysDir := filepath.Join(dir, "overlays")
	workspacesDir := filepath.Join(dir, "workspaces")
	savedSearchesDir := filepath.Join(dir, "searches")
	syncCloudDir := filepath.Join(dir, "cloud")
	searchIndexPath := filepath.Join(dir, "search_index.json")

	os.MkdirAll(sessionsDir, 0700)
	os.MkdirAll(collectionsDir, 0700)
	os.MkdirAll(bookmarksDir, 0700)
	os.MkdirAll(overlaysDir, 0700)
	os.MkdirAll(workspacesDir, 0700)
	os.MkdirAll(savedSearchesDir, 0700)

	srv := daemon.NewServer(sockPath, lockPath, sessionsDir, collectionsDir, bookmarksDir, overlaysDir, workspacesDir, savedSearchesDir, syncCloudDir, searchIndexPath)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Wait for socket to be ready
	for i := 0; i < 50; i++ {
		if conn, err := net.Dial("unix", sockPath); err == nil {
			conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	return sockPath
}

// writeNMMessage marshals a protocol.Message and writes it as an NM frame.
func writeNMMessage(w io.Writer, msg *protocol.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return WriteNMFrame(w, data)
}

// readNMMessage reads an NM frame and unmarshals it into a protocol.Message.
func readNMMessage(r io.Reader) (*protocol.Message, error) {
	data, err := ReadNMFrame(r)
	if err != nil {
		return nil, err
	}
	var msg protocol.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &msg, nil
}

func TestRunHappyPath(t *testing.T) {
	sockPath := startTestDaemon(t)

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, sockPath, stdinR, stdoutW)
	}()

	// Write a hello message via stdin (NM frame)
	hello := &protocol.Message{
		ID:              protocol.MakeID(),
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeHello,
		Payload: mustMarshalJSON(map[string]any{
			"channel":      "stable",
			"extensionId":  "test-ext",
			"instanceId":   "test-inst",
			"userAgent":    "Chrome/130.0 Test",
			"capabilities": []string{"tabs", "groups"},
		}),
	}
	if err := writeNMMessage(stdinW, hello); err != nil {
		t.Fatalf("writeNMMessage hello: %v", err)
	}

	// Read the response from stdout (NM frame)
	resp, err := readNMMessage(stdoutR)
	if err != nil {
		t.Fatalf("readNMMessage response: %v", err)
	}

	if resp.Type != protocol.TypeResponse {
		t.Errorf("response type = %q, want response", resp.Type)
	}
	if resp.ID != hello.ID {
		t.Errorf("response ID = %q, want %q", resp.ID, hello.ID)
	}

	// Verify targetId is in the payload
	var payload map[string]string
	if err := json.Unmarshal(resp.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["targetId"] == "" {
		t.Error("response payload missing targetId")
	}

	// Clean up: cancel context to stop Run
	cancel()
	<-errCh
}

func TestRunDaemonConnectionFailure(t *testing.T) {
	stdinR, _ := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	defer stdinR.Close()
	defer stdoutR.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Run(ctx, "/nonexistent.sock", stdinR, stdoutW)
	if err == nil {
		t.Fatal("expected error for nonexistent socket")
	}
	if !strings.Contains(err.Error(), "connect to daemon") {
		t.Errorf("error = %q, want to contain 'connect to daemon'", err.Error())
	}
}

func TestRunStdinClosed(t *testing.T) {
	sockPath := startTestDaemon(t)

	stdinR, stdinW := io.Pipe()
	_, stdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, sockPath, stdinR, stdoutW)
	}()

	// Give Run a moment to establish the connection
	time.Sleep(100 * time.Millisecond)

	// Close stdin — should cause Run to exit with an error
	stdinW.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error when stdin is closed")
		}
		// Should be a stdin read error
		if !strings.Contains(err.Error(), "stdin") {
			t.Errorf("error = %q, want to contain 'stdin'", err.Error())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for Run to exit after stdin close")
	}
}

func TestRunContextCancel(t *testing.T) {
	sockPath := startTestDaemon(t)

	stdinR, _ := io.Pipe()
	_, stdoutW := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, sockPath, stdinR, stdoutW)
	}()

	// Give Run a moment to start
	time.Sleep(100 * time.Millisecond)

	// Cancel the context
	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Errorf("error = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for Run to exit after context cancel")
	}
}

func TestRunBidirectionalForwarding(t *testing.T) {
	sockPath := startTestDaemon(t)

	// --- Start the shim (simulates an extension connecting via NM) ---
	shimStdinR, shimStdinW := io.Pipe()
	shimStdoutR, shimStdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shimErr := make(chan error, 1)
	go func() {
		shimErr <- Run(ctx, sockPath, shimStdinR, shimStdoutW)
	}()

	// Send hello through the shim to register as an extension
	hello := &protocol.Message{
		ID:              protocol.MakeID(),
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeHello,
		Payload: mustMarshalJSON(map[string]any{
			"channel":      "stable",
			"extensionId":  "shim-ext",
			"instanceId":   "shim-inst",
			"userAgent":    "Chrome/130.0 ShimTest",
			"capabilities": []string{"tabs"},
		}),
	}
	if err := writeNMMessage(shimStdinW, hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	// Read hello response to get targetId
	helloResp, err := readNMMessage(shimStdoutR)
	if err != nil {
		t.Fatalf("read hello response: %v", err)
	}
	var helloPayload map[string]string
	json.Unmarshal(helloResp.Payload, &helloPayload)
	shimTargetID := helloPayload["targetId"]
	if shimTargetID == "" {
		t.Fatal("shim targetId is empty")
	}

	// --- Connect a separate client directly to the daemon ---
	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial client: %v", err)
	}
	t.Cleanup(func() { clientConn.Close() })
	clientR := protocol.NewReader(clientConn)
	clientW := protocol.NewWriter(clientConn)

	// Client sends a tabs.list request targeting the shim extension
	reqID := protocol.MakeID()
	clientW.Write(&protocol.Message{
		ID:              reqID,
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeRequest,
		Action:          "tabs.list",
	})

	// The shim-connected extension should receive the forwarded request via stdout
	fwdReq, err := readNMMessage(shimStdoutR)
	if err != nil {
		t.Fatalf("read forwarded request: %v", err)
	}
	if fwdReq.Action != "tabs.list" {
		t.Errorf("forwarded action = %q, want tabs.list", fwdReq.Action)
	}

	// Extension sends response back through the shim (via stdin)
	if err := writeNMMessage(shimStdinW, &protocol.Message{
		ID:              fwdReq.ID,
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeResponse,
		Payload:         mustMarshalJSON(map[string]any{"tabs": []any{}}),
	}); err != nil {
		t.Fatalf("write response: %v", err)
	}

	// Client should receive the response
	clientResp, err := clientR.Read()
	if err != nil {
		t.Fatalf("client read response: %v", err)
	}
	if clientResp.Type != protocol.TypeResponse {
		t.Errorf("client response type = %q, want response", clientResp.Type)
	}

	// Clean up
	cancel()
	<-shimErr
}

// mustMarshalJSON is a test helper to marshal a value to json.RawMessage.
func mustMarshalJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

// --- Error injection tests ---

func TestWriteNMFrame_DataWriteError(t *testing.T) {
	// failWriter that succeeds for the length prefix (4 bytes) but fails on data write
	w := &failWriter{failAfter: 4}
	err := WriteNMFrame(w, []byte("hello"))
	if err == nil {
		t.Fatal("expected error when data write fails")
	}
	if !strings.Contains(err.Error(), "write frame data") {
		t.Errorf("error should mention write frame data, got: %v", err)
	}
}

func TestWriteNMFrame_LengthWriteError(t *testing.T) {
	// failWriter that fails immediately (on length prefix write)
	w := &failWriter{failAfter: 0}
	err := WriteNMFrame(w, []byte("hello"))
	if err == nil {
		t.Fatal("expected error when length write fails")
	}
	if !strings.Contains(err.Error(), "write frame length") {
		t.Errorf("error should mention write frame length, got: %v", err)
	}
}

func FuzzNMFrame(f *testing.F) {
	// Seed corpus
	f.Add([]byte(`{"type":"request"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"id":"1","action":"tabs.list","payload":null}`))
	f.Add([]byte(`{"nested":{"deep":{"value":42}}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		// Test write -> read round trip
		var buf bytes.Buffer
		if err := WriteNMFrame(&buf, data); err != nil {
			t.Fatalf("WriteNMFrame: %v", err)
		}

		got, err := ReadNMFrame(&buf)
		if err != nil {
			t.Fatalf("ReadNMFrame: %v", err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("round trip mismatch: got %d bytes, want %d bytes", len(got), len(data))
		}
	})
}
