package cli

import (
	"fmt"

	"github.com/re-cinq/assembly-line/internal/config"
	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Output JSON Schema for line.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(string(config.Schema()))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
