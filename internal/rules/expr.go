// Package rules reads the checks.yaml and computed.yaml of a template or a part, their expressions compiled
// with expr, and runs them on a data file. expr's mistakes are told in its words; floe checks only the shape
// of the files, what floe names and orders (the names of computed, $env, a cycle, a for), and that a number
// is finite.
//
// LIMIT: a run of an expression has expr's default budget, 1e6: a list's item and a part split makes count 1,
// and the text it makes a byte for each it adds: join all but its longest part, + the shorter text, toJSON,
// replace and toBase64 of a text what they add to it, toJSON of keys and values or a list its whole text
// (longer than the data, by its indentation and escapes). Over it, expr says memory budget exceeded. Set
// vm.VM's MemoryBudget if templates need more. It does not bound what a run allocates (s + "x" counts 1
// for a copy of s): guard bounds the memory of the whole drawing instead, of the CLI's commands that draw and of
// floe __draw, the server's (drawproc).
package rules

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/vm"
	"github.com/expr-lang/expr/vm/runtime"

	"floe/internal/usererr"
	"floe/internal/yamlpos"
)

// arithmetic makes / and ** (^) fail where expr would give NaN or ±Inf, a division by zero above all, so that
// a comparison with it does not silently hold or fail.
// LIMIT: +, - and * past ±1.8e308 still give ±Inf, and whole numbers past ±9.2e18 wrap around, as does - on
// one above 9.2e18, which doc.Read leaves uint64. computed.yaml catches them in its values (notFiniteIn), the
// expressions of checks.yaml do not, nor log2(0), -Inf. Check their results too if a template needs such
// numbers.
var arithmetic = []expr.Option{
	finite("_divide", "/", runtime.Divide), expr.Operator("/", "_divide"),
	// expr matches an operator as written, and ^ is ** written otherwise.
	finite("_power", "**", runtime.Exponent), expr.Operator("**", "_power"), expr.Operator("^", "_power"),
}

// notFinite is an operation that gave NaN or ±Inf, worded as written: 1 / 0.
type notFinite struct {
	a, b any
	op   string
}

func (e *notFinite) Error() string {
	return fmt.Sprintf("%v %s %v", e.a, e.op, e.b)
}

func finite(name, op string, f func(a, b any) float64) expr.Option {
	return expr.Function(name, func(p ...any) (any, error) {
		r := f(p[0], p[1])
		if math.IsNaN(r) || math.IsInf(r, 0) {
			return nil, &notFinite{a: p[0], b: p[1], op: op}
		}
		return r, nil
	}, new(func(a, b any) float64))
}

// expression is an expression of checks.yaml or computed.yaml as YAML read it, and where it is written.
type expression struct {
	src   string
	file  string
	yaml  []byte        // the file's YAML, which value was read from
	key   *yamlpos.Node // the key it is the value of
	value *yamlpos.Node
}

// compile compiles e for env with opts: its mistake in expr's words, at its place in the expression.
func (e expression) compile(env any, opts ...expr.Option) (*vm.Program, *usererr.Error) {
	p, err := expr.Compile(e.src, slices.Concat([]expr.Option{expr.Env(env)}, arithmetic, opts)...)
	if err != nil {
		at := e.place(err)
		return nil, usererr.LibraryAt(e.file, at.Line, at.Column, brief(err))
	}
	return p, nil
}

// start is where e is written.
func (e expression) start() yamlpos.Pos {
	return yamlpos.OfValue(e.key, e.value)
}

// place returns where in the file the mistake err of expr is: its line and column in the expression added to
// where the expression's characters start, when they are written as read (yamlpos.Written); else, and for a mistake
// without a place, where the expression is written.
func (e expression) place(err error) yamlpos.Pos {
	at := e.start()
	var fe *file.Error
	if !errors.As(err, &fe) || fe.Line == 0 {
		return at
	}
	written, ok := yamlpos.Written(e.yaml, e.value)
	if !ok {
		return at
	}
	// expr counts lines from 1 and columns from 0.
	return yamlpos.Pos{Line: written.Line + fe.Line - 1, Column: written.Column + fe.Column}
}

// brief words a mistake of expr on one line, without the place and snippet it draws under the message.
// LIMIT: expr's English.
func brief(err error) string {
	if fe := (*file.Error)(nil); errors.As(err, &fe) {
		return fe.Message
	}
	return err.Error()
}

// text returns the value n as written: text as yamlpos.Text reads it, a number or true as its characters (1.10,
// not 1.1), and "" for one left empty or null; false for a list or keys and values.
func text(n *yamlpos.Node) (string, bool) {
	switch s, ok := yamlpos.Text(n); {
	case ok:
		return s, true
	case n.Kind != yamlpos.Scalar:
		return "", false
	case n.Null:
		return "", true
	}
	return n.Value, true
}
