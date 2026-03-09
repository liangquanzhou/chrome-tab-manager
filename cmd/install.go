package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	installCheck       bool
	installExtensionID string
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install CTM components (LaunchAgent and/or NM manifest)",
	Long: `Install CTM system components in two stages:

  ctm install                         Install LaunchAgent only (daemon auto-start)
  ctm install --extension-id=XXX      Install LaunchAgent + NM manifest
  ctm install --check                 Report installation status of all components`,
	RunE: func(cmd *cobra.Command, args []string) error {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("find executable: %w", err)
		}
		exe, _ = filepath.EvalSymlinks(exe)

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find home dir: %w", err)
		}

		if installCheck {
			return runInstallCheck(cmd, exe, home)
		}
		return runInstall(cmd, exe, home)
	},
}

// runInstallCheck reports the installation status of all components.
// It works with or without --extension-id.
func runInstallCheck(cmd *cobra.Command, exe, home string) error {
	laPath := filepath.Join(home, "Library", "LaunchAgents", "com.ctm.daemon.plist")

	fmt.Fprintf(cmd.OutOrStdout(), "Binary: %s\n", exe)
	checkFile(cmd, "LaunchAgent", laPath)

	if installExtensionID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Extension ID: %s\n", installExtensionID)
	}

	for _, nm := range nmManifestDirs(home) {
		checkFile(cmd, fmt.Sprintf("NM Manifest (%s)", nm.label),
			filepath.Join(nm.dir, "com.ctm.native_host.json"))
	}

	return nil
}

// runInstall installs components. Without --extension-id, only the LaunchAgent
// is installed. With --extension-id, both LaunchAgent and NM manifest are installed.
func runInstall(cmd *cobra.Command, exe, home string) error {
	// Always install LaunchAgent
	if err := installLaunchAgent(cmd, exe, home); err != nil {
		return err
	}

	// Install NM manifest only if extension-id is provided
	if installExtensionID != "" {
		if err := installNMManifest(cmd, exe, home); err != nil {
			return err
		}
	}

	return nil
}

// installLaunchAgent writes the LaunchAgent plist for daemon auto-start.
func installLaunchAgent(cmd *cobra.Command, exe, home string) error {
	laDir := filepath.Join(home, "Library", "LaunchAgents")
	laPath := filepath.Join(laDir, "com.ctm.daemon.plist")

	if err := os.MkdirAll(laDir, 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.ctm.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>`, exe)

	if err := os.WriteFile(laPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("write LaunchAgent: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Wrote LaunchAgent: %s\n", laPath)
	fmt.Fprintln(cmd.OutOrStdout(), "Run: launchctl load "+laPath)
	return nil
}

// installNMManifest writes the NM manifest to Chrome and Chrome Beta directories.
func installNMManifest(cmd *cobra.Command, exe, home string) error {
	manifest := map[string]any{
		"name":            "com.ctm.native_host",
		"description":     "CTM Native Messaging Host",
		"path":            exe,
		"type":            "stdio",
		"allowed_origins": []string{fmt.Sprintf("chrome-extension://%s/", installExtensionID)},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal NM manifest: %w", err)
	}

	for _, nm := range nmManifestDirs(home) {
		if err := os.MkdirAll(nm.dir, 0755); err != nil {
			return fmt.Errorf("create NM dir (%s): %w", nm.label, err)
		}
		nmPath := filepath.Join(nm.dir, "com.ctm.native_host.json")
		if err := os.WriteFile(nmPath, data, 0644); err != nil {
			return fmt.Errorf("write NM manifest (%s): %w", nm.label, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote NM manifest (%s): %s\n", nm.label, nmPath)
	}
	return nil
}

type nmDir struct {
	label string
	dir   string
}

func nmManifestDirs(home string) []nmDir {
	return []nmDir{
		{"Chrome", filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts")},
		{"Chrome Beta", filepath.Join(home, "Library", "Application Support", "Google", "Chrome Beta", "NativeMessagingHosts")},
	}
}

func checkFile(cmd *cobra.Command, label, path string) {
	var w io.Writer = os.Stdout
	if cmd != nil {
		w = cmd.OutOrStdout()
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(w, "  ok %s: %s\n", label, path)
	} else {
		fmt.Fprintf(w, "  missing %s: not found\n", label)
	}
}

func init() {
	installCmd.Flags().BoolVar(&installCheck, "check", false, "Check installation status")
	installCmd.Flags().StringVar(&installExtensionID, "extension-id", "", "Chrome extension ID (required for NM manifest, find at chrome://extensions)")
	rootCmd.AddCommand(installCmd)
}
