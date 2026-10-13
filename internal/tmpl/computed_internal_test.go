package tmpl

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

var declared = []string{"attributes", "records", "reads_per_sec", "writes_per_sec", "count"}

// Values are computed after those they use, whatever the order computed.yaml writes them in, from the
// items of the data by name; an item the data leaves out is nil, and one named like a function of expr
// leaves the function usable.
func TestCompute(t *testing.T) {
	t.Parallel()

	d, root := data(t, `attributes:
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
	if err := compute([]byte(src), d, declared, root); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"item_bytes": 30, "storage_gb": 0.03, "read_units": 10.0, "writes": 0}
	if !reflect.DeepEqual(d.Calc, want) {
		t.Errorf("Calc = %#v, want %#v", d.Calc, want)
	}
}

// A value the data cannot give is reported at the top of the file, each once: one that uses it is not
// computed.
func TestComputeFailures(t *testing.T) {
	t.Parallel()

	d, root := data(t, "reads_per_sec: 0\nattributes: []\n")
	src := `per_read: 1 / reads_per_sec
twice: per_read * 2
writes: writes_per_sec * 2
huge: 1e308 * 10
`
	err := compute([]byte(src), d, declared, root)
	var p yamlpos.Problems
	if !errors.As(err, &p) {
		t.Fatalf("compute = %v, want problems", err)
	}
	want := []string{
		"d.yaml:1:1: huge = 1e308 * 10 cannot be computed from this file: the value +Inf is not a finite number",
		"d.yaml:1:1: per_read = 1 / reads_per_sec cannot be computed from this file: 1 / 0 is not a finite number",
		"d.yaml:1:1: writes = writes_per_sec * 2 cannot be computed from this file: invalid operation: <nil> * int",
	}
	if got := p.Error(); got != strings.Join(want, "\n") {
		t.Errorf("the failures =\n%s\nwant\n%s", got, strings.Join(want, "\n"))
	}
	if ja := p.Localize(i18n.New("ja")); !strings.Contains(ja, "d.yaml:1:1: per_read = 1 / reads_per_sec をこのファイルから計算できません: ") {
		t.Errorf("the failures in Japanese:\n%s", ja)
	}
}

// A computed.yaml that is not right is the template's mistake, not one of the data.
func TestComputeOfABrokenTemplate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		{"a: b + 1\nb: c + 1\nc: a + 1\n", "the values are computed from each other: a → b → c → a"},
		{"a: a + 1\n", "the values are computed from each other: a → a"},
		{"records: 1\n", "records is an item of the data too"},
		{"my-size: 1\n", "my-size is not a name an expression or page.html can use"},
		{"sum: 1\n", "sum is not a name"},
		{"true: 1\n", "true is not a name"},
		{"in: 1\n", "in is not a name"},
		{"_divide: 1\n", "_divide is not a name"},
		{"a: nope + 1\n", "a: nope + 1: unknown name nope"},
		{"a: 1 +\n", "a: 1 +: unexpected token EOF"},
		{"- a\n", "computed.yaml: "},
	} {
		d, root := data(t, "reads_per_sec: 1\n")
		err := compute([]byte(tt.src), d, declared, root)
		var p yamlpos.Problems
		if err == nil || errors.As(err, &p) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("compute(%q) = %v, want %s", tt.src, err, tt.want)
		}
	}
}
