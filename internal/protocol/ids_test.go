package protocol

import (
	"strings"
	"testing"
)

func TestMakeIDPrefix(t *testing.T) {
	id := MakeID()
	if !strings.HasPrefix(id, "msg_") {
		t.Errorf("MakeID() = %q, want prefix msg_", id)
	}
}

func TestMakeIDUniqueness(t *testing.T) {
	const count = 1000
	seen := make(map[string]bool, count)
	for i := 0; i < count; i++ {
		id := MakeID()
		if seen[id] {
			t.Fatalf("duplicate ID at iteration %d: %s", i, id)
		}
		seen[id] = true
	}
}

func TestMakeIDFormat(t *testing.T) {
	id := MakeID()
	parts := strings.Split(id, "_")
	if len(parts) != 3 {
		t.Errorf("MakeID() = %q, expected 3 parts separated by _", id)
	}
}
