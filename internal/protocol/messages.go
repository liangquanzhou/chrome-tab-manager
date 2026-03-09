package protocol

import (
	"encoding/json"
	"fmt"
)

const ProtocolVersion = 1

type MessageType string

const (
	TypeHello    MessageType = "hello"
	TypeRequest  MessageType = "request"
	TypeResponse MessageType = "response"
	TypeError    MessageType = "error"
	TypeEvent    MessageType = "event"
)

type ErrorCode string

const (
	ErrDaemonUnavailable     ErrorCode = "DAEMON_UNAVAILABLE"
	ErrTargetOffline         ErrorCode = "TARGET_OFFLINE"
	ErrTargetAmbiguous       ErrorCode = "TARGET_AMBIGUOUS"
	ErrExtensionNotConnected ErrorCode = "EXTENSION_NOT_CONNECTED"
	ErrChromeAPIError        ErrorCode = "CHROME_API_ERROR"
	ErrInstallationInvalid   ErrorCode = "INSTALLATION_INVALID"
	ErrProtocolMismatch      ErrorCode = "PROTOCOL_MISMATCH"
	ErrTimeout               ErrorCode = "TIMEOUT"
	ErrUnknownAction         ErrorCode = "UNKNOWN_ACTION"
	ErrInvalidPayload        ErrorCode = "INVALID_PAYLOAD"
)

type Message struct {
	ID              string          `json:"id"`
	ProtocolVersion int             `json:"protocol_version"`
	Type            MessageType     `json:"type"`
	Action          string          `json:"action,omitempty"`
	Target          *TargetSelector `json:"target,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	Error           *ErrorBody      `json:"error,omitempty"`
}

type TargetSelector struct {
	TargetID string `json:"targetId,omitempty"`
	Channel  string `json:"channel,omitempty"`
	Label    string `json:"label,omitempty"`
}

type ErrorBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// ProtocolError is a typed error returned by daemon responses.
// CLI can use errors.As() to extract the ErrorCode for exit code mapping.
type ProtocolError struct {
	Action  string
	Code    ErrorCode
	Message string
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Action, e.Code, e.Message)
}

// ExitCode maps the error code to a CLI exit code.
//
//	0: success
//	1: usage / validation error (handled by cobra)
//	2: daemon unavailable / install invalid
//	3: target resolution error
//	4: action execution error / timeout / chrome API failure
func (e *ProtocolError) ExitCode() int {
	switch e.Code {
	case ErrDaemonUnavailable, ErrInstallationInvalid:
		return 2
	case ErrTargetOffline, ErrTargetAmbiguous, ErrExtensionNotConnected:
		return 3
	case ErrChromeAPIError, ErrTimeout, ErrUnknownAction, ErrInvalidPayload, ErrProtocolMismatch:
		return 4
	default:
		return 4
	}
}
