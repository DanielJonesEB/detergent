package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/re-cinq/assembly-line/internal/config"
	"github.com/re-cinq/assembly-line/internal/git"
	"github.com/re-cinq/assembly-line/internal/state"
	"github.com/spf13/cobra"
)

var statuslineCmd = &cobra.Command{
	Use:   "statusline",
	Short: "One-line status for Claude Code statusline integration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}

		line, err := buildStatusLine(".", cfg)
		if err != nil {
			return err
		}

		fmt.Fprint(os.Stdout, line)
		return nil
	},
}

func buildStatusLine(dir string, cfg *config.Config) (string, error) {
	// Get the watched branch full ref for ancestor checks (STAT-5: on-demand)
	watchedFullRef, _ := git.Run(dir, "rev-parse", cfg.Settings.Watches)

	// Build station summaries with symbols and colors matching line status
	var parts []string
	for _, station := range cfg.Stations {
		info := computeStationInfo(dir, station, watchedFullRef, cfg.Settings.Watches)
		parts = append(parts, fmt.Sprintf("%s%s %s%s", info.color, info.symbol, station.Name, colorReset))
	}

	// Line runner ▶/⏸ symbol, matching status command colors
	lineSymbol := colorGrey + "⏸" + colorReset
	pid, _ := state.ReadPID(dir)
	if pid > 0 && state.IsProcessRunning(pid) {
		lineSymbol = colorGreen + "▶" + colorReset
	}

	result := fmt.Sprintf("%s %s", lineSymbol, strings.Join(parts, " "))

	// SL-2: Check if terminal station has commits not in the watched branch
	if len(cfg.Stations) > 0 {
		terminalStation := cfg.Stations[len(cfg.Stations)-1]
		terminalBranch := git.StationBranchName(terminalStation.Name)
		if git.BranchExists(dir, terminalBranch) {
			hasCommits, err := git.HasCommitsBetween(dir, cfg.Settings.Watches, terminalBranch)
			if err == nil && hasCommits {
				result += " | line changes available - /line-preview or /line-rebase"
			}
		}
	}

	return result, nil
}

func init() {
	rootCmd.AddCommand(statuslineCmd)
}
