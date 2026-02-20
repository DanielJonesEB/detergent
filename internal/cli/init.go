package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/re-cinq/assembly-line/internal/assets"
	"github.com/re-cinq/assembly-line/internal/config"
	"github.com/re-cinq/assembly-line/internal/fileutil"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Initialize line skills and statusline in a repository",
	Long: `Initialize line Claude Code skills and statusline configuration
in the target repository (defaults to current directory).

This command:
  - Copies skill files into .claude/skills/
  - Configures the Claude Code statusline in .claude/settings.local.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("resolving path: %w", err)
		}

		// Verify it's a git repo
		if _, err := os.Stat(filepath.Join(absDir, ".git")); err != nil {
			return fmt.Errorf("%s is not a git repository (no .git directory)", absDir)
		}

		installed, err := initSkills(absDir)
		if err != nil {
			return fmt.Errorf("installing skills: %w", err)
		}
		for _, path := range installed {
			fmt.Printf("  skill  %s\n", path)
		}

		if err := initStatusline(absDir); err != nil {
			return fmt.Errorf("configuring statusline: %w", err)
		}
		fmt.Println("  config .claude/settings.local.json (statusline)")

		// Install hooks based on config
		if cfg, err := config.Load(configPath); err == nil {
			if len(cfg.Gates) > 0 {
				if err := initPreCommitHook(absDir); err != nil {
					return fmt.Errorf("installing pre-commit hook: %w", err)
				}
			}
			if len(cfg.Concerns) > 0 {
				if err := initPostCommitHook(absDir); err != nil {
					return fmt.Errorf("installing post-commit hook: %w", err)
				}
			}
		}

		fmt.Println("\nDone.")
		return nil
	},
}

// initSkills copies all embedded skill files into .claude/skills/.
func initSkills(repoDir string) ([]string, error) {
	var installed []string

	err := fs.WalkDir(assets.Skills, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Target path: .claude/<path> (skills/line-rebase/SKILL.md -> .claude/skills/line-rebase/SKILL.md)
		target := fileutil.ClaudeSubpath(repoDir, path)

		if d.IsDir() {
			return fileutil.EnsureDir(target)
		}

		data, err := assets.Skills.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", path, err)
		}

		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", target, err)
		}

		rel, err := filepath.Rel(repoDir, target)
		if err != nil {
			return fmt.Errorf("computing relative path for %s: %w", target, err)
		}
		installed = append(installed, rel)
		return nil
	})

	return installed, err
}

// initStatusline adds or updates the statusline config in .claude/settings.local.json.
func initStatusline(repoDir string) error {
	lineBin, err := os.Executable()
	if err != nil {
		// Fall back to expecting it in PATH
		lineBin = "line"
	}

	settingsPath := fileutil.ClaudeSubpath(repoDir, "settings.local.json")

	// Ensure .claude/ exists
	if err := fileutil.EnsureDir(fileutil.ClaudeDir(repoDir)); err != nil {
		return err
	}

	// Load existing settings or start fresh
	settings := make(map[string]interface{})
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing existing %s: %w", settingsPath, err)
		}
	}

	// Set statusline
	settings["statusLine"] = map[string]string{
		"command": lineBin + " statusline",
		"type":    "command",
	}

	if err := fileutil.WriteJSON(settingsPath, settings); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}
	return nil
}

const (
	gateBeginMarker = "# BEGIN line gate"
	gateBlock       = `# BEGIN line gate
if command -v line >/dev/null 2>&1; then
    line gate || exit 1
fi
# END line gate`
)

// initPreCommitHook installs or injects a `line gate` call into .git/hooks/pre-commit.
// If no hook exists, a fresh one is created. If one exists, the gate block is injected
// using sentinel markers. Re-running is idempotent: the sentinel is detected and skipped.
func initPreCommitHook(repoDir string) error {
	hookDir := filepath.Join(repoDir, ".git", "hooks")
	hookPath := filepath.Join(hookDir, "pre-commit")

	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return fmt.Errorf("creating hooks directory: %w", err)
	}

	// Check for existing hook
	existing, err := os.ReadFile(hookPath)
	if err == nil {
		return injectGateBlock(hookPath, string(existing))
	}

	// No existing hook — write a fresh one
	content := "#!/bin/sh\n" + gateBlock + "\n"
	if err := os.WriteFile(hookPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("writing pre-commit hook: %w", err)
	}

	fmt.Println("  hook   .git/hooks/pre-commit")
	return nil
}

// injectGateBlock injects the gate block into an existing hook script.
// If the sentinel markers are already present, it's a no-op.
func injectGateBlock(hookPath, content string) error {
	if strings.Contains(content, gateBeginMarker) {
		fmt.Println("  skip   .git/hooks/pre-commit (line gate already present)")
		return nil
	}

	// Insert before the last "exit 0" if present, otherwise append
	var updated string
	if idx := strings.LastIndex(content, "\nexit 0"); idx != -1 {
		// Inject before the final "exit 0", preserving surrounding newlines
		updated = content[:idx] + "\n" + gateBlock + "\n" + content[idx+1:]
	} else {
		// Append to end, ensuring a newline separator
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		updated = content + "\n" + gateBlock + "\n"
	}

	if err := os.WriteFile(hookPath, []byte(updated), 0o755); err != nil {
		return fmt.Errorf("writing pre-commit hook: %w", err)
	}

	fmt.Println("  hook   .git/hooks/pre-commit (injected line gate)")
	return nil
}

const (
	runnerBeginMarker = "# BEGIN line runner"
	runnerBlock       = `# BEGIN line runner
if command -v line >/dev/null 2>&1; then
    line trigger >/dev/null 2>&1
fi
# END line runner`
)

// initPostCommitHook installs or injects a `line trigger` call into .git/hooks/post-commit.
// If no hook exists, a fresh one is created. If one exists, the runner block is injected
// using sentinel markers. Re-running is idempotent: the sentinel is detected and skipped.
func initPostCommitHook(repoDir string) error {
	hookDir := filepath.Join(repoDir, ".git", "hooks")
	hookPath := filepath.Join(hookDir, "post-commit")

	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return fmt.Errorf("creating hooks directory: %w", err)
	}

	// Check for existing hook
	existing, err := os.ReadFile(hookPath)
	if err == nil {
		return injectRunnerBlock(hookPath, string(existing))
	}

	// No existing hook — write a fresh one
	content := "#!/bin/sh\n" + runnerBlock + "\n"
	if err := os.WriteFile(hookPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("writing post-commit hook: %w", err)
	}

	fmt.Println("  hook   .git/hooks/post-commit")
	return nil
}

// injectRunnerBlock injects the runner block into an existing hook script.
// If the sentinel markers are already present, it's a no-op.
func injectRunnerBlock(hookPath, content string) error {
	if strings.Contains(content, runnerBeginMarker) {
		fmt.Println("  skip   .git/hooks/post-commit (line runner already present)")
		return nil
	}

	// Append to end, ensuring a newline separator
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	updated := content + "\n" + runnerBlock + "\n"

	if err := os.WriteFile(hookPath, []byte(updated), 0o755); err != nil {
		return fmt.Errorf("writing post-commit hook: %w", err)
	}

	fmt.Println("  hook   .git/hooks/post-commit (injected line runner)")
	return nil
}
