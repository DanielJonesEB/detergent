package env

import (
	"os"
	"strings"
)

// FilterByPrefixes returns a copy of os.Environ() with variables matching any of the
// given prefixes removed. Prefixes should include the '=' suffix for exact matching
// (e.g., "CLAUDECODE=", not "CLAUDECODE").
func FilterByPrefixes(excludePrefixes ...string) []string {
	result := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		skip := false
		for _, prefix := range excludePrefixes {
			if strings.HasPrefix(e, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			result = append(result, e)
		}
	}
	return result
}
