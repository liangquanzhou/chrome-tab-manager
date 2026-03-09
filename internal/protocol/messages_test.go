package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestMessageRoundTripRequest(t *testing.T) {
	orig := &Message{
		ID:              "msg_1",
		ProtocolVersion: ProtocolVersion,
		Type:            TypeRequest,
		Action:          "tabs.list",
		Target:          &TargetSelector{TargetID: "target_1"},
		Payload:         json.RawMessage(`{"key":"value"}`),
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != orig.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, orig.ID)
	}
	if decoded.ProtocolVersion != orig.ProtocolVersion {
		t.Errorf("ProtocolVersion = %d, want %d", decoded.ProtocolVersion, orig.ProtocolVersion)
	}
	if decoded.Type != orig.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, orig.Type)
	}
	if decoded.Action != orig.Action {
		t.Errorf("Action = %q, want %q", decoded.Action, orig.Action)
	}
	if decoded.Target == nil || decoded.Target.TargetID != "target_1" {
		t.Errorf("Target.TargetID = %v, want target_1", decoded.Target)
	}
	if string(decoded.Payload) != string(orig.Payload) {
		t.Errorf("Payload = %s, want %s", decoded.Payload, orig.Payload)
	}
}

func TestMessageRoundTripResponse(t *testing.T) {
	orig := &Message{
		ID:              "msg_1",
		ProtocolVersion: ProtocolVersion,
		Type:            TypeResponse,
		Action:          "tabs.list",
		Payload:         json.RawMessage(`{"tabs":[]}`),
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Type != TypeResponse {
		t.Errorf("Type = %q, want response", decoded.Type)
	}
	if decoded.Target != nil {
		t.Errorf("Target should be nil for response, got %v", decoded.Target)
	}
	if decoded.Error != nil {
		t.Errorf("Error should be nil for response, got %v", decoded.Error)
	}
}

func TestMessageRoundTripError(t *testing.T) {
	orig := &Message{
		ID:              "msg_1",
		ProtocolVersion: ProtocolVersion,
		Type:            TypeError,
		Error: &ErrorBody{
			Code:    ErrUnknownAction,
			Message: "unknown action: foo.bar",
		},
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if decoded.Error.Code != ErrUnknownAction {
		t.Errorf("Error.Code = %q, want %q", decoded.Error.Code, ErrUnknownAction)
	}
	if decoded.Error.Message != "unknown action: foo.bar" {
		t.Errorf("Error.Message = %q, want %q", decoded.Error.Message, "unknown action: foo.bar")
	}
}

func TestMessageRoundTripEvent(t *testing.T) {
	orig := &Message{
		ID:              "evt_1",
		ProtocolVersion: ProtocolVersion,
		Type:            TypeEvent,
		Action:          "tabs.created",
		Payload:         json.RawMessage(`{"tab":{"id":1},"_target":{"targetId":"target_1"}}`),
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Type != TypeEvent {
		t.Errorf("Type = %q, want event", decoded.Type)
	}
	if decoded.Action != "tabs.created" {
		t.Errorf("Action = %q, want tabs.created", decoded.Action)
	}

	// Verify _target is preserved in payload
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(decoded.Payload, &payload); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}
	if _, ok := payload["_target"]; !ok {
		t.Error("_target missing from event payload")
	}
}

func TestMessageOmitEmpty(t *testing.T) {
	msg := &Message{
		ID:              "msg_1",
		ProtocolVersion: ProtocolVersion,
		Type:            TypeRequest,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	for _, field := range []string{"action", "target", "payload", "error"} {
		if _, ok := raw[field]; ok {
			t.Errorf("field %q should be omitted when empty", field)
		}
	}
}

func TestTargetSelectorFields(t *testing.T) {
	ts := &TargetSelector{
		TargetID: "t1",
		Channel:  "stable",
		Label:    "work",
	}

	data, err := json.Marshal(ts)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded TargetSelector
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.TargetID != "t1" {
		t.Errorf("TargetID = %q, want t1", decoded.TargetID)
	}
	if decoded.Channel != "stable" {
		t.Errorf("Channel = %q, want stable", decoded.Channel)
	}
	if decoded.Label != "work" {
		t.Errorf("Label = %q, want work", decoded.Label)
	}
}

func TestProtocolErrorExitCode(t *testing.T) {
	tests := []struct {
		code ErrorCode
		exit int
	}{
		{ErrDaemonUnavailable, 2},
		{ErrInstallationInvalid, 2},
		{ErrTargetOffline, 3},
		{ErrTargetAmbiguous, 3},
		{ErrExtensionNotConnected, 3},
		{ErrChromeAPIError, 4},
		{ErrTimeout, 4},
		{ErrUnknownAction, 4},
		{ErrInvalidPayload, 4},
		{ErrProtocolMismatch, 4},
	}
	for _, tt := range tests {
		pe := &ProtocolError{Action: "test.action", Code: tt.code, Message: "msg"}
		if got := pe.ExitCode(); got != tt.exit {
			t.Errorf("ProtocolError{Code: %q}.ExitCode() = %d, want %d", tt.code, got, tt.exit)
		}
	}
}

func TestProtocolErrorString(t *testing.T) {
	pe := &ProtocolError{Action: "tabs.list", Code: ErrTargetOffline, Message: "target xyz not found"}
	want := "tabs.list: TARGET_OFFLINE: target xyz not found"
	if got := pe.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestProtocolErrorAs(t *testing.T) {
	pe := &ProtocolError{Action: "tabs.list", Code: ErrTargetOffline, Message: "msg"}
	wrapped := fmt.Errorf("request failed: %w", pe)

	var extracted *ProtocolError
	if !errors.As(wrapped, &extracted) {
		t.Fatal("errors.As failed to extract ProtocolError")
	}
	if extracted.Code != ErrTargetOffline {
		t.Errorf("Code = %q, want %q", extracted.Code, ErrTargetOffline)
	}
}
