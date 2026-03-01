package gitignore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/re-cinq/assembly-line/internal/markers"
)

func block() string {
	return fmt.Sprintf(`%s
/.line/
%s`, markers.Start, markers.End)
}

// Remove removes the assembly-line block from .gitignore.
func Remove(repoDir string) error {
	path := filepath.Join(repoDir, ".gitignore")

	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	content := string(existing)
	if !strings.Contains(content, markers.Start) {
		return nil
	}

	start := strings.Index(content, markers.Start)
	end := strings.Index(content, markers.End)
	if end == -1 {
		return fmt.Errorf(".gitignore: found start marker but no end marker")
	}
	end += len(markers.End)

	before := content[:start]
	after := content[end:]
	after = strings.TrimPrefix(after, "\n")
	result := strings.TrimRight(before, "\n")
	if result != "" && after != "" {
		result += "\n"
	}
	result += after
	if result != "" && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	if err := os.WriteFile(path, []byte(result), 0o644); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}

	return nil
}

// Install adds the assembly-line gitignore entries to .gitignore in the given repo,
// creating the file if needed. The block is idempotent: if markers already
// exist, the content between them is replaced.
func Install(repoDir string) error {
	path := filepath.Join(repoDir, ".gitignore")

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	content, err := markers.Insert(string(existing), block(), "")
	if err != nil {
		return fmt.Errorf(".gitignore: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}

	return nil
}
