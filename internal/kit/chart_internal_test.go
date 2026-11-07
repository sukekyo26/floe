package kit

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func lineOf(lowest any, values ...any) map[string]any {
	xs := make([]any, len(values))
	for i := range values {
		xs[i] = fmt.Sprintf("x%d", i)
	}
	m := map[string]any{"kind": "line", "title": "t", "x": xs, "series": []any{map[string]any{"name": "s", "values": values}}}
	if lowest != nil {
		m["min"] = lowest
	}
	return m
}

func ticksOf(t *testing.T, v map[string]any) []string {
	t.Helper()
	c, err := chartOf(v)
	if err != nil {
		t.Fatal(err)
	}
	words := make([]string, 0, len(c.Ticks))
	for _, k := range c.Ticks {
		words = append(words, k.Text)
	}
	return words
}

// The ticks stand at steps of 1, 2 or 5 times a power of 10, from 0 or from a min no value is below, and whole values
// and their rounding are fixed; values close past a float's or the words' reach draw no words, and end.
func TestChartScale(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		lowest any
		values []any
		want   []string
	}{
		{"from 0 by 50", nil, []any{0, 231}, []string{"0", "50", "100", "150", "200", "250"}},
		{"a min", 90, []any{120, 141}, []string{"90", "110", "130", "150"}},
		{"all 0", nil, []any{0, 0}, []string{"0", "0.2", "0.4", "0.6", "0.8", "1"}},
		{"one 0", nil, []any{0}, []string{"0", "0.2", "0.4", "0.6", "0.8", "1"}},
		{"below 0", nil, []any{-3}, []string{"-3", "-2", "-1", "0"}},
		{"a value below min", 5, []any{10, -3}, []string{"-5", "0", "5", "10"}},
		{"every value below min", 100, []any{1, 2}, []string{"0", "0.5", "1", "1.5", "2"}},
		{"values at min", 5, []any{5, 5}, []string{"5", "6"}},
		{"a fraction", nil, []any{0.1, 0.3}, []string{"0", "0.1", "0.2", "0.3"}},
		{"past 9 digits", nil, []any{uint64(18446744073709551615), uint64(9007199254740993)}, []string{"0", "5e+18", "1e+19", "1.5e+19", "2e+19"}},
		{
			"as many digits as tell them apart", 1230000000000,
			[]any{1230000000000, 1231000000000},
			[]string{"1.23e+12", "1.2302e+12", "1.2304e+12", "1.2306e+12", "1.2308e+12", "1.231e+12"},
		},
		{"a min off the step", 1.5, []any{2, 10}, []string{"1.5", "3.5", "5.5", "7.5", "9.5", "11.5"}},
		{"a min off the step past 0", 90.5, []any{120, 141}, []string{"90.5", "110.5", "130.5", "150.5"}},
		{
			"past 9 digits, off the step", 1234567890123,
			[]any{1234567890123, 2234567890123},
			[]string{"1.235e+12", "1.435e+12", "1.635e+12", "1.835e+12", "2.035e+12", "2.235e+12"},
		},
		{"13 digits close", 1730000000000, []any{1730000000000, 1730000000900}, nil},
		{"past a float's step", 1e15, []any{1e15 + 0.125, 1e15 + 0.25}, nil},
		{"past 2^53", uint64(9007199254740993), []any{uint64(9007199254740992), uint64(9007199254740994)}, nil},
	} {
		got := ticksOf(t, lineOf(tt.lowest, tt.values...))
		if tt.want == nil {
			if strings.Join(got, "") != "" || len(got) < 2 {
				t.Errorf("%s: ticks %q, want at least 2 with no words", tt.name, got)
			}
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: ticks %q, want %q", tt.name, got, tt.want)
		}
	}
}

// A scale across 0 has a line for it, though no tick falls on it, and a word only where one does.
func TestChartZeroLine(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		lowest any
		values []any
		word   string
	}{
		{-0.3, []any{1, 2}, ""},
		{-1, []any{1, 2}, "0"},
		{nil, []any{-1, 2}, "0"},
	} {
		c, err := chartOf(lineOf(tt.lowest, tt.values...))
		if err != nil {
			t.Fatal(err)
		}
		var zero []chartTick
		for _, k := range c.Ticks {
			if k.Zero {
				zero = append(zero, k)
			}
		}
		if len(zero) != 1 || zero[0].Text != tt.word {
			t.Errorf("min %v, %v: the lines of 0 are %+v, want one with %q", tt.lowest, tt.values, zero, tt.word)
		}
	}
}

// Values at a float's ends count no NaN or Inf and stay in the plot, on both kinds, whatever their ticks say.
func TestChartScaleEnds(t *testing.T) {
	t.Parallel()

	for _, values := range [][]any{
		{1.7e308},
		{1.7e308, -1.7e308},
		{5e-324},
		{5e-324, 0.0},
		{-5e-324, 5e-324},
		{1e15, 1e15 + 0.125},
		{1.7976931348623157e308},
		{0.0},
		{-3.0},
	} {
		for _, kind := range []string{"line", "bar"} {
			m := lineOf(nil, values...)
			m["kind"] = kind
			c, err := chartOf(m)
			if err != nil {
				t.Errorf("%v: %v", values, err)
				continue
			}
			for _, s := range c.Series {
				if text := fmt.Sprintf("%+v %+v %+v", c.Ticks, s.Points, s.Bars); strings.Contains(text, "NaN") || strings.Contains(text, "Inf") {
					t.Errorf("%s %v: %s", kind, values, text)
				}
				for _, p := range s.Points {
					if p.Y < 0 || p.Y > 100 {
						t.Errorf("%s %v: a point at %v", kind, values, p.Y)
					}
				}
				for _, b := range s.Bars {
					if b.Y < 0 || b.Y+b.H > 100.01 {
						t.Errorf("%s %v: a bar from %v by %v", kind, values, b.Y, b.H)
					}
				}
			}
		}
	}
}

// A number's label is as num writes it up to 9 digits, counted without its grouping, sign and point, and shorter past them.
func TestChartNumbers(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		value any
		want  string
	}{
		{123456789, "123,456,789"},
		{123499999, "123,499,999"},
		{1234567890, "1.23e+09"},
		{0.3333333333, "0.333"},
		{12345.6789012, "12,346"},
		{1.23456789012, "1.23"},
		{123456789.5, "123,456,790"},
		{0.000000000123, "1.23e-10"},
		{-987654321, "-987,654,321"},
		{uint64(18446744073709551615), "1.84e+19"},
		{12.5, "12.5"},
		{1234567.5, "1,234,567.5"},
	} {
		c, err := chartOf(lineOf(nil, tt.value))
		if err != nil {
			t.Fatal(err)
		}
		if got := c.Series[0].End.Value; got != tt.want {
			t.Errorf("%v: %q, want %q", tt.value, got, tt.want)
		}
	}
}

// Close lines' labels part, long names leave the plot's right, a long x is cut, and x and the bars' numbers thin out by
// the room each width leaves.
func TestChartLabels(t *testing.T) {
	t.Parallel()

	two := func(a, b string, av, bv any) map[string]any {
		return map[string]any{"kind": "line", "title": "t", "x": []any{"p", "q"}, "series": []any{
			map[string]any{"name": a, "values": []any{1, av}}, map[string]any{"name": b, "values": []any{1, bv}},
		}}
	}
	c, err := chartOf(two("a", "b", 100, 100))
	if err != nil {
		t.Fatal(err)
	}
	if gap := math.Abs(c.Series[1].End.Y - c.Series[0].End.Y); gap < labelHeight/plotHeight*100-0.01 {
		t.Errorf("the labels of close lines stand %v apart", gap)
	}
	if !c.Names || c.Right <= c.RightNarrow {
		t.Errorf("short names: Names %v, Right %v, RightNarrow %v", c.Names, c.Right, c.RightNarrow)
	}
	c, err = chartOf(two(strings.Repeat("long ", 20), "b", 1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if c.Names || c.Right != c.RightNarrow {
		t.Errorf("long names: Names %v, Right %v, RightNarrow %v", c.Names, c.Right, c.RightNarrow)
	}

	long := strings.Repeat("a", 60)
	c, err = chartOf(map[string]any{"kind": "line", "title": "t", "x": []any{long}, "series": []any{map[string]any{"name": "s", "values": []any{1}}}})
	if err != nil {
		t.Fatal(err)
	}
	if text := c.Labels[0].Text; !strings.HasSuffix(text, "…") || labelWidth(strings.TrimSuffix(text, "…")) > 8 {
		t.Errorf("a long x is drawn %q", text)
	}
	if c.Rows[0].Name != long {
		t.Errorf("the table holds %q", c.Rows[0].Name)
	}

	xs, values := make([]any, 30), make([]any, 30)
	for i := range xs {
		xs[i], values[i] = fmt.Sprintf("11-%02d", i+1), i+1
	}
	series := func(n int) []any {
		list := make([]any, n)
		for i := range list {
			list[i] = map[string]any{"name": fmt.Sprintf("s%d", i), "values": values}
		}
		return list
	}
	c, err = chartOf(map[string]any{"kind": "line", "title": "t", "x": xs, "series": series(1)})
	if err != nil {
		t.Fatal(err)
	}
	narrow := 0
	for _, l := range c.Labels {
		if !l.Wide {
			narrow++
		}
	}
	if !c.Dense || len(c.Labels) == 30 || narrow < 2 || narrow == len(c.Labels) {
		t.Errorf("30 of x: Dense %v, %d labels, %d of them at a narrow box", c.Dense, len(c.Labels), narrow)
	}
	c, err = chartOf(map[string]any{"kind": "bar", "title": "t", "x": xs, "series": series(4)})
	if err != nil {
		t.Fatal(err)
	}
	if !c.Thin {
		t.Error("bars of 30 of x by 4 series are not thin")
	}
	c, err = chartOf(map[string]any{"kind": "bar", "title": "t", "x": []any{"a", "b"}, "series": []any{map[string]any{"name": "s", "values": []any{1, 2}}}})
	if err != nil {
		t.Fatal(err)
	}
	if c.Thin || c.Series[0].Bars[0].Text != "1" {
		t.Errorf("2 bars: Thin %v, number %q", c.Thin, c.Series[0].Bars[0].Text)
	}
}
