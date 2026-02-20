package cli

import (
	"context"

	"github.com/re-cinq/assembly-line/internal/engine"
	"github.com/spf13/cobra"
)

var runOnce bool

func init() {
	runCmd.Flags().BoolVar(&runOnce, "once", false, "Process pending commits once and exit")
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the line runner",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, repoDir, err := loadConfigAndRepo(configPath)
		if err != nil {
			return err
		}

		if runOnce {
			return engine.RunOnce(cfg, repoDir)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := setupSignalHandler()
		go func() {
			<-sigCh
			cancel()
		}()

		return engine.RunnerLoop(ctx, configPath, cfg, repoDir)
	},
}
