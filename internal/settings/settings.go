package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// writeSettings marshals settings to JSON and writes it to settingsPath.
func writeSettings(settingsPath string, settings map[string]any) error {
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	if err := os.WriteFile(settingsPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}
	return nil
}

// RemoveStatusline removes the statusLine key from .claude/settings.json.
func RemoveStatusline(repoDir string) error {
	settingsPath := filepath.Join(repoDir, ".claude", "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading settings: %w", err)
	}

	settings := make(map[string]any)
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parsing settings: %w", err)
	}

	if _, ok := settings["statusLine"]; !ok {
		return nil
	}
	delete(settings, "statusLine")

	return writeSettings(settingsPath, settings)
}

// ConfigureStatusline sets the Claude Code statusline to use line statusline.
func ConfigureStatusline(repoDir string) error {
	settingsDir := filepath.Join(repoDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		return fmt.Errorf("creating .claude dir: %w", err)
	}

	settingsPath := filepath.Join(settingsDir, "settings.json")

	// Read existing settings if present
	settings := make(map[string]any)
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing existing settings: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading settings: %w", err)
	}

	settings["statusLine"] = map[string]any{
		"type":    "command",
		"command": "line statusline",
	}

	return writeSettings(settingsPath, settings)
}
