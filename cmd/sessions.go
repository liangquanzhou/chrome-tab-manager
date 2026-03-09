package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var sessionsJSONOutput bool

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage saved sessions",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("sessions.list", nil, targetSelector())
		if err != nil {
			return err
		}
		if sessionsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Sessions []struct {
				Name         string `json:"name"`
				TabCount     int    `json:"tabCount"`
				WindowCount  int    `json:"windowCount"`
				GroupCount   int    `json:"groupCount"`
				CreatedAt    string `json:"createdAt"`
				SourceTarget string `json:"sourceTarget"`
			} `json:"sessions"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTABS\tWINDOWS\tGROUPS\tCREATED\tSOURCE")
		for _, s := range result.Sessions {
			fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%s\t%s\n",
				s.Name,
				s.TabCount,
				s.WindowCount,
				s.GroupCount,
				s.CreatedAt,
				s.SourceTarget,
			)
		}
		return w.Flush()
	},
}

var sessionsGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get full session details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"name": args[0]}
		resp, err := connectAndRequest("sessions.get", payload, targetSelector())
		if err != nil {
			return err
		}
		if sessionsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			Session struct {
				Name    string `json:"name"`
				Windows []struct {
					Tabs []struct {
						Title string `json:"title"`
						URL   string `json:"url"`
					} `json:"tabs"`
				} `json:"windows"`
			} `json:"session"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		tabCount := 0
		for _, w := range result.Session.Windows {
			tabCount += len(w.Tabs)
		}
		fmt.Printf("Session %q: %d windows, %d tabs\n", result.Session.Name, len(result.Session.Windows), tabCount)
		for wi, w := range result.Session.Windows {
			fmt.Printf("  Window %d (%d tabs):\n", wi+1, len(w.Tabs))
			for _, t := range w.Tabs {
				fmt.Printf("    %s  %s\n", truncate(t.Title, 40), truncate(t.URL, 50))
			}
		}
		return nil
	},
}

var sessionsSaveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save the current browser state as a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"name": args[0]}
		resp, err := connectAndRequest("sessions.save", payload, targetSelector())
		if err != nil {
			return err
		}
		if sessionsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			Name        string `json:"name"`
			TabCount    int    `json:"tabCount"`
			WindowCount int    `json:"windowCount"`
			GroupCount  int    `json:"groupCount"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		fmt.Printf("Session %q saved (%d tabs, %d windows, %d groups).\n",
			result.Name, result.TabCount, result.WindowCount, result.GroupCount)
		return nil
	},
}

var sessionsRestoreCmd = &cobra.Command{
	Use:   "restore <name>",
	Short: "Restore a saved session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"name": args[0]}
		resp, err := connectAndRequest("sessions.restore", payload, targetSelector())
		if err != nil {
			return err
		}
		if sessionsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			WindowsCreated int `json:"windowsCreated"`
			TabsOpened     int `json:"tabsOpened"`
			TabsFailed     int `json:"tabsFailed"`
			GroupsCreated  int `json:"groupsCreated"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		fmt.Printf("Session %q restored: %d tabs opened (%d windows, %d groups)",
			args[0], result.TabsOpened, result.WindowsCreated, result.GroupsCreated)
		if result.TabsFailed > 0 {
			fmt.Printf(", %d failed", result.TabsFailed)
		}
		fmt.Println(".")
		return nil
	},
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a saved session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"name": args[0]}
		_, err := connectAndRequest("sessions.delete", payload, targetSelector())
		if err != nil {
			return err
		}
		fmt.Printf("Session %q deleted.\n", args[0])
		return nil
	},
}

func init() {
	sessionsCmd.PersistentFlags().BoolVar(&sessionsJSONOutput, "json", false, "Output as JSON")

	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsGetCmd)
	sessionsCmd.AddCommand(sessionsSaveCmd)
	sessionsCmd.AddCommand(sessionsRestoreCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	rootCmd.AddCommand(sessionsCmd)
}
