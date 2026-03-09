package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var downloadsJSONOutput bool

var downloadsCmd = &cobra.Command{
	Use:   "downloads",
	Short: "Manage browser downloads",
}

var downloadsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent downloads",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("downloads.list", nil, targetSelector())
		if err != nil {
			return err
		}
		if downloadsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Downloads []struct {
				ID         int    `json:"id"`
				Filename   string `json:"filename"`
				URL        string `json:"url"`
				State      string `json:"state"`
				TotalBytes int64  `json:"totalBytes"`
			} `json:"downloads"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if len(result.Downloads) == 0 {
			fmt.Println("No downloads.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tFILENAME\tSTATE\tSIZE")
		for _, d := range result.Downloads {
			fmt.Fprintf(w, "%d\t%s\t%s\t%d\n",
				d.ID,
				truncate(d.Filename, 40),
				d.State,
				d.TotalBytes,
			)
		}
		return w.Flush()
	},
}

var downloadsCancelCmd = &cobra.Command{
	Use:   "cancel <downloadId>",
	Short: "Cancel a download",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid download ID %q: %w", args[0], err)
		}
		_, err = connectAndRequest("downloads.cancel", map[string]any{"id": id}, targetSelector())
		if err != nil {
			return err
		}
		fmt.Printf("Download %d cancelled.\n", id)
		return nil
	},
}

func init() {
	downloadsCmd.PersistentFlags().BoolVar(&downloadsJSONOutput, "json", false, "Output as JSON")

	downloadsCmd.AddCommand(downloadsListCmd)
	downloadsCmd.AddCommand(downloadsCancelCmd)
	rootCmd.AddCommand(downloadsCmd)
}
