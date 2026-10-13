package tmpl

import (
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/i18n"
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

// Each item a check fails for is reported at its field, with the field's value when it has one; an
// expression that cannot run on an item, a division by zero among them, is reported there too.
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
	got := failures(t, relations, src)
	want := []string{
		"d.yaml:3:5: attributes[1]: the name secret is taken",
		`d.yaml:6:11: model.patterns[0].id: give each pattern its own id ("AP-1")`,
		`d.yaml:10:11: model.patterns[1].id: give each pattern its own id ("AP-1")`,
		`d.yaml:11:12: model.patterns[1].key: use an attribute as the key ("user_id")`,
		"d.yaml:13:24: model.patterns[1].rate.per_sec: keep reads per second under 10 (the check could not run: 1 / 0 is not a finite number)",
		"d.yaml:14:7: model.patterns[2]: keep reads per second under 10 (the check could not run: ",
		"d.yaml:15:12: model.patterns[2].key: use an attribute as the key",
		"d.yaml:18:11: notes[0].text: keep notes short",
		"d.yaml:20:5: notes[1].text: write the note",
	}
	lines := strings.Split(got.Error(), "\n")
	for _, i := range []int{len(want) - 2, len(want) - 1} {
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
		{"- for: a.b\n  expr: 'true'\n  message: m\n", "checks.yaml:1:8: for: a.b does not lead to a list in d.yaml"},
		{"- for: b\n  expr: 'true'\n  message: m\n", "checks.yaml:1:8: for: b does not lead to a list in d.yaml"},
	} {
		d, root := data(t, "a: [1]\nb: x\n")
		err := relate("checks.yaml", []byte(tt.checks), d, root)
		if err == nil || !strings.HasPrefix(err.Error(), tt.want) {
			t.Errorf("relate(%q) = %v, want %s", tt.checks, err, tt.want)
		}
	}
	d, root := data(t, "a: [1]\n")
	both := "- for: a\n  expr: 'true'\n- for: a\n  message: m\n"
	if err := relate("checks.yaml", []byte(both), d, root); err == nil || err.Error() != "checks.yaml:1:3: write message in the check\nchecks.yaml:3:3: write expr in the check" {
		t.Errorf("relate of two broken checks = %v", err)
	}
	if err := relate("checks.yaml", []byte("- for: a\n  expr: item == 1 && -item / 2 == -0.5\n  message: m\n"), d, root); err != nil {
		t.Errorf("relate of data that holds = %v", err)
	}
}

func failures(t *testing.T, checks, src string) yamlpos.Problems {
	t.Helper()

	d, root := data(t, src)
	var p yamlpos.Problems
	if err := relate("checks.yaml", []byte(checks), d, root); !errors.As(err, &p) {
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
