package skill

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed skills
var skillsFS embed.FS

var skillNames = []string{"line-rebase", "line-preview"}

// Remove removes assembly-line skill directories from .claude/skills.
func Remove(repoDir string) error {
	for _, name := range skillNames {
		dir := filepath.Join(repoDir, ".claude", "skills", name)
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("removing skill %s: %w", name, err)
		}
	}
	return nil
}

// Install installs assembly-line skills into the .claude/skills directory.
func Install(repoDir string) error {
	for _, name := range skillNames {
		dest := filepath.Join(repoDir, ".claude", "skills", name)
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return fmt.Errorf("creating skill dir %s: %w", name, err)
		}

		data, err := skillsFS.ReadFile(filepath.Join("skills", name, "SKILL.md"))
		if err != nil {
			return fmt.Errorf("reading embedded skill %s: %w", name, err)
		}

		if err := os.WriteFile(filepath.Join(dest, "SKILL.md"), data, 0o644); err != nil {
			return fmt.Errorf("writing skill file %s: %w", name, err)
		}
	}

	return nil
}
