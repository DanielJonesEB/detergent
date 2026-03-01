package cli

import (
	"github.com/spf13/cobra"
)

var (
	configPath string
	Version    = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "line",
	Short: "Assembly line - automated tasks on commits via Git hooks",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "path", "p", "line.yaml", "path to config file")
}

func Execute() error {
	return rootCmd.Execute()
}
