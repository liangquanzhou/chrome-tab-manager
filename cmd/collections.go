package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var collectionsJSONOutput bool

var collectionsCmd = &cobra.Command{
	Use:   "collections",
	Short: "Manage collections of saved items",
}

var collectionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all collections",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("collections.list", nil, targetSelector())
		if err != nil {
			return err
		}
		if collectionsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Collections []struct {
				Name      string `json:"name"`
				ItemCount int    `json:"itemCount"`
				CreatedAt string `json:"createdAt"`
				UpdatedAt string `json:"updatedAt"`
			} `json:"collections"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tITEMS\tCREATED\tUPDATED")
		for _, c := range result.Collections {
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
				c.Name,
				c.ItemCount,
				c.CreatedAt,
				c.UpdatedAt,
			)
		}
		return w.Flush()
	},
}

var collectionsGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get full collection details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"name": args[0]}
		resp, err := connectAndRequest("collections.get", payload, targetSelector())
		if err != nil {
			return err
		}
		if collectionsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			Collection struct {
				Name  string `json:"name"`
				Items []struct {
					URL   string `json:"url"`
					Title string `json:"title"`
				} `json:"items"`
			} `json:"collection"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		fmt.Printf("Collection %q (%d items):\n", result.Collection.Name, len(result.Collection.Items))
		for _, item := range result.Collection.Items {
			fmt.Printf("  %s  %s\n", truncate(item.Title, 40), truncate(item.URL, 50))
		}
		return nil
	},
}

var collectionsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new collection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"name": args[0]}
		resp, err := connectAndRequest("collections.create", payload, targetSelector())
		if err != nil {
			return err
		}
		if collectionsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			Name      string `json:"name"`
			CreatedAt string `json:"createdAt"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		fmt.Printf("Collection %q created at %s.\n", result.Name, result.CreatedAt)
		return nil
	},
}

var collectionsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a collection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"name": args[0]}
		_, err := connectAndRequest("collections.delete", payload, targetSelector())
		if err != nil {
			return err
		}
		fmt.Printf("Collection %q deleted.\n", args[0])
		return nil
	},
}

var collectionsAddURL string
var collectionsAddTitle string

var collectionsAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add an item to a collection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{
			"name": args[0],
			"items": []map[string]string{
				{"url": collectionsAddURL, "title": collectionsAddTitle},
			},
		}
		resp, err := connectAndRequest("collections.addItems", payload, targetSelector())
		if err != nil {
			return err
		}
		if collectionsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			Name      string `json:"name"`
			ItemCount int    `json:"itemCount"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		fmt.Printf("Added to %q (now %d items).\n", result.Name, result.ItemCount)
		return nil
	},
}

var collectionsRestoreCmd = &cobra.Command{
	Use:   "restore <name>",
	Short: "Restore all items from a collection as tabs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"name": args[0]}
		resp, err := connectAndRequest("collections.restore", payload, targetSelector())
		if err != nil {
			return err
		}
		if collectionsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		var result struct {
			TabsOpened int `json:"tabsOpened"`
			TabsFailed int `json:"tabsFailed"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		fmt.Printf("Collection %q restored: %d tabs opened", args[0], result.TabsOpened)
		if result.TabsFailed > 0 {
			fmt.Printf(", %d failed", result.TabsFailed)
		}
		fmt.Println(".")
		return nil
	},
}

func init() {
	collectionsCmd.PersistentFlags().BoolVar(&collectionsJSONOutput, "json", false, "Output as JSON")

	collectionsAddCmd.Flags().StringVar(&collectionsAddURL, "url", "", "URL of the item to add (required)")
	collectionsAddCmd.Flags().StringVar(&collectionsAddTitle, "title", "", "Title of the item to add (required)")
	_ = collectionsAddCmd.MarkFlagRequired("url")
	_ = collectionsAddCmd.MarkFlagRequired("title")

	collectionsCmd.AddCommand(collectionsListCmd)
	collectionsCmd.AddCommand(collectionsGetCmd)
	collectionsCmd.AddCommand(collectionsCreateCmd)
	collectionsCmd.AddCommand(collectionsDeleteCmd)
	collectionsCmd.AddCommand(collectionsAddCmd)
	collectionsCmd.AddCommand(collectionsRestoreCmd)
	rootCmd.AddCommand(collectionsCmd)
}
