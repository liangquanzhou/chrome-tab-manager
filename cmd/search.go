package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	searchJSONOutput bool
	searchScopes     []string
	searchLimit      int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across all resources",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		payload := map[string]any{
			"query": query,
			"mode":  "global",
			"limit": searchLimit,
		}
		if len(searchScopes) > 0 {
			payload["scopes"] = searchScopes
		}

		resp, err := connectAndRequest("search.query", payload, targetSelector())
		if err != nil {
			return err
		}
		if searchJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Results []struct {
				Kind       string  `json:"kind"`
				ID         string  `json:"id"`
				Title      string  `json:"title"`
				URL        string  `json:"url"`
				MatchField string  `json:"matchField"`
				Score      float64 `json:"score"`
			} `json:"results"`
			Total int `json:"total"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if len(result.Results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "KIND\tTITLE\tMATCH\tSCORE")
		for _, r := range result.Results {
			title := truncate(r.Title, 50)
			fmt.Fprintf(w, "%s\t%s\t%s\t%.1f\n",
				r.Kind,
				title,
				r.MatchField,
				r.Score,
			)
		}
		fmt.Fprintf(w, "\n%d results\n", result.Total)
		return w.Flush()
	},
}

var searchSavedCmd = &cobra.Command{
	Use:   "saved",
	Short: "Manage saved searches",
}

var searchSavedListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved searches",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("search.saved.list", nil, targetSelector())
		if err != nil {
			return err
		}
		if searchJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Searches []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				CreatedAt string `json:"createdAt"`
			} `json:"searches"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if len(result.Searches) == 0 {
			fmt.Println("No saved searches.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tCREATED")
		for _, s := range result.Searches {
			fmt.Fprintf(w, "%s\t%s\t%s\n", s.ID, s.Name, s.CreatedAt)
		}
		return w.Flush()
	},
}

var searchSavedCreateCmd = &cobra.Command{
	Use:   "create <name> <query>",
	Short: "Create a saved search",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// daemon 期望 query 是 SearchQuery 对象，而非纯字符串
		payload := map[string]any{
			"name": args[0],
			"query": map[string]any{
				"query": args[1],
				"mode":  "global",
			},
		}
		resp, err := connectAndRequest("search.saved.create", payload, targetSelector())
		if err != nil {
			return err
		}
		if searchJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		fmt.Printf("Saved search %q created.\n", args[0])
		return nil
	},
}

var searchSavedDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a saved search",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := connectAndRequest("search.saved.delete", map[string]string{"id": args[0]}, targetSelector())
		if err != nil {
			return err
		}
		fmt.Printf("Saved search %q deleted.\n", args[0])
		return nil
	},
}

func init() {
	searchCmd.PersistentFlags().BoolVar(&searchJSONOutput, "json", false, "Output as JSON")
	searchCmd.Flags().StringSliceVar(&searchScopes, "scope", nil, "Limit search to specific scopes (tabs,sessions,collections,bookmarks,workspaces)")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 50, "Maximum number of results")

	searchSavedCmd.AddCommand(searchSavedListCmd)
	searchSavedCmd.AddCommand(searchSavedCreateCmd)
	searchSavedCmd.AddCommand(searchSavedDeleteCmd)
	searchCmd.AddCommand(searchSavedCmd)
	rootCmd.AddCommand(searchCmd)
}
