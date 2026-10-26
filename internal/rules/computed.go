package rules

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"strings"
	"text/template/parse"

	"github.com/expr-lang/expr"
	exprast "github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/builtin"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/vm"
	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/cycle"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Computed is a computed.yaml read: each expression compiled, in the order to compute them, each after the
// computed values it uses.
type Computed struct {
	exprs    map[string]expression
	programs map[string]*vm.Program
	uses     map[string][]string
	order    []string
	items    []string // the items of the data the expressions see by name
	field    map[string]int
	env      reflect.Type
}

// nameMistake is why n, written at at in file, is no name an expression and page.html (.Calc.n) can use, nil
// for none: what expr's parser or text/template's says of it, in their words, or that they read it as other
// than one name; a function of expr (sum, date…), which a variable named so would hide; or one starting with _,
// as floe's own functions of arithmetic do.
func nameMistake(file string, at yamlpos.Pos, n string) *usererr.Error {
	if _, function := builtin.Index[n]; function || strings.HasPrefix(n, "_") {
		return yamlpos.New(file, at, "rules_computed_name", n)
	}
	tree, err := parser.Parse(n)
	if err != nil {
		return usererr.LibraryAt(file, at.Line, at.Column, brief(err))
	}
	if id, ok := tree.Node.(*exprast.IdentifierNode); !ok || id.Value != n {
		return yamlpos.New(file, at, "rules_computed_name", n)
	}
	trees, err := parse.Parse(pageName, "{{.Calc."+n+"}}", "", "")
	if err != nil {
		// Its head names the text above, not a file one can fix.
		text, _ := strings.CutPrefix(err.Error(), "template: "+pageName+":1: ")
		return usererr.LibraryAt(file, at.Line, at.Column, text)
	}
	if !oneField(trees[pageName], n) {
		return yamlpos.New(file, at, "rules_computed_name", n)
	}
	return nil
}

// pageName names the text nameMistake has text/template read a name in.
const pageName = "page.html"

// oneField reports whether t is {{.Calc.n}} alone, the field n of Calc.
func oneField(t *parse.Tree, n string) bool {
	if t == nil || len(t.Root.Nodes) != 1 {
		return false
	}
	action, ok := t.Root.Nodes[0].(*parse.ActionNode)
	if !ok || len(action.Pipe.Cmds) != 1 || len(action.Pipe.Cmds[0].Args) != 1 {
		return false
	}
	field, ok := action.Pipe.Cmds[0].Args[0].(*parse.FieldNode)
	return ok && slices.Equal(field.Ident, []string{"Calc", n})
}

// ReadComputed reads the computed.yaml src of file, whose expressions see by name the items names of the data
// (those a schema declares at its top), the keys every data file shares (doc.Shared), and the other computed
// values. It tells, all together and Sorted, the mistakes of its shape and names, and of the expressions whose
// names are right, and the values computed from each other among the expressions that compile. An empty file
// computes nothing.
func ReadComputed(file string, src []byte, names []string) (*Computed, error) {
	names = withShared(names)
	c := &Computed{
		exprs: map[string]expression{}, programs: map[string]*vm.Program{}, uses: map[string][]string{},
		order: nil, items: nil, field: map[string]int{}, env: nil,
	}
	body, err := yamlpos.Parse(file, src)
	switch {
	case errors.Is(err, yamlpos.ErrEmpty):
		body = nil
	case err != nil:
		return nil, err //nolint:wrapcheck // mistakes at their lines and columns, or floe's failure
	}
	var found usererr.List
	var keys map[string]ast.Node
	if body != nil {
		if keys, err = c.readExprs(file, body, &found); err != nil {
			return nil, err
		}
	}
	all := slices.Sorted(maps.Keys(keys))
	// A name told here is not compiled, nor seen by the others: neither could tell it apart.
	named := map[string]bool{}
	for _, n := range all {
		mistake := nameMistake(file, yamlpos.Of(keys[n]), n)
		switch {
		case mistake != nil:
			found = append(found, mistake)
		case slices.Contains(names, n):
			found = append(found, yamlpos.New(file, yamlpos.Of(keys[n]), "rules_computed_clash", n))
		default:
			named[n] = true
		}
	}
	// LIMIT: an item whose name an expression cannot use (my-size, or a function of expr such as date) is not
	// seen by name. Give the expressions the data as doc too if a template needs one.
	c.items = slices.DeleteFunc(slices.Clone(names), func(n string) bool {
		return nameMistake(file, yamlpos.Pos{Line: 0, Column: 0}, n) != nil
	})
	c.env = c.envOf(slices.Concat(c.items, slices.DeleteFunc(slices.Clone(all), func(n string) bool { return !named[n] })))
	env := reflect.New(c.env).Elem().Interface()
	for _, n := range all {
		e, ok := c.exprs[n]
		if !named[n] || !ok {
			continue
		}
		p, mistake := e.compile(env)
		if mistake != nil {
			found = append(found, mistake)
			continue
		}
		uses, usesEnv := usedNames(p, named)
		if usesEnv {
			found = append(found, yamlpos.New(file, e.start(), "rules_computed_env"))
			continue
		}
		c.programs[n], c.uses[n] = p, uses
	}
	for _, cycle := range cycle.Of(slices.Sorted(maps.Keys(c.programs)), c.uses) {
		at := yamlpos.Of(keys[cycle[0]])
		found = append(found, yamlpos.New(file, at, "rules_computed_cycle", strings.Join(cycle, " → ")))
	}
	if found != nil {
		return nil, found.Sorted().Err()
	}
	c.order = order(all, c.uses)
	return c, nil
}

// withShared returns names and the keys every data file shares that names leaves out.
func withShared(names []string) []string {
	names = slices.Clone(names)
	for _, key := range doc.Shared {
		if !slices.Contains(names, key) {
			names = append(names, key)
		}
	}
	return names
}

// readExprs reads into c the expressions of body, the top of the computed.yaml file, adding to found those
// that are not text or are left empty, and returns the key of each name, those among them; the mistake of a
// top that is not names and expressions, or floe's failure.
func (c *Computed) readExprs(file string, body ast.Node, found *usererr.List) (map[string]ast.Node, error) {
	m, ok := body.(*ast.MappingNode)
	if !ok {
		return nil, yamlpos.New(file, yamlpos.Of(body), "rules_computed_yaml")
	}
	keys := map[string]ast.Node{}
	for _, kv := range m.Values {
		k := yamlpos.KeyOf(kv)
		n := k.GetToken().Value
		keys[n] = k
		s, ok, err := text(kv.Value)
		if err != nil {
			return nil, err
		}
		if !ok || s == "" {
			*found = append(*found, yamlpos.New(file, yamlpos.OfValue(k, kv.Value), "rules_computed_yaml"))
			continue
		}
		c.exprs[n] = expression{src: s, file: file, key: k, value: kv.Value}
	}
	return keys, nil
}

// envOf returns the struct of the variables vars, each a field of its own typed any, so that an expression may
// use it as whatever the data holds, and notes the field of each.
func (c *Computed) envOf(vars []string) reflect.Type {
	fields := make([]reflect.StructField, len(vars))
	for i, v := range vars {
		fields[i] = reflect.StructField{
			Name: fmt.Sprintf("V%d", i), PkgPath: "", Type: reflect.TypeFor[any](),
			Tag: reflect.StructTag(fmt.Sprintf("expr:%q", v)), Offset: 0, Index: nil, Anonymous: false,
		}
		c.field[v] = i
	}
	return reflect.StructOf(fields)
}

// usedNames returns the names of computed that the program p uses, sorted, and true when p uses $env, to be
// refused: through it p could use one without its name, before it is computed.
func usedNames(p *vm.Program, computed map[string]bool) ([]string, bool) {
	var u used
	node := p.Node()
	exprast.Walk(&node, &u)
	var names []string
	for _, n := range u {
		if n == "$env" {
			return nil, true
		}
		if computed[n] && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	return names, false
}

type used []string

func (u *used) Visit(n *exprast.Node) {
	if id, ok := (*n).(*exprast.IdentifierNode); ok {
		*u = append(*u, id.Value)
	}
}

// order returns names so that each comes after those it uses, which use no names in a cycle.
func order(names []string, uses map[string][]string) []string {
	var out []string
	done := map[string]bool{}
	var visit func(n string)
	visit = func(n string) {
		if done[n] {
			return
		}
		done[n] = true
		for _, u := range uses[n] {
			visit(u)
		}
		out = append(out, n)
	}
	for _, n := range names {
		visit(n)
	}
	return out
}

// Run computes the values of c from those of d, each after those it uses: the values computed, for .Calc of
// page.html, and the mistakes of those d cannot give, Sorted, at their places in computed.yaml. A value that
// uses one that failed is left out silently: the failure is told once.
func (c *Computed) Run(d *doc.Data) (map[string]any, error) {
	env := reflect.New(c.env).Elem()
	// An item the data leaves out stays nil.
	for _, n := range c.items {
		if v := d.Values[n]; v != nil {
			env.Field(c.field[n]).Set(reflect.ValueOf(v))
		}
	}
	values := map[string]any{}
	var found usererr.List
	failed := map[string]bool{}
	for _, n := range c.order {
		if slices.ContainsFunc(c.uses[n], func(u string) bool { return failed[u] }) {
			failed[n] = true
			continue
		}
		e := c.exprs[n]
		out, err := expr.Run(c.programs[n], env.Interface())
		if mistake := e.computeMistake(n, d.File, out, err); mistake != nil {
			found = append(found, mistake)
			failed[n] = true
			continue
		}
		out = doc.Ints(out)
		values[n] = out
		if out != nil {
			env.Field(c.field[n]).Set(reflect.ValueOf(out))
		}
	}
	return values, found.Sorted().Err()
}

// computeMistake is the mistake of the expression n of e computing out with err from the data file data: one
// that stops, or a value that is or holds NaN or ±Inf; nil for none.
func (e expression) computeMistake(n, data string, out any, err error) *usererr.Error {
	var nf *notFinite
	switch {
	case errors.As(err, &nf):
		return yamlpos.New(e.file, e.place(err), "rules_computed_not_finite", n, data, nf.Error())
	case err != nil:
		return yamlpos.New(e.file, e.place(err), "rules_computed_error", n, data, brief(err))
	}
	if f, ok := notFiniteIn(reflect.ValueOf(out)); ok {
		return yamlpos.New(e.file, e.start(), "rules_computed_value_not_finite", n, data, fmt.Sprint(f))
	}
	return nil
}

// notFiniteIn returns NaN or ±Inf in v, a value or a list or keys and values holding it, which +, - and * can
// give, and false for none.
func notFiniteIn(v reflect.Value) (float64, bool) {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		if f := v.Float(); math.IsNaN(f) || math.IsInf(f, 0) {
			return f, true
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			if f, ok := notFiniteIn(v.Index(i)); ok {
				return f, true
			}
		}
	case reflect.Map:
		for it := v.MapRange(); it.Next(); {
			if f, ok := notFiniteIn(it.Value()); ok {
				return f, true
			}
		}
	case reflect.Interface, reflect.Pointer:
		if !v.IsNil() {
			return notFiniteIn(v.Elem())
		}
	default:
	}
	return 0, false
}
