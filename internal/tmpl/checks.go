package tmpl

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/yamlpos"
)

// relation is a check of checks.yaml that the schema cannot say: expr holds for each item of the list
// at for, a path of keys like sections or model.tables. A failure points at the item's field, a path
// of keys under the item, or at the item without one.
type relation struct {
	For     string `yaml:"for"`
	Field   string `yaml:"field"`
	Expr    string `yaml:"expr"`
	Message string `yaml:"message"`
}

// scope is what the expression of a check sees.
type scope struct {
	Item any            `expr:"item"`
	Doc  map[string]any `expr:"doc"`
}

// checkRelations checks the values of d against its template's checks.yaml, if it has one, and reports
// every failure at its line and column under root.
func checkRelations(t *Template, d *Data, root ast.Node) error {
	src, err := t.readFile("checks.yaml")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return relate(src, d, root)
}

var (
	errIncomplete = errors.New("write for, expr and message")
	errNotList    = errors.New("does not lead to a list")
)

// relate checks the values of d against the checks src.
func relate(src []byte, d *Data, root ast.Node) error {
	var checks []relation
	if err := yaml.UnmarshalWithOptions(src, &checks, yaml.Strict()); err != nil {
		return fmt.Errorf("read the checks of %s: %w", d.Template, err)
	}
	var problems yamlpos.Problems
	for i, c := range checks {
		more, err := c.check(d.File, d.Values, root)
		if err != nil {
			return fmt.Errorf("read the checks of %s: check %d: %w", d.Template, i+1, err)
		}
		problems = append(problems, more...)
	}
	if problems != nil {
		return problems.Sorted()
	}
	return nil
}

// check runs c on each item of its list in doc, the values of file. An error is the template's mistake,
// and the problems are the data's.
func (c relation) check(file string, doc map[string]any, root ast.Node) (yamlpos.Problems, error) {
	if c.For == "" || c.Expr == "" || c.Message == "" {
		return nil, errIncomplete
	}
	p, err := compile(c.Expr, scope{Item: nil, Doc: nil}, expr.AsBool())
	if err != nil {
		return nil, err
	}
	steps := strings.Split(c.For, ".")
	list, err := listAt(doc, steps)
	if err != nil {
		return nil, fmt.Errorf("for %s %w", c.For, err)
	}
	var problems yamlpos.Problems
	for i, item := range list {
		out, err := expr.Run(p, scope{Item: item, Doc: doc})
		if held, ok := out.(bool); ok && held && err == nil {
			continue
		}
		at := slices.Concat(steps, []string{strconv.Itoa(i)})
		if c.Field != "" {
			at = append(at, strings.Split(c.Field, ".")...)
		}
		problems = append(problems, c.failure(file, root, at, err))
	}
	return problems, nil
}

// failure reports c failed, or could not run with err, at the node at under root: with its value when
// it is one line of text. A block scalar's token is its | or >, not its text.
func (c relation) failure(file string, root ast.Node, at []string, err error) *yamlpos.Error {
	key, value, path := yamlpos.Find(root, at)
	pos := yamlpos.OfValue(key, value)
	var shown string
	switch value.(type) {
	case *ast.NullNode, *ast.LiteralNode:
	case ast.ScalarNode:
		shown = value.GetToken().Value
	}
	switch {
	case err != nil:
		return yamlpos.New(file, pos, "tmpl_check_error", path, c.Message, brief(err))
	case shown != "" && !strings.Contains(shown, "\n"):
		return yamlpos.New(file, pos, "tmpl_check", path, c.Message, shown)
	default:
		return yamlpos.New(file, pos, "tmpl_check_bare", path, c.Message)
	}
}

// listAt returns the list at steps under values, or nil when the data leaves it out.
func listAt(values map[string]any, steps []string) ([]any, error) {
	var v any = values
	for _, step := range steps {
		if v == nil {
			return nil, nil
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil, errNotList
		}
		v = m[step]
	}
	if v == nil {
		return nil, nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, errNotList
	}
	return list, nil
}
