package rules_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/rules"
)

// names are the items the schema of the tests declares at its top.
var names = []string{"a", "b", "count", "items", "name", "opt", "records", "my-key"}

// compute reads the computed.yaml src and computes it from the data body: its values, name=value sorted, and
// what it tells.
func compute(t *testing.T, src, body string) (string, string) {
	t.Helper()

	c, err := rules.ReadComputed("t1/computed.yaml", []byte(src), names)
	if err != nil {
		return "", told(err)
	}
	values, err := c.Run(data(t, body))
	var shown []string
	for _, n := range slices.Sorted(maps.Keys(values)) {
		shown = append(shown, fmt.Sprintf("%s=%v", n, values[n]))
	}
	return strings.Join(shown, " "), told(err)
}

const (
	notName = " is not a name an expression or page.html can use: write a letter, then letters, digits and _, and not a word or function of expr"
	cycle   = "the values are computed from each other: "
)

// ReadComputed tells the mistakes of its shape and names, of the expressions whose names are right, and the
// cycles among those that compile, all together.
func TestReadComputed(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, src, want string }{
		{"empty", "", ""},
		{"a list", "- a\n", "t1/computed.yaml:1:1: write computed.yaml as names and expressions, like total: sum(items, .price)"},
		{"values", "x: [1]\ny:\nz: 3\n", lines(
			"t1/computed.yaml:1:4: write computed.yaml as names and expressions, like total: sum(items, .price)",
			"t1/computed.yaml:2:1: write computed.yaml as names and expressions, like total: sum(items, .price)",
		)},
		{"names", "zz: 1\n'true': 2\na: 3\nsum: 4\nin: 5\n'1a': 6\n", lines(
			"t1/computed.yaml:2:1: true"+notName,
			"t1/computed.yaml:3:1: a is an item of the data too, so an expression could not tell them apart; rename it",
			"t1/computed.yaml:4:1: sum"+notName,
			"t1/computed.yaml:5:1: in"+notName,
			"t1/computed.yaml:6:1: 1a"+notName,
		)},
		{"a function's name, an item's too", "count: 3\n", "t1/computed.yaml:1:1: count" + notName},
		{"a shared key's name", "title: 3\n", "t1/computed.yaml:1:1: title is an item of the data too, so an expression could not tell them apart; rename it"},
		{"a function's name, then used", "sum: 3\nlen: 3\nx: len + 1\n", lines(
			"t1/computed.yaml:1:1: sum"+notName,
			"t1/computed.yaml:2:1: len"+notName,
			"t1/computed.yaml:3:8: invalid operation: + (mismatched types func(...interface {}) (interface {}, error) and int)",
		)},
		{
			"an item of a function's name as a value", "c: count + 1\n",
			"t1/computed.yaml:1:10: invalid operation: + (mismatched types func([]interface {}, func(interface {}) bool) int and int)",
		},
		{"not declared", "x: zzz\n", "t1/computed.yaml:1:4: unknown name zzz"},
		{"$env", "x: $env.a + 1\ny: $env[\"a\"]\nz: $env\n", lines(
			"t1/computed.yaml:1:4: $env is not for computed.yaml; use the names",
			"t1/computed.yaml:2:4: $env is not for computed.yaml; use the names",
			"t1/computed.yaml:3:4: $env is not for computed.yaml; use the names",
		)},
		{"compile, together", "z: a +\ny: $env.a\nx: zzz\n", lines(
			"t1/computed.yaml:1:6: unexpected token EOF",
			"t1/computed.yaml:2:4: $env is not for computed.yaml; use the names",
			"t1/computed.yaml:3:4: unknown name zzz",
		)},
		{"itself", "p: p + 1\n", "t1/computed.yaml:1:1: " + cycle + "p → p"},
		{"two", "q: p + 1\np: q + 1\n", "t1/computed.yaml:2:1: " + cycle + "p → q → p"},
		{"three", "p: q + 1\nq: r + 1\nr: p + 1\n", "t1/computed.yaml:1:1: " + cycle + "p → q → r → p"},
		{"read by one outside", "x: p + 1\np: q + 1\nq: p + 1\n", "t1/computed.yaml:2:1: " + cycle + "p → q → p"},
		{"read by one before it", "c: q + 1\np: q + 1\nq: p + 1\n", "t1/computed.yaml:2:1: " + cycle + "p → q → p"},
		{"two tangles", "s: r\nr: s\nq: p\np: q\n", lines(
			"t1/computed.yaml:2:1: "+cycle+"r → s → r",
			"t1/computed.yaml:4:1: "+cycle+"p → q → p",
		)},
		{"two ways back", "p: q + r\nq: p\nr: p\n", "t1/computed.yaml:1:1: " + cycle + "p → q → p"},
		{"two cycles in a tangle", "p: s\ns: q + r\nq: s\nr: p\n", "t1/computed.yaml:1:1: " + cycle + "p → s → r → p"},
		{"a branch not taken", "p: 'a > 0 ? 1 : q'\nq: p\n", "t1/computed.yaml:1:1: " + cycle + "p → q → p"},
		{"a cycle and a mistake apart", "p: q + 1\nq: p + 1\nz: a +\n", lines(
			"t1/computed.yaml:1:1: "+cycle+"p → q → p",
			"t1/computed.yaml:3:6: unexpected token EOF",
		)},
		{"a mistake in the cycle", "p: q + 1\nq: p +\n", "t1/computed.yaml:2:6: unexpected token EOF"},
		{"a name's mistake and a cycle apart", "sum: 4\np: q + 1\nq: p + 1\n", lines(
			"t1/computed.yaml:1:1: sum"+notName,
			"t1/computed.yaml:2:1: "+cycle+"p → q → p",
		)},
	} {
		if _, got := compute(t, tt.src, ""); got != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// Run computes each value after those it uses; one the data cannot give is told at its place in the
// expression, and those that use it are left out silently.
func TestComputedRun(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, src, body, values, want string }{
		{"in order", "x: y * 2\ny: a + 1\n", "a: 1\n", "x=4 y=2", ""},
		{"the shared keys", "x: title + \"!\"\ny: lang\n", "title: Plan\n", "x=Plan! y=en", ""},
		{"a function, an item named so", "c: count(items, .price > 1)\n", "count: 5\nitems: [{price: 3}]\n", "c=1", ""},
		{"left out", "x: opt ?? 0\ny: opt\nz: records?.in_1_year\n", "a: 1\n", "x=0 y=<nil> z=<nil>", ""},
		{"a string", "x: '\"$env\"'\n", "a: 1\n", "x=$env", ""},
		{
			"nil", "x: opt + 1\ny: x + 1\n", "opt: null\n", "",
			"t1/computed.yaml:1:8: x cannot be computed from d.yaml: invalid operation: <nil> + int",
		},
		{
			"a key of nil", "x: records.in_1_year\n", "a: 1\n", "",
			"t1/computed.yaml:1:12: x cannot be computed from d.yaml: cannot fetch in_1_year from <nil>",
		},
		{
			"a value of an item left out", "s: sum(items, .price * .qty)\n", "items: [{price: 3}]\n", "",
			"t1/computed.yaml:1:22: s cannot be computed from d.yaml: invalid operation: int * <nil>",
		},
		{
			"text plus one", "n: name + 1\n", "name: x\n", "",
			"t1/computed.yaml:1:9: n cannot be computed from d.yaml: invalid operation: string + int",
		},
		{
			"divided by zero", "ratio: a / b\nr2: ratio * 2\n", "a: 1\nb: 0\n", "",
			"t1/computed.yaml:1:10: ratio cannot be computed from d.yaml: 1 / 0 is not a finite number",
		},
		{"a power", "p: b ^ -1\n", "b: 0\n", "", "t1/computed.yaml:1:6: p cannot be computed from d.yaml: 0 ** -1 is not a finite number"},
		{
			"too big", "big: a * a\nnext: big + 1\n", "a: 1.0e300\n", "",
			"t1/computed.yaml:1:6: big cannot be computed from d.yaml: the value +Inf is not a finite number",
		},
		{
			"too big in a list", "l: '[1, a * a]'\n", "a: 1.0e300\n", "",
			"t1/computed.yaml:1:4: l cannot be computed from d.yaml: the value +Inf is not a finite number",
		},
		{
			"too big in keys and values", "m: '{\"x\": a * a}'\n", "a: 1.0e300\n", "",
			"t1/computed.yaml:1:4: m cannot be computed from d.yaml: the value +Inf is not a finite number",
		},
		{"in order of their places", "y: name + 1\nx: a / b\n", "a: 2\nb: 0\nname: s\n", "", lines(
			"t1/computed.yaml:1:9: y cannot be computed from d.yaml: invalid operation: string + int",
			"t1/computed.yaml:2:6: x cannot be computed from d.yaml: 2 / 0 is not a finite number",
		)},
	} {
		values, got := compute(t, tt.src, tt.body)
		if got != tt.want || values != tt.values {
			t.Errorf("%s: computed %q, told\n%s\nwant %q,\n%s", tt.name, values, got, tt.values, tt.want)
		}
	}
}
