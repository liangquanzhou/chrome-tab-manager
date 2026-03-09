package cmd

import (
	"context"
	"os"

	"ctm/internal/config"
	"ctm/internal/nmshim"

	"github.com/spf13/cobra"
)

var nmshimCmd = &cobra.Command{
	Use:    "nm-shim",
	Short:  "Chrome Native Messaging bridge (internal)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return nmshim.Run(context.Background(), config.SocketPath(), os.Stdin, os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(nmshimCmd)
}
