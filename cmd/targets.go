package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var targetsJSONOutput bool

var targetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "Manage browser targets (connected browser instances)",
}

var targetsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all connected browser targets",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("targets.list", nil, targetSelector())
		if err != nil {
			return err
		}
		if targetsJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Targets []struct {
				TargetID    string `json:"targetId"`
				Channel     string `json:"channel"`
				Label       string `json:"label"`
				IsDefault   bool   `json:"isDefault"`
				ConnectedAt int64  `json:"connectedAt"`
			} `json:"targets"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tCHANNEL\tLABEL\tDEFAULT\tCONNECTED")
		for _, t := range result.Targets {
			isDefault := "no"
			if t.IsDefault {
				isDefault = "yes"
			}
			connected := "yes"
			if t.ConnectedAt == 0 {
				connected = "no"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				t.TargetID,
				t.Channel,
				t.Label,
				isDefault,
				connected,
			)
		}
		return w.Flush()
	},
}

var targetsDefaultCmd = &cobra.Command{
	Use:   "default <targetId>",
	Short: "Set a target as the default",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"targetId": args[0]}
		_, err := connectAndRequest("targets.default", payload, targetSelector())
		if err != nil {
			return err
		}
		fmt.Printf("Default target set to %q.\n", args[0])
		return nil
	},
}

var targetsClearDefaultCmd = &cobra.Command{
	Use:   "clear-default",
	Short: "Clear the default target",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := connectAndRequest("targets.clearDefault", nil, targetSelector())
		if err != nil {
			return err
		}
		fmt.Println("Default target cleared.")
		return nil
	},
}

var targetsLabelCmd = &cobra.Command{
	Use:   "label <targetId> <label>",
	Short: "Set a label on a target",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{
			"targetId": args[0],
			"label":    args[1],
		}
		_, err := connectAndRequest("targets.label", payload, targetSelector())
		if err != nil {
			return err
		}
		fmt.Printf("Target %q labeled as %q.\n", args[0], args[1])
		return nil
	},
}

func init() {
	targetsCmd.PersistentFlags().BoolVar(&targetsJSONOutput, "json", false, "Output as JSON")

	targetsCmd.AddCommand(targetsListCmd)
	targetsCmd.AddCommand(targetsDefaultCmd)
	targetsCmd.AddCommand(targetsClearDefaultCmd)
	targetsCmd.AddCommand(targetsLabelCmd)
	rootCmd.AddCommand(targetsCmd)
}
