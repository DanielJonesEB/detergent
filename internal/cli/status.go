package cli

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/re-cinq/assembly-line/internal/config"
	"github.com/re-cinq/assembly-line/internal/git"
	"github.com/re-cinq/assembly-line/internal/runner"
	"github.com/re-cinq/assembly-line/internal/state"
	"github.com/spf13/cobra"
)

// ANSI color codes for STAT-2 color coding
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorOrange = "\033[33m"
	colorYellow = "\033[93m"
	colorRed    = "\033[31m"
	colorGrey   = "\033[90m"
)

var followFlag bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the assembly line",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}

		if followFlag {
			// Hide cursor and clear screen during follow mode; restore on exit or signal
			fmt.Print("\033[?25l\033[2J")
			showCursor := func() { fmt.Print("\033[?25h") }
			defer showCursor()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt)
			go func() {
				<-sigCh
				showCursor()
				os.Exit(0)
			}()
		}

		for {
			if followFlag {
				// Move cursor to home position (no screen clear to avoid flicker)
				fmt.Print("\033[H")
			}

			if err := printStatus(".", cfg, followFlag); err != nil {
				return err
			}

			if followFlag {
				// Clear from cursor to end of screen (remove stale lines)
				fmt.Print("\033[J")
			}

			if !followFlag {
				break
			}
			time.Sleep(2 * time.Second)
		}
		return nil
	},
}

// stationInfo holds the computed display state for a station.
type stationInfo struct {
	symbol    string
	color     string
	name      string    // "pending", "agent running", "failed", "up to date"
	startTime time.Time // non-zero when agent is running
}

// computeStationInfo returns the display state for a station based on process
// and git state (STAT-5: on-demand computation).
func computeStationInfo(dir string, station config.Station, watchedFullRef, watchedBranch string) stationInfo {
	branchName := git.StationBranchName(station.Name)
	if !git.BranchExists(dir, branchName) {
		return stationInfo{symbol: "○", color: colorYellow, name: "pending"}
	}

	agentPID, startTime, _ := state.ReadStationPID(dir, station.Name)
	if agentPID > 0 && state.IsProcessRunning(agentPID) {
		return stationInfo{symbol: "●", color: colorOrange, name: "agent running", startTime: startTime}
	}
	if state.ReadStationFailed(dir, station.Name) {
		return stationInfo{symbol: "✗", color: colorRed, name: "failed"}
	}
	if watchedFullRef != "" && git.IsAncestor(dir, watchedFullRef, branchName) {
		return stationInfo{symbol: "✓", color: colorGreen, name: "up to date"}
	}
	// STAT-8: If the only commits between station and watched branch are
	// skip-marker commits, the station is still up to date.
	if watchedFullRef != "" && git.OnlySkipCommitsBetween(dir, branchName, watchedBranch, runner.SkipMarkers) {
		return stationInfo{symbol: "✓", color: colorGreen, name: "up to date"}
	}
	return stationInfo{symbol: "○", color: colorYellow, name: "pending"}
}

// formatUptime formats the duration since startTime as a human-readable string.
func formatUptime(startTime time.Time) string {
	d := time.Since(startTime)
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	s = s % 60
	if m < 60 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := m / 60
	m = m % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

func printStatus(dir string, cfg *config.Config, clearEOL bool) error {
	// When clearEOL is true (follow mode), append ANSI erase-to-end-of-line
	// after each line to prevent stale characters from shorter redraws.
	eol := "\n"
	if clearEOL {
		eol = "\033[K\n"
	}

	// STAT-3: Line runner indicator at the top
	pid, _ := state.ReadPID(dir)
	configName := filepath.Base(configPath)
	if pid > 0 && state.IsProcessRunning(pid) {
		fmt.Fprintf(os.Stdout, "%s▶%s %s%s", colorGreen, colorReset, configName, eol)
	} else {
		fmt.Fprintf(os.Stdout, "%s⏸%s %s%s", colorGrey, colorReset, configName, eol)
	}

	// Blank line + column headers
	fmt.Fprintf(os.Stdout, "%s", eol)
	fmt.Fprintf(os.Stdout, "%-21s%-9s%s%s", "Stations", "Head", "Status", eol)

	// Print watched branch
	watchedRef, _ := git.HeadShortRef(dir)
	watchedDirty, _ := git.IsDirty(dir)
	dirtyStr := ""
	if watchedDirty {
		dirtyStr = "(dirty)"
	}
	fmt.Fprintf(os.Stdout, "%-21s%-9s%s%s", cfg.Settings.Watches, watchedRef, dirtyStr, eol)

	// Get the watched branch full ref for ancestor checks (STAT-5: on-demand)
	watchedFullRef, _ := git.Run(dir, "rev-parse", cfg.Settings.Watches)

	// Print each station
	for _, station := range cfg.Stations {
		branchName := git.StationBranchName(station.Name)
		ref := "-"

		if git.BranchExists(dir, branchName) {
			if branchRef, err := git.Run(dir, "rev-parse", "--short", branchName); err == nil {
				ref = branchRef
			}
		}

		info := computeStationInfo(dir, station, watchedFullRef, cfg.Settings.Watches)
		extra := ""
		if !info.startTime.IsZero() {
			// STAT-7: Show uptime duration instead of PID/start time
			extra = fmt.Sprintf(" (%s)", formatUptime(info.startTime))
		}

		fmt.Fprintf(os.Stdout, "%s  %s %-17s%-9s[%s]%s%s%s", info.color, info.symbol, station.Name, ref, info.name, extra, colorReset, eol)
	}

	return nil
}

func init() {
	statusCmd.Flags().BoolVarP(&followFlag, "follow", "f", false, "refresh every 2 seconds")
	rootCmd.AddCommand(statusCmd)
}
