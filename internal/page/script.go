package page

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Script returns a script of floe's for a page: js, a function written whole, called with arg as its JSON. The lines
// of js that are a // comment alone are left out, so that its notes do not go into every page. json.Marshal writes
// <, > and & as \u escapes, so that no text of arg ends the script.
// LIMIT: a line of a template literal that starts with // is left out too; floe's scripts write none.
func Script(js string, arg any) (string, error) {
	b, err := json.Marshal(arg)
	if err != nil {
		return "", fmt.Errorf("page: write the argument of a script: %w", err)
	}
	var code strings.Builder
	for line := range strings.Lines(js) {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			code.WriteString(line)
		}
	}
	return "<script>(" + strings.TrimSpace(code.String()) + ")(" + string(b) + ");</script>", nil
}
