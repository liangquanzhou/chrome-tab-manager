package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var syncJSONOutput bool

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manage sync",
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("sync.status", nil, targetSelector())
		if err != nil {
			return err
		}
		if syncJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Enabled        bool     `json:"enabled"`
			SyncDir        string   `json:"syncDir"`
			LastSync       string   `json:"lastSync"`
			PendingChanges int      `json:"pendingChanges"`
			Conflicts      []string `json:"conflicts"`
			MetaVersion    int      `json:"metaVersion"`
			DeviceID       string   `json:"deviceId"`
			ObjectCount    int      `json:"objectCount"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if result.Enabled {
			fmt.Println("Sync: enabled")
		} else {
			fmt.Println("Sync: disabled (cloud directory not found)")
		}
		fmt.Printf("Sync dir: %s\n", result.SyncDir)
		if result.LastSync != "" {
			fmt.Printf("Last sync: %s\n", result.LastSync)
		}
		if result.DeviceID != "" {
			fmt.Printf("Device ID: %s\n", result.DeviceID)
		}
		if result.MetaVersion > 0 {
			fmt.Printf("Meta version: %d\n", result.MetaVersion)
		}
		if result.ObjectCount > 0 {
			fmt.Printf("Tracked objects: %d\n", result.ObjectCount)
		}
		fmt.Printf("Pending changes: %d\n", result.PendingChanges)
		if len(result.Conflicts) > 0 {
			fmt.Printf("Conflicts: %d\n", len(result.Conflicts))
			for _, c := range result.Conflicts {
				fmt.Printf("  - %s\n", c)
			}
		}
		return nil
	},
}

var syncRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair sync state",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("sync.repair", nil, targetSelector())
		if err != nil {
			return err
		}
		if syncJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Reindexed         bool `json:"reindexed"`
			ObjectCount       int  `json:"objectCount"`
			ConflictsResolved int  `json:"conflictsResolved"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		fmt.Printf("Repair complete: %d objects, %d conflicts resolved\n",
			result.ObjectCount, result.ConflictsResolved)
		return nil
	},
}

func init() {
	syncCmd.PersistentFlags().BoolVar(&syncJSONOutput, "json", false, "Output as JSON")

	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncRepairCmd)
	rootCmd.AddCommand(syncCmd)
}
