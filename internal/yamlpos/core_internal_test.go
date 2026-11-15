package yamlpos

import (
	"strconv"
	"testing"
)

// read12 reads a plain scalar by the rows of the Core Schema, each matching a whole value.
func TestRead12(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		text  string
		kind  kind
		value string // reading.shown()
	}{
		{"017", kindInt, "17"},
		{"-017", kindInt, "-17"},
		{"+1", kindInt, "1"},
		{"00", kindInt, "0"},
		{"0o17", kindInt, "15"},
		{"0x1F", kindInt, "31"},
		{"0x1f", kindInt, "31"},
		{"1e3", kindFloat, "1000"},
		{".5", kindFloat, "0.5"},
		{"1.", kindFloat, "1"},
		{"-.5e-1", kindFloat, "-0.05"},
		{"+.inf", kindFloat, "+Inf"},
		{"-.Inf", kindFloat, "-Inf"},
		{".NAN", kindFloat, "NaN"},
		{"1e400", kindFloat, "+Inf"},
		{"1e-400", kindFloat, "0"},
		// No row: text. A row matches the whole value, not a part of it.
		{"0b101", kindText, ""},
		{"1_000", kindText, ""},
		{"-0x1F", kindText, ""},
		{"-0o17", kindText, ""},
		{"+-1", kindText, ""},
		{"+.nan", kindText, ""},
		{".", kindText, ""},
		{"1e", kindText, ""},
		{"2026 x", kindText, ""},
		{"Plan 2026", kindText, ""},
		{"1.2.3", kindText, ""},
	} {
		r := read12(tt.text)
		got := ""
		switch r.kind {
		case kindInt:
			got = r.i.String()
		case kindFloat:
			got = strconv.FormatFloat(r.f, 'g', -1, 64)
		case kindText, kindNull, kindTrue, kindFalse:
		}
		if r.kind != tt.kind || got != tt.value {
			t.Errorf("read12(%q) = %v %q, want %v %q", tt.text, r.kind, got, tt.kind, tt.value)
		}
	}
}
