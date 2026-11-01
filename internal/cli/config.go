package cli

var configPath string

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "argus.yaml", "Path to argus.yaml")
}
