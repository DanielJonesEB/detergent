package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fission-ai/detergent/internal/config"
	"github.com/fission-ai/detergent/internal/engine"
	gitops "github.com/fission-ai/detergent/internal/git"
	"github.com/spf13/cobra"
)

var (
	statusFollow   bool
	statusInterval float64
)

func init() {
	statusCmd.Flags().BoolVarP(&statusFollow, "follow", "f", false, "Live-update status (like watch)")
	statusCmd.Flags().Float64VarP(&statusInterval, "interval", "n", 2.0, "Seconds between updates (with --follow)")
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status <config-file>",
	Short: "Show the status of each concern",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		errs := config.Validate(cfg)
		if len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "Error: %s\n", e)
			}
			return fmt.Errorf("%d validation error(s)", len(errs))
		}

		configPath, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		repoDir := findGitRoot(filepath.Dir(configPath))
		if repoDir == "" {
			return fmt.Errorf("could not find git repository root")
		}

		if statusFollow {
			return followStatus(cfg, repoDir)
		}
		return showStatus(cfg, repoDir)
	},
}

func followStatus(cfg *config.Config, repoDir string) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	interval := time.Duration(statusInterval * float64(time.Second))
	var lastOutput string

	for {
		var buf bytes.Buffer
		if err := renderStatus(&buf, cfg, repoDir); err != nil {
			fmt.Fprintf(os.Stderr, "\nerror: %s\n", err)
		}
		output := buf.String()

		if output != lastOutput {
			fmt.Print("\033[H\033[2J")
			fmt.Printf("Every %.1fs: detergent status\n\n", statusInterval)
			fmt.Print(output)
			lastOutput = output
		}

		select {
		case <-sigCh:
			fmt.Println()
			return nil
		case <-time.After(interval):
		}
	}
}

func showStatus(cfg *config.Config, repoDir string) error {
	return renderStatus(os.Stdout, cfg, repoDir)
}

func renderStatus(w io.Writer, cfg *config.Config, repoDir string) error {
	repo := gitops.NewRepo(repoDir)
	nameSet := make(map[string]bool)
	for _, c := range cfg.Concerns {
		nameSet[c.Name] = true
	}

	fmt.Fprintln(w, "Concern Status")
	fmt.Fprintln(w, "──────────────────────────────────────")

	for _, c := range cfg.Concerns {
		watchedBranch := c.Watches
		if nameSet[c.Watches] {
			watchedBranch = cfg.Settings.BranchPrefix + c.Watches
		}

		lastSeen, err := engine.LastSeen(repoDir, c.Name)
		if err != nil {
			return err
		}

		head, err := repo.HeadCommit(watchedBranch)
		if err != nil {
			// Branch might not exist yet
			fmt.Fprintf(w, "  ◯  %-20s  (not started - watched branch %s not found)\n", c.Name, watchedBranch)
			continue
		}

		if lastSeen == "" {
			fmt.Fprintf(w, "  ◯  %-20s  pending (never processed)\n", c.Name)
		} else if lastSeen == head {
			fmt.Fprintf(w, "  ✓  %-20s  caught up at %s\n", c.Name, short(lastSeen))
		} else {
			fmt.Fprintf(w, "  ◯  %-20s  pending (last: %s, head: %s)\n", c.Name, short(lastSeen), short(head))
		}
	}

	return nil
}

func short(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}
