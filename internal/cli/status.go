package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fission-ai/detergent/internal/config"
	"github.com/fission-ai/detergent/internal/engine"
	gitops "github.com/fission-ai/detergent/internal/git"
	"github.com/spf13/cobra"
)

func init() {
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

		return showStatus(cfg, repoDir)
	},
}

func showStatus(cfg *config.Config, repoDir string) error {
	repo := gitops.NewRepo(repoDir)
	nameSet := make(map[string]bool)
	for _, c := range cfg.Concerns {
		nameSet[c.Name] = true
	}

	fmt.Println("Concern Status")
	fmt.Println("──────────────────────────────────────")

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
			fmt.Printf("  ◯  %-20s  (not started - watched branch %s not found)\n", c.Name, watchedBranch)
			continue
		}

		if lastSeen == "" {
			fmt.Printf("  ◯  %-20s  pending (never processed)\n", c.Name)
		} else if lastSeen == head {
			fmt.Printf("  ✓  %-20s  caught up at %s\n", c.Name, short(lastSeen))
		} else {
			fmt.Printf("  ◯  %-20s  pending (last: %s, head: %s)\n", c.Name, short(lastSeen), short(head))
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
