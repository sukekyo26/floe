package tmpl

import (
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
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
	}
	lines := strings.Split(got.Error(), "\n")
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

// A checks.yaml that is not right is the template's mistake, not one of the data.
func TestRelationsOfABrokenTemplate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ checks, want string }{
		{"- for: a\n  expr: 'true'\n", "check 1: write for, expr and message"},
		{"- for: a\n  expr: 'true'\n  message: m\n  feild: x\n", `unknown field "feild"`},
		{"- for: a\n  expr: 'itme.x'\n  message: m\n", "check 1: itme.x: unknown name itme"},
		{"- for: a\n  expr: 'item.x +'\n  message: m\n", "check 1: item.x +: unexpected token EOF"},
		{"- for: a.b\n  expr: 'true'\n  message: m\n", "check 1: for a.b does not lead to a list"},
		{"- for: b\n  expr: 'true'\n  message: m\n", "check 1: for b does not lead to a list"},
	} {
		d, root := data(t, "a: [1]\nb: x\n")
		err := relate([]byte(tt.checks), d, root)
		var p yamlpos.Problems
		if err == nil || errors.As(err, &p) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("relate(%q) = %v, want %s", tt.checks, err, tt.want)
		}
	}
	d, root := data(t, "a: [1]\n")
	if err := relate([]byte("- for: a\n  expr: item == 1\n  message: m\n"), d, root); err != nil {
		t.Errorf("relate of data that holds = %v", err)
	}
}

func failures(t *testing.T, checks, src string) yamlpos.Problems {
	t.Helper()

	d, root := data(t, src)
	var p yamlpos.Problems
	if err := relate([]byte(checks), d, root); !errors.As(err, &p) {
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
	d := &Data{File: "d.yaml", Template: "t", Theme: DefaultTheme, Title: "", Values: nil}
	if err := yaml.NodeToValue(root, &d.Values); err != nil {
		t.Fatal(err)
	}
	return d, root
}
