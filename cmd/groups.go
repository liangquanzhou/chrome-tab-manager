package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var groupsJSONOutput bool

var groupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "Manage tab groups",
}

var groupsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tab groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("groups.list", nil, targetSelector())
		if err != nil {
			return err
		}
		if groupsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Groups []struct {
				ID        int    `json:"id"`
				Title     string `json:"title"`
				Color     string `json:"color"`
				Collapsed bool   `json:"collapsed"`
				WindowID  int    `json:"windowId"`
			} `json:"groups"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTITLE\tCOLOR\tCOLLAPSED")
		for _, g := range result.Groups {
			collapsed := "no"
			if g.Collapsed {
				collapsed = "yes"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
				g.ID,
				g.Title,
				g.Color,
				collapsed,
			)
		}
		return w.Flush()
	},
}

var groupsCreateTitle string
var groupsCreateTabIDs []int
var groupsCreateColor string

var groupsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new tab group",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(groupsCreateTabIDs) == 0 {
			return fmt.Errorf("at least one --tab-id is required")
		}
		payload := map[string]any{
			"tabIds": groupsCreateTabIDs,
			"title":  groupsCreateTitle,
		}
		if groupsCreateColor != "" {
			payload["color"] = groupsCreateColor
		}
		resp, err := connectAndRequest("groups.create", payload, targetSelector())
		if err != nil {
			return err
		}
		if groupsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			GroupID int `json:"groupId"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		fmt.Printf("Group %q created (ID %d).\n", groupsCreateTitle, result.GroupID)
		return nil
	},
}

func init() {
	groupsCmd.PersistentFlags().BoolVar(&groupsJSONOutput, "json", false, "Output as JSON")

	groupsCreateCmd.Flags().StringVar(&groupsCreateTitle, "title", "", "Group title (required)")
	groupsCreateCmd.Flags().IntSliceVar(&groupsCreateTabIDs, "tab-id", nil, "Tab IDs to include (repeatable)")
	groupsCreateCmd.Flags().StringVar(&groupsCreateColor, "color", "", "Group color (grey, blue, red, yellow, green, pink, purple, cyan, orange)")
	_ = groupsCreateCmd.MarkFlagRequired("title")

	groupsCmd.AddCommand(groupsListCmd)
	groupsCmd.AddCommand(groupsCreateCmd)
	rootCmd.AddCommand(groupsCmd)
}
