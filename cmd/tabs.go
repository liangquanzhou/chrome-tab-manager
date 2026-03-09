package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var tabsJSONOutput bool

var tabsCmd = &cobra.Command{
	Use:   "tabs",
	Short: "Manage browser tabs",
}

var tabsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all open tabs",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("tabs.list", nil, targetSelector())
		if err != nil {
			return err
		}
		if tabsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Tabs []struct {
				ID        int    `json:"id"`
				Title     string `json:"title"`
				URL       string `json:"url"`
				Active    bool   `json:"active"`
				Pinned    bool   `json:"pinned"`
				GroupID   int    `json:"groupId"`
				WindowID  int    `json:"windowId"`
			} `json:"tabs"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTITLE\tURL\tFLAGS")
		for _, t := range result.Tabs {
			flags := ""
			if t.Active {
				flags += "A"
			}
			if t.Pinned {
				flags += "P"
			}
			if t.GroupID >= 0 {
				flags += "G"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
				t.ID,
				truncate(t.Title, 40),
				truncate(t.URL, 50),
				flags,
			)
		}
		return w.Flush()
	},
}

var tabsOpenActive bool
var tabsOpenDeduplicate bool

var tabsOpenCmd = &cobra.Command{
	Use:   "open <url>",
	Short: "Open a new tab",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{
			"url":         args[0],
			"active":      tabsOpenActive,
			"deduplicate": tabsOpenDeduplicate,
		}
		resp, err := connectAndRequest("tabs.open", payload, targetSelector())
		if err != nil {
			return err
		}
		if tabsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			TabID  int  `json:"tabId"`
			Reused bool `json:"reused"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		if result.Reused {
			fmt.Printf("Tab %d reused.\n", result.TabID)
		} else {
			fmt.Printf("Tab %d opened.\n", result.TabID)
		}
		return nil
	},
}

var tabsCloseCmd = &cobra.Command{
	Use:   "close <tabId>",
	Short: "Close a tab by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tabID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid tab ID %q: %w", args[0], err)
		}
		payload := map[string]any{"tabId": tabID}
		_, err = connectAndRequest("tabs.close", payload, targetSelector())
		if err != nil {
			return err
		}
		fmt.Printf("Tab %d closed.\n", tabID)
		return nil
	},
}

var tabsActivateFocus bool

var tabsActivateCmd = &cobra.Command{
	Use:   "activate <tabId>",
	Short: "Activate (switch to) a tab",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tabID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid tab ID %q: %w", args[0], err)
		}
		payload := map[string]any{
			"tabId": tabID,
			"focus": tabsActivateFocus,
		}
		_, err = connectAndRequest("tabs.activate", payload, targetSelector())
		if err != nil {
			return err
		}
		fmt.Printf("Tab %d activated.\n", tabID)
		return nil
	},
}

var tabsMuteCmd = &cobra.Command{
	Use:   "mute <tabId>",
	Short: "Toggle mute on a tab",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tabID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid tab ID %q: %w", args[0], err)
		}
		resp, err := connectAndRequest("tabs.mute", map[string]any{"tabId": tabID}, targetSelector())
		if err != nil {
			return err
		}
		if tabsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		fmt.Printf("Tab %d mute toggled.\n", tabID)
		return nil
	},
}

var tabsPinCmd = &cobra.Command{
	Use:   "pin <tabId>",
	Short: "Toggle pin on a tab",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tabID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid tab ID %q: %w", args[0], err)
		}
		resp, err := connectAndRequest("tabs.pin", map[string]any{"tabId": tabID}, targetSelector())
		if err != nil {
			return err
		}
		if tabsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		fmt.Printf("Tab %d pin toggled.\n", tabID)
		return nil
	},
}

var tabsMoveWindowID int
var tabsMoveIndex int

var tabsMoveCmd = &cobra.Command{
	Use:   "move <tabId>",
	Short: "Move a tab to a different position or window",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tabID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid tab ID %q: %w", args[0], err)
		}
		payload := map[string]any{"tabId": tabID, "index": tabsMoveIndex}
		if tabsMoveWindowID > 0 {
			payload["windowId"] = tabsMoveWindowID
		}
		resp, err := connectAndRequest("tabs.move", payload, targetSelector())
		if err != nil {
			return err
		}
		if tabsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		fmt.Printf("Tab %d moved.\n", tabID)
		return nil
	},
}

var tabsTextCmd = &cobra.Command{
	Use:   "text <tabId>",
	Short: "Get visible text content of a tab",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tabID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid tab ID %q: %w", args[0], err)
		}
		resp, err := connectAndRequest("tabs.getText", map[string]any{"tabId": tabID}, targetSelector())
		if err != nil {
			return err
		}
		if tabsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		fmt.Print(result.Text)
		return nil
	},
}

var tabsCaptureOutput string

var tabsCaptureCmd = &cobra.Command{
	Use:   "capture [tabId]",
	Short: "Capture a screenshot of a tab (defaults to active tab)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{}
		if len(args) > 0 {
			tabID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid tab ID %q: %w", args[0], err)
			}
			payload["tabId"] = tabID
		}
		resp, err := connectAndRequest("tabs.capture", payload, targetSelector())
		if err != nil {
			return err
		}
		if tabsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			DataURL string `json:"dataUrl"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		if tabsCaptureOutput != "" {
			// Decode base64 data URL and write to file
			// Format: data:image/png;base64,<data>
			idx := strings.Index(result.DataURL, ",")
			if idx < 0 {
				return fmt.Errorf("invalid data URL format")
			}
			decoded, err := base64.StdEncoding.DecodeString(result.DataURL[idx+1:])
			if err != nil {
				return fmt.Errorf("decode base64: %w", err)
			}
			if err := os.WriteFile(tabsCaptureOutput, decoded, 0644); err != nil {
				return fmt.Errorf("write file: %w", err)
			}
			fmt.Printf("Screenshot saved to %s (%d bytes).\n", tabsCaptureOutput, len(decoded))
			return nil
		}
		fmt.Println(result.DataURL)
		return nil
	},
}

func init() {
	tabsCmd.PersistentFlags().BoolVar(&tabsJSONOutput, "json", false, "Output as JSON")

	tabsOpenCmd.Flags().BoolVar(&tabsOpenActive, "active", true, "Make the new tab active")
	tabsOpenCmd.Flags().BoolVar(&tabsOpenDeduplicate, "deduplicate", false, "Reuse existing tab with same URL")

	tabsActivateCmd.Flags().BoolVar(&tabsActivateFocus, "focus", false, "Also focus the browser window")

	tabsMoveCmd.Flags().IntVar(&tabsMoveWindowID, "window", 0, "Target window ID")
	tabsMoveCmd.Flags().IntVar(&tabsMoveIndex, "index", -1, "Target index position")

	tabsCaptureCmd.Flags().StringVarP(&tabsCaptureOutput, "output", "o", "", "Save screenshot to file (PNG)")

	tabsCmd.AddCommand(tabsListCmd)
	tabsCmd.AddCommand(tabsOpenCmd)
	tabsCmd.AddCommand(tabsCloseCmd)
	tabsCmd.AddCommand(tabsActivateCmd)
	tabsCmd.AddCommand(tabsMuteCmd)
	tabsCmd.AddCommand(tabsPinCmd)
	tabsCmd.AddCommand(tabsMoveCmd)
	tabsCmd.AddCommand(tabsTextCmd)
	tabsCmd.AddCommand(tabsCaptureCmd)
	rootCmd.AddCommand(tabsCmd)
}
