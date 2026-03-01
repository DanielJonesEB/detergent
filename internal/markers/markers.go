package markers

import (
	"fmt"
	"strings"
)

const (
	Start = "# >>> assembly-line >>>"
	End   = "# <<< assembly-line <<<"
)

var errMissingEnd = fmt.Errorf("found start marker but no end marker")

// Insert inserts or replaces a marked block in content.
// If markers exist, the block between them is replaced.
// If content is empty and prefix is non-empty, the result is prefix+"\n\n"+block+"\n".
// If content is empty and prefix is empty, the result is block+"\n".
// Otherwise the block is appended (with a preceding blank line).
func Insert(content, block, prefix string) (string, error) {
	if strings.Contains(content, Start) {
		start := strings.Index(content, Start)
		end := strings.Index(content, End)
		if end == -1 {
			return "", errMissingEnd
		}
		end += len(End)
		return content[:start] + block + content[end:], nil
	}
	if content == "" {
		if prefix != "" {
			return prefix + "\n\n" + block + "\n", nil
		}
		return block + "\n", nil
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + "\n" + block + "\n", nil
}
