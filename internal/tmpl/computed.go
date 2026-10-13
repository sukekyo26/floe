package tmpl

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/expr-lang/expr"
	exprast "github.com/expr-lang/expr/ast"
	exprbuiltin "github.com/expr-lang/expr/builtin"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/vm"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/yamlpos"
)

var (
	errName  = errors.New("is not a name an expression or page.html can use; write letters, digits and _")
	errClash = errors.New("is an item of the data too, so an expression could not tell them apart")
	errCycle = errors.New("the values are computed from each other")
	errEnv   = errors.New("$env is not for computed.yaml; use the names")
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

// checkComputed computes the values of d's template's computed.yaml, if it has one, into d.Calc, and
// reports at the top of root those its data cannot give.
func checkComputed(d *Data, root ast.Node) error {
	src, err := builtin.ReadFile(path.Join("builtin", d.Template, "computed.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read the computed values of %s: %w", d.Template, err)
	}
	s, err := schemaOf(d.Template)
	if err != nil {
		return err
	}
	return compute(src, d, slices.Sorted(maps.Keys(s.Properties)), root)
}

// compute computes the values of the computed.yaml src from the values of d, whose items the schema
// declares, in declared. An expression sees the items and the other computed values by name, and a
// value is computed after those it uses, in whatever order src writes them.
func compute(src []byte, d *Data, declared []string, root ast.Node) error {
	c, err := readComputed(src, declared)
	if err != nil {
		return fmt.Errorf("read the computed values of %s: %w", d.Template, err)
	}
	// An item the data leaves out stays nil.
	for _, n := range c.items {
		if v := d.Values[n]; v != nil {
			c.env.Field(c.field[n]).Set(reflect.ValueOf(v))
		}
	}
	d.Calc = map[string]any{}
	var problems yamlpos.Problems
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
			problems = append(problems, yamlpos.New(d.File, yamlpos.Of(root), "tmpl_computed_error", n, c.exprs[n], brief(err)))
			failed[n] = true
			continue
		}
		d.Calc[n] = out
		if out != nil {
			c.env.Field(c.field[n]).Set(reflect.ValueOf(out))
		}
	}
	if problems != nil {
		return problems
	}
	return nil
}

// computed is computed.yaml compiled: an expression and the computed names it uses, by name, the order
// to compute them in, and the variables they see, a field each.
type computed struct {
	exprs    map[string]string
	programs map[string]*vm.Program
	uses     map[string][]string
	order    []string
	items    []string // the items of the data the expressions see
	field    map[string]int
	env      reflect.Value
}

func readComputed(src []byte, declared []string) (*computed, error) {
	var exprs map[string]string
	if err := yaml.UnmarshalWithOptions(src, &exprs, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("computed.yaml: %w", err)
	}
	names := slices.Sorted(maps.Keys(exprs))
	for _, n := range names {
		switch {
		case !usable(n):
			return nil, fmt.Errorf("%s %w", n, errName)
		case slices.Contains(declared, n):
			return nil, fmt.Errorf("%s %w", n, errClash)
		}
	}
	// LIMIT: an item whose name an expression cannot use (my-size, or a function of expr such as date)
	// is not seen by name. Give the expressions the data as doc too if a template needs one.
	items := slices.DeleteFunc(slices.Clone(declared), func(n string) bool { return !usable(n) })
	vars := slices.Concat(items, names)
	fields := make([]reflect.StructField, len(vars))
	c := &computed{
		exprs: exprs, programs: map[string]*vm.Program{}, uses: map[string][]string{}, order: nil,
		items: items, field: map[string]int{}, env: reflect.Value{},
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
		p, err := compile(exprs[n], c.env.Interface())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		c.programs[n] = p
		if c.uses[n], err = usedNames(p, exprs); err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
	}
	order, err := topological(names, c.uses)
	if err != nil {
		return nil, err
	}
	c.order = order
	return c, nil
}

// usedNames returns the names of computed that the program p uses. It refuses $env, through which p
// could use one without its name, before it is computed, or hand the variables to page.html.
func usedNames(p *vm.Program, computed map[string]string) ([]string, error) {
	var u used
	node := p.Node()
	exprast.Walk(&node, &u)
	var names []string
	for _, n := range u {
		if n == "$env" {
			return nil, errEnv
		}
		if _, ok := computed[n]; ok && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	return names, nil
}

type used []string

func (u *used) Visit(n *exprast.Node) {
	if id, ok := (*n).(*exprast.IdentifierNode); ok {
		*u = append(*u, id.Value)
	}
}

// topological orders names so that each comes after those it uses, or reports the names that use
// each other.
func topological(names []string, uses map[string][]string) ([]string, error) {
	const (
		unseen = iota
		visiting
		done
	)
	state := map[string]int{}
	var order, stack []string
	var visit func(n string) error
	visit = func(n string) error {
		switch state[n] {
		case done:
			return nil
		case visiting:
			cycle := slices.Concat(stack[slices.Index(stack, n):], []string{n})
			return fmt.Errorf("%w: %s", errCycle, strings.Join(cycle, " → "))
		}
		state[n] = visiting
		stack = append(stack, n)
		for _, u := range uses[n] {
			if err := visit(u); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = done
		order = append(order, n)
		return nil
	}
	for _, n := range names {
		if err := visit(n); err != nil {
			return nil, err
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
