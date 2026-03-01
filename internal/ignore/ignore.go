package ignore

import (
	"os"
	"path/filepath"

	gitignore "github.com/sabhiram/go-gitignore"
)

const ignoreFile = ".lineignore"

// Matcher checks files against .lineignore patterns.
type Matcher struct {
	gi *gitignore.GitIgnore
}

// Load loads .lineignore from the given directory.
// Returns a Matcher that matches nothing if no .lineignore exists.
func Load(dir string) (*Matcher, error) {
	path := filepath.Join(dir, ignoreFile)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Matcher{}, nil
	}

	gi, err := gitignore.CompileIgnoreFile(path)
	if err != nil {
		return nil, err
	}
	return &Matcher{gi: gi}, nil
}

// AllIgnored returns true if all given file paths match the ignore patterns.
func (m *Matcher) AllIgnored(files []string) bool {
	if m.gi == nil {
		return false
	}
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !m.gi.MatchesPath(f) {
			return false
		}
	}
	return true
}
