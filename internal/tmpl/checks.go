package tmpl

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// relation is a check of checks.yaml that the schema cannot say: expr holds for each item of the list
// at for, a path of keys like sections or model.tables, a list on the way gone through item by item
// (sections.parts is the parts of every section). A failure points at the item's field, a path of
// keys under the item, or at the item without one.
type relation struct {
	For, Field, Expr, Message string

	file string                 // checks.yaml, as its mistakes name it
	at   map[string]yamlpos.Pos // where each key is written, and "" where the check is
	rest string                 // the text of a YAML comment after expr on its last line, or ""
	prog *vm.Program            // expr compiled ahead, for a part's; nil to compile it when run
}

// scope is what the expression of a check sees.
type scope struct {
	Item any            `expr:"item"`
	Doc  map[string]any `expr:"doc"`
}

// checkRelations checks the values of d against its template's checks.yaml, if it has one, and reports
// every failure at its line and column under root.
func checkRelations(t *Template, d *Data, root ast.Node) error {
	src, found, err := t.readOptional("checks.yaml")
	if !found || err != nil {
		return err
	}
	return relate(t.file("checks.yaml"), src, d, root)
}

// relate checks the values of d against the checks src of the file checks.
func relate(checks string, src []byte, d *Data, root ast.Node) error {
	list, err := readChecks(checks, src, false)
	if err != nil {
		return err
	}
	var problems usererr.List
	for _, c := range list {
		more, err := c.check(d.File, d.Values, root)
		if err != nil {
			return err
		}
		problems = append(problems, more...)
	}
	return problems.Sorted().Err()
}

// readChecks reads the checks src of file, a mistake in them at its line and column: the mistakes of
// some are usererr.List, returned with the others. A part's checks may leave for out, to check the
// part itself, or each of its items when it is a list.
func readChecks(file string, src []byte, ofPart bool) ([]relation, error) {
	body, err := yamlpos.Parse(file, src, "tmpl_checks_yaml")
	if errors.Is(err, yamlpos.ErrEmpty) {
		return nil, nil
	}
	if err != nil {
		return nil, err //nolint:wrapcheck // a mistake at its line and column
	}
	if err = plain(file, body); err != nil {
		return nil, err
	}
	seq, ok := body.(*ast.SequenceNode)
	if !ok {
		return nil, yamlpos.New(file, yamlpos.Of(body), "tmpl_checks_yaml")
	}
	var list []relation
	var problems usererr.List
	for _, n := range seq.Values {
		c, e := readCheck(file, n, ofPart)
		if e != nil {
			problems = append(problems, e)
			continue
		}
		list = append(list, c)
	}
	if problems != nil {
		// The checks written right, for floe template check to tell their mistakes too.
		return list, problems
	}
	return list, nil
}

func readCheck(file string, n ast.Node, ofPart bool) (relation, *usererr.Error) {
	c := relation{
		For: "", Field: "", Expr: "", Message: "",
		file: file, at: map[string]yamlpos.Pos{"": yamlpos.Of(n)}, rest: "", prog: nil,
	}
	m, ok := n.(*ast.MappingNode)
	if !ok {
		return c, yamlpos.New(file, yamlpos.Of(n), "tmpl_checks_yaml")
	}
	to := map[string]*string{"for": &c.For, "field": &c.Field, "expr": &c.Expr, "message": &c.Message}
	for _, kv := range m.Values {
		k := yamlpos.KeyOf(kv)
		key := k.GetToken().Value
		dst, ok := to[key]
		if !ok {
			return c, yamlpos.New(file, yamlpos.Of(k), "tmpl_checks_key", key)
		}
		c.at[key] = yamlpos.OfValue(k, kv.Value)
		if key == "expr" {
			c.rest = yamlpos.CommentAfter(kv.Value)
		}
		if *dst, ok = text(kv.Value); !ok {
			return c, yamlpos.New(file, c.at[key], "tmpl_not_text", key)
		}
	}
	for _, key := range []string{"for", "expr", "message"} {
		if *to[key] == "" && (key != "for" || !ofPart) {
			at, written := c.at[key]
			if !written {
				at = c.at[""]
			}
			return c, yamlpos.New(file, at, "tmpl_checks_missing", key)
		}
	}
	return c, nil
}

// text returns the value of n as text, a number or true as written, and false for a list or keys and
// values. One left empty is "".
func text(n ast.Node) (string, bool) {
	if _, ok := n.(ast.ScalarNode); !ok {
		return "", false
	}
	var s string
	err := yaml.NodeToValue(n, &s)
	return s, err == nil
}

// program compiles the expression of c.
func (c relation) program() (*vm.Program, *usererr.Error) {
	return compileWritten(c.file, c.at["expr"], c.Expr, c.rest, scope{Item: nil, Doc: nil}, expr.AsBool())
}

// check runs c on each item of its list in doc, the values of file. An error is the template's mistake,
// and the problems are the data's.
func (c relation) check(file string, doc map[string]any, root ast.Node) (usererr.List, error) {
	// A mistake in the expression first: it is the template's whatever the data.
	if c.prog == nil {
		var err *usererr.Error
		if c.prog, err = c.program(); err != nil {
			return nil, err
		}
	}
	var items []itemAt
	if !itemsAt(doc, strings.Split(c.For, "."), nil, &items) {
		return nil, yamlpos.New(c.file, c.at["for"], "tmpl_checks_not_list", c.For, file)
	}
	return c.run(file, doc, items, root)
}

// run runs c, its expression compiled, on each of items of the values doc of file.
func (c relation) run(file string, doc map[string]any, items []itemAt, root ast.Node) (usererr.List, error) {
	p := c.prog
	var problems usererr.List
	for _, item := range items {
		out, err := expr.Run(p, scope{Item: item.value, Doc: doc})
		if held, ok := out.(bool); ok && held && err == nil {
			continue
		}
		at := item.at
		if c.Field != "" {
			at = slices.Concat(at, strings.Split(c.Field, "."))
		}
		problems = append(problems, c.failure(file, root, at, err))
	}
	return problems, nil
}

// failure reports c failed, or could not run with err, at the node at under root: with its value when
// it is one line of text. A block scalar's token is its | or >, not its text.
func (c relation) failure(file string, root ast.Node, at []string, err error) *usererr.Error {
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
		return yamlpos.New(file, pos, "tmpl_item", path, c.Message)
	}
}

// itemsAt adds to found the items of the lists at steps under v, v being at the path at: a list on the
// way is gone through item by item. A list or value the data leaves out holds none, and so does an item
// of a list on the way that is not keys and values or a list, which a schema may take beside them. It
// returns false when what is at steps is not a list.
func itemsAt(v any, steps, at []string, found *[]itemAt) bool {
	switch v := v.(type) {
	case nil:
		return true
	case map[string]any:
		return len(steps) > 0 && itemsAt(v[steps[0]], steps[1:], slices.Concat(at, steps[:1]), found)
	case []any:
		for i, item := range v {
			here := slices.Concat(at, []string{strconv.Itoa(i)})
			_, isMap := item.(map[string]any)
			_, isList := item.([]any)
			switch {
			case len(steps) == 0:
				*found = append(*found, itemAt{value: item, at: here})
			case !isMap && !isList:
			case !itemsAt(item, steps, here, found):
				return false
			}
		}
		return true
	}
	return false
}
