package tmpl

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/expr-lang/expr"
	exprast "github.com/expr-lang/expr/ast"
	exprbuiltin "github.com/expr-lang/expr/builtin"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/vm"
	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// usable reports whether n is a name an expression and page.html (.Calc.n) can use: not a word of expr
// (true, in, let…), and not one of its functions (sum, date…), which a variable named so would hide.
// It starts with a letter, as floe's own functions of arithmetic start with _.
func usable(n string) bool {
	if _, function := exprbuiltin.Index[n]; function || !identifier.MatchString(n) {
		return false
	}
	tree, err := parser.Parse(n)
	if err != nil {
		return false
	}
	id, ok := tree.Node.(*exprast.IdentifierNode)
	return ok && id.Value == n
}

var identifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// checkComputed computes the values of d's template's computed.yaml, if it has one, into d.Calc, from
// the items its schema s declares, and reports at their expressions those its data cannot give.
func checkComputed(t *Template, d *Data, s *shape) error {
	src, found, err := t.readOptional("computed.yaml")
	if !found || err != nil {
		return err
	}
	return compute(t.file("computed.yaml"), src, d, s.keyNames())
}

// compute computes the values of the computed.yaml src, file in its mistakes, from the values of d,
// whose items the schema declares, in declared. An expression sees the items and the other computed
// values by name, and a value is computed after those it uses, in whatever order src writes them.
func compute(file string, src []byte, d *Data, declared []string) error {
	c, err := readComputed(file, src, declared)
	if err != nil {
		return err
	}
	// An item the data leaves out stays nil.
	for _, n := range c.items {
		if v := d.Values[n]; v != nil {
			c.env.Field(c.field[n]).Set(reflect.ValueOf(v))
		}
	}
	d.Calc = map[string]any{}
	var problems usererr.List
	failed := map[string]bool{}
	for _, n := range c.order {
		if slices.ContainsFunc(c.uses[n], func(u string) bool { return failed[u] }) {
			failed[n] = true // its own failure is reported
			continue
		}
		out, err := expr.Run(c.programs[n], c.env.Interface())
		if err == nil {
			err = finiteValue(out)
		}
		if err != nil {
			problems = append(problems, c.failure(file, n, d.File, err))
			failed[n] = true
			continue
		}
		d.Calc[n] = out
		if out != nil {
			c.env.Field(c.field[n]).Set(reflect.ValueOf(out))
		}
	}
	return problems.Sorted().Err()
}

// failure is the mistake of the expression n of c, in file, stopping with err on the data of the file data,
// whose items c set: the items it uses that the data gives no value for, which computed.yaml is to do
// without as the schema let the data leave them out, else err.
func (c *computed) failure(file, n, data string, err error) *usererr.Error {
	vars := map[string]dataVar{}
	for _, item := range c.items {
		vars[item] = dataVar{value: c.env.Field(c.field[item]).Interface(), at: item}
	}
	if missing := unvalued(c.programs[n], vars); missing != nil {
		return yamlpos.New(file, c.at[n], "tmpl_computed_missing", n, data, strings.Join(missing, ", "))
	}
	// LIMIT: expr's English. Word the failures floe can tell apart, as it does the values left out.
	return yamlpos.New(file, c.at[n], "tmpl_computed_error", n, data, brief(err))
}

// computed is computed.yaml compiled: an expression and the computed names it uses, by name, the order
// to compute them in, and the variables they see, a field each.
type computed struct {
	exprs    map[string]string
	programs map[string]*vm.Program
	uses     map[string][]string
	order    []string
	items    []string               // the items of the data the expressions see
	at       map[string]yamlpos.Pos // where each expression is written
	field    map[string]int
	env      reflect.Value
}

// readComputed reads and compiles the computed.yaml src of file, its mistakes at their lines and
// columns: all those of its values and names at once, then all those of its expressions, then values
// computed from each other.
func readComputed(file string, src []byte, declared []string) (*computed, error) {
	// The names of the expressions read are checked beside the mistakes of the others.
	exprs, where, err := readExprs(file, src)
	var problems usererr.List
	if err != nil && !errors.As(err, &problems) {
		return nil, err
	}
	fail := func(at yamlpos.Pos, key string, args ...any) {
		problems = append(problems, yamlpos.New(file, at, key, args...))
	}
	names := slices.Sorted(maps.Keys(exprs))
	for _, n := range names {
		switch {
		case !usable(n):
			fail(yamlpos.Of(yamlpos.KeyOf(where[n])), "tmpl_computed_name", n)
		case slices.Contains(declared, n):
			fail(yamlpos.Of(yamlpos.KeyOf(where[n])), "tmpl_computed_clash", n)
		}
	}
	if problems != nil {
		return nil, problems.Sorted()
	}
	// LIMIT: an item whose name an expression cannot use (my-size, or a function of expr such as date)
	// is not seen by name. Give the expressions the data as doc too if a template needs one.
	items := slices.DeleteFunc(slices.Clone(declared), func(n string) bool { return !usable(n) })
	vars := slices.Concat(items, names)
	fields := make([]reflect.StructField, len(vars))
	c := &computed{
		exprs: exprs, programs: map[string]*vm.Program{}, uses: map[string][]string{}, order: nil,
		items: items, at: map[string]yamlpos.Pos{}, field: map[string]int{}, env: reflect.Value{},
	}
	for i, v := range vars {
		// Each variable is typed any, so that an expression may use it as whatever the data holds.
		fields[i] = reflect.StructField{
			Name: fmt.Sprintf("V%d", i), PkgPath: "", Type: reflect.TypeFor[any](),
			Tag: reflect.StructTag(fmt.Sprintf("expr:%q", v)), Offset: 0, Index: nil, Anonymous: false,
		}
		c.field[v] = i
	}
	c.env = reflect.New(reflect.StructOf(fields)).Elem()
	for _, n := range names {
		at := yamlpos.OfValue(yamlpos.KeyOf(where[n]), where[n].Value)
		c.at[n] = at
		p, e := compileWritten(file, at, exprs[n], c.env.Interface())
		if e != nil {
			problems = append(problems, e)
			continue
		}
		c.programs[n] = p
		var env bool
		if c.uses[n], env = usedNames(p, exprs); env {
			fail(at, "tmpl_computed_env")
		}
	}
	if problems != nil {
		return nil, problems.Sorted()
	}
	order, cycle := topological(names, c.uses)
	if cycle != nil {
		at := yamlpos.Of(yamlpos.KeyOf(where[cycle[0]]))
		return nil, yamlpos.New(file, at, "tmpl_computed_cycle", strings.Join(cycle, " → "))
	}
	c.order = order
	return c, nil
}

// readExprs reads the names and expressions of the computed.yaml src of file, and where each is written.
// Those whose expression is not text are left out, and their mistakes returned as usererr.List.
func readExprs(file string, src []byte) (map[string]string, map[string]*ast.MappingValueNode, error) {
	exprs, where := map[string]string{}, map[string]*ast.MappingValueNode{}
	body, err := yamlpos.Parse(file, src, "tmpl_computed_yaml")
	if errors.Is(err, yamlpos.ErrEmpty) {
		return exprs, where, nil
	}
	if err != nil {
		return nil, nil, err //nolint:wrapcheck // a mistake at its line and column
	}
	if err = plain(file, body); err != nil {
		return nil, nil, err
	}
	m, ok := body.(*ast.MappingNode)
	if !ok {
		return nil, nil, yamlpos.New(file, yamlpos.Of(body), "tmpl_computed_yaml")
	}
	var problems usererr.List
	for _, kv := range m.Values {
		n := yamlpos.KeyOf(kv).GetToken().Value
		s, ok := text(kv.Value)
		if !ok || s == "" {
			problems = append(problems, yamlpos.New(file, yamlpos.OfValue(yamlpos.KeyOf(kv), kv.Value), "tmpl_computed_yaml"))
			continue
		}
		exprs[n], where[n] = s, kv
	}
	if problems != nil {
		return exprs, where, problems
	}
	return exprs, where, nil
}

// usedNames returns the names of computed that the program p uses, and env true when p uses $env, to be
// refused: through it p could use one without its name, before it is computed, or hand the variables to
// page.html.
func usedNames(p *vm.Program, computed map[string]string) (names []string, env bool) {
	var u used
	node := p.Node()
	exprast.Walk(&node, &u)
	for _, n := range u {
		if n == "$env" {
			return nil, true
		}
		if _, ok := computed[n]; ok && !slices.Contains(names, n) {
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

// topological orders names so that each comes after those it uses, or returns the names that use each
// other as cycle, the first of them last too.
func topological(names []string, uses map[string][]string) (order, cycle []string) {
	const (
		unseen = iota
		visiting
		done
	)
	state := map[string]int{}
	var stack []string
	var visit func(n string) []string
	visit = func(n string) []string {
		switch state[n] {
		case done:
			return nil
		case visiting:
			return slices.Concat(stack[slices.Index(stack, n):], []string{n})
		}
		state[n] = visiting
		stack = append(stack, n)
		for _, u := range uses[n] {
			if found := visit(u); found != nil {
				return found
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = done
		order = append(order, n)
		return nil
	}
	for _, n := range names {
		if cycle = visit(n); cycle != nil {
			return nil, cycle
		}
	}
	return order, nil
}

// finiteValue refuses a value that is or holds NaN or ±Inf, which +, - and * can still give.
func finiteValue(v any) error {
	return finiteIn(reflect.ValueOf(v))
}

func finiteIn(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		if f := v.Float(); math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("the value %v %w", f, errNotFinite)
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			if err := finiteIn(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		for it := v.MapRange(); it.Next(); {
			if err := finiteIn(it.Value()); err != nil {
				return err
			}
		}
	case reflect.Interface, reflect.Pointer:
		if !v.IsNil() {
			return finiteIn(v.Elem())
		}
	default:
	}
	return nil
}
