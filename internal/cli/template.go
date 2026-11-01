package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/OrenVill/Argus/internal/config"
	"github.com/OrenVill/Argus/internal/daemon"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Template admin actions",
}

var templateIgnoreCmd = &cobra.Command{
	Use:   "ignore",
	Short: "Mark a template as ignored",
	RunE: func(cmd *cobra.Command, _ []string) error {
		hash, _ := cmd.Flags().GetString("hash")
		return runTemplateStatus(hash, "ignored")
	},
}

var templateResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a template (same as status=new)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		hash, _ := cmd.Flags().GetString("hash")
		return runTemplateStatus(hash, "new")
	},
}

func runTemplateStatus(hash, status string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return daemon.SetTemplateStatusCLI(ctx, cfg, hash, status)
}

func init() {
	templateIgnoreCmd.Flags().String("hash", "", "Template hash (required)")
	templateResetCmd.Flags().String("hash", "", "Template hash (required)")
	_ = templateIgnoreCmd.MarkFlagRequired("hash")
	_ = templateResetCmd.MarkFlagRequired("hash")
	templateCmd.AddCommand(templateIgnoreCmd, templateResetCmd)
	rootCmd.AddCommand(templateCmd)
}
