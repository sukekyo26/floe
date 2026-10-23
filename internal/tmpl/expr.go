package tmpl

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/expr-lang/expr"
	exprast "github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/vm"
	"github.com/expr-lang/expr/vm/runtime"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// arithmetic makes / and ** (^) fail where expr would give NaN or ±Inf, a division by zero above all,
// so that a comparison with it does not silently hold or fail.
// LIMIT: +, - and * past ±1.8e308 still give ±Inf, and whole numbers past ±9.2e18 wrap around. Check
// their results too if a template needs such numbers.
var arithmetic = []expr.Option{
	finite("_divide", "/", runtime.Divide), expr.Operator("/", "_divide"),
	// expr matches an operator as written, and ^ is ** written otherwise.
	finite("_power", "**", runtime.Exponent), expr.Operator("**", "_power"), expr.Operator("^", "_power"),
}

var errNotFinite = errors.New("is not a finite number")

func finite(name, op string, f func(a, b any) float64) expr.Option {
	return expr.Function(name, func(p ...any) (any, error) {
		r := f(p[0], p[1])
		if math.IsNaN(r) || math.IsInf(r, 0) {
			return nil, fmt.Errorf("%v %s %v %w", p[0], op, p[1], errNotFinite)
		}
		return r, nil
	}, new(func(a, b any) float64))
}

// compile compiles the expression src for the variables of env, a struct whose fields are tagged
// with their names.
func compile(src string, env any, opts ...expr.Option) (*vm.Program, error) {
	p, err := expr.Compile(src, slices.Concat([]expr.Option{expr.Env(env)}, arithmetic, opts)...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", src, err)
	}
	return p, nil
}

// compileWritten compiles src, an expression as YAML read it at at in file, for env with opts.
func compileWritten(
	file string, at yamlpos.Pos, src string, env any, opts ...expr.Option,
) (*vm.Program, *usererr.Error) {
	p, err := compile(src, env, opts...)
	if err != nil {
		return nil, yamlpos.New(file, at, "tmpl_expr", src, brief(err))
	}
	return p, nil
}

// brief words an error of expr on one line, without the snippet it draws under the message.
func brief(err error) string {
	if fe := (*file.Error)(nil); errors.As(err, &fe) {
		return fe.Message
	}
	return err.Error()
}

// dataVar is a variable of an expression that holds a value of the data, written at the path at in it, as
// a mistake words it ("" for the whole).
type dataVar struct {
	value any
	at    string
}

// unvalued returns the paths, as a mistake words them, of the values of the data the program p reads
// through vars that the data gives none for, left out or null: of each read, a.b.c, the first step that
// is nil, but for the last one when p does without it, by ?? (a.b ?? 0) or ?. (a?.b). Reads through
// anything else, a function's result, a variable of let or # in a predicate, are not looked at: expr
// refuses a let named as one of vars.
func unvalued(p *vm.Program, vars map[string]dataVar) []string {
	r := reads{inner: map[exprast.Node]bool{}, spared: map[exprast.Node]bool{}, vars: vars, found: nil}
	node := p.Node()
	exprast.Walk(&node, &r)
	exprast.Walk(&node, &nilReads{&r})
	return r.found
}

// reads notes, walking an expression, the reads that are part of a longer one (a and a.b in a.b.c), and
// those whose value it does without (the left of ??).
type reads struct {
	inner, spared map[exprast.Node]bool
	vars          map[string]dataVar
	found         []string
}

func (r *reads) Visit(n *exprast.Node) {
	switch n := (*n).(type) {
	case *exprast.MemberNode:
		r.inner[n.Node] = true
	case *exprast.BinaryNode:
		if n.Operator == "??" {
			r.spared[n.Left] = true
			if c, ok := n.Left.(*exprast.ChainNode); ok {
				r.spared[c.Node] = true
			}
		}
	}
}

// nilReads adds to found the path to the first nil of each whole read of the data.
type nilReads struct{ *reads }

func (r *nilReads) Visit(n *exprast.Node) {
	switch (*n).(type) {
	case *exprast.IdentifierNode, *exprast.MemberNode:
		if r.inner[*n] {
			return
		}
		if at, ok := r.nilAt(*n); ok && !slices.Contains(r.found, at) {
			r.found = append(r.found, at)
		}
	}
}

// nilAt returns the path to the first nil the read n, a.b.c from a variable of vars, comes to, and false
// when it comes to none, does without it, or is not such a read.
func (r *reads) nilAt(n exprast.Node) (string, bool) {
	var steps []*exprast.MemberNode
	base := n
	for m, ok := base.(*exprast.MemberNode); ok; m, ok = base.(*exprast.MemberNode) {
		steps = append(steps, m)
		base = m.Node
	}
	id, ok := base.(*exprast.IdentifierNode)
	if !ok {
		return "", false
	}
	v, ok := r.vars[id.Value]
	if !ok {
		return "", false
	}
	value, at := v.value, v.at
	for _, m := range slices.Backward(steps) {
		if value == nil {
			return at, !m.Optional // a?.b is nil when a is
		}
		if value, at, ok = step(value, at, m.Property); !ok {
			return "", false
		}
	}
	return at, value == nil && !r.spared[n]
}

// step returns what the property of a read, a key or an index written as such, leads to in value, at the
// path at, and its path; false when it is neither, or value is not keys or a list.
func step(value any, at string, property exprast.Node) (any, string, bool) {
	switch p := property.(type) {
	case *exprast.StringNode:
		m, ok := value.(map[string]any)
		if at != "" {
			at += "."
		}
		return m[p.Value], at + p.Value, ok
	case *exprast.IntegerNode:
		l, ok := value.([]any)
		i := p.Value
		if i < 0 {
			i += len(l)
		}
		if !ok || i < 0 || i >= len(l) {
			return nil, "", false
		}
		return l[i], at + "[" + strconv.Itoa(p.Value) + "]", true
	}
	return nil, "", false
}
