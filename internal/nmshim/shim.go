package nmshim

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"

	"ctm/internal/protocol"
)

const maxMessageSize = 1 << 20 // 1MB

// ReadNMFrame reads a single Chrome Native Messaging frame.
// Wire format: 4-byte little-endian length prefix followed by raw JSON bytes.
func ReadNMFrame(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, fmt.Errorf("read frame length: %w", err)
	}
	if length > maxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}
	if length == 0 {
		return nil, fmt.Errorf("empty frame")
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("read frame data: %w", err)
	}
	return data, nil
}

// WriteNMFrame writes a single Chrome Native Messaging frame.
// Wire format: 4-byte little-endian length prefix followed by raw JSON bytes.
func WriteNMFrame(w io.Writer, data []byte) error {
	length := uint32(len(data))
	if err := binary.Write(w, binary.LittleEndian, length); err != nil {
		return fmt.Errorf("write frame length: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write frame data: %w", err)
	}
	return nil
}

// Run bridges Chrome Native Messaging (stdin/stdout with 4-byte LE frames)
// to the CTM daemon (Unix socket with NDJSON).
func Run(ctx context.Context, socketPath string, stdin io.Reader, stdout io.Writer) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()

	reader := protocol.NewReader(conn)
	writer := protocol.NewWriter(conn)

	errCh := make(chan error, 2)

	// stdin (NM frames) -> socket (NDJSON)
	go func() {
		for {
			data, err := ReadNMFrame(stdin)
			if err != nil {
				errCh <- fmt.Errorf("stdin: %w", err)
				return
			}
			var msg protocol.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				errCh <- fmt.Errorf("decode: %w", err)
				return
			}
			if err := writer.Write(&msg); err != nil {
				errCh <- fmt.Errorf("socket write: %w", err)
				return
			}
		}
	}()

	// socket (NDJSON) -> stdout (NM frames)
	go func() {
		for {
			msg, err := reader.Read()
			if err != nil {
				errCh <- fmt.Errorf("socket read: %w", err)
				return
			}
			data, err := json.Marshal(msg)
			if err != nil {
				errCh <- fmt.Errorf("encode: %w", err)
				return
			}
			if err := WriteNMFrame(stdout, data); err != nil {
				errCh <- fmt.Errorf("stdout: %w", err)
				return
			}
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
