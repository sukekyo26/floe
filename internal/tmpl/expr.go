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

// without is how to write an expression that stops for want of a value the data file data gives none for.
func without(data string) *usererr.Error {
	return usererr.New("tmpl_expr_without", data)
}

// dataVar is a variable of an expression that holds a value of the data, written at the path at in it, as
// a mistake words it ("" for the whole).
type dataVar struct {
	value any
	at    string
}

// unvalued returns the paths, as a mistake words them, of the values of the data, given by vars, that the
// program p stopped on with err for want of: left out or null, the first step that is nil of the read p
// stopped at (a.b.c, or a?.b when a is nil), or of a read p handed to the function or operator it stopped at
// (count(prs, …), w + 1). A read through anything else, a function's result, a variable of let or # in a
// predicate, is not looked at: expr refuses a let named as one of vars.
// A computed value that is nil, as it took one the data leaves out (p: prs), is not followed to it.
// LIMIT: a nil computed value is told in expr's words; follow computed.yaml's names if templates chain them so.
func unvalued(p *vm.Program, err error, vars map[string]dataVar) []string {
	var stop *file.Error
	if !errors.As(err, &stop) {
		return nil
	}
	f := stopAt{at: stop.Location, vars: vars, found: nil}
	node := p.Node()
	exprast.Walk(&node, &f)
	return f.found
}

// stopAt adds to found the paths to the nil the nodes at at read, or hand to what they call.
type stopAt struct {
	at    file.Location
	vars  map[string]dataVar
	found []string
}

func (s *stopAt) Visit(n *exprast.Node) {
	if (*n).Location() != s.at {
		return
	}
	// What the node reads before it goes on, alone: a branch not taken is not read.
	var handed []exprast.Node
	switch n := (*n).(type) {
	case *exprast.IdentifierNode:
		handed = []exprast.Node{n}
	case *exprast.MemberNode:
		handed = []exprast.Node{n, n.Property}
	case *exprast.ConditionalNode:
		handed = []exprast.Node{n.Cond}
	case *exprast.BuiltinNode:
		handed = n.Arguments
	case *exprast.CallNode:
		handed = n.Arguments
	case *exprast.BinaryNode:
		handed = []exprast.Node{n.Left, n.Right}
	case *exprast.UnaryNode:
		handed = []exprast.Node{n.Node}
	case *exprast.SliceNode:
		handed = []exprast.Node{n.Node, n.From, n.To}
	}
	for _, h := range handed {
		if c, ok := h.(*exprast.ChainNode); ok {
			h = c.Node
		}
		if at, ok := s.nilAt(h); ok && !slices.Contains(s.found, at) {
			s.found = append(s.found, at)
		}
	}
}

// nilAt returns the path to the first nil the read n, a.b.c from a variable of vars, comes to, and false
// when it comes to none or is not such a read.
func (s *stopAt) nilAt(n exprast.Node) (string, bool) {
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
	v, ok := s.vars[id.Value]
	if !ok {
		return "", false
	}
	value, at := v.value, v.at
	for _, m := range slices.Backward(steps) {
		if value == nil {
			return at, true
		}
		if value, at, ok = step(value, at, m.Property); !ok {
			return "", false
		}
	}
	return at, value == nil
}

// step returns what the property of a read, a key or an index written as such, leads to in value, at the
// path at, and its path, written as yamlpos.Find writes one; false when it is neither, or value is not keys
// or a list, or has no such index.
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
		return l[i], at + "[" + strconv.Itoa(i) + "]", true
	}
	return nil, "", false
}
