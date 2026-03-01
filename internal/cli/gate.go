package cli

import (
	"fmt"

	"github.com/re-cinq/assembly-line/internal/config"
	"github.com/re-cinq/assembly-line/internal/gate"
	"github.com/spf13/cobra"
)

var gateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Run pre-commit gates",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}

		gates := make([]gate.Gate, len(cfg.Gates))
		for i, g := range cfg.Gates {
			gates[i] = gate.Gate{Name: g.Name, Run: g.Run}
		}

		if err := gate.RunGates(gates, "."); err != nil {
			return fmt.Errorf("gates failed: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(gateCmd)
}
