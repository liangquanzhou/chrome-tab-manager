package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ctm/internal/config"
	"ctm/internal/daemon"

	"github.com/spf13/cobra"
)

var daemonForeground bool

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the CTM daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.EnsureDirs(); err != nil {
			return fmt.Errorf("ensure dirs: %w", err)
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer cancel()

		if daemonForeground {
			log.SetOutput(os.Stderr)
		} else {
			f, err := os.OpenFile(config.LogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				return fmt.Errorf("open log: %w", err)
			}
			defer f.Close()
			log.SetOutput(f)
		}

		srv := daemon.NewServer(
			config.SocketPath(),
			config.LockPath(),
			config.SessionsDir(),
			config.CollectionsDir(),
			config.BookmarksDir(),
			config.OverlaysDir(),
			config.WorkspacesDir(),
			config.SavedSearchesDir(),
			config.SyncDir(),
			config.SearchIndexPath(),
		)

		return srv.Start(ctx)
	},
}

func init() {
	daemonCmd.Flags().BoolVar(&daemonForeground, "foreground", false, "Run in foreground (log to stderr)")
	rootCmd.AddCommand(daemonCmd)
}
