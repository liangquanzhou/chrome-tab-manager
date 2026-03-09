package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"ctm/internal/config"
	"ctm/internal/nmshim"
	"ctm/internal/protocol"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ctm",
	Short: "Terminal-first browser workspace manager",
	Long:  "CTM controls the browser, captures what matters, organizes it into knowledge, and carries it across devices.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&targetFlag, "target", "", "Target browser instance ID")
}

func Execute() {
	// Chrome Native Messaging launches the binary with "chrome-extension://..." as arg.
	// Detect this and automatically enter nm-shim mode.
	if len(os.Args) >= 2 && strings.HasPrefix(os.Args[1], "chrome-extension://") {
		if err := nmshim.Run(context.Background(), config.SocketPath(), os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "nm-shim: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		// Extract structured exit code from ProtocolError
		var pe *protocol.ProtocolError
		if errors.As(err, &pe) {
			os.Exit(pe.ExitCode())
		}
		// Connection errors (daemon unavailable)
		if errors.Is(err, ErrDaemonConnect) {
			os.Exit(ExitDaemonError)
		}
		os.Exit(1)
	}
}
