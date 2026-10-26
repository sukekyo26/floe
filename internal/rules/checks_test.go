package rules_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/rules"
	"github.com/sukekyo26/floe/internal/usererr"
)

func told(err error) string {
	if err == nil {
		return ""
	}
	return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
}

func lines(s ...string) string { return strings.Join(s, "\n") }

// data reads body as the data file d.yaml, after its template: and lang: lines.
func data(t *testing.T, body string) *doc.Data {
	t.Helper()

	d, err := doc.Read("d.yaml", []byte("template: t1\nlang: en\n"+body), "")
	if err != nil {
		t.Fatal(told(err))
	}
	return d
}

// runChecks reads the checks src and runs them on the data body at at, telling what either says.
func runChecks(t *testing.T, src, body string, at ...string) string {
	t.Helper()

	c, err := rules.ReadChecks("t1/checks.yaml", []byte(src))
	if err != nil {
		return told(err)
	}
	return told(c.Run(data(t, body), at))
}

// ReadChecks tells every mistake of a check's shape and expression together, in the order of the file, at
// the place in the expression where expr says it is.
func TestReadChecks(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, src, want string }{
		{"empty", "", ""},
		{"comments alone", "# none yet\n", ""},
		{"a map", "expr: true\n", "t1/checks.yaml:1:1: write checks.yaml as a list of checks, each with expr and message (and for and field if you like)"},
		{"a scalar item", "- true\n", "t1/checks.yaml:1:3: write checks.yaml as a list of checks, each with expr and message (and for and field if you like)"},
		{"text as written", "- expr: true\n  message: 3\n", ""},
		{
			"the shape, together",
			"- expr: doc.a\n- bogus: 1\n  expr: true\n  message: m\n- for: \"\"\n  expr: \"1 +\"\n  message: m\n- expr: [1]\n  message: m\n",
			lines(
				"t1/checks.yaml:1:3: write message in the check",
				"t1/checks.yaml:2:3: bogus is not a key of a check; use for, field, expr and message",
				"t1/checks.yaml:5:8: for is empty: write the path of the list to check, or leave for out to check the whole",
				"t1/checks.yaml:6:12: unexpected token EOF",
				"t1/checks.yaml:8:9: expr takes one value, written as text: not a list or keys and values",
			),
		},
		{"empty expr and field", "- expr: ''\n  field: ''\n  message: m\n", lines(
			"t1/checks.yaml:1:9: write expr in the check",
			"t1/checks.yaml:2:10: field is empty: write the path of the field a failure points at, or leave field out to point at the item",
		)},
		{"syntax", "- expr: doc.a <\n  message: m\n", "t1/checks.yaml:1:15: unexpected token EOF"},
		{"an empty step at the end", "- for: items.\n  expr: true\n  message: m\n", "t1/checks.yaml:1:8: for: items. has an empty step: write a key between each two dots, and no dot at its start or end, like sections.parts"},
		{"an empty step at the start", "- for: .items\n  expr: true\n  message: m\n", "t1/checks.yaml:1:8: for: .items has an empty step: write a key between each two dots, and no dot at its start or end, like sections.parts"},
		{"a dot alone", "- for: .\n  expr: true\n  message: m\n", "t1/checks.yaml:1:8: for: . has an empty step: write a key between each two dots, and no dot at its start or end, like sections.parts"},
		{"an empty step inside", "- field: a..b\n  expr: false\n  message: m\n", "t1/checks.yaml:1:10: field: a..b has an empty step: write a key between each two dots, and no dot at its start or end, like sections.parts"},
		{"for empty, told once", "- for: \"\"\n  expr: true\n  message: m\n", "t1/checks.yaml:1:8: for is empty: write the path of the list to check, or leave for out to check the whole"},
		{"not true or false", "- expr: 1 + 1\n  message: m\n", "t1/checks.yaml:1:9: expected bool, but got int"},
		{"in single quotes", "- expr: 'doc.a <'\n  message: m\n", "t1/checks.yaml:1:16: unexpected token EOF"},
		{"an escape in quotes", "- expr: \"\\\"x\\\" ==\"\n  message: m\n", "t1/checks.yaml:1:9: unexpected token EOF"},
		{"| block, its second line", "- expr: |\n    doc.a < 1 &&\n      doc.b <\n  message: m\n", "t1/checks.yaml:3:14: unexpected token EOF"},
		{"an escape at the end of single quotes", "- expr: 'doc.a + '''\n  message: m\n", "t1/checks.yaml:1:9: literal not terminated"},
		{"a no-break space before a quote", "- expr: \u00a0'\n  message: m\n", "t1/checks.yaml:1:11: literal not terminated"},
		{"an ideographic space before a quote", "- expr: \u3000'x\n  message: m\n", "t1/checks.yaml:1:12: literal not terminated"},
		{"> block", "- expr: >\n    doc.a < 1 &&\n    doc.b <\n  message: m\n", "t1/checks.yaml:1:9: unexpected token EOF"},
	} {
		if got := runChecks(t, tt.src, ""); got != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// Run tells each failure at its place in the data, with its value when it is text on one line; one at the
// top of the data, or in an empty file, at the file alone.
func TestChecksRun(t *testing.T) {
	t.Parallel()

	const items = "items:\n  - {price: 3, qty: 1}\n  - {price: 4, qty: 1}\n"
	for _, tt := range []struct{ name, src, body, want string }{
		{"holds", "- expr: doc.a < doc.b\n  message: m\n", "a: 1\nb: 2\n", ""},
		{"negated int", "- field: a\n  expr: -doc.a / 2 < 0\n  message: m\n", "a: 3\n", ""},
		{"fails", "- field: a\n  expr: doc.a < doc.b\n  message: a below b\n", "a: 5\nb: 2\n", `d.yaml:3:4: a below b ("5")`},
		{"keys and values", "- field: r\n  expr: false\n  message: m\n", "r:\n  x: 1\n", "d.yaml:4:3: m"},
		{"null", "- field: opt\n  expr: doc.opt != nil\n  message: write opt\n", "opt: null\n", "d.yaml:3:1: write opt"},
		{
			"for", "- for: items\n  field: price\n  expr: item.price * item.qty < 4\n  message: line total under 4\n", items,
			`d.yaml:5:13: line total under 4 ("4")`,
		},
		{"flow item", "- for: items\n  expr: item.price < 4\n  message: m\n", items, "d.yaml:5:6: m"},
		{"the whole", "- expr: doc.a > 5\n  message: a over 5\n", "a: 2\n", "d.yaml: a over 5"},
		{"a key not there", "- field: name\n  expr: doc.name != nil\n  message: write a name\n", "a: 1\n", "d.yaml: write a name"},
		{"an empty file", "- expr: false\n  message: m\n", "", "d.yaml: m"},
		{
			"a key not there in an item", "- for: items\n  field: cost\n  expr: item.cost < 10\n  message: cost under 10\n", items,
			lines("d.yaml:4:6: cost under 10 (the check could not run: invalid operation: <nil> < int)",
				"d.yaml:5:6: cost under 10 (the check could not run: invalid operation: <nil> < int)"),
		},
		{"not true or false", "- expr: doc.a + 1\n  message: m\n", "a: 1\n", "d.yaml: m (the check could not run: invalid operation: bool(int))"},
		{"nil", "- expr: doc.opt > 1\n  message: m\n", "a: 1\n", "d.yaml: m (the check could not run: invalid operation: <nil> > int)"},
		{"nil, done without", "- expr: (doc.records?.in_1_year ?? 0) < 1 && doc.opt == nil\n  message: m\n", "a: 1\n", ""},
		{"for to nothing", "- for: opt\n  expr: item > 1\n  message: m\n", "opt: null\n", ""},
		{"for over scalars on the way", "- for: items.qty\n  expr: false\n  message: m\n", "items: [1, {qty: [2]}]\n", "d.yaml:3:19: m (\"2\")"},
		{"for to text", "- for: name\n  expr: true\n  message: m\n", "name: x\n", "t1/checks.yaml:1:8: for: name does not lead to a list in d.yaml"},
		{"for through text", "- for: name.parts\n  expr: true\n  message: m\n", "name: x\n", "t1/checks.yaml:1:8: for: name.parts does not lead to a list in d.yaml"},
		{"for to keys and values", "- for: records\n  expr: true\n  message: m\n", "records: {in_1_year: 1}\n", "t1/checks.yaml:1:8: for: records does not lead to a list in d.yaml"},
		{
			"divided by zero", "- field: a\n  expr: doc.a / doc.b > 1\n  message: ratio over 1\n", "a: 1\nb: 0\n",
			"d.yaml:3:4: ratio over 1 (the check could not run: 1 / 0 is not a finite number)",
		},
		{"zero by zero", "- expr: doc.b / doc.b == 0\n  message: m\n", "b: 0\n", "d.yaml: m (the check could not run: 0 / 0 is not a finite number)"},
		{"a power", "- expr: doc.b ** -1 > 1\n  message: m\n", "b: 0\n", "d.yaml: m (the check could not run: 0 ** -1 is not a finite number)"},
		{"divided", "- expr: doc.a / 2 < 1\n  message: m\n", "a: 1\n", ""},
		{"$env", "- expr: $env.doc.a > 1\n  message: m\n", "a: 1\n", "d.yaml: m"},
		{"the shared keys", "- expr: doc.title == \"Plan\" && doc.lang == \"en\" && doc.template == \"t1\"\n  message: m\n", "title: Plan\n", ""},
		{"the title as written", "- field: title\n  expr: doc.title == \"Plan\"\n  message: m\n", "title: \"  Plan  \"\n", `d.yaml:3:8: m ("  Plan  ")`},
		{"a rule for the title", "- field: title\n  expr: len(doc.title) > 10\n  message: long title\n", "title: Plan\n", `d.yaml:3:8: long title ("Plan")`},
		{"the theme written", "- expr: doc.theme == nil\n  message: m\n", "theme: soft\n", "d.yaml: m"},
		{"the theme left out", "- expr: doc.theme == nil\n  message: m\n", "a: 1\n", ""},
		{"the theme as written", "- expr: doc.theme == \" soft\"\n  message: m\n", "theme: \" soft\"\n", ""},
		{"for to the title", "- for: title\n  expr: true\n  message: m\n", "title: Plan\n", "t1/checks.yaml:1:8: for: title does not lead to a list in d.yaml"},
		{"for to the lang", "- for: lang\n  expr: true\n  message: m\n", "title: Plan\n", "t1/checks.yaml:1:8: for: lang does not lead to a list in d.yaml"},
	} {
		if got := runChecks(t, tt.src, tt.body); got != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// A part's checks run on its value: each of its items when it is a list, doc the list.
func TestChecksRunPart(t *testing.T) {
	t.Parallel()

	const src = "- field: n\n  expr: item.n < len(doc)\n  message: n under the count\n"
	body := "parts:\n  - x:\n      - {n: 0}\n      - {n: 2}\n"
	if got, want := runChecks(t, src, body, "parts", "0", "x"), `d.yaml:6:13: n under the count ("2")`; got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// Within a file the failures are in the order of their places, whatever the order of the checks; the files
// in the order they first come.
func TestChecksOrder(t *testing.T) {
	t.Parallel()

	const body = "items:\n  - {price: 3}\n  - {price: 4}\na: 2\nb: 3\nname: x\n"
	src := "- field: b\n  expr: doc.b < 1\n  message: b under 1\n" +
		"- for: items\n  field: price\n  expr: item.price < 1\n  message: price under 1\n" +
		"- expr: doc.a > 5\n  message: a over 5\n" +
		"- field: a\n  expr: doc.a < 1\n  message: a under 1\n"
	want := lines(`d.yaml: a over 5`, `d.yaml:4:13: price under 1 ("3")`, `d.yaml:5:13: price under 1 ("4")`,
		`d.yaml:6:4: a under 1 ("2")`, `d.yaml:7:4: b under 1 ("3")`)
	if got := runChecks(t, src, body); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
	noList := "- for: name\n  expr: true\n  message: m\n"
	fails := "- field: a\n  expr: doc.a < 1\n  message: a under 1\n"
	listFirst := lines("t1/checks.yaml:1:8: for: name does not lead to a list in d.yaml", `d.yaml:6:4: a under 1 ("2")`)
	if got := runChecks(t, noList+fails, body); got != listFirst {
		t.Errorf("told\n%s\nwant\n%s", got, listFirst)
	}
	dataFirst := lines(`d.yaml:6:4: a under 1 ("2")`, "t1/checks.yaml:4:8: for: name does not lead to a list in d.yaml")
	if got := runChecks(t, fails+noList, body); got != dataFirst {
		t.Errorf("told\n%s\nwant\n%s", got, dataFirst)
	}
}
