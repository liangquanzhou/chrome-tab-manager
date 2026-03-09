package daemon

import (
	"errors"
	"fmt"

	"ctm/internal/protocol"
)

// Sentinel errors for target resolution.
var (
	ErrNoTargetsConnected   = errors.New("no targets connected")
	ErrTargetNotFound       = errors.New("target not found")
	ErrMultipleTargetsAmbig = errors.New("multiple targets connected, specify target or set default")
)

// Sentinel errors for resource lookup.
var (
	ErrSessionNotFound    = errors.New("session not found")
	ErrCollectionNotFound = errors.New("collection not found")
	ErrWorkspaceNotFound  = errors.New("workspace not found")
)

// Sentinel errors for extension communication.
var (
	ErrExtensionTimeout   = errors.New("extension timeout")
	ErrExtensionWriteFail = errors.New("extension write failed")
)

// Sentinel errors for action dispatch.
var (
	ErrUnknownAction = errors.New("unknown action")
)

// TargetNotFoundError wraps ErrTargetNotFound with the specific target ID.
type TargetNotFoundError struct {
	TargetID string
}

func (e *TargetNotFoundError) Error() string {
	return fmt.Sprintf("target %s not found", e.TargetID)
}

func (e *TargetNotFoundError) Unwrap() error {
	return ErrTargetNotFound
}

// ResourceNotFoundError wraps a sentinel "not found" error with a resource name.
type ResourceNotFoundError struct {
	Kind string // "session", "collection", "workspace"
	Name string
	Err  error // ErrSessionNotFound, ErrCollectionNotFound, or ErrWorkspaceNotFound
}

func (e *ResourceNotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Kind, e.Name)
}

func (e *ResourceNotFoundError) Unwrap() error {
	return e.Err
}

// ExtensionTimeoutError wraps ErrExtensionTimeout with the action that timed out.
type ExtensionTimeoutError struct {
	Action string
}

func (e *ExtensionTimeoutError) Error() string {
	return fmt.Sprintf("extension timeout for %s", e.Action)
}

func (e *ExtensionTimeoutError) Unwrap() error {
	return ErrExtensionTimeout
}

// UnknownActionError wraps ErrUnknownAction with the action name.
type UnknownActionError struct {
	Action string
}

func (e *UnknownActionError) Error() string {
	return fmt.Sprintf("unknown action: %s", e.Action)
}

func (e *UnknownActionError) Unwrap() error {
	return ErrUnknownAction
}

// targetErrorCode maps a target resolution error to its protocol error code.
// Uses errors.Is() for stable classification that doesn't depend on message wording.
func targetErrorCode(err error) protocol.ErrorCode {
	switch {
	case errors.Is(err, ErrTargetNotFound):
		return protocol.ErrTargetOffline
	case errors.Is(err, ErrNoTargetsConnected):
		return protocol.ErrExtensionNotConnected
	case errors.Is(err, ErrMultipleTargetsAmbig):
		return protocol.ErrTargetAmbiguous
	default:
		return protocol.ErrTargetAmbiguous
	}
}

// daemonErrorCode maps a daemon-level error to its protocol error code.
func daemonErrorCode(err error) protocol.ErrorCode {
	switch {
	case errors.Is(err, ErrSessionNotFound),
		errors.Is(err, ErrCollectionNotFound),
		errors.Is(err, ErrWorkspaceNotFound):
		return protocol.ErrInvalidPayload
	case errors.Is(err, ErrExtensionTimeout):
		return protocol.ErrTimeout
	case errors.Is(err, ErrExtensionWriteFail):
		return protocol.ErrExtensionNotConnected
	case errors.Is(err, ErrUnknownAction):
		return protocol.ErrUnknownAction
	default:
		return protocol.ErrInvalidPayload
	}
}
