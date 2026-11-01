package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "argus",
	Short: "Argus Tier-1 log anomaly detection daemon",
	Long:  "Tail logs, mask and template with Drain, track state in Redis, and alert via webhooks.",
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
