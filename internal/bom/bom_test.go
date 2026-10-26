package bom_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/bom"
)

// Trim leaves out one mark at the start, and changes no other byte.
func TestTrim(t *testing.T) {
	t.Parallel()

	const mark = "\xef\xbb\xbf"
	for _, tt := range []struct{ src, want string }{
		{mark + "{}", "{}"},
		{mark + mark + "a", mark + "a"},
		{"a", "a"},
		{"", ""},
		{mark, ""},
		{"a" + mark, "a" + mark},
		{"\xff\xfea", "\xff\xfea"},
		{"\xef\xbb", "\xef\xbb"},
	} {
		if got := string(bom.Trim([]byte(tt.src))); got != tt.want {
			t.Errorf("Trim(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}
}
