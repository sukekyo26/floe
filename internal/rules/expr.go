// Package rules reads the checks.yaml and computed.yaml of a template or a part, their expressions compiled
// with expr, and runs them on a data file. expr's mistakes are told in its words; floe checks only the shape
// of the files, what floe names and orders (the names of computed, $env, a cycle, a for), and that a number
// is finite.
package rules

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/vm"
	"github.com/expr-lang/expr/vm/runtime"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// arithmetic makes / and ** (^) fail where expr would give NaN or ±Inf, a division by zero above all, so that
// a comparison with it does not silently hold or fail.
// LIMIT: +, - and * past ±1.8e308 still give ±Inf, and whole numbers past ±9.2e18 wrap around, as does - on
// one above 9.2e18, which doc.Read leaves uint64. computed.yaml catches them in its values (finiteValue), the
// expressions of checks.yaml do not. Check their results too if a template needs such numbers.
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
	key   ast.Node // the key it is the value of
	value ast.Node
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
// where the expression is written, in a scalar on one line (after its quote) and in a | block; else, and for
// a mistake without a place, where the expression is written.
// LIMIT: a quoted scalar with an escape (\ in "", ” in ”) is told where it is written, as the text expr
// read is not the text written; so is a > block, and a | block with an indentation indicator.
func (e expression) place(err error) yamlpos.Pos {
	at := e.start()
	var fe *file.Error
	if !errors.As(err, &fe) || fe.Line == 0 {
		return at
	}
	switch v := e.value.(type) {
	case *ast.LiteralNode:
		// expr's lines are those of the block's text, which starts on the line after its |.
		indent, ok := blockIndent(v)
		if !ok {
			return at
		}
		return yamlpos.Pos{Line: at.Line + fe.Line, Column: indent + 1 + fe.Column}
	case ast.ScalarNode:
		tk := v.GetToken()
		written := strings.TrimSpace(tk.Origin)
		switch {
		case fe.Line != 1 || strings.Contains(written, "\n"):
			return at
		case tk.Type == token.DoubleQuoteType && strings.Contains(written, `\`),
			tk.Type == token.SingleQuoteType && strings.Contains(written[1:len(written)-1], "''"):
			return at
		case tk.Type == token.DoubleQuoteType, tk.Type == token.SingleQuoteType:
			at.Column++
		}
		at.Column += fe.Column
	}
	return at
}

// blockIndent returns the indentation of the | block v, the spaces before its first line that holds text,
// and false for a > block or one with an indentation indicator (|2), whose text is not as written.
func blockIndent(v *ast.LiteralNode) (int, bool) {
	if v.Start.Value != "|" && v.Start.Value != "|-" && v.Start.Value != "|+" {
		return 0, false
	}
	for line := range strings.SplitSeq(v.Value.GetToken().Origin, "\n") {
		if strings.TrimSpace(line) != "" {
			return len(line) - len(strings.TrimLeft(line, " ")), true
		}
	}
	return 0, false
}

// brief words a mistake of expr on one line, without the place and snippet it draws under the message.
// LIMIT: expr's English.
func brief(err error) string {
	if fe := (*file.Error)(nil); errors.As(err, &fe) {
		return fe.Message
	}
	return err.Error()
}

// text returns the value n as written, a number or true as text, and false for a list or keys and values.
// One left empty is "".
func text(n ast.Node) (string, bool) {
	if _, ok := n.(ast.ScalarNode); !ok {
		return "", false
	}
	var s string
	err := yaml.NodeToValue(n, &s)
	return s, err == nil
}
