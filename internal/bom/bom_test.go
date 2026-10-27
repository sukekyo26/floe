package bom_test

import (
	"fmt"
	"strings"
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

// Refuse tells each mark at its line and column, and nothing for a src of none.
func TestRefuse(t *testing.T) {
	t.Parallel()

	const mark = "\xef\xbb\xbf"
	for _, tt := range []struct{ src, want string }{
		{"a: 1\n", ""},
		{mark + "a", "1:1"},
		{"a: " + mark + "x\n", "1:4"},
		{"a\nb" + mark, "2:2"},
		{mark + mark, "1:1 1:2"},
		{"\xef\xbb", ""},
	} {
		found := bom.Refuse("f", []byte(tt.src))
		got := make([]string, 0, len(found))
		for _, e := range found {
			if e.Key != "text_bom" || e.File != "f" {
				t.Errorf("Refuse(%q) = %+v, want text_bom at f", tt.src, e)
			}
			got = append(got, fmt.Sprintf("%d:%d", e.Line, e.Column))
		}
		if strings.Join(got, " ") != tt.want {
			t.Errorf("Refuse(%q) at %v, want %s", tt.src, got, tt.want)
		}
	}
}
