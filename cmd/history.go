package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	historyJSONOutput bool
	historyMaxResults int
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Manage browser history",
}

var historySearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search browser history",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := ""
		if len(args) > 0 {
			query = args[0]
		}
		payload := map[string]any{
			"query":      query,
			"maxResults": historyMaxResults,
		}
		resp, err := connectAndRequest("history.search", payload, targetSelector())
		if err != nil {
			return err
		}
		if historyJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			History []struct {
				ID            string  `json:"id"`
				URL           string  `json:"url"`
				Title         string  `json:"title"`
				LastVisitTime float64 `json:"lastVisitTime"`
				VisitCount    int     `json:"visitCount"`
			} `json:"history"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if len(result.History) == 0 {
			fmt.Println("No history entries found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "TITLE\tURL\tVISITS\tLAST VISIT")
		for _, h := range result.History {
			lastVisit := ""
			if h.LastVisitTime > 0 {
				t := time.UnixMilli(int64(h.LastVisitTime))
				lastVisit = t.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
				truncate(h.Title, 40),
				truncate(h.URL, 50),
				h.VisitCount,
				lastVisit,
			)
		}
		return w.Flush()
	},
}

var historyDeleteCmd = &cobra.Command{
	Use:   "delete <url>",
	Short: "Delete a URL from browser history",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := connectAndRequest("history.delete", map[string]string{"url": args[0]}, targetSelector())
		if err != nil {
			return err
		}
		fmt.Printf("Deleted %q from history.\n", args[0])
		return nil
	},
}

func init() {
	historyCmd.PersistentFlags().BoolVar(&historyJSONOutput, "json", false, "Output as JSON")
	historySearchCmd.Flags().IntVar(&historyMaxResults, "limit", 100, "Maximum results")

	historyCmd.AddCommand(historySearchCmd)
	historyCmd.AddCommand(historyDeleteCmd)
	rootCmd.AddCommand(historyCmd)
}
