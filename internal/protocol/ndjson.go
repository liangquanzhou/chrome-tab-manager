package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

const maxLineSize = 1 << 20 // 1MB

// Reader reads NDJSON messages from an io.Reader.
// Not thread-safe: designed for single-goroutine reads per connection.
type Reader struct {
	scanner *bufio.Scanner
}

func NewReader(r io.Reader) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, maxLineSize), maxLineSize)
	return &Reader{scanner: scanner}
}

func (r *Reader) Read() (*Message, error) {
	for r.scanner.Scan() {
		line := r.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			return nil, fmt.Errorf("ndjson decode: %w", err)
		}
		return &msg, nil
	}
	if err := r.scanner.Err(); err != nil {
		return nil, fmt.Errorf("ndjson scan: %w", err)
	}
	return nil, io.EOF
}

// Writer writes NDJSON messages to an io.Writer.
// Thread-safe: multiple goroutines may write concurrently.
type Writer struct {
	mu sync.Mutex
	w  io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

func (w *Writer) Write(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("ndjson encode: %w", err)
	}
	data = append(data, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	_, err = w.w.Write(data)
	if err != nil {
		return fmt.Errorf("ndjson write: %w", err)
	}
	return nil
}
