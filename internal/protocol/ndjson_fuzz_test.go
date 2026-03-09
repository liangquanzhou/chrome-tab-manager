package protocol

import (
	"strings"
	"testing"
)

func FuzzNDJSON(f *testing.F) {
	f.Add(`{"id":"msg_1","protocol_version":1,"type":"request","action":"tabs.list"}`)
	f.Add(`{"id":"msg_2","protocol_version":1,"type":"response","payload":{"tabs":[]}}`)
	f.Add(`{"id":"msg_3","protocol_version":1,"type":"error","error":{"code":"TIMEOUT","message":"timeout"}}`)
	f.Add(`not json`)
	f.Add(``)
	f.Add(`{"id":""}`)
	f.Add(`{"type":123}`)

	f.Fuzz(func(t *testing.T, data string) {
		r := NewReader(strings.NewReader(data + "\n"))
		// Must not panic regardless of input
		r.Read() //nolint:errcheck
	})
}
