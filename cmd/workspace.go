package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var workspaceJSONOutput bool

var workspaceCmd = &cobra.Command{
	Use:     "workspaces",
	Aliases: []string{"workspace"},
	Short: "Manage workspaces",
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("workspace.list", nil, targetSelector())
		if err != nil {
			return err
		}
		if workspaceJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Workspaces []struct {
				ID              string `json:"id"`
				Name            string `json:"name"`
				SessionCount    int    `json:"sessionCount"`
				CollectionCount int    `json:"collectionCount"`
				CreatedAt       string `json:"createdAt"`
				UpdatedAt       string `json:"updatedAt"`
			} `json:"workspaces"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if len(result.Workspaces) == 0 {
			fmt.Println("No workspaces.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSESSIONS\tCOLLECTIONS\tCREATED")
		for _, ws := range result.Workspaces {
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\n",
				ws.ID,
				ws.Name,
				ws.SessionCount,
				ws.CollectionCount,
				ws.CreatedAt,
			)
		}
		return w.Flush()
	},
}

var workspaceGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get full workspace details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"id": args[0]}
		resp, err := connectAndRequest("workspace.get", payload, targetSelector())
		if err != nil {
			return err
		}
		if workspaceJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			Workspace struct {
				Name        string   `json:"name"`
				Sessions    []string `json:"sessions"`
				Collections []string `json:"collections"`
			} `json:"workspace"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		fmt.Printf("Workspace %q:\n", result.Workspace.Name)
		fmt.Printf("  Sessions:    %v\n", result.Workspace.Sessions)
		fmt.Printf("  Collections: %v\n", result.Workspace.Collections)
		return nil
	},
}

var workspaceCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"name": args[0]}
		resp, err := connectAndRequest("workspace.create", payload, targetSelector())
		if err != nil {
			return err
		}
		if workspaceJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		fmt.Printf("Workspace %q created (id: %s)\n", result.Name, result.ID)
		return nil
	},
}

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"id": args[0]}
		_, err := connectAndRequest("workspace.delete", payload, targetSelector())
		if err != nil {
			return err
		}
		fmt.Printf("Workspace %q deleted.\n", args[0])
		return nil
	},
}

var workspaceSwitchCmd = &cobra.Command{
	Use:   "switch <id>",
	Short: "Switch to a workspace (close current tabs and restore workspace session)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"id": args[0]}
		resp, err := connectAndRequest("workspace.switch", payload, targetSelector())
		if err != nil {
			return err
		}
		if workspaceJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			TabsClosed     int `json:"tabsClosed"`
			WindowsCreated int `json:"windowsCreated"`
			TabsOpened     int `json:"tabsOpened"`
			TabsFailed     int `json:"tabsFailed"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		fmt.Printf("Switched: closed %d tabs, opened %d tabs (%d windows), %d failed\n",
			result.TabsClosed, result.TabsOpened, result.WindowsCreated, result.TabsFailed)
		return nil
	},
}

var workspaceUpdateName string
var workspaceUpdateDescription string
var workspaceUpdateStatus string

var workspaceUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a workspace's name, description, or status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"id": args[0]}
		if cmd.Flags().Changed("name") {
			payload["name"] = workspaceUpdateName
		}
		if cmd.Flags().Changed("description") {
			payload["description"] = workspaceUpdateDescription
		}
		if cmd.Flags().Changed("status") {
			payload["status"] = workspaceUpdateStatus
		}
		resp, err := connectAndRequest("workspace.update", payload, targetSelector())
		if err != nil {
			return err
		}
		if workspaceJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		fmt.Printf("Workspace %q updated.\n", args[0])
		return nil
	},
}

func init() {
	workspaceCmd.PersistentFlags().BoolVar(&workspaceJSONOutput, "json", false, "Output as JSON")

	workspaceUpdateCmd.Flags().StringVar(&workspaceUpdateName, "name", "", "New workspace name")
	workspaceUpdateCmd.Flags().StringVar(&workspaceUpdateDescription, "description", "", "New workspace description")
	workspaceUpdateCmd.Flags().StringVar(&workspaceUpdateStatus, "status", "", "New workspace status (active, archived, template)")

	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceGetCmd)
	workspaceCmd.AddCommand(workspaceCreateCmd)
	workspaceCmd.AddCommand(workspaceDeleteCmd)
	workspaceCmd.AddCommand(workspaceSwitchCmd)
	workspaceCmd.AddCommand(workspaceUpdateCmd)
	rootCmd.AddCommand(workspaceCmd)
}
