package textpos_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/textpos"
)

// Of counts LF, CRLF and CR as one line break each, and runes for the column, a byte no UTF-8 as one.
func TestOf(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		src          string
		at           int
		line, column int
	}{
		{"abc", 0, 1, 1},
		{"abc", 3, 1, 4},
		{"a\nb", 2, 2, 1},
		{"a\r\nb", 3, 2, 1},
		{"a\r\nb", 2, 1, 3}, // the LF of a CRLF is still on its line
		{"a\rb", 2, 2, 1},
		{"é x", 3, 1, 3},
		{"a\fb", 2, 1, 3},
		{"日本x", 6, 1, 3},
		{"a\nbc", 3, 2, 2},
		{"a\r\n\rb\nc", 6, 4, 1},
		{"\tx", 1, 1, 2},
		{"\xff\xfex", 2, 1, 3},
		{"ab\r", 3, 2, 1},
	} {
		if line, column := textpos.Of([]byte(tt.src), tt.at); line != tt.line || column != tt.column {
			t.Errorf("Of(%q, %d) = %d:%d, want %d:%d", tt.src, tt.at, line, column, tt.line, tt.column)
		}
	}
}

// Ending places the character that ends at an offset, one inside a character at its start.
func TestEnding(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		src          string
		at           int
		line, column int
	}{
		{"{", 1, 1, 1},
		{"ab", 0, 1, 1},
		{"a\n日x", 3, 2, 1}, // inside 日
		{"a\n日x", 5, 2, 1},
		{"a\n日x", 6, 2, 2},
	} {
		if line, column := textpos.Ending([]byte(tt.src), tt.at); line != tt.line || column != tt.column {
			t.Errorf("Ending(%q, %d) = %d:%d, want %d:%d", tt.src, tt.at, line, column, tt.line, tt.column)
		}
	}
}
