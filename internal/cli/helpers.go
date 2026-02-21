package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/re-cinq/assembly-line/internal/config"
	"github.com/re-cinq/assembly-line/internal/fileutil"
)

// loadAndValidateConfig loads a config file and validates it, printing errors to stderr.
func loadAndValidateConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		fileutil.LogError("Error: %s", err)
		return nil, err
	}

	errs := config.Validate(cfg)
	if len(errs) > 0 {
		for _, e := range errs {
			fileutil.LogError("Error: %s", e)
		}
		return nil, fmt.Errorf("%d validation error(s)", len(errs))
	}

	return cfg, nil
}

// loadConfigAndRepo loads and validates a config file and resolves the repository root.
// This consolidates the common pattern used across most CLI commands.
// It also ensures core.bare is set to false to prevent corruption from git/VS Code race conditions.
func loadConfigAndRepo(configPath string) (*config.Config, string, error) {
	cfg, err := loadAndValidateConfig(configPath)
	if err != nil {
		return nil, "", err
	}
	repoDir, err := resolveRepo(configPath)
	if err != nil {
		return nil, "", err
	}
	// Ensure core.bare is false — auto-repair if corrupted by race conditions
	ensureCoreBareIsFalse(repoDir)
	return cfg, repoDir, nil
}

// ensureCoreBareIsFalse checks if core.bare is set to true and repairs it if needed.
// This guards against race conditions between VS Code's git extension and the line runner
// that can corrupt the git config by setting core.bare=true.
func ensureCoreBareIsFalse(repoDir string) {
	cmd := exec.Command("git", "config", "core.bare")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return // config not set or error reading it
	}
	if strings.TrimSpace(string(out)) == "true" {
		// Auto-repair the corruption
		repairCmd := exec.Command("git", "config", "core.bare", "false")
		repairCmd.Dir = repoDir
		if err := repairCmd.Run(); err != nil {
			fileutil.LogError("warning: failed to repair core.bare=true: %s", err)
		} else {
			fileutil.LogError("info: repaired corrupted core.bare=true setting")
		}
	}
}

// resolveRepo finds the git repository root from a config file path.
func resolveRepo(configArg string) (string, error) {
	configPath, err := filepath.Abs(configArg)
	if err != nil {
		return "", err
	}
	repoDir := findGitRoot(filepath.Dir(configPath))
	if repoDir == "" {
		return "", fmt.Errorf("could not find git repository root")
	}
	return repoDir, nil
}

// findGitRoot walks up from dir looking for a .git directory.
func findGitRoot(dir string) string {
	return walkUpUntil(dir, func(d string) bool {
		_, err := os.Stat(filepath.Join(d, ".git"))
		return err == nil
	})
}

// walkUpUntil walks up the directory tree from dir, calling check on each directory.
// Returns the first directory where check returns true, or "" if none found.
func walkUpUntil(dir string, check func(string) bool) string {
	for {
		if check(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// findFileUp walks up from dir looking for any of the given filenames.
// Returns the full path to the first file found, or "" if none found.
func findFileUp(dir string, filenames []string) string {
	foundDir := walkUpUntil(dir, func(d string) bool {
		for _, name := range filenames {
			if _, err := os.Stat(filepath.Join(d, name)); err == nil {
				return true
			}
		}
		return false
	})
	if foundDir == "" {
		return ""
	}
	// Return the full path to the first file that exists
	for _, name := range filenames {
		p := filepath.Join(foundDir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// setupSignalHandler creates a signal channel and registers handlers for SIGINT and SIGTERM.
func setupSignalHandler() chan os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	return sigCh
}
