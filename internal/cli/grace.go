package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/OrenVill/Argus/internal/config"
	"github.com/OrenVill/Argus/internal/daemon"
)

var graceCmd = &cobra.Command{
	Use:   "grace",
	Short: "Deploy grace window controls",
}

var graceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a deploy grace window",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		service, _ := cmd.Flags().GetString("service")
		durStr, _ := cmd.Flags().GetString("duration")
		dur, err := time.ParseDuration(durStr)
		if err != nil {
			return fmt.Errorf("duration: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return daemon.SetGraceCLI(ctx, cfg, service, dur)
	},
}

func init() {
	graceStartCmd.Flags().String("service", "", "Service name (defaults to meta.service)")
	graceStartCmd.Flags().String("duration", "30m", "Grace window duration")
	graceCmd.AddCommand(graceStartCmd)
	rootCmd.AddCommand(graceCmd)
}
