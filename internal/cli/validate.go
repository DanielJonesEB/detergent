package cli

import (
	"fmt"
	"os"

	"github.com/re-cinq/assembly-line/internal/config"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate line.yaml and report errors",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		errs := config.Validate(cfg)
		if len(errs) == 0 {
			fmt.Println("valid")
			return nil
		}

		for _, e := range errs {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(1)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
