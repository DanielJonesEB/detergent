package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of line",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("line %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
