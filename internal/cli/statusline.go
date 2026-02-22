package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/re-cinq/assembly-line/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	statuslineCmd.Hidden = true
	rootCmd.AddCommand(statuslineCmd)
}

var statuslineCmd = &cobra.Command{
	Use:   "statusline",
	Short: "Render station line for Claude Code statusline (reads JSON from stdin)",
	RunE: func(cmd *cobra.Command, args []string) error {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		dir := resolveProjectDir(input)
		if dir == "" {
			return nil // silent exit
		}

		configPath := findLineConfig(dir)
		if configPath == "" {
			return nil // not a line project
		}

		cfg, err := config.Load(configPath)
		if err != nil {
			return nil // silent exit on bad config
		}
		if errs := config.Validate(cfg); len(errs) > 0 {
			return nil // silent exit on invalid config
		}

		repoDir := findGitRoot(filepath.Dir(configPath))
		if repoDir == "" {
			return nil
		}

		data := gatherStatuslineData(cfg, repoDir)
		rendered := renderGraph(data)
		if rendered != "" {
			fmt.Print(rendered)
		}
		return nil
	},
}

// claudeCodeInput represents the JSON object Claude Code passes on stdin.
type claudeCodeInput struct {
	CWD       string `json:"cwd"`
	Workspace *struct {
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
}

// resolveProjectDir extracts the project directory from Claude Code's stdin JSON.
func resolveProjectDir(input []byte) string {
	var ci claudeCodeInput
	if err := json.Unmarshal(input, &ci); err != nil {
		return ""
	}
	if ci.Workspace != nil && ci.Workspace.ProjectDir != "" {
		return ci.Workspace.ProjectDir
	}
	return ci.CWD
}

// findLineConfig walks up from dir looking for line.yaml or line.yml.
func findLineConfig(dir string) string {
	return findFileUp(dir, []string{"line.yaml", "line.yml"})
}

func renderStation(name string, stations map[string]StationData) string {
	c := stations[name]
	sym, clr := stateDisplay(c.State, c.LastResult)
	return fmt.Sprintf("%s%s %s%s", clr, name, sym, ansiReset)
}

// renderGraph produces the full ANSI-colored line string from statusline data.
func renderGraph(data StatuslineOutput) string {
	if len(data.Stations) == 0 {
		return ""
	}

	stations := make(map[string]StationData)
	for _, c := range data.Stations {
		stations[c.Name] = c
	}

	// Render as ordered list: source_branch ─── station1 ─── station2 ─── ...
	parts := make([]string, len(data.Stations))
	for i, c := range data.Stations {
		parts[i] = renderStation(c.Name, stations)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s ─── %s", data.SourceBranch, strings.Join(parts, " ── ")))

	if hint := rebaseHint(data); hint != "" {
		sb.WriteString("\n")
		sb.WriteString(hint)
	}

	return sb.String()
}

// rebaseHint returns a prompt to use /line-rebase if the terminal station branch
// has commits ahead of the root watched branch. Returns "" if not applicable.
func rebaseHint(data StatuslineOutput) string {
	if !data.HasUnpickedCommits {
		return ""
	}
	return fmt.Sprintf("\033[1;33m⚠ use /line-rebase to pick up latest changes%s", ansiReset)
}
