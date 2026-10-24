package tmpl

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

var declared = []string{"attributes", "records", "reads_per_sec", "writes_per_sec", "count"}

// compute reads the computed.yaml src and computes it on d, as a template does.
func compute(src []byte, d *Data, own bool) error {
	c, err := readComputed("computed.yaml", src, d.shape.keyNames())
	if err != nil {
		return err
	}
	return c.compute("computed.yaml", d, own)
}

// declaring gives d the shape of a schema declaring the items names, each of any value, and returns it.
func declaring(t *testing.T, d *Data, names []string) *Data {
	t.Helper()

	props := make([]string, len(names))
	for i, n := range names {
		props[i] = fmt.Sprintf("%q: {}", n)
	}
	d.shape = shapeOf(t, `{"type": "object", "properties": {`+strings.Join(props, ", ")+`}}`)
	return d
}

// Values are computed after those they use, whatever the order computed.yaml writes them in, from the
// items of the data by name; an item the data leaves out is nil, and one named like a function of expr
// leaves the function usable.
func TestCompute(t *testing.T) {
	t.Parallel()

	d, _ := data(t, `attributes:
  - { name: pk, avg_bytes: 18 }
  - { name: sk, avg_bytes: 12 }
records: { in_1_year: 1000000 }
reads_per_sec: 10
count: 3
`)
	src := `storage_gb: item_bytes * records.in_1_year / 1e9
item_bytes: sum(attributes, .avg_bytes) + count(attributes, .avg_bytes > 100)
read_units: ceil(item_bytes / 4096) * reads_per_sec
writes: writes_per_sec ?? 0
`
	if err := compute([]byte(src), declaring(t, d, declared), true); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"item_bytes": 30, "storage_gb": 0.03, "read_units": 10.0, "writes": 0}
	if !reflect.DeepEqual(d.Calc, want) {
		t.Errorf("Calc = %#v, want %#v", d.Calc, want)
	}
}

// A key the schema does not declare is not seen, even one named like a computed value.
func TestComputeSeesDeclaredItemsOnly(t *testing.T) {
	t.Parallel()

	d, _ := data(t, "t: 5\n")
	if err := compute([]byte("t: 'nil'\nu: t ?? 0\n"), declaring(t, d, declared), true); err != nil || d.Calc["u"] != 0 {
		t.Errorf("Calc = %v, %v; want u 0", d.Calc, err)
	}
}

// A value the data cannot give is reported at its expression, each once, naming the data file: one that
// uses it is not computed. One that stops for want of a value the data gives none for names that value.
func TestComputeFailures(t *testing.T) {
	t.Parallel()

	d, _ := data(t, "reads_per_sec: 0\nattributes: []\n")
	src := `per_read: 1 / reads_per_sec
twice: per_read * 2
writes: writes_per_sec * 2
huge: 1e308 * 10
list: '[1, 1e308 * 10]'
table: '{"a": [1e308 * 10]}'
`
	err := compute([]byte(src), declaring(t, d, declared), true)
	var p usererr.List
	if !errors.As(err, &p) {
		t.Fatalf("compute = %v, want problems", err)
	}
	want := []string{
		"computed.yaml:1:11: per_read cannot be computed from d.yaml: 1 / 0 is not a finite number",
		"computed.yaml:3:9: writes cannot be computed from d.yaml, which gives no value for writes_per_sec that it uses; " +
			"write the expression to do without one, giving the value for when it is left out with ?? (x ?? []) and reading into it with ?. (x?.y), " +
			"or give it a value in d.yaml",
		"computed.yaml:4:7: huge cannot be computed from d.yaml: the value +Inf is not a finite number",
		"computed.yaml:5:7: list cannot be computed from d.yaml: the value +Inf is not a finite number",
		"computed.yaml:6:8: table cannot be computed from d.yaml: the value +Inf is not a finite number",
	}
	if got := p.Error(); got != strings.Join(want, "\n") {
		t.Errorf("the failures =\n%s\nwant\n%s", got, strings.Join(want, "\n"))
	}
	if ja := p.Localize(i18n.New("ja")); !strings.Contains(ja, "computed.yaml:1:11: per_read を d.yaml から計算できません: ") {
		t.Errorf("the failures in Japanese:\n%s", ja)
	}
}

// A computed.yaml that is not right is the template's mistake, reported at its line and column in
// computed.yaml, each of its names and expressions at once.
func TestComputeOfABrokenTemplate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		{"c: a + 1\na: b + 1\nb: c + 1\n", "computed.yaml:2:1: the values are computed from each other: a → b → c → a"},
		{"a: a + 1\n", "computed.yaml:1:1: the values are computed from each other: a → a"},
		{"x: 1\nrecords: 1\n", "computed.yaml:2:1: records is an item of the data too"},
		{"my-size: 1\n", "computed.yaml:1:1: my-size is not a name an expression or page.html can use"},
		{"sum: 1\n", "computed.yaml:1:1: sum is not a name"},
		{"'true': 1\n", "computed.yaml:1:1: true is not a name"},
		{"true: 1\n", "computed.yaml:1:1: a key is a name: write true as text, in quotes"},
		{"a: &x 1\nb: *x\n", "computed.yaml:1:4: anchors, aliases, merge keys (<<) and tags are not supported"},
		{"in: 1\n", "computed.yaml:1:1: in is not a name"},
		{"_divide: 1\n", "computed.yaml:1:1: _divide is not a name"},
		{"a: nope + 1\n", "computed.yaml:1:4: nope + 1 cannot be read as an expression: unknown name nope"},
		{"a: ($env.z ?? 0) + 1\nz: 3\n", "computed.yaml:1:4: $env is not for computed.yaml"},
		{"e: $env\n", "computed.yaml:1:4: $env is not for computed.yaml"},
		{"a: 1 +\n", "computed.yaml:1:4: 1 + cannot be read as an expression: unexpected token EOF"},
		{"a: [1]\n", "computed.yaml:1:4: write computed.yaml as names and expressions"},
		{"a:\n", "computed.yaml:1:1: write computed.yaml as names and expressions"},
		{"- a\n", "computed.yaml:1:1: write computed.yaml as names and expressions"},
		{"in: 1\nsum: 1\n", "computed.yaml:1:1: in is not a name an expression or page.html can use: write a letter, then letters, digits and _, and not a word or function of expr\ncomputed.yaml:2:1: sum is not a name"},
		{"a: 1 +\nb: nope\n", "computed.yaml:1:4: 1 + cannot be read as an expression: unexpected token EOF\ncomputed.yaml:2:4: nope cannot be read"},
	} {
		d, _ := data(t, "reads_per_sec: 1\n")
		err := compute([]byte(tt.src), declaring(t, d, declared), true)
		if err == nil || !strings.HasPrefix(err.Error(), tt.want) {
			t.Errorf("compute(%q) = %v, want %s", tt.src, err, tt.want)
		}
	}
}

// An expression a # in it cut short, YAML having read a comment there, is refused as YAML reads it, at
// the comment, the author's comment after it too; one in quotes, or before a comment line, is not cut.
func TestComputeCutShort(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		{"a: map([1], # * reads_per_sec)\n", "computed.yaml:1:13: a # after a space began a YAML comment, which may have cut the value short, to map([1],: put the value in quotes if the # is part of it, or write the comment on a line of its own"},
		// The comment that cut it is on the last line of an expression written over lines.
		{"a: reads_per_sec > 0 &&\n  len(filter([1], # > 0)) > 0\n", "computed.yaml:2:19: a # after a space began a YAML comment, which may have cut the value short, to reads_per_sec > 0 && len(filter([1],: put the value in quotes if the # is part of it, or write the comment on a line of its own"},
		// An expression in quotes ends at its quote, and a comment on the next line is the author's.
		{"a: 'len(filter([1],' # > 0)) > 0\n", "computed.yaml:1:4: len(filter([1], cannot be read as an expression: unexpected token EOF"},
		{"a: len(filter([1],\n# > 0)) > 0\nb: 1\n", "computed.yaml:1:4: len(filter([1], cannot be read as an expression: unexpected token EOF"},
		// Cut before its last line, the rest is no YAML.
		{"a: reads_per_sec > 0 &&\n  len(filter([1], # > 0)) > 0 &&\n  reads_per_sec < 9\n", "computed.yaml:2:19: this is not valid YAML: " +
			"a # after a space began a comment, which cut the text short: put text with # in it in quotes (value is not allowed in this context. map key-value is pre-defined)"},
		{"a: nope + 1 # a note\n", "computed.yaml:1:13: a # after a space began a YAML comment, which may have cut the value short, to nope + 1: put the value in quotes if the # is part of it, or write the comment on a line of its own"},
	} {
		d, _ := data(t, "reads_per_sec: 1\n")
		err := compute([]byte(tt.src), declaring(t, d, declared), true)
		if err == nil || err.Error() != tt.want {
			t.Errorf("compute(%q) = %v, want %s", tt.src, err, tt.want)
		}
	}
}

// An explicit key (? name) names its value as a key written plainly does.
func TestComputeExplicitKey(t *testing.T) {
	t.Parallel()

	d, _ := data(t, "reads_per_sec: 1\n")
	if err := compute([]byte("? twice\n: reads_per_sec * 2\n"), declaring(t, d, declared), true); err != nil || d.Calc["twice"] != 2 {
		t.Errorf("Calc = %v, %v; want twice 2", d.Calc, err)
	}
}

// The items an expression that stops uses and the data gives no value for, left out or null, are named: of
// the read it stopped at, or of those it handed the function or operator it stopped at. A value it does
// without (by ??, ?. or a test of nil) is not, nor is one when it stopped for another reason.
func TestComputeLeftOut(t *testing.T) {
	t.Parallel()

	d, _ := data(t, "records: { x: 1 }\nattributes: ~\nparts: []\ntotal: 1\nr: 0\nlists: [{ a: ~ }]\nflag: true\n")
	d.shape = shapeOf(t, `{"type": "object", "properties": {"prs": {}, "writes_per_sec": {}, "total": {}, "r": {}, "parts": {}, "flag": {},
		"records": {"type": "object", "properties": {"x": {}, "y": {}, "in_1_year": {}}, "additionalProperties": false},
		"attributes": {"type": "array", "items": {"type": "object", "properties": {"x": {}}}},
		"lists": {"type": "array", "items": {"type": "object", "properties": {"a": {"type": "object", "properties": {"b": {}}}}}}}}`)
	for src, want := range map[string]string{
		"merged: count(prs, .state == 'merged')\n":         "prs",
		"n: records.in_1_year * 2\n":                       "records.in_1_year",
		"n: attributes.x ?? 0\n":                           "attributes",
		"n: len(attributes ?? []) + writes_per_sec\n":      "writes_per_sec",
		"n: (records?.in_1_year ?? 0) + attributes[0].x\n": "attributes",
		"n: let s = records; s.in_1_year * prs\n":          "prs",
		"n: lists[-1].a.b * 2\n":                           "lists[0].a",
		"n: attributes?.x + 1\n":                           "attributes",
		"n: prs || flag\n":                                 "prs",
		"n: \"prs ? 1 : 0\"\n":                             "prs",
		"n: if prs { 1 } else { 0 }\n":                     "prs",
		"n: lists[prs]\n":                                  "prs",
		"n: parts[0:prs]\n":                                "prs",
		"n: \"prs ? writes_per_sec.a : 0\"\n":              "prs",
	} {
		err := compute([]byte(src), d, true)
		if err == nil || !strings.Contains(err.Error(), "from d.yaml, which gives no value for "+want+" that it uses; ") {
			t.Errorf("%s = %v, want %s left out", src, err, want)
		}
	}
	for src, want := range map[string]string{
		"n: \"prs == nil ? total / len(parts) : len(prs)\"\n": "1 / 0 is not a finite number",
		"n: \"prs == nil ? parts[5] : 0\"\n":                  "index out of range: 5",
		"n: (writes_per_sec ?? records.y ?? 0) + 1 / r\n":     "1 / 0 is not a finite number",
		"n: total && prs\n":                                   "interface conversion: interface {} is int, not bool",
		"n: records.b * 2\n":                                  "invalid operation: <nil> * int",
		"n: split(r, prs)\n":                                  "interface conversion: interface {} is int, not string",
		"n: concat(r, prs)\n":                                 "cannot concat int",
	} {
		err := compute([]byte(src), d, true)
		if err == nil || !strings.Contains(err.Error(), "n cannot be computed from d.yaml: "+want) {
			t.Errorf("%s = %v, want %s", src, err, want)
		}
	}
	for _, src := range []string{"merged: count(prs ?? [], .state == 'merged')\n", "n: attributes?.x ?? records.x\n", "n: flag && prs\n"} {
		if err := compute([]byte(src), d, true); err != nil {
			t.Errorf("%s = %v", src, err)
		}
	}
}

// floe's own computed.yaml or checks.yaml stopping on data its schema took is floe's failure, not a mistake
// to fix: the files are not the reader's.
func TestFloesExpressionStopping(t *testing.T) {
	t.Parallel()

	d, root := data(t, "reads_per_sec: 0\nrows: [{ a: 1 }]\n")
	var one *usererr.Error
	var all usererr.List
	err := compute([]byte("per_read: 1 / reads_per_sec\nw: count(prs, true)\n"), declaring(t, d, []string{"reads_per_sec", "prs"}), false)
	if err == nil || errors.As(err, &one) || errors.As(err, &all) || !strings.HasPrefix(err.Error(), "compute ") {
		t.Errorf("compute of floe's = %v, want its failure", err)
	}
	err = relateSrc([]byte("- for: rows\n  expr: item.a / doc.lim > 0\n  message: m\n"), d, root, false)
	if err == nil || errors.As(err, &one) || errors.As(err, &all) || !strings.HasPrefix(err.Error(), "check checks.yaml: ") {
		t.Errorf("relate of floe's = %v, want its failure", err)
	}
}
