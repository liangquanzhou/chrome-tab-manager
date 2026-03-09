package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestReaderSingleMessage(t *testing.T) {
	input := `{"id":"msg_1","protocol_version":1,"type":"request","action":"tabs.list"}` + "\n"
	r := NewReader(strings.NewReader(input))

	msg, err := r.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if msg.ID != "msg_1" {
		t.Errorf("ID = %q, want msg_1", msg.ID)
	}
	if msg.Action != "tabs.list" {
		t.Errorf("Action = %q, want tabs.list", msg.Action)
	}
}

func TestReaderMultipleMessages(t *testing.T) {
	input := `{"id":"msg_1","protocol_version":1,"type":"request","action":"tabs.list"}
{"id":"msg_2","protocol_version":1,"type":"request","action":"groups.list"}
{"id":"msg_3","protocol_version":1,"type":"event","action":"tabs.created"}
`
	r := NewReader(strings.NewReader(input))

	ids := []string{"msg_1", "msg_2", "msg_3"}
	for i, wantID := range ids {
		msg, err := r.Read()
		if err != nil {
			t.Fatalf("Read[%d]: %v", i, err)
		}
		if msg.ID != wantID {
			t.Errorf("Read[%d].ID = %q, want %q", i, msg.ID, wantID)
		}
	}

	_, err := r.Read()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestReaderSkipsEmptyLines(t *testing.T) {
	input := "\n\n" + `{"id":"msg_1","protocol_version":1,"type":"request"}` + "\n\n\n" +
		`{"id":"msg_2","protocol_version":1,"type":"response"}` + "\n\n"
	r := NewReader(strings.NewReader(input))

	msg1, err := r.Read()
	if err != nil {
		t.Fatalf("Read 1: %v", err)
	}
	if msg1.ID != "msg_1" {
		t.Errorf("msg1.ID = %q, want msg_1", msg1.ID)
	}

	msg2, err := r.Read()
	if err != nil {
		t.Fatalf("Read 2: %v", err)
	}
	if msg2.ID != "msg_2" {
		t.Errorf("msg2.ID = %q, want msg_2", msg2.ID)
	}

	_, err = r.Read()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestReaderTruncatedJSON(t *testing.T) {
	input := `{"id":"msg_1","protocol_version":1` + "\n"
	r := NewReader(strings.NewReader(input))

	_, err := r.Read()
	if err == nil {
		t.Fatal("expected error for truncated JSON")
	}
}

func TestReaderInvalidJSON(t *testing.T) {
	input := "not-json-at-all\n"
	r := NewReader(strings.NewReader(input))

	_, err := r.Read()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestWriterSingleMessage(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	msg := &Message{
		ID:              "msg_1",
		ProtocolVersion: ProtocolVersion,
		Type:            TypeRequest,
		Action:          "tabs.list",
	}

	if err := w.Write(msg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	output := buf.String()
	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}

	var decoded Message
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.ID != "msg_1" {
		t.Errorf("ID = %q, want msg_1", decoded.ID)
	}
}

func TestWriterThreadSafety(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	const goroutines = 50
	const perGoroutine = 20
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				msg := &Message{
					ID:              MakeID(),
					ProtocolVersion: ProtocolVersion,
					Type:            TypeRequest,
					Action:          "test.action",
				}
				if err := w.Write(msg); err != nil {
					t.Errorf("goroutine %d write %d: %v", gid, i, err)
				}
			}
		}(g)
	}
	wg.Wait()

	// Verify all messages are valid NDJSON
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != goroutines*perGoroutine {
		t.Errorf("got %d lines, want %d", len(lines), goroutines*perGoroutine)
	}
	for i, line := range lines {
		var msg Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestRoundTripReaderWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	messages := []*Message{
		{ID: "msg_1", ProtocolVersion: ProtocolVersion, Type: TypeRequest, Action: "tabs.list"},
		{ID: "msg_2", ProtocolVersion: ProtocolVersion, Type: TypeResponse, Payload: json.RawMessage(`{"tabs":[]}`)},
		{ID: "msg_3", ProtocolVersion: ProtocolVersion, Type: TypeError, Error: &ErrorBody{Code: ErrTimeout, Message: "timeout"}},
		{ID: "msg_4", ProtocolVersion: ProtocolVersion, Type: TypeEvent, Action: "tabs.created", Payload: json.RawMessage(`{"tab":{"id":1},"_target":{"targetId":"t1"}}`)},
	}

	for _, msg := range messages {
		if err := w.Write(msg); err != nil {
			t.Fatalf("Write %s: %v", msg.ID, err)
		}
	}

	r := NewReader(&buf)
	for i, want := range messages {
		got, err := r.Read()
		if err != nil {
			t.Fatalf("Read[%d]: %v", i, err)
		}
		if got.ID != want.ID {
			t.Errorf("[%d] ID = %q, want %q", i, got.ID, want.ID)
		}
		if got.Type != want.Type {
			t.Errorf("[%d] Type = %q, want %q", i, got.Type, want.Type)
		}
	}
}

func TestReaderEmptyInput(t *testing.T) {
	r := NewReader(strings.NewReader(""))
	_, err := r.Read()
	if err != io.EOF {
		t.Errorf("expected io.EOF for empty input, got %v", err)
	}
}

func TestReaderOnlyEmptyLines(t *testing.T) {
	r := NewReader(strings.NewReader("\n\n\n"))
	_, err := r.Read()
	if err != io.EOF {
		t.Errorf("expected io.EOF for only empty lines, got %v", err)
	}
}

// --- Error injection tests ---

func TestReaderOversizedMessage(t *testing.T) {
	// Create a valid JSON line larger than maxLineSize (1MB).
	// The scanner should return an error when it encounters this.
	bigValue := strings.Repeat("x", maxLineSize+100)
	line := `{"id":"` + bigValue + `"}` + "\n"
	r := NewReader(strings.NewReader(line))

	_, err := r.Read()
	if err == nil {
		t.Fatal("expected error for oversized message")
	}
	if !strings.Contains(err.Error(), "scan") {
		t.Errorf("error should mention scan, got: %v", err)
	}
}

// errWriter always returns an error on Write.
type errWriter struct{}

func (e *errWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestWriterError(t *testing.T) {
	w := NewWriter(&errWriter{})
	msg := &Message{
		ID:              "msg_1",
		ProtocolVersion: ProtocolVersion,
		Type:            TypeRequest,
		Action:          "tabs.list",
	}
	err := w.Write(msg)
	if err == nil {
		t.Fatal("expected error when writing to broken writer")
	}
	if !strings.Contains(err.Error(), "ndjson write") {
		t.Errorf("error should mention ndjson write, got: %v", err)
	}
}

func TestReaderIOError(t *testing.T) {
	// Use an io.Pipe and close the write end to simulate I/O error
	pr, pw := io.Pipe()
	pw.CloseWithError(io.ErrUnexpectedEOF)

	r := NewReader(pr)
	_, err := r.Read()
	if err == nil {
		t.Fatal("expected error from closed pipe")
	}
}
