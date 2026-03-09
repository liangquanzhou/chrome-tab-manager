package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ctm/internal/client"
	"ctm/internal/config"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check CTM installation and connectivity",
	Long: `Run a full health check of the CTM installation:

  - Binary installed and version
  - Native messaging manifest(s) present and valid
  - LaunchAgent plist present
  - Daemon socket reachable
  - Daemon responds to requests
  - Browser extension(s) connected`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoctor(cmd)
	},
}

func init() {
	doctorCmd.Flags().StringVar(&installExtensionID, "extension-id", "", "Chrome extension ID (if provided, also checks NM manifest)")
	rootCmd.AddCommand(doctorCmd)
}

// doctorResult tracks pass/fail counts across all checks.
type doctorResult struct {
	pass int
	fail int
}

func (r *doctorResult) markPass() { r.pass++ }
func (r *doctorResult) markFail() { r.fail++ }

func runDoctor(cmd *cobra.Command) error {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "CTM Doctor")

	exe, err := os.Executable()
	if err != nil {
		exe = "(unknown)"
	} else {
		exe, _ = filepath.EvalSymlinks(exe)
	}

	home, _ := os.UserHomeDir()

	var result doctorResult

	// --- File checks ---
	doctorCheckBinary(w, exe, &result)
	doctorCheckVersion(w, &result)
	doctorCheckNMManifests(w, home, exe, &result)
	doctorCheckLaunchAgent(w, home, &result)

	// --- Daemon connectivity checks ---
	doctorCheckDaemon(w, &result)

	// --- Summary ---
	total := result.pass + result.fail
	fmt.Fprintf(w, "\n%d/%d checks passed\n", result.pass, total)

	return nil
}

func doctorCheckBinary(w io.Writer, exe string, r *doctorResult) {
	if _, err := os.Stat(exe); err == nil {
		fmt.Fprintf(w, "  ✓ Binary installed: %s\n", exe)
		r.markPass()
	} else {
		fmt.Fprintf(w, "  ✗ Binary not found\n")
		r.markFail()
	}
}

func doctorCheckVersion(w io.Writer, r *doctorResult) {
	fmt.Fprintf(w, "  ✓ Version: %s\n", Version)
	r.markPass()
}

func doctorCheckNMManifests(w io.Writer, home, exe string, r *doctorResult) {
	if home == "" {
		fmt.Fprintln(w, "  ✗ Native messaging manifest: cannot determine home dir")
		r.markFail()
		return
	}
	for _, nm := range nmManifestDirs(home) {
		path := filepath.Join(nm.dir, "com.ctm.native_host.json")
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(w, "  ✓ Native messaging manifest (%s)\n", nm.label)
			r.markPass()
			// Validate manifest content
			doctorValidateNMManifest(w, path, nm.label, exe, r)
		} else {
			fmt.Fprintf(w, "  ✗ Native messaging manifest (%s): not found\n", nm.label)
			r.markFail()
		}
	}
}

// doctorValidateNMManifest reads the NM manifest JSON and verifies key fields.
func doctorValidateNMManifest(w io.Writer, path, label, exe string, r *doctorResult) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(w, "    ✗ %s manifest content: cannot read: %v\n", label, err)
		r.markFail()
		return
	}

	var manifest struct {
		Path           string   `json:"path"`
		Type           string   `json:"type"`
		AllowedOrigins []string `json:"allowed_origins"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		fmt.Fprintf(w, "    ✗ %s manifest content: invalid JSON: %v\n", label, err)
		r.markFail()
		return
	}

	allGood := true

	// Check "path" field points to the actual binary
	if manifest.Path == "" {
		fmt.Fprintf(w, "    ✗ %s manifest path: missing\n", label)
		r.markFail()
		allGood = false
	} else {
		// Resolve symlinks for both paths before comparing
		resolvedManifestPath, resolveErr := filepath.EvalSymlinks(manifest.Path)
		if resolveErr != nil {
			resolvedManifestPath = manifest.Path
		}
		resolvedExe, resolveErr := filepath.EvalSymlinks(exe)
		if resolveErr != nil {
			resolvedExe = exe
		}
		if resolvedManifestPath != resolvedExe {
			fmt.Fprintf(w, "    ✗ %s manifest path: %s (expected %s)\n", label, manifest.Path, exe)
			r.markFail()
			allGood = false
		}
	}

	// Check "type" is "stdio"
	if manifest.Type != "stdio" {
		fmt.Fprintf(w, "    ✗ %s manifest type: %q (expected \"stdio\")\n", label, manifest.Type)
		r.markFail()
		allGood = false
	}

	// Check allowed_origins contains expected extension ID (only when --extension-id is provided)
	if installExtensionID != "" {
		expected := fmt.Sprintf("chrome-extension://%s/", installExtensionID)
		found := false
		for _, origin := range manifest.AllowedOrigins {
			if origin == expected {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(w, "    ✗ %s manifest allowed_origins: does not contain %s\n", label, expected)
			r.markFail()
			allGood = false
		}
	}

	if allGood {
		details := []string{"path ok", "type ok"}
		if installExtensionID != "" {
			details = append(details, "allowed_origins ok")
		}
		fmt.Fprintf(w, "    ✓ %s manifest content valid (%s)\n", label, strings.Join(details, ", "))
		r.markPass()
	}
}

func doctorCheckLaunchAgent(w io.Writer, home string, r *doctorResult) {
	if home == "" {
		fmt.Fprintln(w, "  ✗ LaunchAgent plist: cannot determine home dir")
		r.markFail()
		return
	}
	laPath := filepath.Join(home, "Library", "LaunchAgents", "com.ctm.daemon.plist")
	if _, err := os.Stat(laPath); err == nil {
		fmt.Fprintln(w, "  ✓ LaunchAgent plist")
		r.markPass()
	} else {
		fmt.Fprintln(w, "  ✗ LaunchAgent plist: not found")
		r.markFail()
	}
}

func doctorCheckDaemon(w io.Writer, r *doctorResult) {
	sockPath := config.SocketPath()

	// Step 1: try connecting to the daemon socket (no auto-start)
	c := client.New(sockPath)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		fmt.Fprintln(w, "  ✗ Daemon socket not reachable (is daemon running?)")
		r.markFail()
		// Cannot proceed with further checks
		fmt.Fprintln(w, "  - Daemon responds to requests: skipped")
		fmt.Fprintln(w, "  - Browser extensions connected: skipped")
		c.Close()
		return
	}
	fmt.Fprintln(w, "  ✓ Daemon socket reachable")
	r.markPass()

	// Step 2: send targets.list to verify basic request/response
	resp, err := c.Request(ctx, "targets.list", nil, nil)
	c.Close()
	if err != nil {
		fmt.Fprintf(w, "  ✗ Daemon not responding to requests: %v\n", err)
		r.markFail()
		fmt.Fprintln(w, "  - Browser extensions connected: skipped")
		return
	}
	fmt.Fprintln(w, "  ✓ Daemon responds to requests")
	r.markPass()

	// Step 3: check if any targets (extensions) are connected
	var payload struct {
		Targets []json.RawMessage `json:"targets"`
	}
	if err := json.Unmarshal(resp.Payload, &payload); err != nil {
		fmt.Fprintf(w, "  ✗ Browser extensions: invalid response: %v\n", err)
		r.markFail()
		return
	}

	if len(payload.Targets) == 0 {
		fmt.Fprintln(w, "  ✗ No browser extensions connected")
		r.markFail()
	} else {
		fmt.Fprintf(w, "  ✓ Browser extensions connected: %d\n", len(payload.Targets))
		r.markPass()
	}
}
