package tmpl

import (
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

const relations = `
- for: model.patterns
  field: key
  expr: item.key in map(doc.attributes, .name)
  message: use an attribute as the key
- for: model.patterns
  field: id
  expr: count(doc.model.patterns, .id == item.id) == 1
  message: give each pattern its own id
- for: model.patterns
  field: rate.per_sec
  expr: item.reads / item.rate.per_sec < 10
  message: keep reads per second under 10
- for: attributes
  expr: item.name != "secret"
  message: the name secret is taken
- for: notes
  field: text
  expr: item.text == nil || len(item.text) < 5
  message: keep notes short
- for: notes
  field: text
  expr: item.text != nil
  message: write the note
- for: nowhere.at_all
  expr: "false"
  message: never runs
`

// relateSrc reads the checks.yaml src and checks d against them, as checkRelations does.
func relateSrc(src []byte, d *Data, root ast.Node, own bool) error {
	list, err := readChecks("checks.yaml", src)
	return usererr.Collect(err, relate(list, d, root, own))
}

// Each item a check fails for is reported at its field, with the field's value when it has one; an
// expression that cannot run on an item, a division by zero among them, is reported there too, but one
// that stops for want of a value the data gives none for is the check's, told at its expression.
func TestRelations(t *testing.T) {
	t.Parallel()

	src := `attributes:
  - name: pk
  - name: secret
model:
  patterns:
    - id: AP-1
      key: pk
      reads: 1
      rate: { per_sec: 1 }
    - id: AP-1
      key: user_id
      reads: 1
      rate: { per_sec: 0 }
    - id: AP-2
      key: [pk]
      reads: 1
notes:
  - text: |
      a long note
  - text:
`
	got := failures(t, relations, src, `{"type": "object", "properties": {"attributes": {}, "notes": {},
		"model": {"type": "object", "properties": {"patterns": {"type": "array", "items": {"type": "object",
			"properties": {"id": {}, "key": {}, "reads": {}, "rate": {"type": "object", "properties": {"per_sec": {}}}}}}}}}}`)
	want := []string{
		"d.yaml:3:5: attributes[1]: the name secret is taken",
		`d.yaml:6:11: model.patterns[0].id: give each pattern its own id ("AP-1")`,
		`d.yaml:10:11: model.patterns[1].id: give each pattern its own id ("AP-1")`,
		`d.yaml:11:12: model.patterns[1].key: use an attribute as the key ("user_id")`,
		"d.yaml:13:24: model.patterns[1].rate.per_sec: keep reads per second under 10 (the check could not run: 1 / 0 is not a finite number)",
		"d.yaml:15:12: model.patterns[2].key: use an attribute as the key",
		"d.yaml:18:11: notes[0].text: keep notes short",
		"d.yaml:20:5: notes[1].text: write the note",
		"checks.yaml:12:9: model.patterns[2] of d.yaml cannot be checked, as d.yaml gives no value for model.patterns[2].rate that the expression uses; ",
	}
	lines := strings.Split(got.Error(), "\n")
	for _, i := range []int{len(want) - 3, len(want) - 2} {
		if len(lines) == len(want) && lines[i] != want[i] {
			t.Errorf("a block of text or an empty value is quoted: %s", lines[i])
		}
	}
	if len(lines) != len(want) {
		t.Fatalf("the failures =\n%s\nwant %d", got.Error(), len(want))
	}
	for i, w := range want {
		if !strings.HasPrefix(lines[i], w) {
			t.Errorf("failure %d = %s\nwant %s", i, lines[i], w)
		}
	}
	if ja := got.Localize(i18n.New("ja")); !strings.Contains(ja, "d.yaml:11:12: model.patterns[1].key: use an attribute as the key（「user_id」）") {
		t.Errorf("the failures in Japanese:\n%s", ja)
	}
}

// A check that stops for another reason than a value left out, which it reads where it stopped, is told at the
// item in expr's words: || stopped on a value that is not true or false, not on the nil on its right.
func TestRelationsStoppedOtherwise(t *testing.T) {
	t.Parallel()

	checks := "- for: rows\n  expr: item.public || item.draft\n  message: m\n"
	got := failures(t, checks, "rows:\n  - public: 'yes'\n", `{"properties": {"rows": {"items": {"properties": {"public": {}, "draft": {}}}}}}`).Error()
	if want := "d.yaml:2:5: rows[0]: m (the check could not run: "; !strings.HasPrefix(got, want) {
		t.Errorf("the failures = %s, want %s", got, want)
	}
}

// A list on the way of for is gone through item by item, an item without the list holding none, and
// so does a value beside the keys and values, as a schema may take.
func TestRelationsThroughLists(t *testing.T) {
	t.Parallel()

	checks := "- for: groups.members\n  field: name\n  expr: item.name != 'bad'\n  message: no bad members\n"
	src := "groups:\n  - members:\n      - name: ok\n  - {}\n  - members:\n      - name: ok\n      - name: bad\n  - text\n"
	if got, want := failures(t, checks, src, `{}`).Error(), `d.yaml:7:15: groups[2].members[1].name: no bad members ("bad")`; got != want {
		t.Errorf("the failures = %s, want %s", got, want)
	}
}

// A checks.yaml that is not right is the template's mistake, reported at its line and column in
// checks.yaml.
func TestRelationsOfABrokenTemplate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ checks, want string }{
		{"- for: a\n  expr: 'true'\n", "checks.yaml:1:3: write message in the check"},
		{"- for: a\n  expr: 'true'\n  message: m\n  feild: x\n", "checks.yaml:4:3: feild is not a key of a check"},
		{"- for: a\n  expr: [x]\n  message: m\n", "checks.yaml:2:9: expr must be text"},
		{"for: a\n", "checks.yaml:1:1: write checks.yaml as a list of checks"},
		{"- for: a\n  expr: 'true'\n  1: m\n", "checks.yaml:3:3: a key is a name: write 1 as text, in quotes"},
		{"- a\n", "checks.yaml:1:3: write checks.yaml as a list of checks"},
		{"- for: a\n  expr: 'itme.x'\n  message: m\n", "checks.yaml:2:9: itme.x cannot be read as an expression: unknown name itme"},
		{"- for: a\n  expr: 'item.x +'\n  message: m\n", "checks.yaml:2:9: item.x + cannot be read as an expression: unexpected token EOF"},
		{"- for: a\n  expr: len(filter(doc.a, # > 0)) > 0\n  message: m\n", "checks.yaml:2:27: a # after a space began a YAML comment, which may have cut the value short, to len(filter(doc.a,"},
		{"- for: b.c\n  expr: 'true'\n  message: m\n", "checks.yaml:1:8: for: b.c does not lead to a list in d.yaml"},
		{"- for: b\n  expr: 'true'\n  message: m\n", "checks.yaml:1:8: for: b does not lead to a list in d.yaml"},
		// for and field written empty, unlike left out, are refused at their values.
		{"- for:\n  field: key\n  expr: 'true'\n  message: m\n", "checks.yaml:1:3: for is empty: write the path of the list to check"},
		{"- for: ''\n  expr: 'true'\n  message: m\n", "checks.yaml:1:8: for is empty: write the path of the list to check"},
		{"- for: a\n  field:\n  expr: 'true'\n  message: m\n", "checks.yaml:2:3: field is empty: write the path of the field a failure points at"},
	} {
		d, root := data(t, "a: [1]\nb: x\n")
		err := relateSrc([]byte(tt.checks), d, root, true)
		if err == nil || !strings.HasPrefix(err.Error(), tt.want) {
			t.Errorf("relateSrc(%q) = %v, want %s", tt.checks, err, tt.want)
		}
	}
	d, root := data(t, "a: [1]\n")
	both := "- for: a\n  expr: 'true'\n- for: a\n  message: m\n"
	if err := relateSrc([]byte(both), d, root, true); err == nil || err.Error() != "checks.yaml:1:3: write message in the check\nchecks.yaml:3:3: write expr in the check" {
		t.Errorf("relate of two broken checks = %v", err)
	}
	if err := relateSrc([]byte("- for: a\n  expr: item == 1 && -item / 2 == -0.5\n  message: m\n"), d, root, true); err != nil {
		t.Errorf("relate of data that holds = %v", err)
	}
	// The author's comment after an expression is refused too, as floe cannot tell it from a # of the
	// expression; on a line of its own, it is the author's.
	if err := relateSrc([]byte("- for: a\n  expr: item == 1 # one\n  message: m\n"), d, root, true); err == nil ||
		!strings.HasPrefix(err.Error(), "checks.yaml:2:19: a # after a space began a YAML comment, which may have cut the value short, to item == 1:") {
		t.Errorf("relate of an expression and a comment = %v", err)
	}
	if err := relateSrc([]byte("- for: a\n  # one\n  expr: item == 1\n  message: m\n"), d, root, true); err != nil {
		t.Errorf("relate of an expression under a comment = %v", err)
	}
}

// A check whose for leads to no list is told with the mistakes of the others, a template's and a part's:
// the mistakes of a stage are told together.
func TestRelationsBesideANotList(t *testing.T) {
	t.Parallel()

	checks := "- for: a\n  expr: item > 1\n  message: one\n- for: b\n  expr: 'true'\n  message: two\n" +
		"- for: a\n  expr: item > 5\n  message: three\n"
	want := "d.yaml:2:5: a[0]: one (\"1\")\nd.yaml:2:5: a[0]: three (\"1\")\nchecks.yaml:4:8: for: b does not lead to a list in d.yaml"
	d, root := data(t, "b: x\na: [1]\n")
	if err := relateSrc([]byte(checks), d, root, true); err == nil || err.Error() != want {
		t.Errorf("a template's = %v, want\n%s", err, want)
	}
	// So is an expression that cannot be read, the others run.
	broken := strings.Replace(checks, "expr: 'true'", "expr: 'item +'", 1)
	wantBroken := "checks.yaml:5:9: item + cannot be read as an expression: unexpected token EOF\nd.yaml:2:5: a[0]: one (\"1\")\nd.yaml:2:5: a[0]: three (\"1\")"
	if err := relateSrc([]byte(broken), d, root, true); err == nil || err.Error() != wantBroken {
		t.Errorf("a template's with an expression that cannot be read = %v, want\n%s", err, wantBroken)
	}
	// floe's own checks.yaml whose for leads to no list is floe's failure, not a mistake to fix.
	var told usererr.List
	if err := relateSrc([]byte(checks), d, root, false); told.Add(err) == nil ||
		!strings.Contains(err.Error(), "for b does not lead to a list") {
		t.Errorf("floe's own = %v, want floe's failure", err)
	}

	list, err := readChecks("checks.yaml", []byte(checks))
	if err != nil {
		t.Fatal(err)
	}
	pd, proot := data(t, "p:\n  b: x\n  a: [1]\n")
	p := &part{name: "p", checks: list, dir: "own"} //nolint:exhaustruct // what check reads
	more, err := p.check("d.yaml", itemAt{value: pd.Values["p"], at: []string{"p"}}, proot)
	wantPart := "d.yaml:3:7: p.a[0]: one (\"1\")\nd.yaml:3:7: p.a[0]: three (\"1\")\nchecks.yaml:4:8: for: b does not lead to a list in d.yaml"
	if err != nil || more.Sorted().Err() == nil || more.Sorted().Err().Error() != wantPart {
		t.Errorf("a part's = %v %v, want\n%s", more, err, wantPart)
	}
}

// A check without for checks the whole data file once, and points at its field, or at the top level
// without one.
func TestRelationsOfTheWhole(t *testing.T) {
	t.Parallel()

	src := "status: accepted\noptions:\n  - title: a\n    pick: x\n  - title: b\n    pick: y\n  - title: c\n"
	schema := `{"properties": {"status": {}, "note": {}, "options": {"type": "array", "items": {"properties": {"title": {}, "pick": {}}}}}}`
	for _, tt := range []struct{ checks, want string }{
		{
			"- field: options\n  expr: doc.status == 'proposed' || count(doc.options, .pick != nil) == 1\n  message: pick one\n",
			"d.yaml:3:3: options: pick one",
		},
		{"- expr: count(item.options, .pick != nil) == 1\n  message: pick one\n", "d.yaml:1:1: the top level: pick one"},
		{"- expr: len(doc.note) > 0\n  message: m\n", "checks.yaml:1:9: the top level of d.yaml cannot be checked, as d.yaml gives no value for note"},
	} {
		if got := failures(t, tt.checks, src, schema).Error(); !strings.HasPrefix(got, tt.want) || strings.Contains(got, "\n") {
			t.Errorf("%s= %s\nwant one, %s", tt.checks, got, tt.want)
		}
	}
}

// A part's field without for is a path under the part, or under its items when it is a list, refused only
// where every kind of value its schema takes surely refuses it: a part that may be either takes it on either.
func TestDeclaredOfAPart(t *testing.T) {
	t.Parallel()

	const closed = `"additionalProperties": false, `
	both := `{"type": ["array", "object"], ` + closed + `"properties": {"url": {}}, "items": {"type": "object", "properties": {"rows": {"type": "array"}}}}`
	for _, tt := range []struct{ schema, for_, field, want string }{
		{`{"type": "object", ` + closed + `"properties": {"url": {}}}`, "", "url", ""},
		{`{"type": "object", ` + closed + `"properties": {"url": {}}}`, "", "uri", "tmpl_checks_field_part"},
		{`{"type": "array", "items": {` + closed + `"properties": {"url": {}}}}`, "", "url", ""},
		{`{"type": "array", "items": {` + closed + `"properties": {"url": {}}}}`, "", "uri", "tmpl_checks_field_part"},
		{`{"type": ["array", "object"], ` + closed + `"items": {` + closed + `"properties": {"url": {}}}}`, "", "url", ""},
		{`{"type": ["array", "object"], ` + closed + `"properties": {"url": {}}, "items": {"additionalProperties": false}}`, "", "url", ""},
		{`{"type": ["array", "object"], ` + closed + `"items": {` + closed + `"properties": {"url": {}}}}`, "", "uri", "tmpl_checks_field_part"},
		{`{"type": "array"}`, "", "uri", ""},
		{`{"type": "string"}`, "", "x", "tmpl_checks_field_part"},
		// for goes into the items of a part that may be a list, as itemsAt does on such a value.
		{both, "rows", "", ""},
		{strings.Replace(both, `"items": {`, `"items": {`+closed, 1), "rowz", "", "tmpl_checks_undeclared"},
	} {
		set, err := builtinParts()
		if err != nil {
			t.Fatal(err)
		}
		s, err := compileSchema(set, "t", "t/schema.json", jsonDoc(t, tt.schema))
		if err != nil {
			t.Fatalf("%s: %v", tt.schema, err)
		}
		c := relation{For: tt.for_, Field: tt.field, Expr: "true", Message: "m", file: "checks.yaml", at: map[string]yamlpos.Pos{}}
		e := c.declared(s, true)
		switch {
		case tt.want == "" && e != nil:
			t.Errorf("%s, for %s, field %s = %v", tt.schema, tt.for_, tt.field, e)
		case tt.want != "" && (e == nil || e.Key != tt.want):
			t.Errorf("%s, for %s, field %s = %v, want %s", tt.schema, tt.for_, tt.field, e, tt.want)
		}
	}
}

func failures(t *testing.T, checks, src, schema string) usererr.List {
	t.Helper()

	d, root := data(t, src)
	d.shape = shapeOf(t, schema)
	var p usererr.List
	if err := relateSrc([]byte(checks), d, root, true); !errors.As(err, &p) {
		t.Fatalf("relate = %v, want problems", err)
	}
	return p
}

// data reads src as the data file d.yaml of a template t, for checks given apart from a template.
func data(t *testing.T, src string) (*Data, ast.Node) {
	t.Helper()

	root, err := parse("d.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	values, err := decode("d.yaml", root)
	if err != nil {
		t.Fatal(err)
	}
	return &Data{File: "d.yaml", Template: "t", Theme: DefaultTheme, Title: "", Values: values}, root
}

// A path of checks.yaml is refused only where the compiled schema surely refuses it: a schema
// without the key that refuses other keys, or keys at all. One that may take it (through patterns,
// allOf, anyOf, or by taking any keys) passes, $ref is followed, and a list is refused only where its
// types leave array out. A list on the way of for is gone into, its items.
func TestDeclared(t *testing.T) {
	t.Parallel()

	const closed = `"additionalProperties": false, `
	const draft7 = `"$schema": "http://json-schema.org/draft-07/schema#", `
	for _, tt := range []struct {
		schema, for_, field, want string
	}{
		{`{` + closed + `"properties": {"a": {"type": "array"}}}`, "a", "x", ""},
		{`{"properties": {"a": {"type": "array", "items": {` + closed + `"properties": {"x": {}}}}}}`, "a", "y", "tmpl_checks_field"},
		{`{` + closed + `"properties": {"a": {"type": "array"}}}`, "b", "", "tmpl_checks_undeclared"},
		{`{"properties": {"a": {"type": "array"}}}`, "b", "", ""},
		{`{"properties": {"a": {"type": "string", "anyOf": [{"minLength": 1}]}}}`, "a", "", "tmpl_checks_not_array"},
		{`{"properties": {"a": {}}}`, "a", "x", ""},
		{`{"properties": {"a": {"description": "x"}}}`, "a.x", "", ""},
		{`{"properties": {"a": true}}`, "a.b", "", ""},
		{`{"properties": {"a": false}}`, "a", "", "tmpl_checks_undeclared"},
		{`{` + closed + `"allOf": [{}], "properties": {"rows": {"type": "string"}}}`, "rows", "", "tmpl_checks_not_array"},
		{`{` + closed + `"patternProperties": {"^x": {}}}`, "xs", "", ""},
		{`{"type": "object", "properties": {"a": {"type": "array"}}, "unevaluatedProperties": false}`, "b", "", "tmpl_checks_undeclared"},
		{`{"properties": {"a": {}}, "allOf": [{"properties": {"b": {}}}], "unevaluatedProperties": false}`, "b", "", ""},
		{`{"properties": {"a": {"type": "array", "prefixItems": [{}], "items": {` + closed + `"properties": {}}}}}`, "a", "name", ""},
		{`{"properties": {"a": {"type": "object", ` + closed + `"properties": {}}}}`, "a.x", "", "tmpl_checks_undeclared"},
		{`{"properties": {"a": {"type": "array", "items": {"type": "string"}}}}`, "a.0", "", "tmpl_checks_undeclared"},
		// Without for, field is a path under the top level of the data file.
		{`{` + closed + `"properties": {"options": {"type": "array"}}}`, "", "options", ""},
		{`{` + closed + `"properties": {"options": {"type": "array"}}}`, "", "optionz", "tmpl_checks_field_whole"},
		{`{"properties": {"options": {"type": "array"}}}`, "", "optionz", ""},
		{`{` + closed + `"properties": {"a": {}}}`, "", "", ""},
		// for goes through a list on the way, its items; field takes keys alone.
		{`{"properties": {"s": {"type": "array", "items": {` + closed + `"properties": {"p": {"type": "array"}}}}}}`, "s.p", "", ""},
		{`{"properties": {"s": {"type": "array", "items": {` + closed + `"properties": {"p": {"type": "array"}}}}}}`, "s.q", "", "tmpl_checks_undeclared"},
		{`{"properties": {"s": {"type": "array", "items": {"properties": {"p": {"type": "string"}}}}}}`, "s.p", "", "tmpl_checks_not_array"},
		{`{"properties": {"a": {"type": "array", "items": {"type": "array", "items": {` + closed + `"properties": {"x": {"type": "array"}}}}}}}`, "a.x", "", ""},
		{`{"properties": {"a": {"type": "array"}}}`, "a.x", "", ""},
		{`{"$defs": {"l": {"type": "array", "items": {"$ref": "#/$defs/l"}}}, "properties": {"a": {"$ref": "#/$defs/l"}}}`, "a.x", "", ""},
		{`{"properties": {"s": {"type": "array", "items": {"properties": {"p": {"type": "array", "items": {` + closed + `"properties": {"x": {}}}}}}}}}`, "s", "p.x", "tmpl_checks_field"},
		{`{"properties": {"a": {"type": ["object", "null"]}}}`, "a.x", "", ""},
		{`{"$defs": {"l": {` + closed + `"properties": {"rows": {"type": "array"}}}}, "properties": {"a": {"$ref": "#/$defs/l"}}}`, "a.rows", "", ""},
		{`{"$defs": {"l": {` + closed + `"properties": {"rows": {"type": "array"}}}}, "properties": {"a": {"$ref": "#/$defs/l"}}}`, "a.cols", "", "tmpl_checks_undeclared"},
		{`{` + draft7 + `"definitions": {"l": {"type": "object", "properties": {"rows": {"type": "array"}}}}, "properties": {"a": {"$ref": "#/definitions/l", ` + closed + `"type": "string"}}}`, "a.rows", "", ""},
		{`{` + draft7 + `"definitions": {"l": {"type": "array"}}, "properties": {"a": {"$ref": "#/definitions/l", "type": "string"}}}`, "a", "", ""},
	} {
		set, err := builtinParts()
		if err != nil {
			t.Fatal(err)
		}
		s, err := compileSchema(set, "t", "t/schema.json", jsonDoc(t, tt.schema))
		if err != nil {
			t.Fatalf("%s: %v", tt.schema, err)
		}
		c := relation{For: tt.for_, Field: tt.field, Expr: "true", Message: "m", file: "checks.yaml", at: map[string]yamlpos.Pos{}}
		e := c.declared(s, false)
		switch {
		case tt.want == "" && e != nil:
			t.Errorf("%s, for %s, field %s = %v", tt.schema, tt.for_, tt.field, e)
		case tt.want != "" && (e == nil || e.Key != tt.want):
			t.Errorf("%s, for %s, field %s = %v, want %s", tt.schema, tt.for_, tt.field, e, tt.want)
		}
	}
}
