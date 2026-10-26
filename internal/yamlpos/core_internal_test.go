package yamlpos

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"testing"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/sukekyo26/floe/internal/usererr"
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
		if r.kind != kindText {
			got = r.shown()
		}
		if r.kind != tt.kind || got != tt.value {
			t.Errorf("read12(%q) = %v %q, want %v %q", tt.text, r.kind, got, tt.kind, tt.value)
		}
	}
}

// forms are number-like values, signed and not.
func forms() []string {
	bodies := []string{
		"0", "00", "017", "08", "0o17", "0x1F", "0b101", "1_000", "1_0.0", "1_0.5", "1__0", "0x_1F", "1e3", "1E3",
		"1e+3", "1.5e3", ".5", "1.", "012.5", "012e3", "1.0e+20", "1e400", "1e-400", ".inf", ".nan", "0.0", "1e",
		"99999999999999999999", "030000000000000000000", "18446744073709551615", "9223372036854775808",
		"1_000_000_000_000_000_000_000.0", "0x8000000000000000",
	}
	all := make([]string, 0, 3*len(bodies))
	for _, sign := range []string{"", "+", "-"} {
		for _, b := range bodies {
			all = append(all, sign+b)
		}
	}
	return all
}

func valueOf(t *testing.T, text string) ast.Node {
	t.Helper()
	f, err := parser.ParseBytes([]byte("x: "+text+"\n"), 0)
	if err != nil {
		t.Fatalf("parse %q: %v", text, err)
	}
	m, ok := f.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		t.Fatalf("parse %q: no mapping", text)
	}
	return m.Values[0].Value
}

// Each way of writing a mistake tells reads alike in goccy/go-yaml and YAML 1.2, as the number it names; and
// the two read different numbers only for a 0 at the start, which yaml_number_octal words.
func TestNumberMistakeTellsWhatReadsAlike(t *testing.T) {
	t.Parallel()

	alike := func(written string, as reading) {
		t.Helper()
		n := valueOf(t, written)
		want, ok := plainNumber(n)
		if !ok || numberMistake("a.yaml", n, want, goccy(n)) != nil || !want.same(as) {
			t.Errorf("%q, told as %s, is not read alike as it", written, as.shown())
		}
	}
	leadingZero := regexp.MustCompile(`^[-+]?0[0-9]`)
	for _, text := range forms() {
		n := valueOf(t, text)
		want, ok := plainNumber(n)
		if !ok {
			t.Fatalf("%q: not a plain number-like value", text)
		}
		got := goccy(n)
		err := numberMistake("a.yaml", n, want, got)
		if err == nil {
			continue
		}
		var mistake *usererr.Error
		if !errors.As(err, &mistake) {
			t.Fatalf("%q: %v", text, err)
		}
		switch mistake.Key {
		case "yaml_number_not_number":
			alike(fmt.Sprint(mistake.Args[1]), got)
		case "yaml_number_not_text":
			alike(fmt.Sprint(mistake.Args[2]), want)
		case "yaml_number_octal":
			if !leadingZero.MatchString(text) {
				t.Errorf("%q: read as two numbers without a 0 at its start", text)
			}
			alike(fmt.Sprint(mistake.Args[2]), want)
			alike(fmt.Sprint(mistake.Args[3]), got)
		case "yaml_number_too_big", "yaml_number_outside_float", "yaml_not_finite":
		default:
			t.Errorf("%q: %s", text, mistake.Key)
		}
	}
	// What YAML 1.2 cannot hold is told so, not as another reading.
	for text, key := range map[string]string{
		"030000000000000000000": "yaml_number_too_big", "1e400": "yaml_number_outside_float",
		"1e-400": "yaml_number_outside_float", ".nan": "yaml_not_finite",
	} {
		n := valueOf(t, text)
		want, _ := plainNumber(n)
		var m *usererr.Error
		if err := numberMistake("a.yaml", n, want, goccy(n)); !errors.As(err, &m) || m.Key != key {
			t.Errorf("%q told %v, want %s", text, err, key)
		}
	}
	// Two numbers read otherwise but as octal is floe's failure, not a mistake worded for octal.
	err := numberMistake("a.yaml", valueOf(t, "17"), read12("17"), reading{kind: kindFloat, i: nil, f: 1.5, outside: false})
	if err == nil || errors.As(err, new(*usererr.Error)) {
		t.Error("an integer read as another float is told as a mistake")
	}
	if !math.IsNaN(read12(".nan").f) {
		t.Error(".nan is not NaN")
	}
}
