package rules

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Checks is a checks.yaml read, each expression compiled.
type Checks struct {
	file string
	list []check
}

// check is a check of checks.yaml that the schema cannot say: expr holds for each item of the list at For, a
// path of keys like sections or model.tables, a list on the way gone through item by item, or without For
// once for the whole (each of its items when it is a list). A failure points at the item's Field, a path of
// keys under the item, or at the item without one.
type check struct {
	For, Field, Message string

	forAt yamlpos.Pos
	prog  *vm.Program
}

// scope is what the expression of a check sees.
type scope struct {
	Item any `expr:"item"`
	Doc  any `expr:"doc"`
}

// ReadChecks reads the checks.yaml src of file, each expression compiled for scope, with all the mistakes of
// its shape and its expressions, Sorted. An empty file holds no check.
func ReadChecks(file string, src []byte) (*Checks, error) {
	body, err := yamlpos.Parse(file, src)
	switch {
	case errors.Is(err, yamlpos.ErrEmpty):
		return &Checks{file: file, list: nil}, nil
	case err != nil:
		return nil, err //nolint:wrapcheck // mistakes at their lines and columns, or floe's failure
	}
	c := &Checks{file: file, list: nil}
	var found usererr.List
	seq, ok := body.(*ast.SequenceNode)
	if !ok {
		found = append(found, yamlpos.New(file, yamlpos.Of(body), "rules_checks_yaml"))
	} else {
		for _, n := range seq.Values {
			one, mistakes, err := readCheck(file, n)
			if err != nil {
				return nil, err
			}
			found = append(found, mistakes...)
			if mistakes == nil {
				c.list = append(c.list, one)
			}
		}
	}
	if found != nil {
		return nil, found.Sorted().Err()
	}
	return c, nil
}

// readCheck reads the check n of file: all the mistakes of its keys, and of its expression when it is text; or
// floe's failure.
func readCheck(file string, n ast.Node) (check, usererr.List, error) {
	c := check{For: "", Field: "", Message: "", forAt: yamlpos.Pos{Line: 0, Column: 0}, prog: nil}
	m, ok := n.(*ast.MappingNode)
	if !ok {
		return c, usererr.List{yamlpos.New(file, yamlpos.Of(n), "rules_checks_yaml")}, nil
	}
	var found usererr.List
	var src expression
	at := map[string]yamlpos.Pos{} // where each key is written as text
	to := map[string]*string{"for": &c.For, "field": &c.Field, "expr": &src.src, "message": &c.Message}
	for _, kv := range m.Values {
		k := yamlpos.KeyOf(kv)
		key := k.GetToken().Value
		dst, ok := to[key]
		if !ok {
			found = append(found, yamlpos.New(file, yamlpos.Of(k), "rules_checks_key", key))
			continue
		}
		var err error
		if *dst, ok, err = text(kv.Value); err != nil {
			return c, nil, err
		} else if !ok {
			found = append(found, yamlpos.New(file, yamlpos.OfValue(k, kv.Value), "rules_checks_value", key))
			continue
		}
		at[key] = yamlpos.OfValue(k, kv.Value)
		if key == "expr" {
			src = expression{src: *dst, file: file, key: k, value: kv.Value}
		}
	}
	c.forAt = at["for"]
	for _, key := range []string{"expr", "message"} {
		written, ok := at[key]
		switch {
		case *to[key] != "":
		case ok:
			found = append(found, yamlpos.New(file, written, "rules_checks_missing", key))
		case !slices.ContainsFunc(m.Values, func(kv *ast.MappingValueNode) bool {
			return yamlpos.KeyOf(kv).GetToken().Value == key
		}):
			// One that is not text is told as such.
			found = append(found, yamlpos.New(file, yamlpos.Of(m), "rules_checks_missing", key))
		}
	}
	for _, key := range []string{"for", "field"} {
		if written, ok := at[key]; ok {
			found = append(found, pathMistakes(file, written, key, *to[key])...)
		}
	}
	if src.src != "" {
		var e *usererr.Error
		if c.prog, e = src.compile(scope{Item: nil, Doc: nil}, expr.AsBool()); e != nil {
			found = append(found, e)
		}
	}
	return c, found, nil
}

// pathMistakes refuses path, the for or field (key) of a check written at at in file, written empty or with an
// empty step (items., .items, a..b): left out, they have their meanings, which such a path would take unseen.
func pathMistakes(file string, at yamlpos.Pos, key, path string) usererr.List {
	switch {
	case path == "":
		return usererr.List{yamlpos.New(file, at, "rules_checks_"+key+"_empty")}
	case slices.Contains(strings.Split(path, "."), ""):
		return usererr.List{yamlpos.New(file, at, "rules_checks_step", key, path)}
	}
	return nil
}

// item is a value of the data and the path to it, keys and indexes.
type item struct {
	value any
	at    []string
}

// Run runs the checks on the value at the path at in d, each check's failures at their line and column in
// d, and a for that leads to no list in d at it in checks.yaml, Sorted.
func (c *Checks) Run(d *doc.Data, at []string) error {
	whole := item{value: valueAt(d.Values, at), at: at}
	var found usererr.List
	for _, one := range c.list {
		items, ok := one.items(whole)
		if !ok {
			found = append(found, yamlpos.New(c.file, one.forAt, "rules_checks_not_list", one.For, d.File))
			continue
		}
		for _, it := range items {
			out, err := expr.Run(one.prog, scope{Item: it.value, Doc: whole.value})
			if held, ok := out.(bool); ok && held && err == nil {
				continue
			}
			path := it.at
			if one.Field != "" {
				path = slices.Concat(path, strings.Split(one.Field, "."))
			}
			found = append(found, one.failure(d, path, err))
		}
	}
	return found.Sorted().Err()
}

// valueAt returns what the path at leads to in v, nil where it leads nowhere.
func valueAt(v any, at []string) any {
	for _, step := range at {
		switch x := v.(type) {
		case map[string]any:
			v = x[step]
		case []any:
			i, err := strconv.Atoi(step)
			if err != nil || i < 0 || i >= len(x) {
				return nil
			}
			v = x[i]
		default:
			return nil
		}
	}
	return v
}

// items returns what c checks under whole: the items of the list at c.For, or without it whole itself, each
// of its items when it is a list; false when c.For leads to no list.
func (c check) items(whole item) ([]item, bool) {
	_, isList := whole.value.([]any)
	if c.For == "" && !isList {
		return []item{whole}, true
	}
	var steps []string
	if c.For != "" {
		steps = strings.Split(c.For, ".")
	}
	var found []item
	ok := itemsAt(whole.value, steps, whole.at, &found)
	return found, ok
}

// itemsAt adds to found the items of the lists at steps under v, v being at the path at: a list on the way is
// gone through item by item. A list or value the data leaves out holds none, and so does an item of a list on
// the way that is not keys and values or a list. It returns false when what is at steps is not a list.
func itemsAt(v any, steps, at []string, found *[]item) bool {
	switch v := v.(type) {
	case nil:
		return true
	case map[string]any:
		return len(steps) > 0 && itemsAt(v[steps[0]], steps[1:], slices.Concat(at, steps[:1]), found)
	case []any:
		for i, x := range v {
			here := slices.Concat(at, []string{strconv.Itoa(i)})
			_, isMap := x.(map[string]any)
			_, isList := x.([]any)
			switch {
			case len(steps) == 0:
				*found = append(*found, item{value: x, at: here})
			case !isMap && !isList:
			case !itemsAt(x, steps, here, found):
				return false
			}
		}
		return true
	}
	return false
}

// failure is the mistake of c failing, or not running with err, at the path at in d: at its line and column,
// with its value when it is text on one line. One that Find places at the top of d, or in a file without a
// node, is placed at d alone: at its first key it would read as that key's mistake.
func (c check) failure(d *doc.Data, at []string, err error) *usererr.Error {
	place := func(key string, args ...any) *usererr.Error { return usererr.NewAt(d.File, 0, 0, key, args...) }
	var shown string
	// d.Root is nil for an empty file, which Find cannot take as an ast.Node.
	if d.Root != nil {
		if key, value := yamlpos.Find(d.Root, at); value != ast.Node(d.Root) {
			pos := yamlpos.OfValue(key, value)
			place = func(key string, args ...any) *usererr.Error { return yamlpos.New(d.File, pos, key, args...) }
			switch value.(type) {
			case *ast.NullNode, *ast.LiteralNode:
			case ast.ScalarNode:
				shown = value.GetToken().Value
			}
		}
	}
	var nf *notFinite
	switch {
	case errors.As(err, &nf):
		return place("rules_check_not_finite", c.Message, nf.Error())
	case err != nil:
		return place("rules_check_error", c.Message, brief(err))
	case shown != "" && !strings.Contains(shown, "\n"):
		return place("rules_check", c.Message, shown)
	default:
		return place("rules_item", c.Message)
	}
}
