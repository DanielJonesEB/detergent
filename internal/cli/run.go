package cli

import (
	"github.com/re-cinq/assembly-line/internal/engine"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Process pending commits and exit",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, repoDir, err := loadConfigAndRepo(configPath)
		if err != nil {
			return err
		}

		return engine.RunOnce(cfg, repoDir)
	},
}
