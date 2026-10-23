package tmpl

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

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
// LIMIT: expr's English. Word the failures floe can tell apart, as it does the values left out.
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
// a mistake words it ("" for the whole), of the shape its schema declares.
type dataVar struct {
	value any
	at    string
	shape *shape
}

// unvalued returns the paths, as a mistake words them, of the values of the data, given by vars, that the
// program p stopped on with err for want of: left out or null, the first step that is nil of the read p
// stopped at (a.b.c, or a?.b when a is nil), or of a read p handed to the function or operator it stopped at
// (count(prs, …), w + 1), when it stopped on a nil. A read of a key the schema does not declare, a misspelt
// one, is the expression's to fix, not a value to give; one through anything else, a function's result, a
// variable of let or # in a predicate, is not looked at: expr refuses a let named as one of vars.
// A computed value that is nil, as it took one the data leaves out (p: prs), is not followed to it.
// LIMIT: a nil computed value is told in expr's words; follow computed.yaml's names if templates chain them so.
func unvalued(p *vm.Program, err error, vars map[string]dataVar) []string {
	var stop *file.Error
	if !errors.As(err, &stop) || !stoppedOnNil(stop.Message) {
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
	for _, h := range readFirst(*n) {
		if c, ok := h.(*exprast.ChainNode); ok {
			h = c.Node
		}
		if at, ok := s.nilAt(h); ok && !slices.Contains(s.found, at) {
			s.found = append(s.found, at)
		}
	}
}

// readFirst is what the node n reads before it goes on, alone, which it may stop on: a branch not taken is
// not read.
func readFirst(n exprast.Node) []exprast.Node {
	switch n := n.(type) {
	case *exprast.IdentifierNode:
		return []exprast.Node{n}
	case *exprast.MemberNode:
		return []exprast.Node{n, n.Property}
	case *exprast.ConditionalNode:
		return []exprast.Node{n.Cond}
	case *exprast.BuiltinNode:
		return n.Arguments
	case *exprast.CallNode:
		return n.Arguments
	case *exprast.BinaryNode:
		switch {
		case logical[n.Operator]:
			return []exprast.Node{n.Left}
		case !nilTaking[n.Operator]:
			return []exprast.Node{n.Left, n.Right}
		}
	case *exprast.UnaryNode:
		return []exprast.Node{n.Node}
	case *exprast.SliceNode:
		return []exprast.Node{n.Node, n.From, n.To}
	}
	return nil
}

// stoppedOnNil reports whether expr says, in message, that it stopped on a nil: a function or an operator
// stopping on another of what it took, of a type it does not take, names no nil.
// LIMIT: reads expr's English; a nil it words otherwise is told in its words, naming no value left out.
func stoppedOnNil(message string) bool {
	for _, said := range []string{"<nil>", "is nil", "zero Value"} {
		if strings.Contains(message, said) {
			return true
		}
	}
	return false
}

// nilTaking are the operators that take nil on either side: one of them stops for another reason, two values
// of types it cannot compare.
var nilTaking = map[string]bool{
	"??": true, "==": true, "!=": true, "in": true,
	"matches": true, "contains": true, "startsWith": true, "endsWith": true,
}

// logical are the operators that stop on a nil on their left alone: the right is not true or false to them,
// but the result (true && nil is nil).
var logical = map[string]bool{"&&": true, "||": true, "and": true, "or": true}

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
	value, at, sh := v.value, v.at, v.shape
	for _, m := range slices.Backward(steps) {
		if value == nil {
			return at, true
		}
		if value, at, sh, ok = step(value, at, sh, m.Property); !ok {
			return "", false
		}
	}
	return at, value == nil
}

// step returns what the property of a read, a key or an index written as such, leads to in value, of the
// shape sh, at the path at: its value, its path, written as yamlpos.Find writes one, and its shape; false
// when it is neither or down cannot go there.
func step(value any, at string, sh *shape, property exprast.Node) (any, string, *shape, bool) {
	switch p := property.(type) {
	case *exprast.StringNode:
		if _, ok := value.(map[string]any); !ok {
			return nil, "", nil, false
		}
		v, next, ok := down(value, sh, p.Value)
		if at != "" {
			at += "."
		}
		return v, at + p.Value, next, ok
	case *exprast.IntegerNode:
		l, ok := value.([]any)
		i := p.Value
		if i < 0 {
			i += len(l)
		}
		if !ok {
			return nil, "", nil, false
		}
		v, next, ok := down(value, sh, strconv.Itoa(i))
		return v, at + "[" + strconv.Itoa(i) + "]", next, ok
	}
	return nil, "", nil, false
}

// shapeAt is the shape s declares for what the path at leads to in value, nil when it declares none.
func shapeAt(s *shape, value any, at []string) *shape {
	for _, k := range at {
		var ok bool
		if value, s, ok = down(value, s, k); !ok {
			return nil
		}
	}
	return s
}

// down goes one step, k, into value, of the shape sh: a key of keys and values, or an index of a list. It
// returns what is there and its shape, and false when value is neither, has no such index, or sh declares
// no such key or items.
func down(value any, sh *shape, k string) (any, *shape, bool) {
	if sh == nil {
		return nil, nil, false
	}
	in := sh.inside()
	switch v := value.(type) {
	case map[string]any:
		next, declared := in.keys[k]
		return v[k], next, declared
	case []any:
		i, err := strconv.Atoi(k)
		if err != nil || i < 0 || i >= len(v) || in.each == nil {
			return nil, nil, false
		}
		return v[i], in.each, true
	}
	return nil, nil, false
}
