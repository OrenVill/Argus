package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/OrenVill/Argus/internal/config"
	"github.com/OrenVill/Argus/internal/daemon"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the Argus daemon",
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return daemon.Run(ctx, cfg)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
