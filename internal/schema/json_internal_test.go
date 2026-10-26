package schema

import (
	"testing"

	"github.com/sukekyo26/floe/internal/yamlpos"
)

// jsonPlace counts lines at \n, \r and \r\n, each once, and columns in characters, a byte no UTF-8 as one.
func TestJSONPlace(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		src  string
		at   int
		want yamlpos.Pos
	}{
		{"abc", 2, yamlpos.Pos{Line: 1, Column: 3}},
		{"日本x", 6, yamlpos.Pos{Line: 1, Column: 3}},
		{"a\nbc", 3, yamlpos.Pos{Line: 2, Column: 2}},
		{"a\r\nbc", 4, yamlpos.Pos{Line: 2, Column: 2}},
		{"a\rbc", 3, yamlpos.Pos{Line: 2, Column: 2}},
		{"a\r\n\rb\nc", 6, yamlpos.Pos{Line: 4, Column: 1}},
		{"\tx", 1, yamlpos.Pos{Line: 1, Column: 2}},
		{"\xff\xfex", 2, yamlpos.Pos{Line: 1, Column: 3}},
		{"ab\r", 3, yamlpos.Pos{Line: 2, Column: 1}},
	} {
		if got := jsonPlace([]byte(tt.src), tt.at); got != tt.want {
			t.Errorf("jsonPlace(%q, %d) = %v, want %v", tt.src, tt.at, got, tt.want)
		}
	}
}
