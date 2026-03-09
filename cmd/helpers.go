package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"ctm/internal/client"
	"ctm/internal/config"
	"ctm/internal/protocol"
)

// ExitCodes for CLI commands (per codex-guide-v4).
const (
	ExitSuccess     = 0
	ExitUsageError  = 1 // usage / validation (handled by cobra)
	ExitDaemonError = 2 // daemon unavailable / install invalid
	ExitTargetError = 3 // target resolution error
	ExitActionError = 4 // action execution error / timeout / chrome API failure
)

// ErrDaemonConnect is returned when the CLI cannot connect to the daemon.
// Used for typed exit code mapping instead of string matching.
var ErrDaemonConnect = errors.New("cannot connect to daemon")

var targetFlag string

// targetSelector builds a TargetSelector from the --target flag.
//
// Current CLI target resolution rule:
//   - Only --target (targetId) is exposed as a CLI flag.
//   - channel and label selectors exist in the protocol but are NOT
//     exposed to CLI users yet. When needed, add --target-channel and
//     --target-label flags here and populate the corresponding
//     TargetSelector fields.
//   - When --target is omitted, nil is returned, which tells the daemon
//     to use the default target (or the only connected target).
func targetSelector() *protocol.TargetSelector {
	if targetFlag != "" {
		return &protocol.TargetSelector{TargetID: targetFlag}
	}
	return nil
}

// connectAndRequest connects to the daemon, sends a request, and returns the
// response. If the initial connection fails, it attempts to auto-start the
// daemon exactly once and retries.
//
// Auto-start is intentionally enabled for all commands that call this function.
// Commands that must NOT auto-start (doctor, install --check) do not call
// connectAndRequest — they operate locally without a daemon connection.
//
// Timeout: 15 s covers the full round-trip (connect + auto-start retry +
// request + Chrome extension response). For most actions this is generous;
// sessions.restore may approach the limit for very large sessions.
func connectAndRequest(action string, payload any, target *protocol.TargetSelector) (*protocol.Message, error) {
	c := client.New(config.SocketPath())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	defer c.Close()

	if err := c.Connect(ctx); err != nil {
		// Auto-start fallback: try to start daemon and retry
		if startErr := tryAutoStartDaemon(); startErr == nil {
			if err2 := c.Connect(ctx); err2 == nil {
				return c.Request(ctx, action, payload, target)
			}
		}
		return nil, fmt.Errorf("%w (is it running?): %w", ErrDaemonConnect, err)
	}
	return c.Request(ctx, action, payload, target)
}

func tryAutoStartDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Skip auto-start when running inside a test binary to prevent
	// orphan "*.test daemon" processes that accumulate across test runs.
	if strings.HasSuffix(exe, ".test") {
		return fmt.Errorf("auto-start disabled in test binary")
	}
	cmd := exec.Command(exe, "daemon")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	// Release the child process
	go cmd.Wait()
	// Wait briefly for daemon to start listening
	time.Sleep(500 * time.Millisecond)
	return nil
}

func printJSON(data json.RawMessage) {
	var buf bytes.Buffer
	if json.Indent(&buf, data, "", "  ") == nil {
		buf.WriteTo(os.Stdout)
		fmt.Println()
	} else {
		fmt.Println(string(data))
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
